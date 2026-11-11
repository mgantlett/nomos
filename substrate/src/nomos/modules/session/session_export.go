// Package session implements polymorphic Tier 1 transcript discovery, Sentinel
// governance scoring, and RLVR/DPO dataset synthesis.
package session

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

// SessionDatasetRecord joins session telemetry, Sentinel PRM scores, Git Note receipts, and Mneme learnings (#SessionDatasetRecord).
type SessionDatasetRecord struct {
	Session        AgentSession         `json:"session"`
	Sentinel       *SentinelScorecard   `json:"sentinel,omitempty"`
	Trajectory     *SessionTrajectory   `json:"trajectory,omitempty"`
	Deliberation   *DeliberationMetrics `json:"deliberation,omitempty"`
	ToolsSummary   map[string]int       `json:"tools_summary,omitempty"`
	ReleaseCommit  string               `json:"release_commit,omitempty"`
	DoDGatesPassed int                  `json:"dod_gates_passed,omitempty"`
	SHA256Receipt  string               `json:"sha256_receipt,omitempty"`
	PitfallsFixed  []string             `json:"pitfalls_fixed,omitempty"`
	PatternsMinted []string             `json:"patterns_minted,omitempty"`
}

type releaseJoinIndex struct {
	byTask    map[string]task.ReleaseNotePayload
	bySession map[string]task.ReleaseNotePayload
}

type mnemeJoinIndex struct {
	pitfalls map[string][]string
	patterns map[string][]string
}

// ExportSessionDataset synthesizes joined RLVR, DPO, and SFT dataset records across discovered sessions.
// @intent software_physics_engine_spec.md #SessionDatasetRecord
func ExportSessionDataset(rootOverride, repoRoot string, limit int) ([]SessionDatasetRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	resolvedRoot := resolveExportRepoRoot(repoRoot)
	sessions, err := DiscoverSessions(rootOverride, resolvedRoot)
	if err != nil {
		return nil, err
	}
	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	relIdx := loadReleaseJoinIndex(resolvedRoot)
	memIdx := loadMnemeJoinIndex(config.ResolveMnemeDBPath())

	out := make([]SessionDatasetRecord, 0, len(sessions))
	for _, s := range sessions {
		rec := SessionDatasetRecord{Session: s}
		if detail, dErr := GetSessionDetail(rootOverride, resolvedRoot, s.ID); dErr == nil && detail != nil {
			rec.Sentinel = detail.Sentinel
			rec.Trajectory = detail.Trajectory
			rec.Deliberation = detail.Deliberation
			rec.ToolsSummary = detail.ToolsSummary
		}
		attachReleaseAndMneme(&rec, relIdx, memIdx)
		out = append(out, rec)
	}
	return out, nil
}

func resolveExportRepoRoot(repoRoot string) string {
	if strings.TrimSpace(repoRoot) != "" {
		return repoRoot
	}
	if root, err := workspace.FindRepoRoot(""); err == nil && root != "" {
		return root
	}
	return "."
}

func attachReleaseAndMneme(rec *SessionDatasetRecord, relIdx releaseJoinIndex, memIdx mnemeJoinIndex) {
	taskKey := strings.ToUpper(strings.TrimSpace(rec.Session.TaskKey))
	if note, ok := relIdx.bySession[rec.Session.ID]; ok {
		applyReleaseNoteFields(rec, note)
		if taskKey == "" {
			taskKey = strings.ToUpper(strings.TrimSpace(note.TaskKey))
		}
	} else if taskKey != "" {
		if note, ok := relIdx.byTask[taskKey]; ok {
			applyReleaseNoteFields(rec, note)
		}
	}
	if taskKey != "" {
		rec.PitfallsFixed = memIdx.pitfalls[taskKey]
		rec.PatternsMinted = memIdx.patterns[taskKey]
	}
}

func applyReleaseNoteFields(rec *SessionDatasetRecord, note task.ReleaseNotePayload) {
	rec.ReleaseCommit = note.CommitSHA
	rec.DoDGatesPassed = note.DoDGatesPassed
	rec.SHA256Receipt = note.SHA256Receipt
}

func loadReleaseJoinIndex(repoRoot string) releaseJoinIndex {
	idx := releaseJoinIndex{
		byTask:    make(map[string]task.ReleaseNotePayload),
		bySession: make(map[string]task.ReleaseNotePayload),
	}
	cmd := workspace.SafeGitCommand(repoRoot, "log", "-n", "200", "--notes=refs/notes/nomos/releases", "--pretty=format:===NOMOS_NOTE===%n%N")
	raw, err := cmd.Output()
	if err != nil || len(raw) == 0 {
		return idx
	}
	for _, block := range strings.Split(string(raw), "===NOMOS_NOTE===") {
		trimmed := strings.TrimSpace(block)
		if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
			continue
		}
		var note task.ReleaseNotePayload
		if uErr := json.Unmarshal([]byte(trimmed), &note); uErr != nil || note.TaskKey == "" {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(note.TaskKey))
		if _, exists := idx.byTask[key]; !exists {
			idx.byTask[key] = note
		}
		if note.SessionID != "" {
			if _, exists := idx.bySession[note.SessionID]; !exists {
				idx.bySession[note.SessionID] = note
			}
		}
	}
	return idx
}

func loadMnemeJoinIndex(dbPath string) mnemeJoinIndex {
	idx := mnemeJoinIndex{
		pitfalls: make(map[string][]string),
		patterns: make(map[string][]string),
	}
	if strings.TrimSpace(dbPath) == "" {
		return idx
	}
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return idx
	}
	store, err := mem.NewStore(dbPath)
	if err != nil {
		return idx
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// non-fatal read-only close
		}
	}()

	records, err := store.ListAll()
	if err != nil {
		return idx
	}
	for _, r := range records {
		key := strings.ToUpper(strings.TrimSpace(r.TaskID))
		if key == "" {
			continue
		}
		if r.Kind == mem.KindPitfall {
			idx.pitfalls[key] = append(idx.pitfalls[key], r.Content)
		} else if r.Kind == mem.KindPattern {
			idx.patterns[key] = append(idx.patterns[key], r.Content)
		}
	}
	return idx
}
