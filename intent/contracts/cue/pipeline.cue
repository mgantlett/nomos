package schemas

// SensorType defines the available telemetry drivers in the Go substrate.
#SensorType: "git" | "ast" | "test_coverage" | "secret" | "links" | "traceability" | "custom"

// DefenseRing defines the 3 canonical architectural rings of defense.
#DefenseRing: "syntactic" | "dynamic" | "cryptographic"

// CanonicalMatrix models the finite 7x3 Invariant Cells of the Nomos DoD.
#CanonicalMatrix: {
	laws:  7
	rings: 3
	cells: 21
}

// GateSpec defines the declarative schema for an individual sensor probe.
#GateSpec: close({
	id:       string & != "" & =~"^[a-z0-9_-]+$"
	name:     string & != ""
	law:      int & >=1 & <=7
	ring:     #DefenseRing
	domain:   string & != ""
	sensor:   #SensorType
	phase:    "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN" | *"REVIEW"
	enabled:  bool | *true
	remedy:   string & != ""
	rule_key: string | *""
})

// InvariantCell defines the canonical schema for each of the 21 cells in the 7x3 matrix.
#InvariantCell: close({
	id:        string & != "" & =~"^[a-z0-9_-]+$"
	name:      string & != ""
	law:       int & >=1 & <=7
	ring:      #DefenseRing
	domain:    string & != ""
	phase:     "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN" | *"REVIEW"
	enabled:   bool | *true
	remedy:    string & != ""
	probe_ids: [...string]
})

// Pipeline defines the ordered collection of 21 Invariant Cells and their underlying sensor probes.
#Pipeline: close({
	version: string & =~"^\\d+\\.\\d+\\.\\d+$" | *"1.0.0"
	cells?:  [#InvariantCell, ...#InvariantCell]
	gates:   [#GateSpec, ...#GateSpec]
})

// CapabilityDetail defines the single-item drill-down payload returned by 'nomos capabilities <id>' (Law 6).
#CapabilityDetail: close({
	id:          string & != ""
	name:        string & != ""
	type:        "hook" | "directive" | "engine" | "gate" | "cli"
	plane:       "intent" | "substrate"
	status:      string & != ""
	law?:        int & >=1 & <=7
	ring?:       #DefenseRing
	domain?:     string
	invocation:  string
	description: string
	remedy?:     string
	probe_ids?:  [...string]
})

// Default canonical v4.0 7x3 Canonical Invariant DoD Verification Pipeline
pipeline: #Pipeline & {
	version: "1.0.0"
	cells: [
		{
			id:        "cell_1_1"
			name:      "Syntactic Cognitive Budget Invariant"
			law:       1
			ring:      "syntactic"
			domain:    "Governance & Control"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "reduce cyclomatic complexity, file length, cognitive debt, and eliminate UI anti-mask shims (Law 1)"
			probe_ids: ["complexity", "max_file_loc", "cognitive_debt", "boundary", "ui_anti_mask"]
		},
		{
			id:        "cell_1_2"
			name:      "Phase Discipline State Machine Invariant"
			law:       1
			ring:      "dynamic"
			domain:    "Governance & Control"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "verify phase state transitions adhere to IDLE->PLAN->EDIT->REVIEW->SYNC (Law 1)"
			probe_ids: ["phase_discipline"]
		},
		{
			id:        "cell_1_3"
			name:      "CapBAC Task Lease Authorization Invariant"
			law:       1
			ring:      "cryptographic"
			domain:    "Governance & Control"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "ensure mutations possess a cryptographically valid active task lease (Law 1/7)"
			probe_ids: ["capbac_lease"]
		},
		{
			id:        "cell_2_1"
			name:      "AST Code Veracity Invariant"
			law:       2
			ring:      "syntactic"
			domain:    "Epistemology & Verification"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "author TDD test pairs, eliminate placeholder shims, document exported symbols, prune dead code, and verify binary asset references (Law 2)"
			probe_ids: ["tdd", "anti_shim", "docstrings", "dead_code", "uasset_dependency"]
		},
		{
			id:        "cell_2_2"
			name:      "Polyglot Test Execution & Coverage Invariant"
			law:       2
			ring:      "dynamic"
			domain:    "Epistemology & Verification"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "all unit tests must pass with exit code 0, meet statement coverage thresholds, and satisfy perceptual visual fidelity (Law 2)"
			probe_ids: ["go_toolchain", "coverage", "visual_regression"]
		},
		{
			id:        "cell_2_3"
			name:      "Adversarial Critic Attestation Invariant"
			law:       2
			ring:      "cryptographic"
			domain:    "Epistemology & Verification"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "index an authentic adversarial review receipt with score >= 80.0 and verify mathematical gate integrity (Law 2)"
			probe_ids: ["review_receipt", "gate_integrity_proof"]
		},
		{
			id:        "cell_3_1"
			name:      "Worktree Boundaries Invariant"
			law:       3
			ring:      "syntactic"
			domain:    "State & Isolation"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "unstage files outside active task worktree boundary (Law 3)"
			probe_ids: ["staged_boundaries"]
		},
		{
			id:        "cell_3_2"
			name:      "Hermetic Worktree & Git Subprocess Invariant"
			law:       3
			ring:      "dynamic"
			domain:    "State & Isolation"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "execute mutations strictly in isolated worktrees and route git commands through SafeGitCommand (Law 3)"
			probe_ids: ["hermetic_worktree", "git_hygiene", "ecosystem_drift"]
		},
		{
			id:        "cell_3_3"
			name:      "Two-Phase Commit Release Receipt Invariant"
			law:       3
			ring:      "cryptographic"
			domain:    "State & Isolation"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "release commits must be attested in Git Notes (refs/notes/nomos/releases) via 2PC sync (Law 3)"
			probe_ids: ["release_receipt"]
		},
		{
			id:        "cell_4_1"
			name:      "SSoT Broken Link & Traceability Invariant"
			law:       4
			ring:      "syntactic"
			domain:    "Contracts & Schemas"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "fix broken links, validate CUE steelman, and tag exported symbols with valid Intent anchors (Law 4)"
			probe_ids: ["broken_wire", "intent_traceability", "intent_steelman"]
		},
		{
			id:        "cell_4_2"
			name:      "Contract Schema Integrity Invariant"
			law:       4
			ring:      "dynamic"
			domain:    "Contracts & Schemas"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "synchronize frontend TypeScript DTOs and Zod schemas with compiled Go structs (Law 4)"
			probe_ids: ["contract_parity"]
		},
		{
			id:        "cell_4_3"
			name:      "Intent Substrate Byte Parity Invariant"
			law:       4
			ring:      "cryptographic"
			domain:    "Contracts & Schemas"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "ensure perfect byte parity between intent schemas, rules, directives, and embedded Go assets (Law 4)"
			probe_ids: ["intent_parity"]
		},
		{
			id:        "cell_5_1"
			name:      "Portability & Forbidden Literals Invariant"
			law:       5
			ring:      "syntactic"
			domain:    "Portability & Environment"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "eliminate hardcoded workstation paths, proprietary IP leaks, and duplicate magic strings (Law 5)"
			probe_ids: ["forbidden_literals", "ip_leak", "magic_strings", "magic_string_duplication"]
		},
		{
			id:        "cell_5_2"
			name:      "Dynamic Ingress & Port Ingress Invariant"
			law:       5
			ring:      "dynamic"
			domain:    "Portability & Environment"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "declare all environment variables in .env.example and synchronize reverse proxy ingress routes (Law 5)"
			probe_ids: ["infra_ingress"]
		},
		{
			id:        "cell_5_3"
			name:      "Multi-Seat Topology Parity Invariant"
			law:       5
			ring:      "cryptographic"
			domain:    "Portability & Environment"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "synchronize multi-seat environment configuration and daemon port topology (Law 5)"
			probe_ids: ["topology_parity"]
		},
		{
			id:        "cell_6_1"
			name:      "Commit Schema Invariant"
			law:       6
			ring:      "syntactic"
			domain:    "Action Space & Surface"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "format commit messages with canonical dual-layer structure [KEY] type(scope): description (Law 6)"
			probe_ids: ["commit_schema"]
		},
		{
			id:        "cell_6_2"
			name:      "CLI Zero-Alias & Deprecation Invariant"
			law:       6
			ring:      "dynamic"
			domain:    "Action Space & Surface"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "eliminate command aliases, convenience wrappers, and deprecated command invocations (Law 6)"
			probe_ids: ["zero_alias", "zero_filesystem_alias"]
		},
		{
			id:        "cell_6_3"
			name:      "Canonical Key Singularity Invariant"
			law:       6
			ring:      "cryptographic"
			domain:    "Action Space & Surface"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "ensure all task keys strictly adhere to INT-<N> or SUB-<N> without custom prefixes (Law 6)"
			probe_ids: ["canonical_singularity"]
		},
		{
			id:        "cell_7_1"
			name:      "Configuration Immutability & Secret Leak Invariant"
			law:       7
			ring:      "syntactic"
			domain:    "Safety & Budgets"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "keep nomos.yaml immutable under substrate tasks and eliminate committed credentials or keys (Law 7)"
			probe_ids: ["secret", "config_immutability", "plane_separation"]
		},
		{
			id:        "cell_7_2"
			name:      "Unchecked Error Hygiene Invariant"
			law:       7
			ring:      "dynamic"
			domain:    "Safety & Budgets"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "explicitly handle and propagate all returned errors fail-closed (Law 7)"
			probe_ids: ["unchecked_errors"]
		},
		{
			id:        "cell_7_3"
			name:      "Threshold Integrity Invariant"
			law:       7
			ring:      "cryptographic"
			domain:    "Safety & Budgets"
			phase:     "REVIEW"
			enabled:   true
			remedy:    "do not tamper with or lower gate verification thresholds in configuration (Law 7)"
			probe_ids: ["threshold_integrity"]
		},
	]
	gates: [
		{
			id:      "phase_discipline"
			name:    "Phase Discipline Invariant"
			law:     1
			ring:    "dynamic"
			domain:  "Governance & Control"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "verify phase state before proceeding"
		},
		{
			id:      "hermetic_worktree"
			name:    "Hermetic Worktree Invariant"
			law:     3
			ring:    "dynamic"
			domain:  "State & Isolation"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "execute mutations strictly in isolated transient worktrees"
		},
		{
			id:      "staged_boundaries"
			name:    "Worktree Boundaries Invariant"
			law:     3
			ring:    "syntactic"
			domain:  "State & Isolation"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "unstage files outside active worktree boundary"
		},
		{
			id:      "canonical_singularity"
			name:    "Canonical Singularity Invariant"
			law:     6
			ring:    "cryptographic"
			domain:  "Action Space & Surface"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "ensure task keys adhere strictly to INT-<N> or SUB-<N>"
		},
		{
			id:      "commit_schema"
			name:    "Commit Schema Invariant Gate"
			law:     6
			ring:    "syntactic"
			domain:  "Action Space & Surface"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "format commit messages with dual-layer structure"
		},
		{
			id:      "go_toolchain"
			name:    "Go Toolchain Invariant"
			law:     2
			ring:    "dynamic"
			domain:  "Epistemology & Verification"
			sensor:  "test_coverage"
			phase:   "REVIEW"
			enabled: true
			remedy:  "fix failing Go unit tests"
		},
		{
			id:       "coverage"
			name:     "Statement Coverage Invariant"
			law:      2
			ring:     "dynamic"
			domain:   "Epistemology & Verification"
			sensor:   "test_coverage"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "author unit tests to meet statement coverage threshold"
			rule_key: "coverage_violations"
		},
		{
			id:      "tdd"
			name:    "TDD Test Pair Invariant"
			law:     2
			ring:    "syntactic"
			domain:  "Epistemology & Verification"
			sensor:  "test_coverage"
			phase:   "REVIEW"
			enabled: true
			remedy:  "create corresponding _test.go file for all new implementation packages"
		},
		{
			id:      "forbidden_literals"
			name:    "Forbidden Literals Invariant"
			law:     5
			ring:    "syntactic"
			domain:  "Portability & Environment"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "resolve paths dynamically via configuration"
		},
		{
			id:      "anti_shim"
			name:    "Anti-Shim AST Invariant"
			law:     2
			ring:    "syntactic"
			domain:  "Epistemology & Verification"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "implement real algorithmic logic; eliminate panic shims and empty functions"
		},
		{
			id:      "git_hygiene"
			name:    "Git Subprocess Hygiene Invariant"
			law:     3
			ring:    "dynamic"
			domain:  "State & Isolation"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "route git commands through workspace.SafeGitCommand"
		},
		{
			id:       "docstrings"
			name:     "Docstring Density Invariant"
			law:      2
			ring:     "syntactic"
			domain:   "Epistemology & Verification"
			sensor:   "ast"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "add docstring comments to exported functions and types"
			rule_key: "docstring_density_violations"
		},
		{
			id:       "complexity"
			name:     "Cognitive Complexity Invariant"
			law:      1
			ring:     "syntactic"
			domain:   "Governance & Control"
			sensor:   "ast"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "decompose complex functions into modular units"
			rule_key: "complexity_violations"
		},
		{
			id:      "broken_wire"
			name:    "Broken Wire & Link Invariant"
			law:     4
			ring:    "syntactic"
			domain:  "Contracts & Schemas"
			sensor:  "links"
			phase:   "REVIEW"
			enabled: true
			remedy:  "fix broken markdown links and anchor targets"
		},
		{
			id:      "secret"
			name:    "Secret & Credential Leak Invariant"
			law:     7
			ring:    "syntactic"
			domain:  "Safety & Budgets"
			sensor:  "secret"
			phase:   "REVIEW"
			enabled: true
			remedy:  "remove unencrypted credentials or private keys"
		},
		{
			id:       "max_file_loc"
			name:     "Max File LOC Invariant"
			law:      1
			ring:     "syntactic"
			domain:   "Governance & Control"
			sensor:   "ast"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "decompose oversized files into modular sibling files"
			rule_key: "file_loc_violations"
		},
		{
			id:       "cognitive_debt"
			name:     "Cognitive Debt Score Invariant"
			law:      1
			ring:     "syntactic"
			domain:   "Governance & Control"
			sensor:   "ast"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "refactor high cognitive debt source files"
			rule_key: "cds_violations"
		},
		{
			id:      "ecosystem_drift"
			name:    "Ecosystem Cross-Repo Drift Invariant"
			law:     3
			ring:    "dynamic"
			domain:  "State & Isolation"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "sync trunk with remote origin before release"
		},
		{
			id:      "boundary"
			name:    "Open-Core Boundary Invariant"
			law:     1
			ring:    "syntactic"
			domain:  "Governance & Control"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "preserve open-core boundaries"
		},
		{
			id:      "ip_leak"
			name:    "IP Leak Detector Gate"
			law:     5
			ring:    "syntactic"
			domain:  "Portability & Environment"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "remove sovereign IP from open-core codebase"
		},
		{
			id:      "magic_strings"
			name:    "Magic Strings & Topology Hardcoding Gate"
			law:     5
			ring:    "syntactic"
			domain:  "Portability & Environment"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "extract magic strings into constants or config"
		},
		{
			id:       "magic_string_duplication"
			name:     "Magic String Duplication Gate"
			law:      5
			ring:     "syntactic"
			domain:   "Portability & Environment"
			sensor:   "ast"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "deduplicate repeated string literals"
			rule_key: "magic_string_duplication_violations"
		},
		{
			id:      "config_immutability"
			name:    "Intent Config Immutability Invariant"
			law:     7
			ring:    "syntactic"
			domain:  "Safety & Budgets"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "keep nomos.yaml immutable under substrate tasks"
		},
		{
			id:      "threshold_integrity"
			name:    "Threshold Integrity Gate"
			law:     7
			ring:    "cryptographic"
			domain:  "Safety & Budgets"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "do not mutate gate thresholds in configuration"
		},
		{
			id:      "intent_traceability"
			name:    "Intent Traceability Validation Gate"
			law:     4
			ring:    "syntactic"
			domain:  "Contracts & Schemas"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "tag exported functions with @intent anchors"
		},
		{
			id:      "intent_parity"
			name:    "Intent Substrate Parity Gate"
			law:     4
			ring:    "cryptographic"
			domain:  "Contracts & Schemas"
			sensor:  "traceability"
			phase:   "REVIEW"
			enabled: true
			remedy:  "ensure parity between intent rules and substrate embeddings"
		},
		{
			id:      "review_receipt"
			name:    "Adversarial Review Receipt Gate"
			law:     2
			ring:    "cryptographic"
			domain:  "Epistemology & Verification"
			sensor:  "traceability"
			phase:   "REVIEW"
			enabled: true
			remedy:  "index an adversarial review receipt for intent tasks"
		},
		{
			id:      "gate_integrity_proof"
			name:    "Gate Mathematical Integrity Proof"
			law:     2
			ring:    "cryptographic"
			domain:  "Epistemology & Verification"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "ensure all active gates contain non-trivial algorithmic verification logic"
		},
		{
			id:      "unchecked_errors"
			name:    "Unchecked Error Hygiene Gate"
			law:     7
			ring:    "dynamic"
			domain:  "Safety & Budgets"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "explicitly handle all returned error values"
		},
		{
			id:      "dead_code"
			name:    "Dead-Code Reachability Gate"
			law:     2
			ring:    "syntactic"
			domain:  "Epistemology & Verification"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "prune unreachable commands, dead stubs, and dormant symbols"
		},
		{
			id:      "intent_steelman"
			name:    "Intent Steel-Man Parity Gate"
			law:     4
			ring:    "syntactic"
			domain:  "Contracts & Schemas"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "run 'nomos verify' to validate declarative CUE schemas and specification frontmatters"
		},
		{
			id:      "contract_parity"
			name:    "Contract Parity Gate"
			law:     4
			ring:    "dynamic"
			domain:  "Contracts & Schemas"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "align TypeScript DTO interfaces and Zod decoders with compiled Go struct fields"
		},
		{
			id:      "ui_anti_mask"
			name:    "UI Anti-Mask Sentinel Gate"
			law:     1
			ring:    "syntactic"
			domain:  "Governance & Control"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "replace defensive truthiness fallback guards with strict runtime validation or explicit length check on guaranteed arrays (ADR-0066)"
		},
		{
			id:      "infra_ingress"
			name:    "Infra Ingress & Port Gate"
			law:     5
			ring:    "dynamic"
			domain:  "Portability & Environment"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "declare all environment variables in .env.example and synchronize reverse proxy ingress routes (Law 5)"
		},
		{
			id:      "capbac_lease"
			name:    "CapBAC Task Lease Authorization Invariant"
			law:     1
			ring:    "cryptographic"
			domain:  "Governance & Control"
			sensor:  "custom"
			phase:   "REVIEW"
			enabled: true
			remedy:  "ensure mutations possess a cryptographically valid active task lease (Law 1/7)"
		},
		{
			id:      "release_receipt"
			name:    "Two-Phase Commit Release Receipt Invariant"
			law:     3
			ring:    "cryptographic"
			domain:  "State & Isolation"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "release commits must be attested in Git Notes (refs/notes/nomos/releases) via 2PC sync (Law 3)"
		},
		{
			id:      "topology_parity"
			name:    "Multi-Seat Topology Parity Invariant"
			law:     5
			ring:    "cryptographic"
			domain:  "Portability & Environment"
			sensor:  "custom"
			phase:   "REVIEW"
			enabled: true
			remedy:  "synchronize multi-seat environment configuration and daemon port topology (Law 5)"
		},
		{
			id:      "zero_alias"
			name:    "CLI Zero-Alias & Deprecation Invariant"
			law:     6
			ring:    "dynamic"
			domain:  "Action Space & Surface"
			sensor:  "ast"
			phase:   "REVIEW"
			enabled: true
			remedy:  "eliminate command aliases, convenience wrappers, and deprecated command invocations (Law 6)"
		},
		{
			id:       "zero_filesystem_alias"
			name:     "Zero Filesystem Alias Invariant"
			law:      6
			ring:     "dynamic"
			domain:   "Action Space & Surface"
			sensor:   "git"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "remove filesystem symlinks; symlinks are strictly prohibited under Law 6"
			rule_key: "zero_filesystem_alias_violations"
		},
		{
			id:      "plane_separation"
			name:    "Plane Separation Invariant"
			law:     7
			ring:    "syntactic"
			domain:  "Safety & Budgets"
			sensor:  "git"
			phase:   "REVIEW"
			enabled: true
			remedy:  "revert cross-plane mutations and allocate a twin task in the opposite plane (Law 7)"
		},
		{
			id:       "uasset_dependency"
			name:     "UAsset Dependency Invariant (Law 2)"
			law:      2
			ring:     "syntactic"
			domain:   "Epistemology & Verification"
			sensor:   "custom"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "ensure all imported assets exist in Content/ before release (Law 2)"
			rule_key: "uasset_dependency_violations"
		},
		{
			id:       "visual_regression"
			name:     "Visual Regression Invariant (Law 2)"
			law:      2
			ring:     "dynamic"
			domain:   "Epistemology & Verification"
			sensor:   "custom"
			phase:    "REVIEW"
			enabled:  true
			remedy:   "resolve rendering regressions, black screen, or missing textures (Law 2)"
			rule_key: "visual_regression_violations"
		},
	]
}

