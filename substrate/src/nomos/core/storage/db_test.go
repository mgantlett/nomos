package storage

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenDB(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_nomos.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing db: %v", closeErr)
		}
	}()

	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys;").Scan(&fkEnabled); err != nil {
		t.Fatalf("failed checking foreign keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Errorf("expected foreign_keys=1, got %d", fkEnabled)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("failed checking busy timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("expected busy_timeout=5000, got %d", busyTimeout)
	}
}

func TestInitSchema_Idempotency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_idempotent.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing db: %v", closeErr)
		}
	}()

	if err := InitSchema(db); err != nil {
		t.Fatalf("repeated InitSchema failed: %v", err)
	}
}

func TestOpenDB_InvalidPath(t *testing.T) {
	// Root or invalid read-only directory to verify fail-closed behavior
	_, err := OpenDB("/proc/nonexistent/nomos.db")
	if err == nil {
		t.Errorf("expected error when opening database in invalid path")
	}
}

func TestMigrateLegacyColumns(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy_v3.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing db: %v", closeErr)
		}
	}()

	// Query newly migrated column to verify it exists
	var schemaType string
	query := "SELECT schema_type FROM tasks WHERE key = 'nonexistent'"
	err = db.QueryRow(query).Scan(&schemaType)
	// ErrNoRows proves the column exists in table definition!
	if err == nil || !strings.Contains(err.Error(), "no rows") {
		// If column was missing, SQLite would error: "no column named schema_type"
		if strings.Contains(err.Error(), "no column named") {
			t.Fatalf("schema_type column was not migrated: %v", err)
		}
	}
}

func TestInitSchema_UserVersionFastPath(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fastpath.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing db: %v", closeErr)
		}
	}()

	var uv int
	if err := db.QueryRow("PRAGMA user_version;").Scan(&uv); err != nil {
		t.Fatalf("query user_version: %v", err)
	}
	if uv != currentSchemaUserVersion {
		t.Fatalf("expected user_version=%d, got %d", currentSchemaUserVersion, uv)
	}
	if !isSchemaAtCurrentVersion(db) {
		t.Fatalf("expected isSchemaAtCurrentVersion to return true after OpenDB")
	}

	db2, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("second OpenDB failed: %v", err)
	}
	if closeErr := db2.Close(); closeErr != nil {
		t.Fatalf("close second OpenDB: %v", closeErr)
	}
}

func TestMigrateLegacyColumns_SurfacesRealSQLiteError(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "broken_migration.db"))
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec("DROP TABLE tasks;"); err != nil {
		t.Fatalf("drop tasks: %v", err)
	}
	if err := migrateLegacyColumns(db); err == nil {
		t.Fatalf("expected migrateLegacyColumns to return error when tasks table is missing")
	}
}

