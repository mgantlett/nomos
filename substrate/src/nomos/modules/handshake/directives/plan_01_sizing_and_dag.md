---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: planning
classification: public
date: "2026-09-27"
items:
    - "Cognitive Load Index (CLI): Tasks sized <= 3 routed to atomic commits; sized >= 6 require twin-plane decomposition"
    - "4-Phase Long-Horizon DAG Protocol: Declarative Upfront Scaffolding -> Wavefront Discovery -> Phase Discipline -> Terminal Release"
    - "Parent Cycle Preservation Invariant: Cycles remain IN_PROGRESS until all child tasks in the DAG reach terminal state"
    - "Dynamic Story Sizing Formula: Estimated size = burden + depth; auto-infers non-zero baselines from CLI tags (ADR-0016)"
    - "Zero-Flat-File Planning Boundary: Draft implementation plans strictly in brain artifact directory"
    - "Bidirectional Verifiability: Every plan requires explicit acceptance criteria and DoD boundary"
last_updated: "2026-10-27"
phases:
    - PLAN
plane: intent
priority: high
status: active
summary: "Cognitive task sizing (burden + depth), 4-phase long-horizon DAG protocol, auto-triage formulas, and wavefront discovery."
title: "Cognitive Sizing & DAG Planning Directives"
type: directive
version: 4.0.0
---

# Cognitive Sizing & DAG Planning Directives

## Executive Orientation
This directive defines the authoritative operational invariants for backlog triage, cognitive task sizing, dependency graph management, and execution cycles across the Nomos open-core ecosystem.

---

## 1. Core Planning Invariants

- **Cognitive Load Index (CLI) & Task Sizing**:
  - `cli:low` (Sizing ≤ 3): Leaf refactors, docstrings, unit tests, and typo fixes. Fast, atomic single-commit tasks.
  - `cli:medium` (Sizing 4..5): Single-module features, isolated schema migrations, and contract updates.
  - `cli:high` (Sizing ≥ 6): Core architectural designs, multi-plane coordinate systems, and kernel state machines. Recommended to decompose into atomic twin pairs (`INT-*` / `SUB-*`).
- **Dynamic Story Sizing (ADR-0016)**:
  - Story points are mathematically calculated as:
    `estimated_size = ContextBurden (b) + LogicDepth (d)`
  - Baseline for unclassified tasks is set to `b=1, d=1` (Size 2) rather than 0.
  - CLI tag inference: `cli:high` → `b=3, d=3` (Size 6); `cli:medium` → `b=2, d=2` (Size 4); `cli:low` → `b=1, d=1` (Size 2).
- **Zero-Flat-File Planning Boundary**:
  - Writing flat planning files (`implementation_plan.md`, `specs/`, `plans/`) into repository source trees is strictly blocked by the `Flat-File Planning Artifact Blocker` gate. All planning artifacts reside strictly in `<appDataDir>/brain/<conversation-id>/`.
- **Bidirectional Verifiability**:
  - A task cannot progress from `PLAN` to `EDIT` without a deterministic implementation plan containing testable acceptance criteria and explicit rigor boundaries.

---

## 2. The 4-Phase Long-Horizon DAG Protocol

To execute multi-day, cross-plane initiatives autonomously with zero drift, agents must adhere to the 4-Phase DAG Protocol:

```
+─────────────────────────────────────────────────────────────────────────────+
|                        LONG-HORIZON DAG ARCHITECTURE                        |
|                                                                             |
|   1. Declarative Upfront Scaffolding (nomos task create --depends-on)       |
|                                   │                                         |
|                                   ▼                                         |
|   2. Topological Wavefront Discovery (nomos task dag)                       |
|                                   │                                         |
|                                   ▼                                         |
|   3. Phase Discipline Execution (nomos task start / edit / review)          |
|                                   │                                         |
|                                   ▼                                         |
|   4. Terminal 2PC Atomic Release (nomos task sync -F <walkthrough>)         |
+─────────────────────────────────────────────────────────────────────────────+
```

### Phase 1: Declarative Upfront Scaffolding (Zero-Ad-Hoc Planning)
Before writing production code, the cross-plane dependency graph must be populated into the SQLite Living Ledger:
1. **Bilateral Twin Scaffolding**: Create twin pairs across Intent and Substrate planes via `nomos task pair "<Title>"`.
2. **Explicit Dependency Chains**: Declare dependency ordering upfront using `--depends-on <KEY>` during `nomos task create`.
3. **Plane Isolation**: Tasks strictly belong to either the Intent Plane (`INT-*`) or Substrate Plane (`SUB-*`). Cross-plane mutations fail closed.

### Phase 2: Topological Wavefront Discovery
- **Wavefront Query (`nomos task dag`)**:
  - Queries the SQLite DAG for unblocked tasks whose prerequisite tasks have reached `DONE` status.
- **Backlog Inspection (`nomos task list`)**:
  - Displays registered tasks, status, plane, priority, and dependencies.

### Phase 3: Phase Discipline & Hermetic Execution
- **Task Lifecycle**:
  - Transition task through `PLAN` → `EDIT` (`nomos task start <KEY>`, `nomos task edit <KEY>`).
  - Execute mutations strictly within the transient worktree `worktrees/<repo>-<KEY>`.
  - Validate DoD quality gates before committing (`nomos task review <KEY>`).

### Phase 4: Terminal 2PC Atomic Release
- **Atomic Sync (`nomos task sync -F <walkthrough-file> <KEY>`)**:
  - Merges task branch into trunk `main`, records cryptographic release receipt in Git Notes, marks task `DONE`, and deletes transient worktree.

---

## 3. Command Reference

| Action | Canonical Command | Architectural Purpose |
| :--- | :--- | :--- |
| **Discover Next Unblocked Task** | `nomos task dag` | Discovers next unblocked task on the active topological wavefront. |
| **List Backlog Tasks** | `nomos task list` | Inspects all registered Living Ledger tasks and current statuses. |
| **Inspect Task Details** | `nomos task show <KEY>` | Displays full record details, metadata, and dependencies. |
| **Provision Bilateral Twin Pair**| `nomos task pair "<Title>"` | Atomically scaffolds synchronized INT-* and SUB-* twin tasks. |
| **Provision Isolated Worktree** | `nomos task start <KEY>` | Transitions task to PLAN/EDIT and provisions transient worktree. |
| **Review Definition of Done** | `nomos task review <KEY>` | Evaluates the 42 DoD quality gates for the active worktree. |
| **Atomic 2PC Release** | `nomos task sync -F <file> <KEY>` | Merges to trunk, mints Git Notes release receipt, and closes task. |
| **Provision 1-Turn Chore** | `nomos task chore "<Title>"` | Atomically allocates a `SUB-*` task and enters an `EDIT` worktree in 1 turn. |
