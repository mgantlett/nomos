package mem

import (
	"os"
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
	touched := []string{"substrate/pkg/scheduler/ops_test.go"}

	traps := []GateTrapEntry{
		{GateName: "Go Toolchain Gate", Message: "Go compilation/test failed: --- FAIL: TestOps (0.12s)\n    ops_test.go:76: expected 1, got 0\nFAIL\tgithub.com/acme/pkg/scheduler\t0.15s"},
		{GateName: "Statement Coverage Invariant (Law 2)", Message: "Go test coverage execution failed: --- FAIL: TestOps (0.12s)\n    ops_test.go:76: expected 1, got 0\nFAIL\tgithub.com/acme/pkg/scheduler\t0.15s"},
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

func TestDistillTaskLearnings_PolyglotDiagnosticsTouchedAlignmentAndCleanFix(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := t.TempDir()
	daemonDir := filepath.Join(repo, "substrate", "infra", "daemons")
	uiDir := filepath.Join(repo, "substrate", "membrane", "analytics")
	for _, d := range []string{daemonDir, uiDir} {
		if err := mkdirAllTest(d); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	writeTestFile(t, filepath.Join(daemonDir, "kev_daemon.py"), "def run_daemon():\n    return 1\n")
	writeTestFile(t, filepath.Join(uiDir, "dora_summary.ts"), "export function compileSummary() { return 1; }\n")

	touched := []string{"substrate/membrane/analytics/dora_summary.ts"}
	traps := []GateTrapEntry{
		{
			GateName: "Cognitive Complexity Invariant",
			Message: "detected 2 complexity/arity violation(s):\n" +
				"    - substrate/infra/daemons/kev_daemon.py:12 (run_daemon) cyclomatic complexity 18 exceeds max 15\n" +
				"    - substrate/membrane/analytics/dora_summary.ts:34 (compileSummary) cyclomatic complexity 17 exceeds max 15",
		},
		{
			GateName: "Go Toolchain Gate",
			Message:  "JavaScript/TypeScript test suite failed: TAP version 13\n# Subtest: dora\nnot ok 1 - expected metric rounding",
		},
	}
	if err := RecordGateTrap(repo, "SUB-1833", touched, traps); err != nil {
		t.Fatalf("RecordGateTrap: %v", err)
	}
	walkthrough := "# Walkthrough\n**1. Executive UI Cleanup:**\n- Removed the miswired Task Alerts dummy toast button\n"
	if _, err := DistillTaskLearnings(repo, "SUB-1833", walkthrough, touched); err != nil {
		t.Fatalf("DistillTaskLearnings: %v", err)
	}

	got := RecallRepoPitfallsReadOnly(repo, touched, 5)
	if len(got) != 2 {
		t.Fatalf("expected 2 distilled pitfalls, got %d: %v", len(got), got)
	}
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "kev_daemon.py") || strings.Contains(joined, "substrate/infra/daemons") {
		t.Fatalf("untouched file leaked into scope or Verified Fix:\n%s", joined)
	}
	if !strings.Contains(joined, "dora_summary.ts:34") || !strings.Contains(joined, "Refactored dora_summary.ts [compileSummary]") {
		t.Fatalf("expected touched polyglot violation and clean symbol fix:\n%s", joined)
	}
	if strings.Contains(joined, ":**") {
		t.Fatalf("bold colon section header leaked into fix summary:\n%s", joined)
	}
	for _, item := range got {
		if strings.Contains(item, "Cognitive Complexity Invariant") && strings.Contains(item, "Task Alerts") {
			t.Fatalf("unrelated walkthrough prose leaked into symbol-delta fix:\n%s", item)
		}
	}
	if !strings.Contains(joined, "not ok 1 - expected metric rounding") {
		t.Fatalf("expected TAP 'not ok' failure line instead of bare TAP header:\n%s", joined)
	}
}

func TestScopedOnlyReinforcementAndStructuralASTGraduation(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "theta-repo")
	cliTouched := []string{"substrate/cli/runner.go"}

	if err := RecordGateTrap(repo, "SUB-20", cliTouched, []GateTrapEntry{
		{GateName: "Unchecked Error Hygiene Invariant", Message: "detected 1 unchecked error discard(s):\n  - substrate/cli/runner.go:22: blank discard"},
		{GateName: "Max File LOC Invariant", Message: "substrate/cli/runner.go has 405 LOC (exceeds max 400)"},
	}); err != nil {
		t.Fatalf("RecordGateTrap: %v", err)
	}
	if _, err := DistillTaskLearnings(repo, "SUB-20", "- Checked close error and split helpers in runner.go", cliTouched); err != nil {
		t.Fatalf("DistillTaskLearnings: %v", err)
	}

	// Unscoped recalls and recalls from an unrelated directory must NOT inflate access_count
	_ = RecallRepoPitfallsForTask(repo, "SUB-21", nil, 3)
	_ = RecallRepoPitfallsForTask(repo, "SUB-22", []string{"substrate/membrane/App.svelte"}, 3)

	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	all, _ := store.ListAll()
	_ = store.Close()
	for _, m := range all {
		if m.AccessCount != 0 {
			t.Fatalf("expected 0 access_count from unscoped/other-dir recalls on %s, got %d", m.Topic, m.AccessCount)
		}
		if strings.HasPrefix(m.Topic, "pitfall:unchecked_error") && !strings.Contains(m.Topic, "unchecked_error_hygiene_invariant:") {
			t.Fatalf("expected un-truncated gate name in topic, got %q", m.Topic)
		}
	}

	// Even after 3 scoped recalls on a single-occurrence built-in AST gate, Tier A->B must stay empty
	for _, tk := range []string{"SUB-23", "SUB-24", "SUB-25"} {
		_ = RecallRepoPitfallsForTask(repo, tk, cliTouched, 3)
	}
	if cands := RecallGraduationCandidates(repo, 5); len(cands) != 0 {
		t.Fatalf("expected 0 graduation candidates for 1x built-in Max File LOC / Unchecked Error gates, got %v", cands)
	}
}

func mkdirAllTest(path string) error {
	return os.MkdirAll(path, 0755)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

