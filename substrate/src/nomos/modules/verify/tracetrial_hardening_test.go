package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

type panicProbeGate struct {
	reason string
}

func (g *panicProbeGate) Name() string { return "Simulated Panic Probe Gate" }

func (g *panicProbeGate) Evaluate(_ *vscope.VerifyContext) (*vscope.GateResult, error) {
	panic(g.reason)
}

func TestTimedEvaluate_AndPipeline_RecoverGatePanicFailClosed(t *testing.T) {
	res, err := vscope.TimedEvaluate(nil, &vscope.VerifyContext{RepoRoot: t.TempDir()})
	if err == nil || res != nil {
		t.Fatalf("expected nil gate evaluation to fail closed, got res=%v err=%v", res, err)
	}

	probe := &panicProbeGate{reason: "simulated AST sensor crash"}
	res, err = vscope.TimedEvaluate(probe, &vscope.VerifyContext{RepoRoot: t.TempDir()})
	if err == nil || res != nil {
		t.Fatalf("VEC-TTB-04: expected panicking gate to fail closed with error, got res=%v err=%v", res, err)
	}
	nomosErr, ok := err.(*types.NomosError)
	if !ok || nomosErr.Code != types.ErrCodeCore {
		t.Fatalf("VEC-TTB-04: expected ERR_CORE NomosError from panic recovery, got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "simulated AST sensor crash") {
		t.Fatalf("VEC-TTB-04: expected panic message preserved in error, got: %v", err)
	}

	pipe := NewPipeline([]vscope.Gate{probe})
	rep, pipeErr := pipe.Execute(&vscope.VerifyContext{RepoRoot: t.TempDir(), TaskID: "SUB-99"})
	if pipeErr == nil || rep != nil {
		t.Fatalf("VEC-TTB-04: expected Pipeline.Execute to abort fail-closed on gate panic, got rep=%v err=%v", rep, pipeErr)
	}
}

func TestThresholdIntegrityGate_RejectsWorktreeGovernanceDowngrades(t *testing.T) {
	trunkDir := filepath.Join(t.TempDir(), "trunk")
	wtDir := filepath.Join(t.TempDir(), "worktree-INT-1")
	if err := os.MkdirAll(trunkDir, 0755); err != nil {
		t.Fatalf("mkdir trunk: %v", err)
	}
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("mkdir wt: %v", err)
	}

	strictTrunk := `project: tracetrial-demo
default_branch: main
governance:
  boundary:
    enforce: true
    lock_dependencies: true
  quality_gates:
    strict_mode: true
    thresholds:
      max_cyclomatic_complexity: 15
      max_file_loc: 400
      max_func_loc: 60
      min_test_coverage_pct: 55
`
	if err := os.WriteFile(filepath.Join(trunkDir, "nomos.yaml"), []byte(strictTrunk), 0644); err != nil {
		t.Fatalf("write trunk nomos.yaml: %v", err)
	}

	verifyDowngradeVector(t, trunkDir, wtDir, strings.Replace(strictTrunk, "strict_mode: true", "strict_mode: false", 1), "strict_mode")
	verifyDowngradeVector(t, trunkDir, wtDir, strings.Replace(strictTrunk, "enforce: true", "enforce: false", 1), "boundary.enforce")
	verifyDowngradeVector(t, trunkDir, wtDir, strings.Replace(strictTrunk, "lock_dependencies: true", "lock_dependencies: false", 1), "boundary.lock_dependencies")
	verifyDowngradeVector(t, trunkDir, wtDir, "project: [unclosed yaml\n", "malformed")

	// Brownfield trunk baseline (WorktreePath == "") with strict_mode: false continues to pass
	brownfieldDir := filepath.Join(t.TempDir(), "brownfield")
	_ = os.MkdirAll(brownfieldDir, 0755)
	_ = os.WriteFile(filepath.Join(brownfieldDir, "nomos.yaml"), []byte(strings.Replace(strictTrunk, "strict_mode: true", "strict_mode: false", 1)), 0644)
	gate := &ThresholdIntegrityGate{}
	res, err := gate.Evaluate(&vscope.VerifyContext{RepoRoot: brownfieldDir})
	if err != nil || res == nil || !res.Passed {
		t.Fatalf("expected brownfield trunk baseline strict_mode=false to pass, got res=%+v err=%v", res, err)
	}
}

func verifyDowngradeVector(t *testing.T, trunkDir, wtDir, mutatedYAML, expectedSubstr string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(wtDir, "nomos.yaml"), []byte(mutatedYAML), 0644); err != nil {
		t.Fatalf("write worktree nomos.yaml: %v", err)
	}
	gate := &ThresholdIntegrityGate{}
	res, err := gate.Evaluate(&vscope.VerifyContext{RepoRoot: trunkDir, WorktreePath: wtDir, TaskID: "INT-1"})
	if err != nil {
		t.Fatalf("unexpected error evaluating ThresholdIntegrityGate: %v", err)
	}
	if res == nil || res.Passed || !strings.Contains(res.Message, expectedSubstr) {
		t.Fatalf("VEC-TTB-03: expected ThresholdIntegrityGate to fail closed with %q, got: %+v", expectedSubstr, res)
	}
}
