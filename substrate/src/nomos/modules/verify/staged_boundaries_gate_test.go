package verify

import (
	"testing"
)

func TestStagedBoundariesGate_Name(t *testing.T) {
	gate := &StagedBoundariesGate{}
	if gate.Name() != "Worktree Boundaries Invariant (Law 3)" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}
}

func TestStagedBoundariesGate_EvaluateClean(t *testing.T) {
	gate := &StagedBoundariesGate{}
	ctx := &VerifyContext{
		RepoRoot:     t.TempDir(),
		WorktreePath: t.TempDir(),
	}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.Passed {
		t.Errorf("expected clean evaluate to pass, got: %+v", res)
	}
}
