---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
lineage:
  - INT-732
  - SUB-1209
plane: intent
ssot_repository: nomos
status: active
title: "Pure-Go SQLite Persistence and Task State Store Specification"
type: spec
version: "4.0.0"
---

# Pure-Go SQLite Persistence and Task State Store Specification

---

## 1. Executive Summary & Problem Statement

This specification codifies pure-Go SQLite persistence contracts and schemas in Nomos v4 (`INT-732` / `SUB-1209`). It governs task lifecycles, CapBAC leases, and capability telemetry without CGO dependencies.

Nomos v3 implemented SQLite persistence via `mattn/go-sqlite3`. While feature-complete, this dependency introduced severe architectural liability:
1. **CGO Compilation Bottlenecks**: Required GCC/Clang toolchains, causing multi-second compilation latency, cross-compilation failures on non-standard architectures, and complex dynamic linking.
2. **Memory Leaks and Thread Contention**: CGO boundary crossing introduced non-deterministic thread contention in high-frequency gate evaluation loops.
3. **In-Memory Volatility in Early v4**: Initial v4 bootstrap builds held tasks in an in-memory stub (`MemoryTaskStore`), losing state across CLI invocations.

Nomos v4 transitions persistence to **pure Go** via `modernc.org/sqlite` in `substrate/src/nomos/core/storage/`.

---

## 2. Axiomatic Invariants & Governing Laws

### 2.1 `INV-SQL-001`: Pure-Go CGO-Free Driver (Law 2: Non-Verbal Proof)
The Nomos database subsystem must compile with `CGO_ENABLED=0`. All database operations must execute via `modernc.org/sqlite` without foreign C compiler toolchains or shared object dependencies.

### 2.2 `INV-SQL-002`: Relational-Go SSoT Schema Parity (Law 4: SSoT)
The SQLite schema (`storage.SchemaDDL`) must maintain 1:1 structural parity with authoritative compiled Go contracts:
- `tasks` table maps to `task.TaskRecord` (`substrate/src/nomos/modules/task/task.go`).
- `leases` table maps to `storage.SaveLease`, `storage.GetActiveLease`, and `storage.RevokeLease` (`substrate/src/nomos/core/storage/leases.go`).
- `telemetry_capabilities` table maps to `telemetry.CapabilityMetrics` (`substrate/src/nomos/modules/telemetry/registry.go`).

### 2.3 `INV-SQL-003`: Dynamic Database Resolution (Law 5: Zero-Hardcoded-Path)
The substrate prohibits string literals for database filesystem paths in source code. All paths must resolve dynamically via `config.ResolveGlobalDBPath()`.

### 2.4 `INV-SQL-004`: Atomic Fail-Closed Transactions (Law 7: Fail-Closed Pre-Flight)
All task mutations and phase transitions must execute inside serializable SQLite transactions:
- Database must initialize in Write-Ahead Logging (`PRAGMA journal_mode=WAL;`).
- The connection enforces foreign key integrity (`PRAGMA foreign_keys=ON;`).
- The connection sets busy timeouts to at least 5000ms (`PRAGMA busy_timeout=5000;`) to eliminate write contention.

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

## 4. Compiled Go SSoT Contracts

The authoritative pure-Go database, CapBAC lease, and telemetry contracts reside in `substrate/src/nomos/core/storage/` and `substrate/src/nomos/modules/telemetry/`:

```go
package storage

import "database/sql"

// SchemaDDL defines the authoritative relational SQLite schema (Law 4 SSoT).
const SchemaDDL = `...`

// OpenDB opens a pure-Go SQLite connection pool with WAL and foreign key pragmas.
func OpenDB(dbPath string) (*sql.DB, error)

// InitSchema executes idempotent table definitions, indexes, and runtime pragmas.
func InitSchema(db *sql.DB) error

// SaveLease records or updates a CapBAC capability lease in SQLite.
func SaveLease(db *sql.DB, leaseID, taskKey, phase string, stepBudget, tokensUsed int) error

// GetActiveLease retrieves the active capability lease for a task.
func GetActiveLease(db *sql.DB, taskKey string) (string, string, int, int, error)

// RevokeLease marks all leases for a task key as invalid.
func RevokeLease(db *sql.DB, taskKey string) error
```

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

---

## 5. Table-Driven Verification Vectors

| Compliance Vector | Invariant | Substrate Implementation | Expected Behavior |
| :--- | :--- | :--- | :--- |
| `TC-SQL-01` | Law 2 (`CGO_ENABLED=0`) | `modernc.org/sqlite` | Compiles with CGO disabled; exit 0 |
| `TC-SQL-02` | Law 6 (Task Key Singularity) | Table check constraint | Rejects non-conforming keys (`ERR_CANONICAL_VIOLATION`) |
| `TC-SQL-03` | Law 5 (Zero-Hardcoded-Path) | `paths.go` DB resolver | Path dynamically resolves to `.nomos/data/db/` or config override |
| `TC-SQL-04` | Law 7 (Fail-Closed) | Foreign key pragma | Foreign key mismatch rejects transaction (`ERR_FOREIGN_KEY_VIOLATION`) |

---

## 6. Cross-Plane Traceability & DoD Verification

1. **Compiled Go SSoT Paths:**
   - `substrate/src/nomos/core/storage/db.go` (`SchemaDDL`, `OpenDB`, `InitSchema`) and `substrate/src/nomos/core/storage/maintain.go`.
   - `substrate/src/nomos/core/storage/leases.go` (`SaveLease`, `GetActiveLease`, `RevokeLease`).
   - `substrate/src/nomos/modules/task/task.go` (`TaskRecord`) and `substrate/src/nomos/modules/telemetry/registry.go` (`CapabilityMetrics`).
2. **7x3 Invariant Matrix Coordinates:**
   - `Cell 1.3 (capbac_lease)` verifies active SQLite CapBAC task leases across mutating phases.
   - `Cell 5.1 (forbidden_literals)` and `Cell 6.3 (canonical_singularity)` enforce dynamic DB path resolution and `INT-<N>` / `SUB-<N>` key singularity.
   - `Cell 2.2 (go_toolchain, coverage)` and `Cell 7.2 (unchecked_errors)` enforce `CGO_ENABLED=0` unit execution and fail-closed SQLite error handling.
3. **Deterministic Verification:**
   - Execute `nomos verify` across all 42 quality gates and assert exit code `0`.
