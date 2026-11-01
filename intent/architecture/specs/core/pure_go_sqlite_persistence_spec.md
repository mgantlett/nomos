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
The SQLite schema must maintain 1:1 structural parity with authoritative CUE contracts:
- `tasks` table maps to `#TaskRecord` (`intent/contracts/cue/task.cue`).
- `leases` table maps to `#CapBACLease` (`intent/contracts/cue/capbac.cue`).
- `journal` table maps to `#SessionJournalEvent` (`intent/contracts/cue/session_journal.cue`).

### `INV-SQL-003`: Dynamic Database Resolution (Law 5: Zero-Hardcoded-Path)
Database filesystem locations are strictly forbidden from appearing as string literals in source code. All paths must resolve dynamically via `config.ResolveGlobalDBPath(repoRoot)`.

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
    key          TEXT PRIMARY KEY,
    slug         TEXT NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    plane        TEXT NOT NULL CHECK(plane IN ('intent', 'substrate')),
    status       TEXT NOT NULL CHECK(status IN ('todo', 'in_progress', 'blocked', 'done', 'triage')),
    dependencies TEXT NOT NULL DEFAULT '[]',
    phase        TEXT NOT NULL DEFAULT 'IDLE',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_plane ON tasks(plane);
```

### 3.2 `leases` Table Schema (CapBAC Object Capabilities)
```sql
CREATE TABLE IF NOT EXISTS leases (
    lease_id    TEXT PRIMARY KEY,
    task_key    TEXT NOT NULL,
    phase       TEXT NOT NULL,
    step_budget INTEGER NOT NULL,
    step_count  INTEGER NOT NULL DEFAULT 0,
    acquired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at  DATETIME NOT NULL,
    status      TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')),
    FOREIGN KEY(task_key) REFERENCES tasks(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_leases_task_key ON leases(task_key);
CREATE INDEX IF NOT EXISTS idx_leases_status ON leases(status);
```

### 3.3 `journal` Table Schema (Deterministic Session Audit)
```sql
CREATE TABLE IF NOT EXISTS journal (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_key   TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    timestamp  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(task_key) REFERENCES tasks(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_journal_task_key ON journal(task_key);
```

---

## 4. Table-Driven Protocol Compliance Vectors

| Compliance Vector | Invariant | Substrate Implementation | Expected Behavior |
| :--- | :--- | :--- | :--- |
| `TC-SQL-01` | Law 2 (`CGO_ENABLED=0`) | `modernc.org/sqlite` | Compiles with CGO disabled; exit 0 |
| `TC-SQL-02` | Law 6 (Task Key Singularity) | Table check constraint | Rejects non-conforming keys (`ERR_CANONICAL_VIOLATION`) |
| `TC-SQL-03` | Law 5 (Zero-Hardcoded-Path) | `paths.go` DB resolver | Path dynamically resolves to `.nomos/data/db/` or config override |
| `TC-SQL-04` | Law 7 (Fail-Closed) | Foreign key pragma | Foreign key mismatch rejects transaction (`ERR_FOREIGN_KEY_VIOLATION`) |

