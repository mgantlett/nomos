---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: verification
classification: public
date: "2026-09-27"
flow:
    - REVIEW
items:
    - "The Seven Laws & 21 Invariant Cells: 3 concentric defense rings (Syntactic, Dynamic, Cryptographic) with 42 DoD quality gates"
    - "Non-Verbal AST Proofs: Verification requires exit code 0 and AST proofs, never conversational prose"
    - "Mandatory Commit Trailers: Task, Plane, Tier, Executor, and Skip receipts declared in commit payload"
    - "Ed25519 Cryptographic Receipts: Release sync signs commit SHA and walkthrough hash to refs/notes/nomos/releases"
last_updated: "2026-10-29T00:00:00Z"
phases:
    - REVIEW
plane: intent
priority: high
status: active
summary: "The Seven Laws of Deterministic Engineering, 21 Invariant Cells Suite across 3 Defense Rings (42 DoD Quality Gates), non-verbal AST verification, and Ed25519 commit receipts."
title: "The Seven Laws DoD Verification & Cryptographic Release Directives"
type: spec
---

# The Seven Laws DoD Verification & Cryptographic Release Directives

## 1. Executive Orientation
This directive defines the quality gate verification and cryptographic release procedures enforced during `REVIEW` and `SYNC` phases under Law 1 (Dual-Core Inversion) and Law 2 (Non-Verbal Proof).

---

## 2. Core Verification Invariants

### The Seven Laws & 21 Invariant Cells Suite
Nomos evaluates mutations deterministically across The Seven Laws and 21 Invariant Cells in `nomos verify`:
1. *Ring 1: Syntactic Defense Ring (AST & Static Constraints)*: Code veracity, cognitive budget, hermetic worktree boundaries, SSoT schema validity, portability, and key singularity.
2. *Ring 2: Dynamic Defense Ring (Runtime & Phase Discipline)*: Phase discipline, test execution ($\ge 55\%$ coverage), git subprocess hygiene, contract parity, and runtime ingress.
3. *Ring 3: Cryptographic Defense Ring (Byte Parity & Receipts)*: CapBAC lease auth, release receipts in Git Notes, adversarial review receipts ($\ge 80.0$), byte parity, and secret scanning.

### Non-Verbal Proof Law (Law 2)
Zero trust in conversational claims. Code correctness requires full compilation exit code 0 and deterministic satisfaction across all 21 Invariant Cells (42 DoD Quality Gates).

### Mandatory Commit Trailers
Every release commit merged into protected trunk `main` must declare compliant git commit trailers:
- `Task: <KEY>` (matching `^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)
- `Plane: <intent|substrate>` (strictly matching `#TaskRecord.plane` from `task.cue`)
- `Tier: <T1|T2>` (matching `tier: "T1" | "T2"`)
- `Executor: <agent-id>` (canonical agent identity, e.g. `antigravity-ide`)
- `Skip-Receipt: <reason>` (Optional; permitted strictly on documentation-only intent chore commits when authorized by CapBAC lease; strictly forbidden on any commit mutating compiled Go or TypeScript substrate code, failing closed with `ERR_INVALID_SKIP_RECEIPT`).

---

## 3. Ed25519 Cryptographic Receipt DTO Schema

On successful two-phase release sync (`nomos task sync`), the substrate attaches an Ed25519 signed receipt to `refs/notes/nomos/releases`:

```json
{
  "task_key": "SUB-740",
  "commit_sha": "a1b2c3d4e5f67890123456789abcdef012345678",
  "walkthrough_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "gates_passed": 42,
  "gates_total": 42,
  "gate_results": {
    "ast_build_veracity": true,
    "cognitive_budget": true,
    "parity_engine": true,
    "byte_parity_audit": true
  },
  "signer": "ed25519:nomos-substrate-totem",
  "timestamp": "2026-10-15T00:30:00Z"
}
```

---

## 4. Table-Driven Test Matrix & Vectors

| Case ID | Input Commit Payload | Metric / Field Tested | Expected Gate Outcome | Emitted Error Constant |
| :--- | :--- | :--- | :--- | :--- |
| `TC-DOD-01` | Full 7 Laws & 21 Invariant Cells Exit 0 | Standard Verification | Pass (Exit 0) | `N/A` |
| `TC-DOD-02` | Function with $CC = 16$ | Ring 2 Complexity ($CC \le 15$) | Fail Closed | `ERR_COMPLEXITY_EXCEEDED` |
| `TC-DOD-03` | Function with $CDS = 48$ | Ring 2 Cognitive Debt ($CDS \le 45$) | Fail Closed | `ERR_COGNITIVE_DEBT_EXCEEDED` |
| `TC-DOD-04` | `Plane: membrane` | Commit Trailer Parity | Fail Closed | `ERR_INVALID_PLANE_TRAILER` |
| `TC-DOD-05` | `Tier: 1` | Commit Trailer Parity | Fail Closed | `ERR_INVALID_TIER_TRAILER` |
| `TC-DOD-06` | `Skip-Receipt` on Go file | Skip Receipt Safety Check | Fail Closed | `ERR_INVALID_SKIP_RECEIPT` |
| `TC-DOD-07` | Missing Ed25519 signature | Release Receipt Verification | Fail Closed | `ERR_RECEIPT_VERIFICATION_FAILED` |
