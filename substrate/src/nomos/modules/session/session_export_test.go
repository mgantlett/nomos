package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

func TestExportSessionDataset_JoinsSentinelAndMnemeLearnings(t *testing.T) {
	tempBrain := t.TempDir()
	mnemePath := filepath.Join(t.TempDir(), "mneme_export.db")
	t.Setenv("NOMOS_MNEME_DB", mnemePath)

	store, err := mem.NewStore(mnemePath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if err := store.Save(&mem.Memory{
		RepoID:  "nomos",
		Kind:    mem.KindPitfall,
		Topic:   "pitfall:max_file_loc_invariant:substrate/cli",
		Content: "[Max File LOC Invariant] Trap -> Verified Fix (SUB-777): split helper",
		TaskID:  "SUB-777",
	}); err != nil {
		t.Fatalf("save pitfall: %v", err)
	}
	if err := store.Save(&mem.Memory{
		RepoID:  "nomos",
		Kind:    mem.KindPattern,
		Topic:   "pattern:substrate/cli",
		Content: "[Pattern in substrate/cli (SUB-777)] files=[cmd_session.go]",
		TaskID:  "SUB-777",
	}); err != nil {
		t.Fatalf("save pattern: %v", err)
	}
	_ = store.Close()

	sessID := "22222222-3333-4444-5555-666666666666"
	logDir := filepath.Join(tempBrain, sessID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir logDir: %v", err)
	}
	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Implement SUB-777"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","content":"Verifying","tool_calls":[{"name":"run_command","args":{"CommandLine":"cd worktrees/nomos-SUB-777 && nomos verify && git commit -m \"feat: sub-777\""}}]}`,
	}
	if err := os.WriteFile(filepath.Join(logDir, "transcript.jsonl"), []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	records, err := ExportSessionDataset(tempBrain, t.TempDir(), 5)
	if err != nil {
		t.Fatalf("ExportSessionDataset failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 exported session record, got %d", len(records))
	}
	rec := records[0]
	if rec.Sentinel == nil || rec.Sentinel.OverallScore != 100 {
		t.Fatalf("expected Sentinel score 100, got %+v", rec.Sentinel)
	}
	if len(rec.PitfallsFixed) != 1 || len(rec.PatternsMinted) != 1 {
		t.Fatalf("TC-SES-06: expected 1 pitfall and 1 pattern joined for SUB-777, got pitfalls=%d patterns=%d", len(rec.PitfallsFixed), len(rec.PatternsMinted))
	}
}
