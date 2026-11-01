---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: verification
classification: public
complexity: high
date: "2026-09-28"
description: Adversarial Red-Teaming, AST Mutation Critique & Invariant Proof
hitl_requirements: none
last_updated: "2026-10-15T00:30:00Z"
name: Nomos Adversarial Audit Directive
phases:
    - REVIEW
plane: intent
status: active
title: "Nomos Adversarial Audit Directive"
type: spec
version: "4.0.0"
---

# Nomos Adversarial Audit Directive

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Perform independent adversarial red-team critique, AST mutation analysis, and formal invariant validation during the **`REVIEW`** phase.

> **Law 2 (The Non-Verbal Proof Law)**: Zero trust in conversational claims; code correctness requires compilation exit code 0, AST proof, and adversarial critic signoff.

---

## 1. Executive Orientation and Phase Boundary

Adversarial audit executes strictly during the **`REVIEW`** phase within the isolated task worktree prior to two-phase release sync. In accordance with **ADR-0031 (Open-Core Boundary)**, open-core audits leverage the user-configured Tier 1 model harness under indirect control without proprietary daemons.

---

## 2. Universal Hemisphere Review Protocols

### 1. Substrate Hemisphere Protocol (`plane: "substrate"`)
- **First-Principles Distrust:** Verify all claims through non-verbal proofs (`nomos verify`), never conversational explanations.
- **AST Quality Gates:**
  - Cyclomatic Complexity $CC \le 15$ on all modified functions (`gates.cue`).
  - Cognitive Debt Score $CDS \le 45$ with ReLU clamping.
  - Docstring density $\ge 10\%$ on every exported symbol.
  - Zero forbidden literals and strict typed constants.
  - Single-layer boundary compliance (no cross-plane leaks).
- **Concurrency & Resource Verification:**
  - Check for race conditions (`go test -race`).
  - Verify goroutine lifecycle guarantees, bounded channel buffers, and context cancellation propagation.

### 2. Intent Hemisphere Protocol (`plane: "intent"`)
- **Deterministic Enforcement Evaluation:**
  - Penalize any workflow relying on LLM memory or stochastic prompt compliance.
  - Require mechanical substrate enforcement (Go binaries, SQLite state, AST checks).
- **Cognitive Tiering Audit:**
  - Verify task sizing strictly against compiled tiers (`T1` vs `T2`).
  - Enforce SSoT protocol symmetry with dynamically discovered protocol specifications.

---

## 3. Sequential Execution Instructions

1. **Substrate Pre-Flight Verification**:
   Execute the Go-native quality gate suite:
   ```bash
   nomos verify
   ```
2. **Adversarial Critique**:
   Dispatch adversarial review to evaluate blast radius, dead code, error paths, and security boundaries.
3. **Intent Parity Verification**:
   Verify cryptographic parity across Intent schemas and ADR execution frontiers:
   ```bash
   nomos verify
   ```
4. **Record Findings in Nomos Mneme**:
   Persist review outcome into persistent memory via canonical CLI:
   ```bash
   nomos mem save "Review: [Topic]. Findings: [Full Markdown Body]"
   ```
5. **Register Actionable Living Ledger Tasks**:
   Scaffold tracking tasks for any discovered defects:
   ```bash
   nomos task create "Remediate [Finding]" --plane substrate
   ```

---

## 4. The Seven Laws & 21 Invariant Cells DoD Checks

- [ ] All 7 Laws and 21 Invariant Cells pass cleanly with exit code 0 (`nomos verify`).
- [ ] No regression in Cyclomatic Complexity ($CC \le 15$) or Cognitive Debt ($CDS \le 45$).
- [ ] Cryptographic Intent-Substrate parity verified via `nomos verify`.
- [ ] Review findings persisted to Nomos Mneme via `nomos mem save`.

---

## 5. Table-Driven Test Matrix & Vectors

| Case ID | Verification Target | Invariant Evaluated | Substrate Gate | Expected Outcome | Emitted Error Constant |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `TC-AUD-01` | Clean Worktree Diff | The Seven Laws & 21 Invariant Cells Suite | `nomos verify` | Pass (Exit 0) | `N/A` |
| `TC-AUD-02` | Function with $CC = 16$ | Complexity Invariant ($CC \le 15$) | `ComplexityGate` | Fail Closed | `ERR_COMPLEXITY_EXCEEDED` |
| `TC-AUD-03` | Broken CUE Schema Hash | Single Source of Truth (Law 4) | `IntentParityGate` | Fail Closed | `ERR_INTENT_SUBSTRATE_DRIFT` |
| `TC-AUD-04` | Task Key with Custom Prefix | Canonical Singularity (Law 6) | `TaskKeyGate` | Fail Closed | `ERR_CANONICAL_VIOLATION` |
