---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-16"
last_updated: "2026-11-04"
lineage:
  - INT-610
  - INT-614
  - INT-917
plane: intent
ssot_repository: nomos
status: active
title: "The Seven Laws of Deterministic Engineering and the 7x3 Canonical Gate Matrix Specification"
type: spec
version: "4.0.0"
---

# The Seven Laws of Deterministic Engineering & The 7x3 Canonical Gate Matrix Specification

---

## 1. Executive Summary & Problem Statement

Prior specifications treated Definition of Done (DoD) quality gates as an open-ended checklist growing sequentially from 12 to 29, 33, and beyond. As new sensors and polyglot requirements (Go, TypeScript, Python, Svelte) emerged, teams reflexively introduced new sequential gate numbers.

This open-ended sprawl violates fundamental architectural principles:
1. **Unbounded Complexity (Violation of Law 1)**: An open-ended gate suite prevents static reasoning about system completeness and defense depth.
2. **Taxonomic Ambiguity**: Sequential integers obscured whether a probe verified syntactic hygiene, dynamic execution, or cryptographic provenance.
3. **Fluctuating Documentation**: Downstream specifications, reference guides, and interactive labs drifted out of sync as gate integers incremented.

This specification closes the gate suite algebraically. Every verification predicate in Nomos registers as a sensor bound to a coordinate $(L_i, R_j)$ in a finite $7 \times 3$ matrix.

---

## 2. Axiomatic Invariants & Governing Laws

### `INV-CANON-001`: Finite Matrix Closure
The Nomos DoD verification suite contains 21 Invariant Cells from the $7 \times 3$ Cartesian product of 7 Laws and 3 Rings. Zero gates exist outside this matrix.

### `INV-CANON-002`: Explicit Cell Binding
Every quality gate specification registered in `matrix.go` (`DefaultBaselineCells`) and `pipeline_loader.go` declares three mandatory coordinates:
1. `law`: An integer in $[1, 7]$.
2. `ring`: An enumeration in `{"syntactic", "dynamic", "cryptographic"}`.
3. `domain`: The canonical domain string of the corresponding Law.
Missing canonical cell bindings trigger immediate pipeline rejection under Law 7.

### `INV-CANON-003`: Bilateral Parity Governance
The Universal Contract & Schema Parity Gate implements **Cell (4, 2)** [Law 4: SSoT $\times$ Ring 2: Dynamic Execution] in open-core under the Open-Core Indirect-Control Boundary. It verifies `nomos.schema.json` reflection parity and engine grounding at pre-flight and pre-release. Sovereign extensions (`gsi-sov`) own cross-hemisphere multi-tier AST isomorphism independently.

### `INV-CANON-004`: Matrix Completeness & Multi-Sensor Cells
A canonical invariant cell $\mathcal{C}(L_i, R_j)$ holds zero, one, or multiple sensor drivers (such as Go and TypeScript AST parsers in `Cell 2.1`). Zero active sensors mark a cell *sparse* yet valid. Every active sensor registers into exactly one canonical cell.

---

## 3. The Three Orthogonal Defense Rings

The architecture partitions system verification across three orthogonal defense rings:

```
                              THE 7 LAWS OF NOMOS
                                       │
      ┌────────────────────────────────┼────────────────────────────────┐
      ▼                                ▼                                ▼
Ring 1: SYNTACTIC                Ring 2: DYNAMIC                 Ring 3: CRYPTOGRAPHIC
(AST & Static Structure)         (Runtime & State Machines)      (Receipts & Attestations)
```

1. **Ring 1: Syntactic AST (Static Space)**:
   - Evaluates static code properties, file boundaries, Tree-sitter abstract syntax trees, and declarative schemas without executing subprocesses or mutating runtime state.
   - *Time Budget*: $< 100\text{ms}$.
2. **Ring 2: Dynamic Execution (Runtime Space)**:
   - Evaluates active runtime behaviors, state machine transitions, compilation exit codes, verification execution suites, and live bilateral drift transduction.
   - *Time Budget*: $< 5\text{s}$.
3. **Ring 3: Cryptographic Provenance (Attestation Space)**:
   - Evaluates non-repudiable cryptographic artifacts, SHA-256 byte parity, CapBAC capability leases, `refs/notes` release receipts, and mathematical gate mutation proofs.
   - *Time Budget*: $< 500\text{ms}$.

---

## 4. The 7x3 Canonical Gate Matrix (The 21 Invariant Cells)

The Cartesian product defines the canonical gate suite:
$$\mathcal{M}_{7 \times 3} = \{ \mathcal{C}(L_i, R_j) \mid i \in [1, 7], j \in [1, 3] \}$$

| Law # | Law & Domain | Ring 1: Syntactic AST Gate | Ring 2: Dynamic Execution Gate | Ring 3: Cryptographic Provenance Gate |
| :---: | :--- | :--- | :--- | :--- |
| **1** | **Dual-Core Inversion** *(Governance & Control)* | **Cell (1, 1)**: Cognitive Debt AST (`CDS <= 45`, `cyclomatic <= 15`, `LOC <= 60`) | **Cell (1, 2)**: Phase Discipline State Machine Transition Validation | **Cell (1, 3)**: CapBAC Active Task Lease Authorization Verification |
| **2** | **Non-Verbal Proof** *(Epistemology & Verification)* | **Cell (2, 1)**: AST Anti-Shim, Dead-Code & Docstring Density Scanner | **Cell (2, 2)**: Polyglot Verification & Coverage (`go test ./...`, `cov >= 55%`) | **Cell (2, 3)**: Gate Integrity & 2PC Receipt Proof (`100%` kill, `0%` FPR) |
| **3** | **Hermetic Worktree** *(State & Isolation)* | **Cell (3, 1)**: Worktree Boundary AST & Staged Path Invariant Scanner | **Cell (3, 2)**: Safe Git Subprocess Isolation (`workspace.SafeGitCommand`) | **Cell (3, 3)**: Two-Phase Commit (2PC) Release Certificate in Git Notes (`refs/notes/nomos/releases`) |
| **4** | **Single Source of Truth** *(Contracts & Schemas)* | **Cell (4, 1)**: Markdown Broken Link, Frontmatter & AST Traceability Gate | **Cell (4, 2)**: Universal Contract & Schema Parity Gate (`nomos.schema.json`, $\Delta(t) = 0$) | **Cell (4, 3)**: Cryptographic SHA-256 Byte-Parity Audit Gate (`AGENTS.md`, ambient hooks, directives) |
| **5** | **Zero-Hardcoded-Path** *(Portability & Environment)* | **Cell (5, 1)**: AST Path Literal, IP Leak & Magic String Invariant Scanner | **Cell (5, 2)**: Dynamic Workspace Discovery & Environment Ingress Engine | **Cell (5, 3)**: Multi-Seat Topology & Port Ingress Cryptographic Audit |
| **6** | **Canonical Singularity** *(Action Space & Surface)* | **Cell (6, 1)**: Commit Schema AST Parser & Validator (`commit_schema`) | **Cell (6, 2)**: CLI Zero-Alias & Filesystem Alias Engine (`zero_alias`, `zero_filesystem_alias`) | **Cell (6, 3)**: Pure 2-Plane Key Singularity Attestation (`INT-*` / `SUB-*` strictly enforced) |
| **7** | **Fail-Closed Pre-Flight** *(Safety & Budgets)* | **Cell (7, 1)**: Configuration Immutability, Secret & Plane Separation Scanner | **Cell (7, 2)**: Unchecked Error Hygiene & Resource Breaker Gate | **Cell (7, 3)**: Threshold Integrity & Signed Release Readiness Attestation |

---

## 5. Polyglot Extensibility via Driver Polymorphism

Algebraic closure governs the $7 \times 3$ matrix:
$$\forall \text{ Language } \Lambda \in \{\text{Go}, \text{TypeScript}, \text{Python}, \text{Svelte}, \text{Rust}\}, \quad \text{Gates}(\Lambda) \subseteq \mathcal{M}_{7 \times 3}$$

Adding a new language to Nomos follows strict registration rules:
1. Engineers **never** allocate sequential flat gate integers (`34`, `35`, or $N+1$) outside the closed 21-cell matrix.
2. The language registers a pluggable driver within the corresponding canonical cell:
   - TypeScript/Svelte Tree-sitter queries register inside **Cell (2, 1)** [Syntactic Anti-Shim] and **Cell (4, 1)** [Syntactic Broken Wire].
   - Vitest or Pytest runners register inside **Cell (2, 2)** [Dynamic Execution].
   - Frontend DTO contracts register inside **Cell (4, 2)** [Bilateral Parity].
3. The total invariant cell count remains closed at 21 across all 42 automated DoD sensor probes.

---

## 6. Formal Go SSoT Contract (`substrate/src/nomos/modules/verify/matrix.go`)

The compiled Go matrix registry in `substrate/src/nomos/modules/verify/matrix.go` defines:

```go
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

// LawInfo defines the canonical metadata for one of the Seven Laws of Deterministic Engineering.
type LawInfo struct {
	Law         int    `json:"law"`
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
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

// DefaultBaselineCells returns the canonical 21 Invariant Cells mapping the subordinate sensor probes.
func DefaultBaselineCells() []InvariantCellSpec
```

---

## 7. Table-Driven Verification Vectors

| Vector ID | Invariant Tested | Input Registration | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-MAT-01` | `INV-CANON-001` (Closure) | Gate registered with `law: 8` or `ring: "custom"` | `FAIL` (Pipeline schema rejection) |
| `TC-MAT-02` | `INV-CANON-002` (Explicit Binding) | Sensor probe omits `law`, `ring`, or `remedy` field | `FAIL` (Fail-closed loader error) |
| `TC-MAT-03` | `INV-CANON-003` (Bilateral Parity) | Schema or embedded asset SHA-256 drift ($\Delta(t) > 0$) | `FAIL` (`Cell (4, 2)` / `Cell (4, 3)` violation) |
| `TC-MAT-04` | `INV-CANON-004` (Driver Polymorphism) | Go and WASM Tree-sitter probes bound to `Cell (2, 1)` | `PASS` (`21/21` cells, `42/42` probes satisfied) |

---

## 8. Cross-Plane Traceability & DoD Verification

1. **Compiled Matrix SSoT**: `substrate/src/nomos/modules/verify/matrix.go` models the 21 cells (`DefaultBaselineCells`) and validates gate registrations.
2. **Substrate Sensor Mapping**: `substrate/src/nomos/modules/verify/pipeline_loader.go` binds all 42 Go sensor probes to `(Law, Ring)` coordinates.
3. **Visualizer & Proof Rendering**: `substrate/cli/cmd_verify.go` segments gate output into the $7 \times 3$ matrix and executes `Cell 2.3 (gate_integrity_proof)`.
4. **Deterministic Verification**: Run `nomos verify` to evaluate `Cell 4.1 (intent_traceability)`, `Cell 4.2 (contract_parity)`, and `Cell 4.3 (intent_parity)` with exit code `0`.
