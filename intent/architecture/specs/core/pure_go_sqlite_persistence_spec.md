---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - INT-732
    - SUB-1209
title: Pure-Go SQLite Persistence and Task State Store Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Pure-Go SQLite Persistence and Task State Store Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify architectural contracts and formal data schemas for the deterministic pure-Go SQLite persistence layer in Nomos v4, governing global task lifecycles, CapBAC capability leases, and session journals without CGO dependencies.

---

## 1. Context & Problem Statement

In Nomos v3, SQLite persistence was implemented using `mattn/go-sqlite3`. While feature-complete, this dependency introduced severe architectural liability:
1. **CGO Compilation Bottlenecks**: Required GCC/Clang toolchains, causing multi-second compilation latency, cross-compilation failures on non-standard architectures, and complex dynamic linking.
2. **Memory Leaks and Thread Contention**: CGO boundary crossing introduced non-deterministic thread contention in high-frequency gate evaluation loops.
3. **In-Memory Volatility in Early v4**: In the initial v4 bootstrap, tasks were temporarily stored in an in-memory stub (`MemoryTaskStore`), losing state across CLI command invocations.

Nomos v4 transitions persistence to **pure Go** via `modernc.org/sqlite`.

---

## 2. Invariant Contracts

### `INV-SQL-001`: Pure-Go CGO-Free Driver (Law 2: Non-Verbal Proof)
The Nomos database subsystem must compile with `CGO_ENABLED=0`. All database operations must execute via `modernc.org/sqlite` without foreign C compiler toolchains or shared object dependencies.

### `INV-SQL-002`: Relational-CUE Schema Parity (Law 4: SSoT)
The SQLite schema must maintain 1:1 structural parity with authoritative contracts:
- `tasks` table maps to `task.TaskRecord` (`#TaskRecord` in `intent/contracts/cue/task.cue`).
- `leases` table maps to `storage.LeaseRecord` (`#CapBACLease` in `intent/contracts/cue/capbac.cue`).
- `telemetry_capabilities` table maps to `telemetry.CapabilityMetrics` (`#CapabilityMetrics` in `intent/contracts/cue/telemetry.cue`).

### `INV-SQL-003`: Dynamic Database Resolution (Law 5: Zero-Hardcoded-Path)
Database filesystem locations are strictly forbidden from appearing as string literals in source code. All paths must resolve dynamically via `config.ResolveGlobalDBPath()`.

### `INV-SQL-004`: Atomic Fail-Closed Transactions (Law 7: Fail-Closed Pre-Flight)
All task mutations and phase transitions must execute inside serializable SQLite transactions:
- Database must initialize in Write-Ahead Logging (`PRAGMA journal_mode=WAL;`).
- Foreign keys must be strictly enforced (`PRAGMA foreign_keys=ON;`).
- Busy timeouts must be configured to at least 5000ms (`PRAGMA busy_timeout=5000;`) to eliminate lock contention.

---

## 3. Formal Relational Schema Specification

### 3.1 `tasks` Table Schema
```sql
CREATE TABLE IF NOT EXISTS tasks (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    key            TEXT NOT NULL UNIQUE,
    plane          TEXT NOT NULL CHECK(plane IN ('intent', 'substrate')),
    project        TEXT NOT NULL DEFAULT '',
    schema_type    TEXT NOT NULL DEFAULT 'substrate',
    title          TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'BACKLOG',
    tier           TEXT NOT NULL DEFAULT 'T1',
    priority       INTEGER NOT NULL DEFAULT 50,
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    completed_at   TEXT,
    dependencies   TEXT NOT NULL DEFAULT '[]',
    twin_key       TEXT DEFAULT '',
    arc_id         TEXT DEFAULT '',
    intent_delta   TEXT DEFAULT 'none',
    burden         INTEGER NOT NULL DEFAULT 0,
    context_burden INTEGER NOT NULL DEFAULT 0,
    arch_layer     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tasks_key ON tasks(key);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_plane ON tasks(plane);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);
```

### 3.2 `leases` Table Schema (CapBAC Object Capabilities)
```sql
CREATE TABLE IF NOT EXISTS leases (
    lease_id    TEXT PRIMARY KEY,
    task_key    TEXT NOT NULL,
    phase       TEXT NOT NULL DEFAULT 'IDLE',
    step_budget INTEGER NOT NULL DEFAULT 50,
    tokens_used INTEGER NOT NULL DEFAULT 0,
    issued_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    valid       INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY(task_key) REFERENCES tasks(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_leases_task_key ON leases(task_key);
CREATE INDEX IF NOT EXISTS idx_leases_valid ON leases(valid);
```

### 3.3 `telemetry_capabilities` Table Schema (Local Capability Utilization)
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

---

## 4. Table-Driven Protocol Compliance Vectors

| Compliance Vector | Invariant | Substrate Implementation | Expected Behavior |
| :--- | :--- | :--- | :--- |
| `TC-SQL-01` | Law 2 (`CGO_ENABLED=0`) | `modernc.org/sqlite` | Compiles with CGO disabled; exit 0 |
| `TC-SQL-02` | Law 6 (Task Key Singularity) | Table check constraint | Rejects non-conforming keys (`ERR_CANONICAL_VIOLATION`) |
| `TC-SQL-03` | Law 5 (Zero-Hardcoded-Path) | `paths.go` DB resolver | Path dynamically resolves to `.nomos/data/db/` or config override |
| `TC-SQL-04` | Law 7 (Fail-Closed) | Foreign key pragma | Foreign key mismatch rejects transaction (`ERR_FOREIGN_KEY_VIOLATION`) |

