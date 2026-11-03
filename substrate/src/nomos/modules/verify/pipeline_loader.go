// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"sync"
)

var (
	registryMu   sync.RWMutex
	gateRegistry = make(map[string]func() Gate)
)

// RegisterGate registers a gate constructor for a given canonical ID.
func RegisterGate(id string, ctor func() Gate) {
	registryMu.Lock()
	defer registryMu.Unlock()
	gateRegistry[id] = ctor
}

// GateByID retrieves a gate instance by its canonical sensor probe ID.
func GateByID(id string) Gate {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if ctor, exists := gateRegistry[id]; exists {
		return ctor()
	}
	return nil
}

// RegisteredGateCount returns the number of active baseline quality gates.
func RegisteredGateCount() int {
	return len(DefaultBaselineGates())
}

// IDToGateNameMap returns a map from sensor probe ID to canonical display name.
func IDToGateNameMap() map[string]string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m := make(map[string]string, len(gateRegistry))
	for id, ctor := range gateRegistry {
		m[id] = ctor().Name()
	}
	return m
}

// GateNameToIDMap returns a map from canonical display name to sensor probe ID.
func GateNameToIDMap() map[string]string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	m := make(map[string]string, len(gateRegistry))
	for id, ctor := range gateRegistry {
		m[ctor().Name()] = id
	}
	return m
}

// CanonicalGateOrder defines the statutory sequence of the 21 invariant cells' baseline gates.
var CanonicalGateOrder = []string{
	"phase_discipline", "hermetic_worktree", "staged_boundaries", "canonical_singularity",
	"commit_schema", "go_toolchain", "coverage", "tdd",
	"forbidden_literals", "anti_shim", "git_hygiene", "docstrings",
	"complexity", "broken_wire", "secret", "max_file_loc",
	"cognitive_debt", "ecosystem_drift", "boundary", "ip_leak",
	"magic_strings", "magic_string_duplication", "config_immutability", "threshold_integrity",
	"intent_traceability", "intent_parity", "review_receipt", "gate_integrity_proof", "unchecked_errors",
	"dead_code", "intent_steelman", "contract_parity", "ui_anti_mask", "infra_ingress",
	"capbac_lease", "release_receipt", "topology_parity", "zero_alias",
	"zero_filesystem_alias", "plane_separation", "uasset_dependency", "visual_regression",
}

// DefaultBaselineGates returns the canonical sequence of baseline verification gates.
func DefaultBaselineGates() []Gate {
	registryMu.RLock()
	defer registryMu.RUnlock()

	gates := make([]Gate, 0, len(CanonicalGateOrder))
	for _, id := range CanonicalGateOrder {
		if ctor, ok := gateRegistry[id]; ok {
			gates = append(gates, ctor())
		}
	}
	return gates
}

