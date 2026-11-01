// Package schemas defines the formal CUE contract for Contract Schema Verification & Anti-Mask Drift.
// Governing Law: Law 4 (SSoT) and Law 5 (Zero-Hardcoded-Path).
package schemas

import "strings"

// #ContractsBoundaryConfig captures the configurable cross-hemisphere contracts declared in nomos.yaml.
#ContractsBoundaryConfig: close({
	contracts:     #ContractVerificationConfig
	membrane?:     #MembraneVerificationConfig
	infra:         #InfraVerificationConfig
	auto_promote?: bool
})

// #ContractVerificationConfig governs Intent (CUE/Specs) <-> Substrate Core (Backend DTOs) schema integrity.
#ContractVerificationConfig: close({
	enabled:       bool | *true
	spec_roots:    [...(string & strings.MinRunes(1))] & [_, ...]
	cue_roots:     [...(string & strings.MinRunes(1))] & [_, ...]
	backend_roots: [...(string & strings.MinRunes(1))] & [_, ...]
	strict_types:  bool | *true
})

// #MembraneVerificationConfig governs Substrate Core (Backend DTOs) <-> Membrane (UI Components) integrity.
#MembraneVerificationConfig: close({
	enabled:                bool | *true
	backend_dto_roots:      [...(string & strings.MinRunes(1))] & [_, ...]
	frontend_roots:         [...(string & strings.MinRunes(1))] & [_, ...]
	banish_defensive_masks: bool | *true
	fail_on_contract_drift: bool | *true
})

// #InfraVerificationConfig governs Substrate Core (Port/Env Bindings) <-> Infra (Nginx, Caddy, Nix, Env) integrity.
#InfraVerificationConfig: close({
	enabled:            bool | *true
	ingress_configs:    [...(string & strings.MinRunes(1))] & [_, ...]
	env_declarations:   [...(string & strings.MinRunes(1))] & [_, ...]
	port_scan:          bool | *true
	fail_on_orphan_env: bool | *true
})
