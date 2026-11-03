---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: verification
classification: public
date: "2026-10-13"
flow:
    - SYNC
items:
    - "Atomic 2PC Release Engine: Deterministic Phase 1 pre-flight gatekeeper and Phase 2 trunk merge"
    - "Hermetic Trunk Immutability: Main branch remains read-only outside 2PC sync execution"
    - "Walkthrough Markdown Validation: Required release artifact proving AST changes"
    - "Tamper-Evident Git Notes Receipts: Release receipt committed to refs/notes/nomos/releases"
last_updated: "2026-10-15T00:30:00Z"
phases:
    - SYNC
plane: intent
priority: high
status: active
summary: "Atomic Two-Phase Commit (2PC) release mechanics, walkthrough diff validation, and Git Notes release certificates."
title: "Atomic Two-Phase Commit (2PC) Release & Receipt Directives"
type: spec
version: "4.0.0"
---

# Atomic Two-Phase Commit (2PC) Release & Receipt Directives

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Maintain trunk immutability (Law 3) and execution atomicity through a deterministic two-phase commit protocol and tamper-evident Git Notes receipts.

---

## 1. Executive Orientation
This directive defines the mandatory two-phase commit protocol, compensating rollback state machine, and cryptographic receipt mechanisms enforced during the **`SYNC`** phase of the task lifecycle under Law 1 (Dual-Core Inversion) and Law 7 (Fail-Closed Pre-Flight).

All task mutations originate within an isolated transient worktree (`worktrees/<repo>-<key>`). Direct mutation of trunk (`main`) is physically read-only. The transition to production release is governed exclusively by compiled Substrate mechanics (`nomos task sync`).

---

## 2. Core Architectural Invariants

### 1. The Two-Phase Commit Protocol
Every release operates in two deterministic phases governed by Go transaction logic:

- **Phase 1 (Prepare & Verify - Read-Only Validation)**:
  - Validates active CapBAC lease capability in state `EDIT`.
  - Verifies workspace cleanliness (zero uncommitted edits outside transient worktree).
  - Executes The Seven Laws & 21 Invariant Cells Suite (42 DoD Quality Gates) via `nomos verify`.
  - Validates author release walkthrough artifact passed via `-F <path>`.
  - Any failure in Phase 1 aborts the sync without mutating trunk `main`.

- **Phase 2 (Commit, Certificate & Rollback Recovery)**:
  - Records pre-merge trunk SHA (`pre_merge_sha`).
  - Executes fast-forward or squash merge into trunk `main`.
  - Atomically mints an immutable `ReleaseReceipt` into Git Notes (`refs/notes/nomos/releases`).
  - Updates `#TaskRecord.status` from `SYNC` to `DONE` in SQLite.
  - Transitions workspace lifecycle phase from `SYNC` to `IDLE` (`types.PhaseState`).
  - Revokes task CapBAC lease and unlinks transient worktree.

---

## 3. Fail-Closed Rollback State Machine (Law 7)

If any operation in Phase 2 encounters a fault, the release engine executes a compensating transaction:

```text
[Phase 2 Initiated] 
       │
       ▼
[Trunk Merge] ── (fails) ──► Abort; Working tree preserved in EDIT; main untouched
       │ (passes)
       ▼
[Git Notes Receipt Write] ── (fails) ──► ROLLBACK: git update-ref refs/heads/main pre_merge_sha
       │                                 Emit ERR_RECEIPT_PERSISTENCE_FAILED; Lease kept in EDIT
       ▼ (passes)
[SQLite Status -> DONE] ── (fails) ──► Retry with exponential backoff; WAL journal recovery
       │ (passes)
       ▼
[Worktree Unlink] ── (fails) ──► Log warning; Trunk release preserved; Worktree marked orphaned
```

---

## 4. Formal DTO Schemas

### ReleaseReceipt Contract
```json
{
  "task_key": "INT-597",
  "trunk_commit": "a1b2c3d4e5f67890123456789abcdef012345678",
  "worktree_head": "f1e2d3c4b5a60798123456789abcdef012345678",
  "released_at": "2026-10-15T00:30:00Z",
  "author": "Nomos Architect <architect@nomos.dev>",
  "walkthrough_path": ".tmp/walkthrough.md",
  "gates_passed": 42,
  "gates_total": 42,
  "gate_results": {
    "ast_build_veracity": true,
    "cognitive_budget": true,
    "parity_engine": true,
    "byte_parity_audit": true
  }
}
```

### Walkthrough Artifact Specification (`-F <path>`)
The walkthrough artifact provided to `nomos task sync -F <path>` must contain:
1. `# Walkthrough: <TASK_KEY>`
2. `## Overview`: Executive statement of intent and problem solved.
3. `## Changes`: Explicit file list with diff descriptions.
4. `## Verification`: Exact non-verbal command proofs (`nomos verify`) with exit code 0.

---

## 5. Table-Driven Test Matrix & Vectors

| Case ID | Release Scenario | Phase 1 Status | Phase 2 Action | Trunk State | Final Task Status | Emitted Error Constant |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `TC-2PC-01` | Clean Release | Pass (7/7 Laws) | Merge + Receipt | Merged | `DONE` | `N/A` |
| `TC-2PC-02` | Failing DoD Gate | Fail (Cell (1, 2)) | Aborted | Untouched | `EDIT` | `ERR_GATE_VERIFICATION_FAILED` |
| `TC-2PC-03` | Missing Walkthrough File | Fail Closed | Aborted | Untouched | `EDIT` | `ERR_WALKTHROUGH_NOT_FOUND` |
| `TC-2PC-04` | Receipt Persistence Failure | Pass (7/7 Laws) | Compensating Rollback | Reset to Pre-Merge | `EDIT` | `ERR_RECEIPT_PERSISTENCE_FAILED` |
| `TC-2PC-05` | Direct Push to `main` | N/A | Blocked by Hook | Untouched | N/A | `ERR_TRUNK_MUTATION_PROHIBITED` |
