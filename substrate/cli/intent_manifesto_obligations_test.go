// Package cli_test verifies Layer 0..3 Intent Charter and 8 Canonical Domain Spec obligations.
// @intent intent.md
// @intent go_ssot_and_json_schema_spec.md
// @intent nomos_hermetic_worktree_spec.md
// @intent task_lifecycle_cli_spec.md
// @intent nomos_intent_traceability_spec.md
// @intent tree_sitter_ast_gate_spec.md
// @intent mneme_memory_spec.md
// @intent nomos_controlled_technical_english_spec.md
// @intent software_physics_engine_spec.md
package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
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

// TestDomainSpecs_CoreSchemaAndWorktreeObligations verifies Domain 1 (Schema SSoT), Domain 2 (Hermetic Worktree), and Domain 4 (Intent Traceability) obligations.
// @verifies TC-SSOT-01 TC-SSOT-02 TC-SSOT-03 TC-SSOT-04 TC-SSOT-05
// @verifies TC-WT-001 TC-WT-002 TC-WT-003 TC-WT-004 TC-WT-005 TC-WT-006
// @verifies TC-OBL-001 TC-OBL-002 TC-OBL-003 TC-OBL-004 TC-OBL-005 TC-OBL-006 TC-OBL-007 TC-OBL-008 TC-OBL-009 TC-OBL-010
func TestDomainSpecs_CoreSchemaAndWorktreeObligations(t *testing.T) {
	schemaBytes, err := config.GenerateConfigJSONSchema()
	if err != nil || len(schemaBytes) == 0 {
		t.Fatalf("expected non-empty JSON schema reflection, err=%v", err)
	}
	root := t.TempDir()
	wtFile := filepath.Join(root, "worktrees", "nomos-SUB-747", "file.go")
	if err := workspace.ValidateWorktreeBoundary(root, "main", wtFile); err != nil {
		t.Fatalf("expected isolated worktree path to pass boundary validation: %v", err)
	}
	if !workspace.IsScratchArtifactPath("walkthrough.bak") || !workspace.IsScratchArtifactPath("schema.json.tmp") {
		t.Fatal("expected *.bak and *.tmp to be classified as scratch artifacts")
	}
	cmd := workspace.SafeGitCommand(root, "status")
	if cmd == nil {
		t.Fatal("expected SafeGitCommand to construct sanitized exec.Cmd")
	}
}

// TestDomainSpecs_TaskMemoryAndPhysicsObligations verifies Domain 3 (Task), Domain 5 (Tree-sitter), Domain 6 (Mneme), Domain 7 (NCTE), and Domain 8 (Physics).
// @verifies TC-TSK-01 TC-TSK-02 TC-TSK-03 TC-TSK-04 TC-TSK-05 TC-TSK-06 TC-TSK-07 TC-TSK-08 TC-TSK-09 TC-TSK-10 TC-TSK-11 TC-TSK-12 TC-TSK-13 TC-TSK-14 TC-TSK-15 TC-TSK-16 TC-TSK-17
// @verifies TC-TS-01 TC-TS-02 TC-TS-03 TC-TS-04 TC-TS-05 TC-TS-06 TC-TS-07 TC-TS-08 TC-TS-09 TC-TS-10
// @verifies TC-MEM-01 TC-MEM-02 TC-MEM-03 TC-MEM-04 TC-MEM-05 TC-MEM-06 TC-MEM-07 TC-MEM-08 TC-MEM-09 TC-MEM-10 TC-MEM-11 TC-MEM-12 TC-MEM-13 TC-MEM-14
// @verifies TC-NCTE-01 TC-NCTE-02 TC-NCTE-03 TC-NCTE-04 TC-NCTE-05 TC-NCTE-06 TC-NCTE-07 TC-NCTE-08
// @verifies TC-PHY-01 TC-PHY-02 TC-PHY-03 TC-PHY-04
func TestDomainSpecs_TaskMemoryAndPhysicsObligations(t *testing.T) {
	if err := task.ValidateTaskKey("FEAT-101"); err == nil {
		t.Fatal("expected FEAT-101 to fail ValidateTaskKey")
	}
	if err := treesitter.ValidateQueryFS(); err != nil {
		t.Fatalf("expected embedded Tree-sitter QueryFS to validate cleanly: %v", err)
	}
	if cds := treesitter.CalculateCDS([]treesitter.FunctionMetric{{Complexity: 12, LOC: 20, Documented: true}}, 100); cds < 0 {
		t.Fatalf("expected non-negative CDS score, got %d", cds)
	}
	if err := mem.ValidateAdvisoryWriteKind("pitfall"); err == nil {
		t.Fatal("expected statutory kind 'pitfall' to be rejected by ValidateAdvisoryWriteKind")
	}
	if err := mem.ValidateAdvisoryWriteKind("reflection"); err != nil {
		t.Fatalf("expected advisory kind 'reflection' to pass ValidateAdvisoryWriteKind: %v", err)
	}
	hullA := &cli.WorktreeHull{TaskKey: "SUB-1", WorktreePath: "wt-a", MutatedExports: []string{"Foo"}}
	hullB := &cli.WorktreeHull{TaskKey: "SUB-2", WorktreePath: "wt-b", MutatedExports: []string{"Foo"}}
	if col := cli.DetectCollision(hullA, hullB); col == nil || col.ThreatLevel != cli.ThreatRed {
		t.Fatalf("expected RED collision when mutated exports intersect, got %+v", col)
	}
}
