package verify

import (
	"testing"
)

func TestGateRegistry(t *testing.T) {
	gate := GateByID("anti_shim")
	if gate == nil {
		t.Fatalf("expected anti_shim gate to be registered")
	}
	if gate.Name() != "Anti-Shim AST Invariant" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}

	nameMap := IDToGateNameMap()
	if nameMap["anti_shim"] != "Anti-Shim AST Invariant" {
		t.Errorf("expected anti_shim in name map")
	}

	idMap := GateNameToIDMap()
	if idMap["Anti-Shim AST Invariant"] != "anti_shim" {
		t.Errorf("expected Anti-Shim AST Invariant in id map")
	}
}

func TestDefaultBaselineGates(t *testing.T) {
	gates := DefaultBaselineGates()
	if len(gates) != 42 {
		t.Fatalf("expected 42 baseline gates to be returned, got %d", len(gates))
	}
}
