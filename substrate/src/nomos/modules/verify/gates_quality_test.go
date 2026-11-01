package verify

import (
	"testing"
)

func TestQualityGatesRegistration(t *testing.T) {
	for _, id := range []string{"tdd", "coverage", "docstrings", "dead_code", "visual_regression"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestTDDGateClean(t *testing.T) {
	g := &TDDGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected tdd gate to pass: %s", res.Message)
	}
}

func TestDocstringsGateClean(t *testing.T) {
	g := &DocstringDensityGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected docstrings gate to pass: %s", res.Message)
	}
}

func TestVisualRegressionGateClean(t *testing.T) {
	g := &VisualRegressionGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected visual regression gate to pass: %s", res.Message)
	}
}
