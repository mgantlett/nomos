package verify

import (
	"testing"
)

func TestCapBACLeaseGate_Baseline(t *testing.T) {
	gate := &CapBACLeaseGate{}
	if gate.Name() != "CapBAC Task Lease Authorization Invariant" {
		t.Fatalf("unexpected name: %s", gate.Name())
	}

	ctx := &VerifyContext{
		IsBaseline: true,
		TaskID:     "",
		Phase:      "IDLE",
	}

	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass for baseline: %v", res.Message)
	}
}

func TestReleaseReceiptGate(t *testing.T) {
	gate := &ReleaseReceiptGate{}
	if gate.Name() != "Two-Phase Commit Release Receipt Invariant" {
		t.Fatalf("unexpected name: %s", gate.Name())
	}

	ctx := &VerifyContext{
		RepoRoot: ".",
	}

	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass for release receipt gate: %v", res.Message)
	}
}
