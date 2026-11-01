package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntentGatesRegistration(t *testing.T) {
	for _, id := range []string{"intent_traceability", "intent_steelman", "intent_parity", "topology_parity", "threshold_integrity", "uasset_dependency"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestIntentSteelmanGateClean(t *testing.T) {
	g := &IntentSteelmanGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected intent steelman gate to pass: %s", res.Message)
	}
}

func TestTopologyParityGateClean(t *testing.T) {
	g := &TopologyParityGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected topology parity gate to pass: %s", res.Message)
	}
}

func TestThresholdIntegrityGateClean(t *testing.T) {
	g := &ThresholdIntegrityGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected threshold integrity gate to pass: %s", res.Message)
	}
}

func TestUAssetDependencyGateClean(t *testing.T) {
	g := &UAssetDependencyGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected uasset dependency gate to pass: %s", res.Message)
	}
}

func TestIntentParityGate_RejectsLegacyWorkflowsAndHookDrift(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "AGENTS.md"), []byte("# Protocol"), 0644); err != nil {
		t.Fatalf("failed writing AGENTS.md: %v", err)
	}
	legacyDir := filepath.Join(tempDir, ".agents", "workflows")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("failed creating legacy workflows dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "nomos-idle.md"), []byte("# Legacy"), 0644); err != nil {
		t.Fatalf("failed writing legacy playbook: %v", err)
	}

	g := &IntentParityGate{}
	res, err := g.Evaluate(&VerifyContext{WorktreePath: tempDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed || !strings.Contains(res.Message, "deprecated Nomos workflow playbook") {
		t.Errorf("expected failure on legacy workflow playbook, got passed=%v msg=%s", res.Passed, res.Message)
	}
}
