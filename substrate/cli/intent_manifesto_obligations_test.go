// Package cli_test verifies Stage 0 Intent Charter obligations TC-INT-01 through TC-INT-06.
// @intent intent.md
// @intent nomos_intent_traceability_spec.md
package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

// TestIntentCharter_PlaneAndCanonicalSingularity verifies Pure 2-Plane and Canonical Singularity task key invariants.
// @verifies TC-INT-01
// @verifies TC-INT-02
func TestIntentCharter_PlaneAndCanonicalSingularity(t *testing.T) {
	for _, validKey := range []string{"INT-592", "SUB-2134", "INT-1941-charter"} {
		if err := task.ValidateTaskKey(validKey); err != nil {
			t.Fatalf("expected canonical 2-plane key %q to pass ValidateTaskKey, got %v", validKey, err)
		}
	}
	for _, invalidKey := range []string{"MEM-101", "CUST-1", "OPS-42", "sub-2134"} {
		if err := task.ValidateTaskKey(invalidKey); err == nil {
			t.Fatalf("expected non-canonical key %q to fail closed under Law 6", invalidKey)
		}
	}
}

// TestIntentCharter_ContractSignatureParity verifies intent/intent.md fenced Go SSoT contracts match compiled Go AST.
// @verifies TC-INT-03
func TestIntentCharter_ContractSignatureParity(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed resolving working directory: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	srcDir := filepath.Join(repoRoot, "substrate", "src")
	issues := verify.VerifySpecTypeSignatures(repoRoot, srcDir, []string{"intent/intent.md"})
	if len(issues) != 0 {
		t.Fatalf("expected zero contract drift on intent/intent.md, got: %v", issues)
	}

	tmpDir := t.TempDir()
	driftedSpec := filepath.Join(tmpDir, "drifted_spec.md")
	driftedBody := "```go\npackage config\ntype ContractsConfig struct {\n\tMissingPhantomField bool\n}\n```\n"
	if wErr := os.WriteFile(driftedSpec, []byte(driftedBody), 0600); wErr != nil {
		t.Fatalf("failed writing drifted fixture: %v", wErr)
	}
	driftIssues := verify.VerifySpecTypeSignatures(repoRoot, srcDir, []string{driftedSpec})
	if len(driftIssues) == 0 {
		t.Fatal("expected drifted ContractsConfig field to fail VerifySpecTypeSignatures")
	}
}

// TestIntentCharter_NCTEAmbiguousModalRejection verifies NCTE-100 rejects banned modal verbs in Intent prose.
// @verifies TC-INT-04
func TestIntentCharter_NCTEAmbiguousModalRejection(t *testing.T) {
	policy := ncte.DefaultPolicy()
	engine := ncte.NewEngine(&policy)
	prose := "# Sample\n\nThe runtime should maybe skip validation.\n"
	rep, err := engine.LintMarkdown("sample_spec.md", []byte(prose))
	if err != nil {
		t.Fatalf("unexpected LintMarkdown error: %v", err)
	}
	if rep == nil || len(rep.Diagnostics) == 0 {
		t.Fatal("expected NCTE-100 linter to reject banned modal verb in prose")
	}
}

// TestIntentCharter_DatabaseEncapsulationAndHermeticBoundary verifies Law 1 DB encapsulation and Law 3 worktree lockouts.
// @verifies TC-INT-05
// @verifies TC-INT-06
func TestIntentCharter_DatabaseEncapsulationAndHermeticBoundary(t *testing.T) {
	rawSQLCmd := "sqlite3 " + constants.FileMnemeDB + " 'SELECT count(*) FROM memories'"
	out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": rawSQLCmd},
	})
	if dec := parseDecision(t, out); dec.Decision != "deny" {
		t.Fatalf("expected direct sqlite3 access against %s to be denied, got %+v", constants.FileMnemeDB, dec)
	}

	repoRoot := t.TempDir()
	trunkTarget := filepath.Join(repoRoot, "substrate", "cli", "root.go")
	if bErr := workspace.ValidateWorktreeBoundary(repoRoot, "main", trunkTarget); bErr == nil {
		t.Fatal("expected direct trunk mutation outside worktrees/ on main to fail closed")
	}
}
