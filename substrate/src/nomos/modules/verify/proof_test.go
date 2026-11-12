package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"testing"
)

func TestEvaluateMutationSensitivity(t *testing.T) {
	killed, total, err := EvaluateMutationSensitivity("", nil)
	if err != nil {
		t.Fatalf("unexpected mutation proof error: %v", err)
	}
	if killed != total {
		t.Errorf("expected 100%% mutants killed, got %d/%d", killed, total)
	}
}

func TestEvaluateMetamorphicSpecificity(t *testing.T) {
	passed, total, err := EvaluateMetamorphicSpecificity(nil)
	if err != nil {
		t.Fatalf("unexpected specificity proof error: %v", err)
	}
	if passed != total {
		t.Errorf("expected 100%% clean constructs to pass, got %d/%d", passed, total)
	}
}

func TestEvaluateNonVacuousGates(t *testing.T) {
	nonVac, total, err := EvaluateNonVacuousGates("")
	if err != nil {
		t.Fatalf("unexpected non-vacuous gates audit error: %v", err)
	}
	if total == 0 {
		t.Fatalf("expected to discover gate Evaluate functions, got 0")
	}
	if nonVac != total {
		t.Errorf("expected all gates to be non-vacuous, got %d/%d", nonVac, total)
	}
}

func TestEvaluateGateIntegrityAttestation(t *testing.T) {
	report, err := EvaluateGateIntegrityAttestation("")
	if err != nil {
		t.Fatalf("unexpected proof evaluation error: %v", err)
	}
	if !report.Passed {
		t.Errorf("expected gate integrity attestation to pass, got: %+v", report)
	}
	if report.MutationScore < 100.0 {
		t.Errorf("expected mutation score 100.0%%, got %.1f%%", report.MutationScore)
	}
	if report.FalsePositiveRate > 0.0 {
		t.Errorf("expected false positive rate 0.0%%, got %.1f%%", report.FalsePositiveRate)
	}
}

func TestGateIntegrityProofGate(t *testing.T) {
	gate := &GateIntegrityProofGate{}
	if gate.Name() != "Gate Integrity & Invariant Attestation (Law 2)" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}

	ctx := &vscope.VerifyContext{RepoRoot: ""}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected gate evaluation error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected gate to pass, got message: %s", res.Message)
	}
}
