package mem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPerTaskReinforcementDeduplicationAndReadOnlySearch(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "epsilon-repo")
	touched := []string{"substrate/cli/runner.go"}

	if err := RecordGateTrap(repo, "SUB-10", touched, []GateTrapEntry{{
		GateName: "Forbidden Literals Gate",
		Message:  "substrate/cli/runner.go:19: forbidden hardcoded literal token detected",
	}}); err != nil {
		t.Fatalf("RecordGateTrap: %v", err)
	}
	if _, err := DistillTaskLearnings(repo, "SUB-10", "- Replaced literal with config lookup", touched); err != nil {
		t.Fatalf("DistillTaskLearnings: %v", err)
	}

	// 5 recalls inside the SAME task (SUB-11) must increment access_count only once (+1)
	for i := 0; i < 5; i++ {
		_ = RecallRepoPitfallsForTask(repo, "SUB-11", touched, 3)
	}

	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// ReadOnly CLI search must NOT increment access_count (+0)
	for i := 0; i < 4; i++ {
		if _, sErr := store.SearchWithOptions("forbidden hardcoded literal", SearchOptions{
			RepoID:   "epsilon-repo",
			ReadOnly: true,
			Limit:    3,
		}); sErr != nil {
			t.Fatalf("SearchWithOptions ReadOnly: %v", sErr)
		}
	}

	all, err := store.ListAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("expected 1 pitfall record, got len=%d err=%v", len(all), err)
	}
	if all[0].AccessCount != 1 {
		t.Fatalf("TC-MEM-10: expected access_count=1 after 5 same-task handshakes + 4 ReadOnly searches, got %d", all[0].AccessCount)
	}

	// Recalls from two NEW distinct tasks (SUB-12, SUB-13) bring access_count to 3 -> surfaces Tier A->B policy graduation
	_ = RecallRepoPitfallsForTask(repo, "SUB-12", touched, 3)
	_ = RecallRepoPitfallsForTask(repo, "SUB-13", touched, 3)
	cands := RecallGraduationCandidates(repo, 3)
	if len(cands) != 1 || !strings.Contains(cands[0], "[Tier A->B (recurred 1x · 3x recalled)") {
		t.Fatalf("TC-MEM-08/10: expected policy pitfall to surface Tier A->B at 3 distinct task recalls, got %v", cands)
	}
}

func TestDistillTaskLearnings_SynthesizesGoSymbolDelta(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "zeta-repo")
	cliDir := filepath.Join(repo, "substrate", "cli")
	if err := os.MkdirAll(cliDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	goSrc := "package cli\n\nfunc splitUnquotedChainSegments(s string) []string { return nil }\nfunc advanceQuoteState(s string, i int) byte { return 0 }\nfunc chainSeparatorWidth(s string, i int) int { return 0 }\n"
	if err := os.WriteFile(filepath.Join(cliDir, "cmd_hook_runtime.go"), []byte(goSrc), 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	touched := []string{"substrate/cli/cmd_hook_runtime.go"}
	if err := RecordGateTrap(repo, "SUB-1764", touched, []GateTrapEntry{{
		GateName: "Cognitive Complexity Invariant",
		Message:  "- substrate/cli/cmd_hook_runtime.go:202 (splitUnquotedChainSegments) cyclomatic complexity 16 exceeds max 15",
	}}); err != nil {
		t.Fatalf("RecordGateTrap: %v", err)
	}
	if _, err := DistillTaskLearnings(repo, "SUB-1764", "- Implemented hook execution-hygiene upgrades", touched); err != nil {
		t.Fatalf("DistillTaskLearnings: %v", err)
	}

	recalled := RecallRepoPitfallsReadOnly(repo, touched, 3)
	if len(recalled) != 1 {
		t.Fatalf("expected 1 pitfall, got %d", len(recalled))
	}
	wantSub := "Refactored cmd_hook_runtime.go [splitUnquotedChainSegments, advanceQuoteState, chainSeparatorWidth]"
	if !strings.Contains(recalled[0], wantSub) {
		t.Fatalf("TC-MEM-11: expected symbol delta %q in %q", wantSub, recalled[0])
	}
}

func TestUnscopedOneOffPitfallCoolingAndStatusQuery(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "eta-repo")
	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	staleCreated := time.Now().UTC().Add(-20 * 24 * time.Hour)
	staleOneOff := &Memory{
		RepoID:    "eta-repo",
		Kind:      KindPitfall,
		Topic:     "pitfall:cognitive_complexity_invariant:.github/ISSUE_TEMPLATE",
		Content:   "[Cognitive Complexity Invariant in .github/ISSUE_TEMPLATE] Trap: stale one-off -> Verified Fix (SUB-1): split",
		TaskID:    "SUB-1",
		Status:    StatusActive,
		CreatedAt: staleCreated,
	}
	if sErr := store.Save(staleOneOff); sErr != nil {
		t.Fatalf("Save staleOneOff: %v", sErr)
	}

	// Unscoped handshake (no touched files) must cool out a 20-day-old single-occurrence pitfall
	if unscoped := RecallRepoPitfallsReadOnly(repo, nil, 3); len(unscoped) != 0 {
		t.Fatalf("TC-MEM-12: expected 20d one-off pitfall cooled out of unscoped recall, got %v", unscoped)
	}

	// Touching a file inside .github/ISSUE_TEMPLATE must STILL surface the pitfall
	if scoped := RecallRepoPitfallsReadOnly(repo, []string{".github/ISSUE_TEMPLATE/01_bug_report.yml"}, 3); len(scoped) != 1 {
		t.Fatalf("TC-MEM-12: expected scoped file match to recall older directory pitfall, got %v", scoped)
	}

	// Dismissing transitions to tombstoned and excludes from StatusActive Query while matching StatusTombstoned
	if _, dErr := store.Dismiss(staleOneOff.ID); dErr != nil {
		t.Fatalf("Dismiss: %v", dErr)
	}
	activeOnly, _ := store.Query(MemoryFilter{RepoID: "eta-repo", Status: StatusActive})
	tombstonedOnly, _ := store.Query(MemoryFilter{RepoID: "eta-repo", Status: StatusTombstoned})
	if len(activeOnly) != 0 || len(tombstonedOnly) != 1 {
		t.Fatalf("TC-MEM-13: expected active=0 tombstoned=1, got active=%d tombstoned=%d", len(activeOnly), len(tombstonedOnly))
	}
}

