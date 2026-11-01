---
authors:
    - Mark Gantlett
    - Antigravity
classification: public
date: "2026-10-12"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Nomos Launchpad: Invisible Harness & Intent Activity Telemetry Specification"
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos Launchpad: Invisible Harness & Intent Activity Telemetry Specification

## 1. Overview & Strategic Intent
As Nomos approaches its open-core launch, the developer experience must achieve two fundamental architectural requirements:
1. **Total Invariance & Zero Trust**: Strict machine enforcement of 29 Definition of Done (DoD) quality gates, trunk locking (Law 3), and AST fitness proofs (Law 2).
2. **Invisible Ergonomics**: The developer and their Tier 1 coding agent (AGY-IDE, Claude Code, Cursor, opencode) must experience zero friction, zero command-line nagging, and zero cognitive overhead during standard iteration flow.

This specification formalizes the **Invisible Harness Paradigm** and the **Intent Activity Telemetry Architecture**.

---

## 2. The Invisible Harness Architecture

The Invisible Harness ensures that Nomos operates silently in the background with **zero custom slash commands, zero conversational workflow playbooks, and zero skill walls**:
- **Zero-Config Bootstrap (`nomos init`)**: Discovers project language and topology, scaffolding universal protocol files (`AGENTS.md`, `CLAUDE.md`), ambient IDE lifecycle hooks (`.agents/hooks.json`, `.agents/scripts/`), and minimal configuration discovered via `config.ResolveConfigPath()` in $< 2$ seconds—while pruning any legacy `.agents/workflows/` files.
- **Ambient Session Orientation & Defense Hooks (`SessionStart`, `PreToolUse` & `nomos hook init`)**: Automatically executes `nomos handshake` on session boot and installs CapBAC `pre-commit` and `commit-msg` hooks. If an agent attempts direct mutation on protected trunk (`main`), the hook intercepts the mutation and provides an immediate one-step remedy: `nomos task chore [title]`.
- **One-Step Maintenance Chores (`nomos task chore`)**: Incepts the task, registers it in the SQLite Living Ledger, provisions a transient worktree, and enters the `EDIT` phase atomically.
- **Microsecond DoD Evaluations**: All DoD quality gates execute in parallel compiled Go routines, verifying entire codebases in $< 1.5$ seconds.

---

## 3. Intent Activity Telemetry Architecture

With ambient lifecycle hooks, universal protocol rules, and Intent directives embedded internally and delivered Just-In-Time by `nomos handshake`, auditing cognitive engagement is essential.

### 3.1 Relational Storage & Dynamic Resolution
Database locations resolve dynamically through configuration discovery (`config.ResolveGlobalDBPath()` and `config.ResolveMnemeDBPath()`). Telemetry is recorded in the `telemetry_intent_activity` table:

```sql
CREATE TABLE IF NOT EXISTS telemetry_intent_activity (
    id TEXT PRIMARY KEY,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    seat TEXT NOT NULL,
    phase TEXT NOT NULL,
    active_task TEXT DEFAULT '',
    directive_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    details_json TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_telemetry_intent_task ON telemetry_intent_activity(active_task);
CREATE INDEX IF NOT EXISTS idx_telemetry_intent_directive ON telemetry_intent_activity(directive_id);
```

### 3.2 In-Line Interface Contracts & Go DTO Structs

The Substrate implements intent activity recording in `substrate/src/nomos/modules/telemetry/registry.go`:

```go
// IntentActivityEvent captures an intent or cognitive directive execution event.
type IntentActivityEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Seat        string    `json:"seat"`
	Phase       string    `json:"phase"`
	ActiveTask  string    `json:"active_task"`
	DirectiveID string    `json:"directive_id"`
	EventType   string    `json:"event_type"`
	DetailsJSON string    `json:"details_json"`
}
```

Declarative CUE schema counterpart in `intent/contracts/cue/telemetry.cue`:

```cue
package schemas

// IntentActivityEvent captures an intent or cognitive directive execution event.
#IntentActivityEvent: close({
	id:           string & strings.MinRunes(1)
	timestamp:    string
	seat:         string & strings.MinRunes(1)
	phase:        "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"
	active_task:  string | *""
	directive_id: string & strings.MinRunes(1)
	event_type:   "directive_dispatched" | "review_receipt_recorded" | "reflection_saved" | "gate_enforced"
	details_json: string | *"{}"
})
```

### 3.3 Canonical Event Types
1. `directive_dispatched`: Logged when `nomos handshake` injects a JIT directive into the active session context.
2. `review_receipt_recorded`: Logged when an adversarial review finding is stored in Nomos Mneme.
3. `reflection_saved`: Logged when architectural reflections or release walkthroughs are vectorized into the persistent memory store.
4. `gate_enforced`: Logged when `nomos verify` runs Definition of Done validation against Intent contracts.

---

## 4. The 3-Pass Progressive Developer Journey

To eliminate cognitive overload, the onboarding path is partitioned into three progressive passes:

| Pass | Target Audience | Primary Tooling | Key Competency |
| :--- | :--- | :--- | :--- |
| **Pass 1: Solo Quickstart** | Individual developer exploring Nomos | `init`, `chore`, `verify`, `sync` | Safe worktree sandboxing, trunk protection, and 42 DoD gates. |
| **Pass 2: Team Flow & Receipts** | Engineering teams shipping production code | `status`, `intent check`, `mem recall`, Git Notes | Cryptographic release receipts, semantic memory recall, Intent-to-Substrate parity. |
| **Pass 3: Deep Customization** | Platform engineers, architects, DevSecOps | CUE schemas, custom gates, hooks, plugins, CapBAC | Shaping custom enterprise guardrails, AST policies, and lifecycle hooks. |

---

## 5. Fail-Closed Pre-Flight Gating & Transactional Rollback (Law 7)

1. **Atomic Chore Provisioning**: When executing `nomos task chore [title]`:
   - If worktree creation fails after the task record is created in SQLite, the command initiates an immediate fail-closed rollback or registers the failure in `cli_audit_events`.
   - The phase state machine does not transition to `EDIT` unless physical worktree boundaries are verified clean.
2. **Fail-Safe Telemetry Recording**:
   - Telemetry ingestion into `telemetry_intent_activity` executes via `telemetry.RecordWorkflowInvocationSafe`.
   - If SQLite write lock contention occurs, telemetry emission degrades safely without panicking or masking user operations.

---

## 6. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-LAUNCH-001` | Pre-Commit Hook | Direct `git commit` mutation on protected `main` branch | Intercepted with exit code 1; output contains remedy `nomos task chore [title]` | Law 3 (Hermetic Worktrees) & Law 7 |
| `TC-LAUNCH-002` | `nomos task chore` | `nomos task chore "Fix parser edge case"` | Task created with `SUB-` key, transient worktree mounted, phase transitioned to `EDIT` | Law 3 & Law 6 (Canonical Singularity) |
| `TC-LAUNCH-003` | `RecordIntentActivity` | `IntentActivityEvent{DirectiveID: "review_02", EventType: "directive_dispatched"}` | Row persisted in `telemetry_intent_activity`, unifies cleanly with `#IntentActivityEvent` | Law 4 (SSoT CUE Parity) |
| `TC-LAUNCH-004` | Telemetry Ingestion Failure | Degraded database connection (`db = nil`) | Function returns error gracefully without application panic | Invariant 2 (Fail-Safe Telemetry) |

---

## 7. Verification Boundaries & DoD Evidence

1. **Telemetry Registry Assertion**: The `telemetry_intent_activity` table is initialized and verified in `telemetry.Registry`.
2. **Zero-Overhead Ingestion**: Event emission maintains sub-millisecond overhead ($< 1$ms per write).
3. **Markdown Integrity**: All markdown relative links resolve deterministically via Gate 14 (`BrokenWireGate`).
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 29 Definition of Done quality gates with zero regressions.
