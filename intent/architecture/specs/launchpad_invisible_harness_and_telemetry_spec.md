---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-12"
last_updated: "2026-11-06"
lineage:
  - INT-944
  - INT-953
  - SUB-1890
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Launchpad: Invisible Harness & Capability Telemetry Specification"
type: spec
version: "4.8.0"
---

# Nomos Launchpad: Invisible Harness & Capability Telemetry Specification

---

## 1. Executive Summary & Problem Statement

The Nomos open-core developer experience satisfies two fundamental architectural requirements:
1. **Total Invariance & Zero Trust**: Strict machine enforcement of 42 Definition of Done (DoD) quality gates, trunk locking (Law 3), and AST fitness proofs (Law 2).
2. **Invisible Ergonomics**: Developers and Tier 1 agents (AGY-IDE, Claude Code, Cursor, opencode) experience zero friction, CLI nagging, or cognitive overhead during iteration.

This specification formalizes the **Invisible Harness Paradigm** and the **Capability Invocation Telemetry Architecture**.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Ambient hooks orient Tier 1 agents automatically without manual workflow ceremony. | `nomos hook session-start`, `nomos handshake` |
| **2** | **The Non-Verbal Proof Law** | Parallel compiled Go routines evaluate all 42 DoD probes under 1.5s. | `nomos verify` (`Pipeline.Execute`) |
| **3** | **The Hermetic Worktree Law** | Direct trunk mutations intercept cleanly and direct callers to atomic worktree provisioning. | `pre-commit` hook, `nomos task chore` |
| **4** | **The Single Source of Truth Law** | Telemetry persistence binds directly to compiled `telemetry.CapabilityMetrics` Go contracts. | `substrate/src/nomos/modules/telemetry/registry.go` |
| **5** | **The Zero-Hardcoded-Path Law** | Database and configuration paths resolve dynamically at runtime. | `config.ResolveGlobalDBPath()`, `config.ResolveConfigPath()` |
| **6** | **The Canonical Singularity Law** | Zero custom slash commands, duplicate workflow playbooks, or CLI aliases exist. | `Cell 6.2 (zero_alias)` |
| **7** | **The Fail-Closed Pre-Flight Law** | Worktree provisioning failures roll back cleanly before entering `EDIT`; telemetry degrades safely. | Transactional task/worktree guard |

---

## 3. The Invisible Harness Architecture

The Invisible Harness ensures that Nomos operates silently in the background with **zero custom slash commands, zero conversational workflow playbooks, and zero skill walls**:
- **Zero-Config Bootstrap (`nomos init`) & Hook Install (`nomos hook install`)**: `nomos init` scaffolds `AGENTS.md`, `CLAUDE.md`, `.agents/hooks.json`, `.opencode/plugins/nomos.ts`, and `nomos.yaml` in $< 2$ seconds. It also prunes legacy `.agents/workflows/` files and configures `refs/notes/nomos/releases` push refspecs. `nomos hook install` merges `session-start`, `pre-tool`, `post-tool`, and `stop` hooks into `--user` or project manifests additively.
- **Ambient Defense Hooks (`nomos hook`)**: Executes `nomos handshake` on start, guards pre-tool boundaries and egress, runs pure-Go post-tool syntax checks, and gates `stop`. In OpenCode, `.opencode/plugins/nomos.ts` binds `"tool.execute.before"` and `"tool.execute.after"` to `nomos hook` in $< 5\text{ms}$ via `spawnSync`. Denied mutations throw an `Error` with `nomos task chore "<title>"`, and post-tool advisories append to `output.output`.
- **One-Step Maintenance Chores (`nomos task chore`)**: Incepts the task, registers it in the SQLite Living Ledger, provisions a transient worktree, and enters the `EDIT` phase atomically.
- **Microsecond DoD Evaluations**: All 42 DoD quality gates execute in parallel compiled Go routines, verifying entire codebases in $< 1.5$ seconds.

---

## 4. Capability Invocation Telemetry Architecture

`nomos handshake` delivers embedded lifecycle hooks, protocol rules, and Just-In-Time directives while `nomos capabilities` surfaces local CLI and directive utilization across both planes.

### 4.1 Relational Storage & Dynamic Resolution
Database locations resolve dynamically through `config.ResolveGlobalDBPath()` and `config.ResolveMnemeDBPath()`. `nomos_global.db` persists local invocation metrics in the `telemetry_capabilities` table:

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

### 4.2 In-Line Interface Contracts & Go DTO Structs

The Substrate implements non-blocking capability invocation recording in `substrate/src/nomos/modules/telemetry/registry.go`:

```go
package telemetry

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

### 4.3 Canonical Recording Entrypoints
1. `RecordInvocationSafe(dbPath, id)`: Increments substrate `cli_command` usage counters non-blockingly.
2. `RecordWorkflowInvocationSafe(dbPath, id, name, capType)`: Increments intent-plane orientation/directive capability counters (such as `nomos handshake`).
3. `GetCapabilityMetrics(dbPath)`: Reads aggregated `#CapabilityMetrics` records surfaced by `nomos capabilities [--json]`.

---

## 5. The 3-Pass Progressive Developer Journey

Nomos partitions developer onboarding into three progressive passes to eliminate cognitive overload:

| Pass | Target Audience | Primary Tooling | Key Competency |
| :--- | :--- | :--- | :--- |
| **Pass 1: Solo Quickstart** | Individual developer exploring Nomos | `init`, `task chore`, `verify`, `task sync` | Safe worktree sandboxing, trunk protection, and 42 DoD gates. |
| **Pass 2: Team Flow & Receipts** | Engineering teams shipping production code | `status`, `capabilities`, `mem recall`, Git Notes | Cryptographic release receipts, semantic memory recall, Intent-to-Substrate parity. |
| **Pass 3: Deep Customization** | Platform engineers, architects, DevSecOps | `nomos.yaml` (`nomos.schema.json`), `hook`, plugins, CapBAC | Shaping enterprise guardrails, AST thresholds, and lifecycle hooks. |

---

## 6. Fail-Closed Pre-Flight Gating & Transactional Rollback (Law 7)

1. **Atomic Chore Provisioning**: When executing `nomos task chore <title>`:
   - If worktree provisioning errors after SQLite task insertion, the command aborts fail-closed without leaving an unleased `EDIT` state.
   - The phase state machine enters `EDIT` only after verifying physical worktree boundaries.
2. **Fail-Safe Telemetry Recording**:
   - Capability telemetry ingestion into `telemetry_capabilities` executes via `telemetry.RecordInvocationSafe` and `telemetry.RecordWorkflowInvocationSafe`.
   - If SQLite write contention or an invalid path occurs, telemetry emission degrades silently without panicking or blocking user operations.

---

## 7. Table-Driven Verification Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-LAUNCH-001` | Pre-Commit Hook | Direct `git commit` mutation on protected `main` branch | Intercepted with exit code 1; output contains remedy `nomos task chore "<title>"` | Law 3 (Hermetic Worktrees) & Law 7 |
| `TC-LAUNCH-002` | `nomos task chore` | `nomos task chore "Fix parser edge case"` | Task created with `SUB-` key, transient worktree mounted, phase transitioned to `EDIT` | Law 3 & Law 6 (Canonical Singularity) |
| `TC-LAUNCH-003` | `RecordInvocationWithDetails` | `RecordWorkflowInvocationSafe(dbPath, "wf-handshake", "Nomos Handshake", "human_workflow")` | Row persisted in `telemetry_capabilities` matching `telemetry.CapabilityMetrics` | Law 4 (Compiled Go SSoT) |
| `TC-LAUNCH-004` | Telemetry Ingestion Failure | Unwritable database path | Function returns gracefully without application panic | Fail-Safe Telemetry |
| `TC-LAUNCH-005` | `nomos init` OpenCode Scaffolding | Workspace containing custom `.opencode/plugins/custom.ts` | Idempotently writes `.opencode/plugins/nomos.ts` while preserving `custom.ts` byte-for-byte | Law 1 & Law 6 |
| `TC-LAUNCH-006` | `.opencode/plugins/nomos.ts` Runtime | OpenCode `tool.execute.before` and `tool.execute.after` events | Invokes `nomos hook pre-tool` and `nomos hook post-tool` in `<5ms`; throws `Error` on `deny` | Law 1 & Law 3 |
| `TC-LAUNCH-007` | `nomos hook install --claude --user` | Existing `$CLAUDE_CONFIG_DIR/settings.json` with custom user hooks | Additively merges `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` without clobbering existing entries | Law 1 & Law 6 |
| `TC-LAUNCH-008` | `EnsureHooksInstalled` Push Refspec | Repository with `remote.origin.url` configured | Additively ensures `refs/notes/nomos/releases:refs/notes/nomos/releases` push refspec idempotently | Law 2 & Law 4 |

---

## 8. Cross-Plane Traceability & DoD Verification

- **Substrate SSoT**: `substrate/src/nomos/modules/telemetry/registry.go`, `substrate/src/nomos/core/storage/`, `substrate/cli/` (`init`, `hook`, `capabilities`, `task chore`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 1.2 (phase_discipline)`, `Cell 3.2 (hermetic_worktree)`, `Cell 4.1 (broken_wire)`, `Cell 6.2 (zero_alias)`.
- **Deterministic Verification**:
  1. `storage.SchemaDDL` initializes `telemetry_capabilities` with sub-millisecond (`< 1ms`) non-blocking ingestion verified by `telemetry/registry_test.go`.
  2. `nomos verify` evaluates all 42 Definition of Done quality gates with exit code `0` and zero regressions.
