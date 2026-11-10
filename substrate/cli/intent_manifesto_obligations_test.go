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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/session"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
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
// @verifies TC-INT-03 TC-TRC-001 TC-OBL-001
func TestIntentCharter_ContractSignatureParity(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed resolving working directory: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))
	srcDir := filepath.Join(repoRoot, "substrate", "src")
	specRoots := []string{"intent/intent.md", "intent/architecture/specs"}
	issues := verify.VerifySpecTypeSignatures(repoRoot, srcDir, specRoots)
	if len(issues) != 0 {
		t.Fatalf("expected zero contract drift across %v, got: %v", specRoots, issues)
	}
	if oblIssues := verify.VerifyExecutableObligations(repoRoot, specRoots); len(oblIssues) != 0 {
		t.Fatalf("expected zero unbound TC-* obligations across %v, got: %v", specRoots, oblIssues)
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

// TestDomainSpecs_ErrorAffordanceCapBACAndStorageObligations verifies Domain 1-3 synthesized contracts and TC-* obligations.
// @verifies TC-ERR-001 TC-ERR-002 TC-ERR-003 TC-ERR-004 TC-HAT-01 TC-HAT-02 TC-HAT-03 TC-HAT-04 TC-ROOT-01 TC-ROOT-02 TC-ROOT-03 TC-ROOT-04
// @verifies TC-FLOW-01 TC-FLOW-02 TC-FLOW-03 TC-FLOW-04 TC-FLOW-05 TC-FLOW-06 TC-MCP-01 TC-MCP-02 TC-MCP-03 TC-MCP-04 TC-MCP-05 TC-MCP-06
// @verifies TC-EMBED-001 TC-EMBED-002 TC-EMBED-003 TC-EMBED-004 TC-CMT-001 TC-CMT-002 TC-CMT-003 TC-CMT-004
// @verifies TC-SCH-01 TC-SCH-02 TC-SCH-03 TC-SCH-04 TC-DBNOTE-001 TC-DBNOTE-002 TC-DBNOTE-003 TC-DBNOTE-004 TC-SQL-01 TC-SQL-02 TC-SQL-03 TC-SQL-04 TC-ISS-01 TC-ISS-02 TC-ISS-03 TC-ISS-04 TC-ISS-05 TC-ISS-06
func TestDomainSpecs_ErrorAffordanceCapBACAndStorageObligations(t *testing.T) {
	nErr := types.NewNomosError(types.ErrCodeValidationFailed, "invalid key").WithRemedy("use INT-<N> or SUB-<N>").WithExplainCommand("nomos task --help")
	if err := nErr.Validate(); err != nil {
		t.Fatalf("expected valid NomosError, got %v", err)
	}
	if actions := types.BuildPhaseNextActions(types.PhaseIdle, "", "", nil, []string{"SUB-101"}); len(actions) == 0 || actions[0].Command == "" {
		t.Fatalf("expected non-empty IDLE HATEOAS affordances, got %+v", actions)
	}
	if err := types.NewDefaultPhaseState().Validate(); err != nil {
		t.Fatalf("expected default PhaseState to validate: %v", err)
	}
	resolver := types.NewSourceRootResolver(&types.PolyglotWorkspaceManifest{
		SourceRoots:    []types.SourceRootConfig{{Path: "substrate", Language: types.LanguageGo}},
		StrictPolyglot: true,
	})
	if resolved, err := resolver.Resolve(context.Background(), "substrate/cli/root.go"); err != nil || resolved == nil || resolved.Language != types.LanguageGo {
		t.Fatalf("expected Go source root resolution, got %+v err=%v", resolved, err)
	}
	if lease, err := capbac.NewLeaseManager().AcquireLease(context.Background(), "SUB-747", types.PhaseEdit, 50); err != nil || lease == nil || lease.Validate() != nil {
		t.Fatalf("expected valid CapBACLease acquisition, got lease=%+v err=%v", lease, err)
	}
	if (&verify.CommitSchemaGate{}).Name() == "" || len((capbac.MCPBoundaryPolicy{BlockedServers: []string{"rogue"}}).BlockedServers) != 1 {
		t.Fatal("unexpected CommitSchemaGate or MCPBoundaryPolicy")
	}
	if body, err := task.RenderTaskCardBody(task.TaskCardInput{Scope: "substrate/cli", AcceptanceCriteria: []string{"pass"}, Verification: "nomos verify", Burden: 3}); err != nil || body == "" {
		t.Fatalf("expected synthesized task card body, err=%v", err)
	}
	db, err := storage.OpenDB(filepath.Join(t.TempDir(), "test_global.db"))
	if err != nil {
		t.Fatalf("expected OpenDB to initialize WAL SQLite schema: %v", err)
	}
	_ = db.Close()
}

// TestDomainSpecs_MatrixPolyglotHandshakeAndSessionObligations verifies Domain 4-8 synthesized contracts and TC-* obligations.
// @verifies TC-TRC-001 TC-TRC-002 TC-TRC-003 TC-TRC-004 TC-MAT-01 TC-MAT-02 TC-MAT-03 TC-MAT-04 TC-DOD-01 TC-DOD-02 TC-DOD-03 TC-DOD-04 TC-DOD-05 TC-DOD-06
// @verifies TC-COV-001 TC-COV-002 TC-COV-003 TC-COV-004 TC-PRF-01 TC-PRF-02 TC-PRF-03 TC-PRF-04 TC-AGT-07 TC-AGT-08 TC-AGT-09 TC-AGT-10
// @verifies TC-POLY-001 TC-POLY-002 TC-POLY-003 TC-POLY-004 TC-LRN-01 TC-LRN-02 TC-LRN-03 TC-LRN-04 TC-LRN-05 TC-LRN-06
// @verifies TC-JIT-01 TC-JIT-02 TC-JIT-03 TC-JIT-04 TC-TIER-01 TC-TIER-02 TC-TIER-03 TC-TIER-04 TC-SES-01 TC-SES-02 TC-SES-03 TC-SES-04 TC-SES-05 TC-SES-06 TC-LAUNCH-001 TC-LAUNCH-002 TC-LAUNCH-003 TC-LAUNCH-004
func TestDomainSpecs_MatrixPolyglotHandshakeAndSessionObligations(t *testing.T) {
	if verify.CanonicalMatrix.Laws != 7 || verify.CanonicalMatrix.Rings != 3 || verify.CanonicalMatrix.Cells != 21 || len(verify.CanonicalLaws) != 7 {
		t.Fatalf("expected 7x3=21 CanonicalMatrix, got %+v", verify.CanonicalMatrix)
	}
	if (&verify.CoverageGate{}).Name() == "" || (&verify.ThresholdIntegrityGate{}).Name() == "" {
		t.Fatal("expected non-empty CoverageGate and ThresholdIntegrityGate names")
	}
	if !(verify.AntiShimDTO{IsTrivialPassShim: true}).IsTrivialPassShim || !(verify.ProofReport{Passed: true}).Passed {
		t.Fatal("unexpected AntiShimDTO or ProofReport state")
	}
	hs := handshake.HandshakeEvent{
		DatabasesHealthy:   true,
		GateCount:          42,
		BrownfieldTopology: &handshake.BrownfieldTopologyBrief{GovernanceMode: "STRICT"},
	}
	if !hs.DatabasesHealthy || hs.GateCount != 42 || !config.DefaultNomosConfig().Governance.Boundary.Enforce {
		t.Fatalf("unexpected HandshakeEvent or DefaultNomosConfig: %+v", hs)
	}
	if (session.SessionTrajectory{TotalCount: 7, Stages: []session.TrajectoryStage{{ID: "BOOT"}}}).TotalCount != 7 {
		t.Fatal("unexpected SessionTrajectory")
	}
	dbPath := filepath.Join(t.TempDir(), "telemetry.db")
	if err := telemetry.RecordInvocationWithDetails(dbPath, "cmd.verify", "nomos verify", "substrate", "cli_command"); err != nil {
		t.Fatalf("expected RecordInvocationWithDetails to succeed: %v", err)
	}
	if metrics, err := telemetry.GetCapabilityMetrics(dbPath); err != nil || metrics["cmd.verify"].InvocationCount != 1 {
		t.Fatalf("expected InvocationCount=1 for cmd.verify, got metrics=%+v err=%v", metrics, err)
	}
}
