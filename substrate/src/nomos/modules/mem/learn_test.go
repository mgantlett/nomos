package mem

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNomosLearnTrapDistillationAndRepoIsolation(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())

	repoA := filepath.Join(t.TempDir(), "alpha-service")
	repoB := filepath.Join(t.TempDir(), "beta-service")

	touched := []string{"substrate/src/billing.py"}
	traps := []GateTrapEntry{
		{
			GateName: "Go Toolchain Gate",
			Message:  "Python syntax compilation failed: IndentationError",
		},
	}

	if err := RecordGateTrap(repoA, "SUB-1", touched, traps); err != nil {
		t.Fatalf("RecordGateTrap failed: %v", err)
	}

	walkthrough := "# Walkthrough\n- Fixed Python indentation and added explicit type annotations in billing.py\n"
	distilled, err := DistillTaskLearnings(repoA, "SUB-1", walkthrough, touched)
	if err != nil {
		t.Fatalf("DistillTaskLearnings failed: %v", err)
	}
	if distilled != 1 {
		t.Fatalf("expected 1 distilled pitfall, got %d", distilled)
	}

	recalledA := RecallRepoPitfalls(repoA, touched, 3)
	if len(recalledA) != 1 {
		t.Fatalf("expected 1 recalled pitfall in repoA, got %d", len(recalledA))
	}
	if !strings.Contains(recalledA[0], "IndentationError") || !strings.Contains(recalledA[0], "Verified Fix (SUB-1)") {
		t.Fatalf("unexpected distilled pitfall content: %s", recalledA[0])
	}

	// Strict repo partitioning: repoB must not recall repoA's pitfalls
	recalledB := RecallRepoPitfalls(repoB, touched, 3)
	if len(recalledB) != 0 {
		t.Fatalf("expected 0 recalled pitfalls in isolated repoB, got %v", recalledB)
	}

	// Superseding duplicate gate+scope trap on subsequent task
	if err := RecordGateTrap(repoA, "SUB-2", touched, []GateTrapEntry{{
		GateName: "Go Toolchain Gate",
		Message:  "Python pytest suite failed: AssertionError in billing",
	}}); err != nil {
		t.Fatalf("second RecordGateTrap failed: %v", err)
	}
	if _, err := DistillTaskLearnings(repoA, "SUB-2", "- Resolved billing rounding edge case", touched); err != nil {
		t.Fatalf("second DistillTaskLearnings failed: %v", err)
	}

	afterSupersede := RecallRepoPitfalls(repoA, touched, 5)
	if len(afterSupersede) != 1 {
		t.Fatalf("expected older pitfall on identical topic to be superseded (1 active), got %d: %v", len(afterSupersede), afterSupersede)
	}
	if !strings.Contains(afterSupersede[0], "SUB-2") {
		t.Fatalf("expected newest SUB-2 pitfall to remain active, got %s", afterSupersede[0])
	}
}

func TestDistillTaskLearnings_ExtractsSalientFailLineAndDeduplicatesPairedGates(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "gamma-service")
	touched := []string{"substrate/pkg/cockpit/ops_test.go"}
	multiOut := "Go compilation/test failed: ok\tgithub.com/acme/cmd/a\t0.010s\n--- FAIL: TestOps (0.12s)\n    ops_test.go:76: expected 1, got 0\nFAIL\tgithub.com/acme/pkg/cockpit\t0.15s"

	traps := []GateTrapEntry{
		{GateName: "Go Toolchain Gate", Message: multiOut},
		{GateName: "Statement Coverage Invariant (Law 2)", Message: multiOut},
	}
	if err := RecordGateTrap(repo, "SUB-9", touched, traps); err != nil {
		t.Fatalf("RecordGateTrap: %v", err)
	}
	distilled, err := DistillTaskLearnings(repo, "SUB-9", "- Fixed ops_test expectation", touched)
	if err != nil || distilled != 1 {
		t.Fatalf("expected paired identical test trap deduplicated to 1 pitfall, got distilled=%d err=%v", distilled, err)
	}
	recalled := RecallRepoPitfalls(repo, touched, 3)
	if len(recalled) != 1 || !strings.Contains(recalled[0], "--- FAIL: TestOps") || !strings.Contains(recalled[0], "ops_test.go:76") {
		t.Fatalf("expected salient --- FAIL line extracted instead of ok line, got %v", recalled)
	}
}

