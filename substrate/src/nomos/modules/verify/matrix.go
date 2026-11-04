// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

// Ring constants represent the three orthogonal defense rings of the Nomos DoD.
const (
	RingSyntactic     = "syntactic"
	RingDynamic       = "dynamic"
	RingCryptographic = "cryptographic"
)

// CanonicalMatrixModel represents the finite 7x3 Invariant Cells of the Nomos DoD (#CanonicalMatrix).
type CanonicalMatrixModel struct {
	Laws  int `json:"laws"`
	Rings int `json:"rings"`
	Cells int `json:"cells"`
}

// CanonicalMatrix is the singleton 7x3 Definition of Done matrix definition.
var CanonicalMatrix = CanonicalMatrixModel{
	Laws:  7,
	Rings: 3,
	Cells: 21,
}

// LawInfo defines the canonical metadata for one of the Seven Laws of Deterministic Engineering.
type LawInfo struct {
	Law         int    `json:"law"`
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
}

// CanonicalLaws lists the Seven Laws of Nomos in statutory order.
var CanonicalLaws = [7]LawInfo{
	{
		Law:         1,
		Name:        "The Dual-Core Inversion Law",
		Domain:      "Governance & Control",
		Description: "Substrate Go core governs state & execution; Tier 1 agent operates as bounded co-processor.",
	},
	{
		Law:         2,
		Name:        "The Non-Verbal Proof Law",
		Domain:      "Epistemology & Verification",
		Description: "Zero trust in conversational claims; correctness requires compilation exit 0 and AST proof.",
	},
	{
		Law:         3,
		Name:        "The Hermetic Worktree Law",
		Domain:      "State & Isolation",
		Description: "Protected trunk branches are physically read-only; mutations occur strictly in isolated worktrees.",
	},
	{
		Law:         4,
		Name:        "The Single Source of Truth Law",
		Domain:      "Contracts & Schemas",
		Description: "Every contract, schema, and config possesses exactly one compiled Go SSoT with byte parity.",
	},
	{
		Law:         5,
		Name:        "The Zero-Hardcoded-Path Law",
		Domain:      "Portability & Environment",
		Description: "Path literals and repository structures strictly prohibited; paths resolve dynamically.",
	},
	{
		Law:         6,
		Name:        "The Canonical Singularity Law",
		Domain:      "Action Space & Surface",
		Description: "Zero aliases, duplicates, or convenience wrappers tolerated in CLI engines or contract schemas.",
	},
	{
		Law:         7,
		Name:        "The Fail-Closed Pre-Flight Law",
		Domain:      "Safety & Budgets",
		Description: "Mutations require explicit declarative configuration and pass mandatory DoD quality gates.",
	},
}

// InvariantCellSpec defines the declarative schema for each of the 21 Invariant Cells in the 7x3 matrix.
type InvariantCellSpec struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Law      int      `json:"law"`
	Ring     string   `json:"ring"`
	Domain   string   `json:"domain"`
	Phase    string   `json:"phase"`
	Enabled  bool     `json:"enabled"`
	Remedy   string   `json:"remedy"`
	ProbeIDs []string `json:"probe_ids"`
}

// LawReport summarizes the verification outcome of a single Law.
type LawReport struct {
	Law            int           `json:"law"`
	Name           string        `json:"name"`
	Domain         string        `json:"domain"`
	Passed         bool          `json:"passed"`
	TotalCells     int           `json:"total_cells"`
	PassedCells    int           `json:"passed_cells"`
	Cells          []*CellReport `json:"cells"`
	DurationMicros int64         `json:"duration_micros"`
}

// CellReport summarizes the evaluation of one of the 21 Invariant Cells.
type CellReport struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Law            int           `json:"law"`
	Ring           string        `json:"ring"`
	Domain         string        `json:"domain"`
	Passed         bool          `json:"passed"`
	Remedy         string        `json:"remedy,omitempty"`
	ProbeCount     int           `json:"probe_count"`
	PassedProbes   int           `json:"passed_probes"`
	Probes         []*GateResult `json:"probes"`
	DurationMicros int64         `json:"duration_micros"`
}

// DefaultBaselineCells returns the canonical 21 Invariant Cells mapping the subordinate sensor probes.
func DefaultBaselineCells() []InvariantCellSpec {
	cells := make([]InvariantCellSpec, 0, 21)
	cells = append(cells, law1Cells()...)
	cells = append(cells, law2Cells()...)
	cells = append(cells, law3Cells()...)
	cells = append(cells, law4Cells()...)
	cells = append(cells, law5Cells()...)
	cells = append(cells, law6Cells()...)
	cells = append(cells, law7Cells()...)
	return cells
}

func law1Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_1_1", Name: "Syntactic Cognitive Budget Invariant", Law: 1, Ring: RingSyntactic,
			Domain: "Governance & Control", Phase: "REVIEW", Enabled: true,
			Remedy:   "reduce cyclomatic complexity, file length, cognitive debt, and eliminate UI anti-mask shims (Law 1)",
			ProbeIDs: []string{"complexity", "max_file_loc", "cognitive_debt", "boundary", "ui_anti_mask"},
		},
		{
			ID: "cell_1_2", Name: "Phase Discipline State Machine Invariant", Law: 1, Ring: RingDynamic,
			Domain: "Governance & Control", Phase: "REVIEW", Enabled: true,
			Remedy:   "verify phase state transitions adhere to IDLE->PLAN->EDIT->REVIEW->SYNC (Law 1)",
			ProbeIDs: []string{"phase_discipline"},
		},
		{
			ID: "cell_1_3", Name: "CapBAC Task Lease Authorization Invariant", Law: 1, Ring: RingCryptographic,
			Domain: "Governance & Control", Phase: "REVIEW", Enabled: true,
			Remedy:   "ensure mutations possess a cryptographically valid active task lease (Law 1/7)",
			ProbeIDs: []string{"capbac_lease"},
		},
	}
}

func law2Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_2_1", Name: "AST Code Veracity Invariant", Law: 2, Ring: RingSyntactic,
			Domain: "Epistemology & Verification", Phase: "REVIEW", Enabled: true,
			Remedy:   "author TDD test pairs, eliminate placeholder shims, document exported symbols, prune dead code, and verify binary asset references (Law 2)",
			ProbeIDs: []string{"tdd", "anti_shim", "docstrings", "dead_code", "uasset_dependency"},
		},
		{
			ID: "cell_2_2", Name: "Polyglot Test Execution & Coverage Invariant", Law: 2, Ring: RingDynamic,
			Domain: "Epistemology & Verification", Phase: "REVIEW", Enabled: true,
			Remedy:   "all unit tests must pass with exit code 0, meet statement coverage thresholds, and satisfy perceptual visual fidelity (Law 2)",
			ProbeIDs: []string{"go_toolchain", "coverage", "visual_regression"},
		},
		{
			ID: "cell_2_3", Name: "Gate Mathematical Integrity & 2PC Receipt Invariant", Law: 2, Ring: RingCryptographic,
			Domain: "Epistemology & Verification", Phase: "REVIEW", Enabled: true,
			Remedy:   "verify mathematical gate mutation integrity (100% mutant kill, 0% FPR) and 2PC walkthrough receipt (Law 2)",
			ProbeIDs: []string{"review_receipt", "gate_integrity_proof"},
		},
	}
}

func law3Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_3_1", Name: "Worktree Boundaries Invariant", Law: 3, Ring: RingSyntactic,
			Domain: "State & Isolation", Phase: "REVIEW", Enabled: true,
			Remedy:   "unstage files outside active task worktree boundary (Law 3)",
			ProbeIDs: []string{"staged_boundaries"},
		},
		{
			ID: "cell_3_2", Name: "Hermetic Worktree & Git Subprocess Invariant", Law: 3, Ring: RingDynamic,
			Domain: "State & Isolation", Phase: "REVIEW", Enabled: true,
			Remedy:   "execute mutations strictly in isolated worktrees and route git commands through SafeGitCommand (Law 3)",
			ProbeIDs: []string{"hermetic_worktree", "git_hygiene", "ecosystem_drift"},
		},
		{
			ID: "cell_3_3", Name: "Two-Phase Commit Release Receipt Invariant", Law: 3, Ring: RingCryptographic,
			Domain: "State & Isolation", Phase: "REVIEW", Enabled: true,
			Remedy:   "release commits must be attested in Git Notes (refs/notes/nomos/releases) via 2PC sync (Law 3)",
			ProbeIDs: []string{"release_receipt"},
		},
	}
}

func law4Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_4_1", Name: "SSoT Broken Link & Traceability Invariant", Law: 4, Ring: RingSyntactic,
			Domain: "Contracts & Schemas", Phase: "REVIEW", Enabled: true,
			Remedy:   "fix broken links, validate Intent frontmatter, and tag exported symbols with valid Go AST/spec anchors (Law 4)",
			ProbeIDs: []string{"broken_wire", "intent_traceability", "intent_steelman"},
		},
		{
			ID: "cell_4_2", Name: "Contract Schema Integrity Invariant", Law: 4, Ring: RingDynamic,
			Domain: "Contracts & Schemas", Phase: "REVIEW", Enabled: true,
			Remedy:   "synchronize frontend TypeScript DTOs and Zod schemas with compiled Go structs (Law 4)",
			ProbeIDs: []string{"contract_parity"},
		},
		{
			ID: "cell_4_3", Name: "Intent Substrate Byte Parity Invariant", Law: 4, Ring: RingCryptographic,
			Domain: "Contracts & Schemas", Phase: "REVIEW", Enabled: true,
			Remedy:   "ensure perfect byte parity between intent schemas, rules, directives, and embedded Go assets (Law 4)",
			ProbeIDs: []string{"intent_parity"},
		},
	}
}

func law5Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_5_1", Name: "Portability & Forbidden Literals Invariant", Law: 5, Ring: RingSyntactic,
			Domain: "Portability & Environment", Phase: "REVIEW", Enabled: true,
			Remedy:   "eliminate hardcoded workstation paths, proprietary IP leaks, and duplicate magic strings (Law 5)",
			ProbeIDs: []string{"forbidden_literals", "ip_leak", "magic_strings", "magic_string_duplication"},
		},
		{
			ID: "cell_5_2", Name: "Dynamic Ingress & Port Ingress Invariant", Law: 5, Ring: RingDynamic,
			Domain: "Portability & Environment", Phase: "REVIEW", Enabled: true,
			Remedy:   "declare all environment variables in .env.example and synchronize reverse proxy ingress routes (Law 5)",
			ProbeIDs: []string{"infra_ingress"},
		},
		{
			ID: "cell_5_3", Name: "Multi-Seat Topology Parity Invariant", Law: 5, Ring: RingCryptographic,
			Domain: "Portability & Environment", Phase: "REVIEW", Enabled: true,
			Remedy:   "synchronize multi-seat environment configuration and daemon port topology (Law 5)",
			ProbeIDs: []string{"topology_parity"},
		},
	}
}

func law6Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_6_1", Name: "Commit Schema Invariant", Law: 6, Ring: RingSyntactic,
			Domain: "Action Space & Surface", Phase: "REVIEW", Enabled: true,
			Remedy:   "format commit messages as [<KEY>] <type>(<scope>): <summary> with flowing rationale prose and Task: <KEY> trailer (Law 6)",
			ProbeIDs: []string{"commit_schema"},
		},
		{
			ID: "cell_6_2", Name: "CLI Zero-Alias & Deprecation Invariant", Law: 6, Ring: RingDynamic,
			Domain: "Action Space & Surface", Phase: "REVIEW", Enabled: true,
			Remedy:   "eliminate command aliases, convenience wrappers, and deprecated command invocations (Law 6)",
			ProbeIDs: []string{"zero_alias", "zero_filesystem_alias"},
		},
		{
			ID: "cell_6_3", Name: "Canonical Key Singularity Invariant", Law: 6, Ring: RingCryptographic,
			Domain: "Action Space & Surface", Phase: "REVIEW", Enabled: true,
			Remedy:   "ensure all task keys strictly adhere to INT-<N> or SUB-<N> without custom prefixes (Law 6)",
			ProbeIDs: []string{"canonical_singularity"},
		},
	}
}

func law7Cells() []InvariantCellSpec {
	return []InvariantCellSpec{
		{
			ID: "cell_7_1", Name: "Configuration Immutability & Secret Leak Invariant", Law: 7, Ring: RingSyntactic,
			Domain: "Safety & Budgets", Phase: "REVIEW", Enabled: true,
			Remedy:   "keep nomos.yaml immutable under substrate tasks and eliminate committed credentials or keys (Law 7)",
			ProbeIDs: []string{"secret", "config_immutability", "plane_separation"},
		},
		{
			ID: "cell_7_2", Name: "Unchecked Error Hygiene Invariant", Law: 7, Ring: RingDynamic,
			Domain: "Safety & Budgets", Phase: "REVIEW", Enabled: true,
			Remedy:   "explicitly handle and propagate all returned errors fail-closed (Law 7)",
			ProbeIDs: []string{"unchecked_errors"},
		},
		{
			ID: "cell_7_3", Name: "Threshold Integrity Invariant", Law: 7, Ring: RingCryptographic,
			Domain: "Safety & Budgets", Phase: "REVIEW", Enabled: true,
			Remedy:   "do not tamper with or lower gate verification thresholds in configuration (Law 7)",
			ProbeIDs: []string{"threshold_integrity"},
		},
	}
}
