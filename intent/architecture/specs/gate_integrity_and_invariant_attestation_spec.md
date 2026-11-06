---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
lineage:
  - INT-740
  - INT-741
  - SUB-1224
  - SUB-1225
plane: intent
ssot_repository: nomos
status: active
title: "Gate Integrity & Invariant Attestation Specification (Mutation Sensitivity, Metamorphic Invariance, AST Anti-Vacuity)"
type: spec
version: "4.0.0"
---

# Gate Integrity & Invariant Attestation Specification

---

## 1. Executive Summary & Problem Statement

Under Law 2 (The Non-Verbal Proof Law), the substrate distrusts conversational claims of code correctness. Verification pipelines face two structural defect modalities:

1. **False Negative (Type II Error / Trivial Pass):** A gate returns `Passed: true` without evaluating ASTs or catching synthetic flaws, admitting defective code into trunk.
2. **False Positive (Type I Error / Spurious Alarm):** A gate rejects clean, compliant code due to brittle regexes, improper relative path resolution, or faulty boundary thresholds.

### 1.1 Epistemic Precision
Nomos distinguishes between two verification regimes:
- **Deductive Mathematical Proof:** Formal theorem-proving across infinite state spaces (such as Coq, Lean, or Z3 SMT solvers).
- **Deterministic Invariant Attestation:** Empirical mutation sensitivity ($MS = 1.0$), metamorphic reference invariance ($FPR = 0.0\%$), and static AST structural anti-vacuity audits over finite codebase topologies.

This specification codifies the three deterministic attestation vectors executed under `Cell 2.3 (gate_integrity_proof)` via `nomos verify proof [path]`.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **2** | **The Non-Verbal Proof Law** | Gates prove 100% mutant sensitivity ($MS = 1.00$) and 0% false positives. | `Cell 2.3 (gate_integrity_proof)`, `Cell 2.1 (anti_shim)` |
| **3** | **The Hermetic Worktree Law** | Synthetic AST fault injection executes strictly in memory without disk writes. | In-memory `go/ast` cloning (`verify/proof.go`) |
| **4** | **The Single Source of Truth Law** | Attestation telemetry binds directly to compiled `verify.ProofReport` JSON contracts. | `substrate/src/nomos/modules/verify/proof.go` |
| **6** | **The Canonical Singularity Law** | `nomos verify proof [path]` serves as the single CLI entrypoint for gate attestation. | `Cell 6.2 (zero_alias)` |
| **7** | **The Fail-Closed Pre-Flight Law** | Surviving mutants, non-zero false positives, or recursive self-invocation loops fail closed. | `EvaluateGateIntegrityAttestation` exit code 1 |

---

## 3. The Three Attestation Vectors

### 3.1 Vector 1: Mutation Testing Sensitivity ($MS = 1.00$)

A gate suite possesses sensitivity if and only if it kills 100% of non-equivalent synthetic mutants:

$$MS = \frac{\text{Mutants Killed } (K)}{\text{Total Mutants } (M)} = 1.00 \quad (100\%)$$

Nomos executes in-memory AST fault injections directly against sampled functions from the target codebase (or standard synthetic fallbacks when no Go source files exist). The 10-mutant operator catalog covers:
1. **Mutant 1 (Empty Body Stub):** Guts a function body into zero statements (`func Stub() {}`). Assertion: `AntiShimGate` kills mutant (`Passed: false`).
2. **Mutant 2 (Panic Placeholder Stub):** Replaces body with `panic("todo")`. Assertion: `AntiShimGate` kills mutant.
3. **Mutant 3 (Function LOC Overrun):** Injects statement padding exceeding `MaxFuncLOC > 60`. Assertion: `FileLOCGate` kills mutant.
4. **Mutant 4 (File LOC Overrun):** Synthesizes a file exceeding `MaxFileLOC > 400`. Assertion: `FileLOCGate` kills mutant.
5. **Mutant 5 (Blank Identifier Discard):** Discards error value via `_ = fn()`. Assertion: `UncheckedErrorsGate` kills mutant.
6. **Mutant 6 (Trivial Gate Result Shim):** Replaces `Evaluate` body with single statement `return &GateResult{Passed: true}`. Assertion: `GateIntegrityProofGate` kills mutant.
7. **Mutant 7 (Conditional Wrapper Pass Shim):** Encloses trivial pass in single branch `if ctx != nil { return &GateResult{Passed: true} }`. Assertion: `GateIntegrityProofGate` kills mutant.
8. **Mutant 8 (Forbidden Path Literal):** Injects hardcoded workspace path literal into source. Assertion: `ForbiddenLiteralsGate` kills mutant.
9. **Mutant 9 (Docstring Stripping):** Strips all exported comments to force docstring density below threshold. Assertion: `DocstringDensityGate` kills mutant.
10. **Mutant 10 (Single Return Discard):** Strips business logic down to premature return statement. Assertion: `AntiShimGate` kills mutant.

**Hermetic Invariant (Law 3):** All mutations execute strictly in memory on AST copies without modifying physical source files on disk.

### 3.2 Vector 2: Metamorphic Reference Invariance ($FPR = 0.00\%$)

A gate suite is specific if and only if its False Positive Rate ($FPR$) on compliant constructs is strictly zero:

$$FPR = \frac{\text{False Positives } (FP)}{\text{False Positives } (FP) + \text{True Negatives } (TN)} = 0.00 \quad (0\%)$$

The engine evaluates clean constructs:
- Real compliant functions from the target codebase pass `AntiShimGate` without false alarms.
- Bounded LOC metrics pass `FileLOCGate`.
- Checked error handling passes `UncheckedErrorsGate`.
- Canonical baseline sequence passes all registered statutory gates.

**Self-Audit Recursion Invariant:** Vector 2 evaluation excludes `GateIntegrityProofGate` from recursive self-invocation to prevent re-entrancy cycles.

### 3.3 Vector 3: AST Structural Anti-Vacuity Meta-Audit

To prevent quality gate implementations from devolving into trivial constant returns, Nomos performs static AST structural meta-audits on all `Gate.Evaluate` methods:

$$\forall g \in \text{Gates}, \quad \text{Evaluate}(g) \text{ must not resolve to an unconditional } \&\text{GateResult}\{\text{Passed: true}\}$$

Every gate function must:
1. Contain operational inspection logic.
2. Reject single-statement `Passed: true` returns.
3. Reject trivial conditional wrapper shims (`if ctx != nil { return &GateResult{Passed: true} }` or `if true { return &GateResult{Passed: true} }`).

---

## 4. Machine-Enforced CLI & Compiled Go SSoT Report Contract

Executed via:
```bash
nomos verify proof [path]
```

Output schema:
```text
  Nomos Gate Integrity & Invariant Attestation (Cell 2.3)
  Target Codebase: <path> (<N> Go files analyzed)
────────────────────────────────────────────────────────────────────────────
  Vector 1: Mutation Testing Sensitivity (MS)       [100.0% · 10/10 Killed]   PASS
  Vector 2: Metamorphic Invariance Check (FPR)       [0.0% · 9/9 Passed]       PASS
  Vector 3: AST Structural Anti-Vacuity Audit        [44/44 Non-Trivial]       PASS
────────────────────────────────────────────────────────────────────────────
   Deterministic Gate Integrity Invariants Satisfied · Exit Code 0
```

Package `substrate/src/nomos/modules/verify/proof.go` defines the compiled attestation report contract:

```go
package verify

// ProofReport summarizes the outcomes of the Three Mathematical Proofs of Gate Integrity (#GateIntegrityReport).
type ProofReport struct {
	TargetDir             string  `json:"target_dir"`
	IsCodebaseTargeted    bool    `json:"is_codebase_targeted"`
	CodebaseFilesScanned  int     `json:"code_files_scanned,omitempty"`
	MutationKilled        int     `json:"mutation_killed"`
	MutationTotal         int     `json:"mutation_total"`
	MutationScore         float64 `json:"mutation_score"`
	FalsePositiveRate     float64 `json:"false_positive_rate"`
	ReferenceGatesPass    int     `json:"reference_gates_pass"`
	ReferenceGatesTotal   int     `json:"reference_gates_total"`
	NonVacuousGates       int     `json:"non_vacuous_gates"`
	TotalGatesEvaluated   int     `json:"total_gates_evaluated"`
	Passed                bool    `json:"passed"`
	DurationTotalMicros   int64   `json:"duration_total_micros"`
	DurationVector1Micros int64   `json:"duration_vector1_micros"`
	DurationVector2Micros int64   `json:"duration_vector2_micros"`
	DurationVector3Micros int64   `json:"duration_vector3_micros"`
}

// EvaluateGateIntegrityAttestation executes all three deterministic attestation vectors of gate integrity.
func EvaluateGateIntegrityAttestation(repoRoot string) (*ProofReport, error)
```

---

## 5. Table-Driven Verification Vectors

| Vector ID | Attestation Dimension | Input Scenario | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-PRF-01` | Vector 1 (Sensitivity) | Synthetic mutant escapes gate detection (`mutation_score < 100.0`) | `FAIL` (Exit 1, `MutationScore < 100.0`) |
| `TC-PRF-02` | Vector 2 (Specificity) | Clean reference fixture triggers false alarm (`false_positive_rate > 0.0`) | `FAIL` (Exit 1, `FalsePositiveRate > 0.0`) |
| `TC-PRF-03` | Vector 3 (Anti-Vacuity) | Gate implementation returns unconditional `GateResult{Passed: true}` | `FAIL` (`ERR_TRIVIAL_PASS_SHIM`) |
| `TC-PRF-04` | Full Cell 2.3 Proof | 100% mutant kill rate, 0% FPR, and 100% non-vacuous AST bodies | `PASS` (Exit 0, `ProofReport.Passed == true`) |

---

## 6. Cross-Plane Traceability & DoD Verification

- **Intent Lineage**: `INT-740`, `INT-741`.
- **Substrate SSoT**: `SUB-1224`, `SUB-1225` (`substrate/src/nomos/modules/verify/proof.go`, `substrate/src/nomos/modules/verify/gates_proof.go`, `substrate/src/nomos/modules/verify/anti_shim.go`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 2.1 (anti_shim)`, `Cell 2.3 (gate_integrity_proof)`.
- **Deterministic Verification**: `nomos verify` and `nomos verify proof` execute all three attestation vectors in memory and exit `0` only when `MutationScore == 100.0`, `FalsePositiveRate == 0.0`, and `NonVacuousGates == TotalGatesEvaluated`.
