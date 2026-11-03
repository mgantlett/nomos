---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-16"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - INT-610
    - INT-614
title: The Seven Laws of Deterministic Engineering and the 7x3 Canonical Gate Matrix Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# The Seven Laws of Deterministic Engineering & The 7x3 Canonical Gate Matrix Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Establish the finite, deterministically closed $7 \times 3$ Canonical Gate Matrix for the Nomos DoD verification pipeline, binding the 7 Laws of Nomos across three orthogonal Defense Rings (Syntactic AST, Dynamic Execution, Cryptographic Provenance) into exactly 21 Invariant Cells.

---

## 1. Problem Statement & Architectural Motivation
Prior to this specification, Definition of Done (DoD) quality gates in Nomos were treated as an open-ended, fluctuating checklist (growing historically from 12 to 29, 33, and beyond). As new sensors and polyglot requirements (Go, TypeScript, Python, Svelte) emerged, teams reflexively introduced new sequential gate numbers.

This open-ended sprawl violates fundamental architectural principles:
1. **Unbounded Complexity (Violation of Law 1)**: An open-ended gate suite prevents static reasoning about system completeness and defense depth.
2. **Taxonomic Ambiguity**: It was unclear whether a check was verifying syntactic source hygiene, dynamic execution behavior, or cryptographic provenance.
3. **Fluctuating Documentation**: Downstream specifications, handbook articles, and academy interactive labs drifted out of sync as gate integers incremented.

This specification closes the gate suite algebraically. Every verification predicate in Nomos is a sensor bound to a coordinate $(L_i, R_j)$ in a finite $7 \times 3$ matrix.

---

## 2. The Three Orthogonal Defense Rings

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
   - Evaluates active runtime behaviors, state machine transitions, compilation exit codes, test execution suites, and live bilateral drift transduction.
   - *Time Budget*: $< 5\text{s}$.
3. **Ring 3: Cryptographic Provenance (Attestation Space)**:
   - Evaluates non-repudiable cryptographic artifacts, SHA-256 byte parity, CapBAC capability leases, git note release receipts, and sovereign adversarial critic attestations.
   - *Time Budget*: $< 500\text{ms}$.

---

## 3. The 7x3 Canonical Gate Matrix (The 21 Invariant Cells)

The canonical gate suite is defined as the Cartesian product:
$$\mathcal{M}_{7 \times 3} = \{ \mathcal{C}(L_i, R_j) \mid i \in [1, 7], j \in [1, 3] \}$$

| Law # | Law & Domain | Ring 1: Syntactic AST Gate | Ring 2: Dynamic Execution Gate | Ring 3: Cryptographic Provenance Gate |
| :---: | :--- | :--- | :--- | :--- |
| **1** | **Dual-Core Inversion** *(Governance & Control)* | **Cell (1, 1)**: Cognitive Debt & Complexity AST (CDS $\le 45$, cyclomatic $\le 15$, LOC $\le 60$) | **Cell (1, 2)**: Phase Discipline State Machine Transition Validation | **Cell (1, 3)**: CapBAC Active Task Lease Authorization Verification |
| **2** | **Non-Verbal Proof** *(Epistemology & Verification)* | **Cell (2, 1)**: AST Anti-Shim, Dead-Code Reachability & Docstring Density Scanner | **Cell (2, 2)**: Polyglot Test Runner & Coverage Engine (`go test ./...` exit 0, cov $\ge 55\%$) | **Cell (2, 3)**: Adversarial Review Receipt & Gate Mathematical Integrity Proof (score $\ge 80.0$) |
| **3** | **Hermetic Worktree** *(State & Isolation)* | **Cell (3, 1)**: Worktree Boundary AST & Staged Path Invariant Scanner | **Cell (3, 2)**: Safe Git Subprocess Isolation (`workspace.SafeGitCommand`) | **Cell (3, 3)**: Two-Phase Commit (2PC) Release Certificate in Git Notes (`refs/notes/nomos/releases`) |
| **4** | **Single Source of Truth** *(Contracts & Schemas)* | **Cell (4, 1)**: Markdown Broken Link, Frontmatter & AST Traceability Gate | **Cell (4, 2)**: Universal Contract & Schema Parity Gate (`nomos.schema.json`, $\Delta(t) = 0$) | **Cell (4, 3)**: Cryptographic SHA-256 Byte-Parity Audit Gate (`AGENTS.md`, ambient hooks, directives) |
| **5** | **Zero-Hardcoded-Path** *(Portability & Environment)* | **Cell (5, 1)**: AST Path Literal, IP Leak & Magic String Invariant Scanner | **Cell (5, 2)**: Dynamic Workspace Discovery & Environment Ingress Engine | **Cell (5, 3)**: Multi-Seat Topology & Port Ingress Cryptographic Audit |
| **6** | **Canonical Singularity** *(Action Space & Surface)* | **Cell (6, 1)**: CLI Zero-Alias AST Parser & Schema Validator | **Cell (6, 2)**: Deprecated Command & Shim Tripwire Engine | **Cell (6, 3)**: Pure 2-Plane Key Singularity Attestation (`INT-*` / `SUB-*` strictly enforced) |
| **7** | **Fail-Closed Pre-Flight** *(Safety & Budgets)* | **Cell (7, 1)**: Configuration Immutability & Contract Boundary Scanner | **Cell (7, 2)**: Unchecked Error Hygiene & Resource Breaker Gate | **Cell (7, 3)**: Threshold Integrity & Signed Release Readiness Attestation |

---

## 4. Polyglot Extensibility via Driver Polymorphism

A critical theorem of the $7 \times 3$ matrix is **algebraic closure**:
$$\forall \text{ Language } \Lambda \in \{\text{Go}, \text{TypeScript}, \text{Python}, \text{Svelte}, \text{Rust}\}, \quad \text{Gates}(\Lambda) \subseteq \mathcal{M}_{7 \times 3}$$

When a new language is introduced to Nomos:
1. We **never** allocate Gate 34, 35, or $N+1$.
2. The language registers a pluggable driver within the corresponding canonical cell:
   - TypeScript/Svelte Tree-sitter queries register inside **Cell (2, 1)** [Syntactic Anti-Shim] and **Cell (4, 1)** [Syntactic Broken Wire].
   - Vitest or Pytest runners register inside **Cell (2, 2)** [Dynamic Execution].
   - Frontend DTO contracts register inside **Cell (4, 2)** [Bilateral Parity].
3. The total invariant cell count remains closed at 21 (enforced by 42 automated DoD sensor probes).

---

---

## 5. Formal Go SSoT Contract (`verify/matrix.go`)

The compiled Go matrix registry in `substrate/src/nomos/modules/verify/matrix.go` defines:

```go
package verify

// DefenseRing defines the 3 canonical architectural rings of defense.
type DefenseRing string

const (
	RingSyntactic     DefenseRing = "syntactic"
	RingDynamic       DefenseRing = "dynamic"
	RingCryptographic DefenseRing = "cryptographic"
)

// InvariantCell defines the canonical schema for each of the 21 cells in the 7x3 matrix.
type InvariantCell struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Law      int         `json:"law"`
	Ring     DefenseRing `json:"ring"`
	Domain   string      `json:"domain"`
	Phase    string      `json:"phase"`
	Enabled  bool        `json:"enabled"`
	Remedy   string      `json:"remedy"`
	ProbeIDs []string    `json:"probe_ids"`
}
```

---

## 6. Invariant Contracts

### `INV-CANON-001`: Finite Matrix Closure
The Nomos DoD verification suite is strictly composed of exactly 21 Invariant Cells corresponding to the $7 \times 3$ Cartesian product of the 7 Laws and the 3 Defense Rings. No gate shall exist outside this matrix.

### `INV-CANON-002`: Explicit Cell Binding
Every quality gate specification registered in `matrix.go` (`CanonicalMatrixCells`) and `pipeline_loader.go` must declare:
1. `law`: An integer in $[1, 7]$.
2. `ring`: An enumeration in `{"syntactic", "dynamic", "cryptographic"}`.
3. `domain`: The canonical domain string of the corresponding Law.
Failure to bind to a canonical cell results in an immediate fail-closed pipeline rejection (Law 7).

### `INV-CANON-003`: Bilateral Parity Governance
The Universal Contract & Schema Parity Gate is the open-core implementation of **Cell (4, 2)** [Law 4: SSoT $\times$ Ring 2: Dynamic Execution]. It verifies `nomos.schema.json` reflection parity and engine grounding at pre-flight and pre-release. Cross-hemisphere multi-tier AST isomorphism is decoupled to the sovereign extension domain (`gsi-sov`).

### `INV-CANON-004`: Matrix Completeness & Multi-Sensor Cells
A canonical invariant cell $\mathcal{C}(L_i, R_j)$ may contain zero, one, or multiple sensor drivers (e.g. Go and TypeScript AST parsers both contributing to Cell (2, 1)). A cell with zero active sensors is declared *sparse* but valid. However, every active sensor must strictly register into one canonical cell.

---

## 7. Verification & Transduction Mechanics
1. **Compiled Matrix SSoT**:
   - `substrate/src/nomos/modules/verify/matrix.go` models the 21 cells and validates gate registrations.
2. **Substrate Sensor Mapping**:
   - `substrate/src/nomos/modules/verify/pipeline_loader.go` binds Go sensors to `(Law, Ring)` coordinates.
3. **Visualizer Rendering**:
   - CLI output visually segments gates into the $7 \times 3$ matrix, reporting pass/fail status per invariant cell.

---

## 8. Table-Driven Verification Vectors

| Vector ID | Invariant Tested | Input Registration | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-MAT-01` | `INV-CANON-001` (Closure) | Gate registered with `law: 8` or `ring: "custom"` | `FAIL` (Pipeline schema rejection) |
| `TC-MAT-02` | `INV-CANON-002` (Explicit Binding) | Sensor probe omits `law`, `ring`, or `remedy` field | `FAIL` (Fail-closed loader error) |
| `TC-MAT-03` | `INV-CANON-003` (Bilateral Parity) | Schema or embedded asset SHA-256 drift ($\Delta(t) > 0$) | `FAIL` (`Cell (4, 2)` / `Cell (4, 3)` violation) |
| `TC-MAT-04` | `INV-CANON-004` (Driver Polymorphism) | Go and WASM Tree-sitter probes bound to `Cell (2, 1)` | `PASS` (`21/21` cells, `42/42` probes satisfied) |
