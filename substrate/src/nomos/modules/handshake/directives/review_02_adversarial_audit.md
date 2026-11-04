---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: verification
classification: public
complexity: high
date: "2026-09-28"
description: Empirical Execution, AST Mutation Proof & Spec-to-Code Conformance Verification
hitl_requirements: none
last_updated: "2026-11-04T00:30:00Z"
name: Nomos Empirical & Invariant Verification Directive
phases:
    - REVIEW
plane: intent
status: active
title: "Nomos Empirical & Invariant Verification Directive"
type: spec
version: "4.5.0"
---

# Nomos Empirical & Invariant Verification Directive

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Perform empirical test execution, AST mutation analysis, and spec-to-code conformance verification during the **`REVIEW`** phase without burning tokens on abstract nitpick prose.

> **Law 2 (The Non-Verbal Proof Law)**: Zero trust in conversational claims; code correctness requires compilation exit code 0, empirical test proof, and deterministic AST invariant satisfaction.

---

## 1. Executive Orientation and Phase Boundary

Verification executes strictly during the **`REVIEW`** phase within the isolated task worktree prior to two-phase release sync (`SYNC`). Per **ADR-0031 (Open-Core Boundary)**, open-core verification relies 100% on deterministic compiled Go gates, empirical test execution, and bilateral `INT`/`SUB` traceability rather than external LLM review daemons.

---

## 2. Universal Hemisphere Review Protocols

### 1. Substrate Hemisphere Protocol (`plane: "substrate"`)
- **Empirical & Formal Verification First:** Prove behavior via `nomos verify` (42 gates), unit/integration tests, and mathematical gate integrity (`GateIntegrityProofGate`), never abstract style commentary.
- **AST Quality Gates:**
  - Cyclomatic Complexity $CC \le 15$ on all modified functions (`verify.DefaultThresholds`).
  - Cognitive Debt Score $CDS \le 45$ with ReLU clamping.
  - Docstring density $\ge 10\%$ on every exported symbol.
  - Zero forbidden literals and strict typed constants.
  - Single-layer boundary compliance (no cross-plane mutations).
- **Concurrency & Resource Verification:**
  - Check for race conditions (`go test -race`) on concurrent packages.
  - Verify goroutine lifecycle guarantees, bounded channel buffers, and context cancellation propagation.

### 2. Intent Hemisphere Protocol (`plane: "intent"`)
- **Enforced Spec-to-Code Lockstep:**
  - Reject any workflow relying on conversational compliance or unverified prose promises.
  - Ensure paired `INT-*` acceptance criteria map directly to `@intent` AST anchors and deterministic tests in the `SUB-*` twin.

---

## 3. Sequential Execution Instructions

1. **Deterministic Gate Suite Execution**:
   Execute the Go-native 42-gate verification suite:
   ```bash
   nomos verify
   ```
2. **Targeted Concurrency & Spec Conformance Check**:
   Run `go test -race ./...` on modified concurrent modules and verify every acceptance criterion in the paired `INT-*` task card is covered by an executable test. Do **not** generate speculative style/YAGNI prose reviews or dump unstructured review essays into `mneme.db`.
3. **Prepare Concise 2PC Walkthrough Receipt**:
   Write concise empirical verification proof to `.tmp/walkthrough-<KEY>.md` for atomic two-phase commit release (`nomos task sync <KEY> -F .tmp/walkthrough-<KEY>.md`).
4. **Register Actionable Living Ledger Tasks (Only on Real Defects)**:
   If verification uncovers an out-of-scope defect, register a concrete task card:
   ```bash
   nomos task create "Remediate [Defect]" --plane substrate
   ```

---

## 4. The Seven Laws & 21 Invariant Cells DoD Checks

- [ ] All 7 Laws and 21 Invariant Cells pass cleanly with exit code 0 (`nomos verify`).
- [ ] No regression in Cyclomatic Complexity ($CC \le 15$) or Cognitive Debt ($CDS \le 45$).
- [ ] Cryptographic Intent-Substrate parity verified via `nomos verify`.
- [ ] Empirical walkthrough receipt prepared for `nomos task sync -F`.

---

## 5. Table-Driven Test Matrix & Vectors

| Case ID | Verification Target | Invariant Evaluated | Substrate Gate | Expected Outcome | Emitted Error Constant |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `TC-AUD-01` | Clean Worktree Diff | The Seven Laws & 21 Invariant Cells Suite | `nomos verify` | Pass (Exit 0) | `N/A` |
| `TC-AUD-02` | Function with $CC = 16$ | Complexity Invariant ($CC \le 15$) | `ComplexityGate` | Fail Closed | `ERR_COMPLEXITY_EXCEEDED` |
| `TC-AUD-03` | Drifted Protocol / Schema Hash | Single Source of Truth (Law 4) | `IntentParityGate` / `ContractParityGate` | Fail Closed | `ERR_INTENT_SUBSTRATE_DRIFT` |
| `TC-AUD-04` | Task Key with Custom Prefix | Canonical Singularity (Law 6) | `TaskKeyGate` | Fail Closed | `ERR_CANONICAL_VIOLATION` |
