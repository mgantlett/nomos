// Package mtpo evaluates write-time AST hazards and sync-time serializability verdicts.
package mtpo

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
)

// EvaluateActiveHazards compares current against active trajectories ordered ahead of current.Seq.
func EvaluateActiveHazards(current *MTPOLedgerEntry, activePeers []MTPOLedgerEntry) []MTPOConflict {
	if current == nil {
		return nil
	}
	var conflicts []MTPOConflict
	for i := range activePeers {
		peer := &activePeers[i]
		if peer.TaskKey == current.TaskKey || peer.Seq >= current.Seq {
			continue
		}
		if c, ok := detectOrderedHazard(peer, current); ok {
			conflicts = append(conflicts, c)
		}
	}
	return conflicts
}

// VerifySyncSerializability checks cross-worktree AST serializability against merged predecessors.
// @intent software_physics_engine_spec.md #SerializabilityVerdict
// @intent nomos_hermetic_worktree_spec.md
func VerifySyncSerializability(dbPath, repoRoot, taskKey, currentTrunkCommit string) (*SerializabilityVerdict, error) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	return evaluateDBSerializability(db, normalizePathSlash(repoRoot), strings.ToUpper(strings.TrimSpace(taskKey)), strings.TrimSpace(currentTrunkCommit))
}

func evaluateDBSerializability(db *sql.DB, canonRepo, upperKey, trunkSHA string) (*SerializabilityVerdict, error) {
	current, gErr := queryLatestTaskEntry(db, canonRepo, upperKey, StatusActive)
	if gErr != nil || current == nil {
		return &SerializabilityVerdict{TaskKey: upperKey, Passed: true}, nil
	}
	activePeers, aErr := queryEntriesByStatus(db, canonRepo, StatusActive)
	if aErr != nil {
		return nil, aErr
	}
	verdict := &SerializabilityVerdict{
		TaskKey:       upperKey,
		Seq:           current.Seq,
		Passed:        true,
		ActiveHazards: EvaluateActiveHazards(current, activePeers),
	}
	if trunkSHA == "" || current.BaseCommit == "" || current.BaseCommit == trunkSHA {
		return verdict, nil
	}
	return attachMergedPredecessorConflicts(db, canonRepo, trunkSHA, current, verdict)
}

func attachMergedPredecessorConflicts(db *sql.DB, canonRepo, trunkSHA string, current *MTPOLedgerEntry, verdict *SerializabilityVerdict) (*SerializabilityVerdict, error) {
	var baseCommitSeq int64
	if err := db.QueryRow(`SELECT base_commit_seq FROM worktree_mtpo WHERE seq = ?`, current.Seq).Scan(&baseCommitSeq); err != nil {
		baseCommitSeq = 0
	}
	committed, err := queryCommittedAfterSnapshot(db, canonRepo, baseCommitSeq, current.TaskKey)
	if err != nil {
		return nil, err
	}
	for i := range committed {
		if conflict, ok := detectOrderedHazard(&committed[i], current); ok {
			verdict.MergedConflict = append(verdict.MergedConflict, conflict)
		}
	}
	if len(verdict.MergedConflict) > 0 {
		verdict.Passed = false
		verdict.Remedy = fmt.Sprintf(
			"rebase %s onto main (%s) and re-run 'nomos verify' before syncing: git -C %s rebase main && nomos verify",
			current.TaskKey, trunkSHA, current.WorktreePath,
		)
	}
	return verdict, nil
}

func queryCommittedAfterSnapshot(db *sql.DB, canonRepo string, baseCommitSeq int64, excludeTaskKey string) ([]MTPOLedgerEntry, error) {
	rows, err := db.Query(
		`SELECT seq, repo_root, task_key, worktree_path, base_commit, status, write_files, write_symbols, read_symbols, referenced_imports, updated_at
		 FROM worktree_mtpo
		 WHERE repo_root = ? AND status = ? AND task_key != ? AND commit_seq > ?
		 ORDER BY commit_seq ASC`,
		canonRepo, StatusCommitted, excludeTaskKey, baseCommitSeq,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := rows.Close(); cErr != nil {
			return
		}
	}()
	var out []MTPOLedgerEntry
	for rows.Next() {
		e, sErr := scanEntryRows(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func detectOrderedHazard(earlier, later *MTPOLedgerEntry) (MTPOConflict, bool) {
	overlapFiles := intersectSorted(earlier.WriteFiles, later.WriteFiles)
	wwSymbols := intersectSorted(earlier.WriteSymbols, later.WriteSymbols)
	rwSymbols := mergeSortedStringSets(
		intersectSorted(earlier.WriteSymbols, later.ReadSymbols),
		intersectSorted(later.WriteSymbols, earlier.ReadSymbols),
	)
	if len(overlapFiles) == 0 && len(wwSymbols) == 0 && len(rwSymbols) == 0 {
		return MTPOConflict{}, false
	}
	kind := HazardReadWrite
	allSyms := rwSymbols
	if len(overlapFiles) > 0 || len(wwSymbols) > 0 {
		kind = HazardWriteWrite
		allSyms = mergeSortedStringSets(wwSymbols, rwSymbols)
	}
	adv := fmt.Sprintf(
		"⚠️  [Nomos MTPO Advisory] Trajectory %s (seq #%d) overlaps predecessor %s (seq #%d) [%s] files=%v symbols=%v",
		later.TaskKey, later.Seq, earlier.TaskKey, earlier.Seq, kind, overlapFiles, allSyms,
	)
	return MTPOConflict{
		EarlierSeq:         earlier.Seq,
		EarlierTaskKey:     earlier.TaskKey,
		LaterSeq:           later.Seq,
		LaterTaskKey:       later.TaskKey,
		HazardKind:         kind,
		OverlappingFiles:   overlapFiles,
		OverlappingSymbols: allSyms,
		Advisory:           adv,
	}, true
}

// FormatAdvisoryConflicts joins non-empty MTPO conflict advisories for PostToolUse injection.
func FormatAdvisoryConflicts(conflicts []MTPOConflict) string {
	if len(conflicts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		if c.Advisory != "" {
			lines = append(lines, c.Advisory)
		}
	}
	return strings.Join(lines, "\n")
}

func intersectSorted(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	set := make(map[string]bool, len(a))
	for _, x := range a {
		set[x] = true
	}
	var out []string
	for _, y := range b {
		if set[y] {
			out = append(out, y)
		}
	}
	return out
}
