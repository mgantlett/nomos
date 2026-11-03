---
authors:
    - Mark Gantlett
    - Antigravity
classification: public
date: "2026-10-12"
last_updated: "2026-11-03"
plane: intent
status: active
title: "Nomos Launchpad: Invisible Harness & Capability Telemetry Specification"
type: spec
version: "4.2.0"
ssot_repository: nomos
---

# Nomos Launchpad: Invisible Harness & Capability Telemetry Specification

## 1. Overview & Strategic Intent
The Nomos open-core developer experience satisfies two fundamental architectural requirements:
1. **Total Invariance & Zero Trust**: Strict machine enforcement of 42 Definition of Done (DoD) quality gates, trunk locking (Law 3), and AST fitness proofs (Law 2).
2. **Invisible Ergonomics**: The developer and their Tier 1 coding agent (AGY-IDE, Claude Code, Cursor, opencode) must experience zero friction, zero command-line nagging, and zero cognitive overhead during standard iteration flow.

This specification formalizes the **Invisible Harness Paradigm** and the **Capability Invocation Telemetry Architecture**.

---

## 2. The Invisible Harness Architecture

The Invisible Harness ensures that Nomos operates silently in the background with **zero custom slash commands, zero conversational workflow playbooks, and zero skill walls**:
- **Zero-Config Bootstrap (`nomos init`)**: Discovers project language and topology, scaffolding universal protocol files (`AGENTS.md`, `CLAUDE.md`), ambient IDE lifecycle hooks (`.agents/hooks.json`, `.agents/scripts/`), and minimal configuration discovered via `config.ResolveConfigPath()` in $< 2$ seconds—while pruning any legacy `.agents/workflows/` files.
- **Ambient Session Orientation & Defense Hooks (`nomos hook session-start`, `nomos hook pre-tool`, `nomos hook post-tool`)**: Automatically executes `nomos handshake` on session boot and installs CapBAC `pre-commit` and `commit-msg` hooks. If an agent attempts direct mutation on protected trunk (`main`), the hook intercepts the mutation and provides an immediate one-step remedy: `nomos task chore "<title>"`.
- **One-Step Maintenance Chores (`nomos task chore`)**: Incepts the task, registers it in the SQLite Living Ledger, provisions a transient worktree, and enters the `EDIT` phase atomically.
- **Microsecond DoD Evaluations**: All 42 DoD quality gates execute in parallel compiled Go routines, verifying entire codebases in $< 1.5$ seconds.

---

## 3. Capability Invocation Telemetry Architecture

With ambient lifecycle hooks, universal protocol rules, and Intent directives embedded internally and delivered Just-In-Time by `nomos handshake`, tracking local CLI and directive utilization across planes is surfaced through `nomos capabilities`.

### 3.1 Relational Storage & Dynamic Resolution
Database locations resolve dynamically through configuration discovery (`config.ResolveGlobalDBPath()` and `config.ResolveMnemeDBPath()`). Local invocation metrics are persisted in the `telemetry_capabilities` table of `nomos_global.db`:

```sql
CREATE TABLE IF NOT EXISTS telemetry_capabilities (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    plane            TEXT NOT NULL DEFAULT 'substrate',
    type             TEXT NOT NULL DEFAULT 'cli_command',
    enabled          INTEGER NOT NULL DEFAULT 1,
    invocation_count INTEGER NOT NULL DEFAULT 0,
    last_accessed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_telemetry_capabilities_plane ON telemetry_capabilities(plane);
CREATE INDEX IF NOT EXISTS idx_telemetry_capabilities_type ON telemetry_capabilities(type);
```

### 3.2 In-Line Interface Contracts & Go DTO Structs

The Substrate implements non-blocking capability invocation recording in `substrate/src/nomos/modules/telemetry/registry.go`:

```go
// CapabilityMetrics represents the persisted invocation metrics for a capability (#CapabilityMetrics).
type CapabilityMetrics struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Plane           string `json:"plane"`
	Type            string `json:"type"`
	Enabled         bool   `json:"enabled"`
	InvocationCount int    `json:"invocation_count"`
	LastAccessedAt  string `json:"last_accessed_at"`
}
```

JSON payload serialized by `nomos capabilities --json`:

```json
{
  "id": "cmd-verify",
  "name": "nomos verify",
  "plane": "substrate",
  "type": "cli_command",
  "enabled": true,
  "invocation_count": 42,
  "last_accessed_at": "2026-11-03T18:00:00Z"
}
```

### 3.3 Canonical Recording Entrypoints
1. `RecordInvocationSafe(dbPath, id)`: Increments substrate `cli_command` usage counters non-blockingly.
2. `RecordWorkflowInvocationSafe(dbPath, id, name, capType)`: Increments intent-plane orientation/directive capability counters (such as `nomos handshake`).
3. `GetCapabilityMetrics(dbPath)`: Reads aggregated `#CapabilityMetrics` records surfaced by `nomos capabilities [--json]`.

---

## 4. The 3-Pass Progressive Developer Journey

To eliminate cognitive overload, the onboarding path is partitioned into three progressive passes:

| Pass | Target Audience | Primary Tooling | Key Competency |
| :--- | :--- | :--- | :--- |
| **Pass 1: Solo Quickstart** | Individual developer exploring Nomos | `init`, `task chore`, `verify`, `task sync` | Safe worktree sandboxing, trunk protection, and 42 DoD gates. |
| **Pass 2: Team Flow & Receipts** | Engineering teams shipping production code | `status`, `capabilities`, `mem recall`, Git Notes | Cryptographic release receipts, semantic memory recall, Intent-to-Substrate parity. |
| **Pass 3: Deep Customization** | Platform engineers, architects, DevSecOps | `nomos.yaml` (`nomos.schema.json`), `hook`, plugins, CapBAC | Shaping enterprise guardrails, AST thresholds, and lifecycle hooks. |

---

## 5. Fail-Closed Pre-Flight Gating & Transactional Rollback (Law 7)

1. **Atomic Chore Provisioning**: When executing `nomos task chore <title>`:
   - If worktree creation fails after the task record is created in SQLite, the command aborts fail-closed without leaving an unleased `EDIT` state.
   - The phase state machine does not transition to `EDIT` unless physical worktree boundaries are verified clean.
2. **Fail-Safe Telemetry Recording**:
   - Capability telemetry ingestion into `telemetry_capabilities` executes via `telemetry.RecordInvocationSafe` and `telemetry.RecordWorkflowInvocationSafe`.
   - If SQLite write lock contention or an invalid path occurs, telemetry emission degrades silently without panicking or blocking user operations.

---

## 6. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-LAUNCH-001` | Pre-Commit Hook | Direct `git commit` mutation on protected `main` branch | Intercepted with exit code 1; output contains remedy `nomos task chore "<title>"` | Law 3 (Hermetic Worktrees) & Law 7 |
| `TC-LAUNCH-002` | `nomos task chore` | `nomos task chore "Fix parser edge case"` | Task created with `SUB-` key, transient worktree mounted, phase transitioned to `EDIT` | Law 3 & Law 6 (Canonical Singularity) |
| `TC-LAUNCH-003` | `RecordInvocationWithDetails` | `RecordWorkflowInvocationSafe(dbPath, "wf-handshake", "Nomos Handshake", "human_workflow")` | Row persisted in `telemetry_capabilities` matching `telemetry.CapabilityMetrics` | Law 4 (Compiled Go SSoT) |
| `TC-LAUNCH-004` | Telemetry Ingestion Failure | Unwritable database path | Function returns gracefully without application panic | Fail-Safe Telemetry |

---

## 7. Verification Boundaries & DoD Evidence

1. **Telemetry Registry Assertion**: The `telemetry_capabilities` table is initialized in `storage.SchemaDDL` and verified in `telemetry/registry_test.go`.
2. **Zero-Overhead Ingestion**: Event emission maintains sub-millisecond overhead ($< 1$ms per write).
3. **Markdown Integrity**: All markdown relative links resolve deterministically via `BrokenWireGate`.
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
