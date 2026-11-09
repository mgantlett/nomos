package mem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSweepRepoPitfallHygiene_DeadScopesExpiredTestsAndCrossFolderHabits(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())

	repo := filepath.Join(t.TempDir(), "nomos")
	for _, dir := range []string{
		filepath.Join(repo, ".git"),
		filepath.Join(repo, "substrate", "cli"),
		filepath.Join(repo, "substrate", "src", "nomos", "modules", "verify"),
		filepath.Join(repo, "substrate", "src", "nomos", "modules", "mem"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			t.Errorf("Close: %v", cErr)
		}
	}()

	seedMem := func(topic, content string) int {
		rec := &Memory{
			RepoID:  "nomos",
			Kind:    KindPitfall,
			Topic:   topic,
			Content: content,
			TaskID:  "SUB-100",
			Status:  StatusActive,
		}
		if sErr := store.Save(rec); sErr != nil {
			t.Fatalf("Save(%s): %v", topic, sErr)
		}
		return rec.ID
	}

	deadID := seedMem(
		"pitfall:cognitive_complexity_invariant:modules/verify",
		"[Cognitive Complexity Invariant in modules/verify] Trap: old path -> Verified Fix (SUB-90): split helper",
	)
	testFailID := seedMem(
		"pitfall:go_toolchain_gate:substrate/cli",
		"[Go Toolchain Gate in substrate/cli] Trap: --- FAIL: TestHookPostTool (0.00s) -> Verified Fix (SUB-91): updated trace",
	)
	seedMem(
		"pitfall:unchecked_error_hygiene_invariant:substrate/cli",
		"[Unchecked Error Hygiene Invariant in substrate/cli] Trap: runner.go:12 blank discard -> Verified Fix (SUB-92): handle err",
	)
	seedMem(
		"pitfall:unchecked_error_hygiene_invariant:substrate/src/nomos/modules/verify",
		"[Unchecked Error Hygiene Invariant in substrate/src/nomos/modules/verify] Trap: gates.go:44 blank discard -> Verified Fix (SUB-93): handle err",
	)
	seedMem(
		"pitfall:unchecked_error_hygiene_invariant:substrate/src/nomos/modules/mem",
		"[Unchecked Error Hygiene Invariant in substrate/src/nomos/modules/mem] Trap: learn.go:88 blank discard -> Verified Fix (SUB-94): handle err",
	)

	// Unscoped handshake recall must exclude unit-test assertion failures even before 72h expiry
	unscoped := RecallRepoPitfallsReadOnly(repo, nil, 5)
	for _, line := range unscoped {
		if strings.Contains(line, "--- FAIL: TestHookPostTool") {
			t.Fatalf("expected unscoped recall to exclude unit-test assertion failure, got: %s", line)
		}
	}

	// Sweep 4 days in the future: dead directory + expired test trace must tombstone, and 3-dir habit must consolidate
	future := time.Now().UTC().Add(96 * time.Hour)
	rep := SweepRepoPitfallHygiene(store, repo, "nomos", future)
	if rep.DeadScopesTombstoned != 1 {
		t.Fatalf("expected 1 dead scope tombstoned, got %d", rep.DeadScopesTombstoned)
	}
	if rep.ExpiredTestsTombstoned != 1 {
		t.Fatalf("expected 1 expired test trace tombstoned, got %d", rep.ExpiredTestsTombstoned)
	}
	if rep.CrossScopeHabitsUpdated != 1 {
		t.Fatalf("expected 1 cross-scope habit pattern synthesized, got %d", rep.CrossScopeHabitsUpdated)
	}

	deadRec, gErr := store.Get(deadID)
	if gErr != nil || deadRec.Status != StatusTombstoned {
		t.Fatalf("expected dead-scope pitfall #%d tombstoned, got %+v (err=%v)", deadID, deadRec, gErr)
	}
	testRec, tErr := store.Get(testFailID)
	if tErr != nil || testRec.Status != StatusTombstoned {
		t.Fatalf("expected expired test pitfall #%d tombstoned, got %+v (err=%v)", testFailID, testRec, tErr)
	}

	patterns := RecallRepoPatternsReadOnly(repo, "unchecked error blank discard", nil, 5)
	if len(patterns) != 1 || !strings.Contains(patterns[0], "Repo-Wide Habit") {
		t.Fatalf("expected 1 consolidated Repo-Wide Habit pattern, got %v", patterns)
	}
}

func TestIntentSteelManClassifiedAsBuiltInStatutoryGate(t *testing.T) {
	if !isBuiltInStructuralASTTopic("pitfall:intent_steel-man_parity_gate:architecture/specs") {
		t.Fatalf("expected Intent Steel-Man Parity Gate to be recognized as built-in statutory gate")
	}
}
