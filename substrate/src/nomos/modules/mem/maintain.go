// Package mem implements automatic deduplication, compaction, and buffer hygiene for Nomos Mneme.
package mem

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

const (
	defaultInactiveRetentionDays = 30
	defaultTrapMaxAge            = 7 * 24 * time.Hour
	defaultMinMemVacuumBytes     = 2 * 1024 * 1024
	defaultGlobalRepoTag         = "global"
	trapFilePrefix               = "active_traps_"
)

// MaintainOptions configures deterministic SQLite maintenance thresholds for Nomos Mneme.
type MaintainOptions struct {
	InactiveRetentionDays int
	MinVacuumBytes        int64
}

// MaintainReport summarizes rows compacted and checkpointed during Mneme maintenance.
type MaintainReport struct {
	DuplicatesSuperseded int  `json:"duplicates_superseded"`
	InactivePurged       int  `json:"inactive_purged"`
	Vacuumed             bool `json:"vacuumed"`
	WALCheckpointed      bool `json:"wal_checkpointed"`
}

// NormalizeMemorySignature builds a canonical deduplication key from repository, kind, and whitespace-normalized content.
func NormalizeMemorySignature(repoID string, kind MemoryKind, content string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(content)))
	if len(fields) == 0 {
		return ""
	}
	repo := strings.ToLower(strings.TrimSpace(repoID))
	if repo == "" {
		repo = defaultGlobalRepoTag
	}
	k := strings.ToLower(strings.TrimSpace(string(kind)))
	if k == "" {
		k = string(KindReflection)
	}
	return repo + "|" + k + "|" + strings.Join(fields, " ")
}

func (s *Store) findActiveDuplicateIDLocked(repoID string, kind MemoryKind, content string) int {
	targetSig := NormalizeMemorySignature(repoID, kind, content)
	if targetSig == "" || s.db == nil {
		return 0
	}
	rows, err := s.db.Query(
		`SELECT id, repo_id, kind, content FROM memories WHERE (status = ? OR status = '') AND LOWER(kind) = LOWER(?) ORDER BY id DESC`,
		string(StatusActive), string(kind),
	)
	if err != nil {
		return 0
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var rRepo, rKind, rContent string
		if rows.Scan(&id, &rRepo, &rKind, &rContent) == nil {
			if NormalizeMemorySignature(rRepo, MemoryKind(rKind), rContent) == targetSig {
				return id
			}
		}
	}
	return 0
}

func (s *Store) reinforceExistingLocked(id int) {
	if s.db == nil || id <= 0 {
		return
	}
	if _, err := s.db.Exec(`UPDATE memories SET access_count = access_count + 1 WHERE id = ?`, id); err != nil {
		return
	}
}

// Maintain compacts active duplicate clones, purges expired inactive memories, and checkpoints the SQLite WAL.
func (s *Store) Maintain(opts MaintainOptions) (*MaintainReport, error) {
	retentionDays := opts.InactiveRetentionDays
	if retentionDays <= 0 {
		retentionDays = defaultInactiveRetentionDays
	}

	all, err := s.ListAll()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rep := &MaintainReport{}
	rep.DuplicatesSuperseded = s.deduplicateActiveClonesLocked(all)
	rep.InactivePurged = s.purgeExpiredInactiveLocked(retentionDays)
	minVac := opts.MinVacuumBytes
	if minVac <= 0 {
		minVac = defaultMinMemVacuumBytes
	}
	rep.Vacuumed = storage.VacuumIfFragmented(s.db, minVac)
	rep.WALCheckpointed = s.checkpointAndOptimizeLocked()
	return rep, nil
}

func (s *Store) deduplicateActiveClonesLocked(all []*Memory) int {
	groups := make(map[string][]*Memory)
	for _, rec := range all {
		if rec == nil || (rec.Status != StatusActive && rec.Status != "") {
			continue
		}
		if sig := NormalizeMemorySignature(rec.RepoID, rec.Kind, rec.Content); sig != "" {
			groups[sig] = append(groups[sig], rec)
		}
	}

	superseded := 0
	for _, list := range groups {
		if len(list) < 2 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID > list[j].ID })
		canonicalID := list[0].ID
		for _, dup := range list[1:] {
			if s.markCloneSupersededLocked(dup.ID, canonicalID) {
				superseded++
			}
		}
	}
	return superseded
}

func (s *Store) markCloneSupersededLocked(dupID, canonicalID int) bool {
	res, err := s.db.Exec(
		`UPDATE memories SET status = ?, superseded_by = ? WHERE id = ? AND (status = ? OR status = '')`,
		string(StatusSuperseded), canonicalID, dupID, string(StatusActive),
	)
	if err != nil {
		return false
	}
	n, rErr := res.RowsAffected()
	return rErr == nil && n > 0
}

func (s *Store) purgeExpiredInactiveLocked(retentionDays int) int {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour).Format(time.RFC3339)
	res, err := s.db.Exec(
		`DELETE FROM memories WHERE status IN (?, ?) AND created_at != '' AND created_at < ?`,
		string(StatusSuperseded), string(StatusTombstoned), cutoff,
	)
	if err != nil {
		return 0
	}
	n, rErr := res.RowsAffected()
	if rErr != nil {
		return 0
	}
	return int(n)
}

func (s *Store) checkpointAndOptimizeLocked() bool {
	if s.db == nil {
		return false
	}
	_, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE); PRAGMA optimize;`)
	return err == nil
}

// Dismiss transitions an active or superseded memory record to tombstoned status (@intent: #Memory).
func (s *Store) Dismiss(id int) (*Memory, error) {
	if id <= 0 {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "memory ID must be a positive integer, got %d", id).
			WithRemedy("run 'nomos mem list' to inspect valid numeric memory IDs")
	}
	rec, err := s.Get(id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.NewNomosError(types.ErrCodeValidationFailed, "memory #%d not found", id).
				WithRemedy("run 'nomos mem list' to inspect valid numeric memory IDs")
		}
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed reading memory #%d: %v", id, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, uErr := s.db.Exec(`UPDATE memories SET status = ? WHERE id = ?`, string(StatusTombstoned), id); uErr != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed dismissing memory #%d: %v", id, uErr)
	}
	rec.Status = StatusTombstoned
	return rec, nil
}

// RemoveTaskTrapBuffer deletes any transient gate-trap buffer file for an aborted or completed task.
func RemoveTaskTrapBuffer(repoRoot, taskKey string) {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" {
		return
	}
	_ = os.Remove(trapBufferPath(repoRoot, taskKey))
	removeTaskRecallLedger(repoRoot, taskKey)
}

func isTransientTaskMemFile(name string) bool {
	if !strings.HasSuffix(name, ".json") {
		return false
	}
	return strings.HasPrefix(name, trapFilePrefix) || strings.HasPrefix(name, recallLedgerPrefix)
}

// SweepStaleTrapBuffers removes orphaned transient gate-trap JSON files older than maxAge.
func SweepStaleTrapBuffers(repoRoot string, maxAge time.Duration) int {
	if strings.TrimSpace(repoRoot) == "" {
		return 0
	}
	if maxAge <= 0 {
		maxAge = defaultTrapMaxAge
	}
	tmpDir := filepath.Join(repoRoot, ".tmp")
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !isTransientTaskMemFile(e.Name()) {
			continue
		}
		if fi, iErr := e.Info(); iErr == nil && fi.ModTime().Before(cutoff) {
			if os.Remove(filepath.Join(tmpDir, e.Name())) == nil {
				removed++
			}
		}
	}
	return removed
}
