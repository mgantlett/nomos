package autofix

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

func runRepairGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func TestBuildVerifyFixPlan_ExtractsLocationAndClassifiesSafeAutoFixes(t *testing.T) {
	results := []*vscope.GateResult{
		{GateName: "Passing Gate", Passed: true},
		{
			GateName: "Contract Schema Integrity Invariant",
			ProbeID:  "contract_parity",
			Law:      4,
			Passed:   false,
			Message:  "nomos.schema.json has drifted from worktree config.EcosystemConfig AST",
			Remedy:   "regenerate nomos.schema.json",
		},
		{
			GateName: "Cognitive Complexity Invariant",
			ProbeID:  "cognitive_complexity",
			Law:      2,
			Passed:   false,
			Message:  "substrate/cli/cmd_xray.go:42 exceeds cognitive complexity budget",
			Remedy:   "extract helper functions",
		},
	}

	plan := BuildVerifyFixPlan(false, 40, 42, results)
	if plan.Passed || len(plan.Recipes) != 2 {
		t.Fatalf("expected 2 repair recipes, got %+v", plan)
	}
	if !plan.Recipes[0].SafeAutoFix || plan.Recipes[0].ActionKind != "auto_schema_sync" || plan.Recipes[0].File != nomosSchemaFileName {
		t.Fatalf("expected contract_parity recipe to be auto_schema_sync, got %+v", plan.Recipes[0])
	}
	if plan.Recipes[1].SafeAutoFix || plan.Recipes[1].File != "substrate/cli/cmd_xray.go" || plan.Recipes[1].Line != 42 {
		t.Fatalf("expected complexity recipe at substrate/cli/cmd_xray.go:42, got %+v", plan.Recipes[1])
	}
	rendered := FormatFixPlanTerminal(plan)
	if !strings.Contains(rendered, "Nomos Deterministic Repair Plan") || !strings.Contains(rendered, "substrate/cli/cmd_xray.go:42") {
		t.Fatalf("unexpected rendered repair plan:\n%s", rendered)
	}
}

func TestApplySafeAutoFixes_FormatsGoAndSyncsSchemaWhileBlockingBaseline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fix-wt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runRepairGit(t, dir, "init", "-b", "main")
	runRepairGit(t, dir, "config", "user.email", "dev@example.com")
	runRepairGit(t, dir, "config", "user.name", "Fix Tester")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# seed\n"), 0644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	runRepairGit(t, dir, "add", "README.md")
	runRepairGit(t, dir, "commit", "-m", "chore: seed")

	unformattedGo := "package demo\nfunc   Add(a int,b int)int{return a+b}\n"
	goPath := filepath.Join(dir, "demo.go")
	if err := os.WriteFile(goPath, []byte(unformattedGo), 0644); err != nil {
		t.Fatalf("write demo.go: %v", err)
	}
	schemaPath := filepath.Join(dir, nomosSchemaFileName)
	if err := os.WriteFile(schemaPath, []byte(`{"drifted": true}`), 0644); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	plan := &VerifyFixPlan{
		Recipes: []VerifyFixRecipe{
			{GateName: "Contract Schema Integrity Invariant", ProbeID: "contract_parity", ActionKind: "auto_schema_sync", SafeAutoFix: true},
		},
	}

	if _, bErr := ApplySafeAutoFixes(dir, true, plan); bErr == nil {
		t.Fatal("expected ApplySafeAutoFixes to fail closed when isBaseline is true")
	}

	applied, err := ApplySafeAutoFixes(dir, false, plan)
	if err != nil {
		t.Fatalf("ApplySafeAutoFixes failed: %v", err)
	}
	if len(applied) < 2 {
		t.Fatalf("expected both schema sync and gofmt applied, got %v", applied)
	}
	gotGo, rErr := os.ReadFile(goPath)
	if rErr != nil || !strings.Contains(string(gotGo), "func Add(a int, b int) int { return a + b }") {
		t.Fatalf("expected demo.go to be formatted, got %q (err=%v)", string(gotGo), rErr)
	}
	gotSchema, sErr := os.ReadFile(schemaPath)
	if sErr != nil || strings.Contains(string(gotSchema), `"drifted"`) {
		t.Fatalf("expected nomos.schema.json regenerated from Go SSoT, err=%v", sErr)
	}
}
