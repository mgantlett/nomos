// Package storage provides pure-Go SQLite connection pooling, schema initialization,
// and relational persistence primitives conforming to authoritative CUE contracts.
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	_ "modernc.org/sqlite"
)

// SchemaDDL defines the authoritative relational schema matching CUE contracts.
const SchemaDDL = `
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

CREATE TABLE IF NOT EXISTS journal (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_key   TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '{}',
    timestamp  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    FOREIGN KEY(task_key) REFERENCES tasks(key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_journal_task_key ON journal(task_key);
CREATE INDEX IF NOT EXISTS idx_journal_timestamp ON journal(timestamp);

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

CREATE TABLE IF NOT EXISTS telemetry_intent_activity (
    id           TEXT PRIMARY KEY,
    timestamp    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    seat         TEXT NOT NULL,
    phase        TEXT NOT NULL,
    active_task  TEXT DEFAULT '',
    directive_id TEXT NOT NULL,
    event_type   TEXT NOT NULL,
    details_json TEXT DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_telemetry_intent_task ON telemetry_intent_activity(active_task);
CREATE INDEX IF NOT EXISTS idx_telemetry_intent_directive ON telemetry_intent_activity(directive_id);
CREATE INDEX IF NOT EXISTS idx_telemetry_intent_timestamp ON telemetry_intent_activity(timestamp);
`

// OpenDB opens a pure-Go SQLite connection pool with WAL and foreign key pragmas.
func OpenDB(dbPath string) (*sql.DB, error) {
	target := dbPath
	if target == "" {
		target = config.ResolveGlobalDBPath()
	}
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed to create database directory %s: %v", dir, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", target)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed to open sqlite database at %s: %v", target, err)
	}

	if err := pingAndInit(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, types.NewNomosError(types.ErrCodeSubstrate, "%v (close error: %v)", err, closeErr)
		}
		return nil, err
	}
	return db, nil
}

func pingAndInit(db *sql.DB) error {
	if err := db.Ping(); err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "database ping failed: %v", err)
	}
	return InitSchema(db)
}

// InitSchema executes idempotent table definitions, indexes, and runtime pragmas.
func InitSchema(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA busy_timeout = 5000;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return types.NewNomosError(types.ErrCodeInvariant, "failed to apply pragma %s: %v", pragma, err)
		}
	}

	if _, err := db.Exec(SchemaDDL); err != nil {
		return types.NewNomosError(types.ErrCodeInvariant, "failed to initialize sqlite schema: %v", err)
	}
	return migrateLegacyColumns(db)
}

func migrateLegacyColumns(db *sql.DB) error {
	migrations := []string{
		"ALTER TABLE tasks ADD COLUMN project TEXT DEFAULT '';",
		"ALTER TABLE tasks ADD COLUMN schema_type TEXT NOT NULL DEFAULT 'substrate';",
		"ALTER TABLE tasks ADD COLUMN tier TEXT NOT NULL DEFAULT 'T1';",
		"ALTER TABLE tasks ADD COLUMN priority INTEGER NOT NULL DEFAULT 50;",
		"ALTER TABLE tasks ADD COLUMN completed_at TEXT;",
		"ALTER TABLE tasks ADD COLUMN dependencies TEXT NOT NULL DEFAULT '[]';",
		"ALTER TABLE tasks ADD COLUMN twin_key TEXT DEFAULT '';",
		"ALTER TABLE tasks ADD COLUMN arc_id TEXT DEFAULT '';",
		"ALTER TABLE tasks ADD COLUMN intent_delta TEXT DEFAULT 'none';",
		"ALTER TABLE tasks ADD COLUMN burden INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE tasks ADD COLUMN context_burden INTEGER NOT NULL DEFAULT 0;",
		"ALTER TABLE tasks ADD COLUMN arch_layer INTEGER NOT NULL DEFAULT 0;",
		"CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);",
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				// safe to continue
			}
		}
	}
	return nil
}
