package storage

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMaintainRuntimeStorage_CapsTablesAndRotatesLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NOMOS_HOME", home)

	db, err := OpenDB("")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE journal (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create legacy journal: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE telemetry_intent_activity (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create legacy telemetry_intent_activity: %v", err)
	}
	if err := InitSchema(db); err != nil {
		t.Fatalf("InitSchema: %v", err)
	}
	var retiredCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('journal', 'telemetry_intent_activity')`).Scan(&retiredCount); err != nil || retiredCount != 0 {
		t.Fatalf("expected legacy journal and telemetry_intent_activity tables to be dropped, got count=%d err=%v", retiredCount, err)
	}
	_ = db.Close()

	// Create oversized log file in ~/.nomos/data/logs
	logsDir := filepath.Join(home, "data", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	bigLog := filepath.Join(logsDir, "runtime.log")
	payload := bytes.Repeat([]byte("a"), 2048)
	if err := os.WriteFile(bigLog, payload, 0644); err != nil {
		t.Fatalf("write bigLog: %v", err)
	}
	if rotated := RotateDirectoryLogs(logsDir, 1024, 256); rotated != 1 {
		t.Fatalf("expected 1 log rotated, got %d", rotated)
	}
	fi, err := os.Stat(bigLog)
	if err != nil || fi.Size() != 256 {
		t.Fatalf("expected truncated live log of 256 bytes, got %v (err=%v)", fi, err)
	}
	if _, bErr := os.Stat(bigLog + ".1"); bErr != nil {
		t.Fatalf("expected .1 rotated archive to exist: %v", bErr)
	}

	rep, err := MaintainRuntimeStorage(true)
	if err != nil || !rep.WALCheckpointed {
		t.Fatalf("expected MaintainRuntimeStorage success, got rep=%+v err=%v", rep, err)
	}
	if ShouldRunPeriodicMaintenance(false) {
		t.Fatalf("expected periodic maintenance to throttle within 24h window")
	}
}

func TestMaintainRuntimeStorage_PrunesExpiredLeasesAndRetiredDBs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("NOMOS_HOME", home)

	db, err := OpenDB("")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO tasks (key, plane, title, created_at) VALUES ('SUB-1', 'substrate', 'Test task', '2026-10-30 12:00:00')`); err != nil {
		t.Fatalf("insert task: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := db.Exec(`INSERT INTO leases (lease_id, task_key, valid) VALUES (?, 'SUB-1', 0)`, fmt.Sprintf("lease-%d", i)); err != nil {
			t.Fatalf("insert lease: %v", err)
		}
	}
	if pruned := capExpiredLeases(db, 2); pruned != 4 {
		t.Fatalf("expected 4 expired leases pruned, got %d", pruned)
	}
	normalizeLegacyTimestamps(db)
	var createdAt string
	if err := db.QueryRow(`SELECT created_at FROM tasks WHERE key = 'SUB-1'`).Scan(&createdAt); err != nil || createdAt != "2026-10-30T12:00:00Z" {
		t.Fatalf("expected normalized RFC3339 timestamp, got %q (err=%v)", createdAt, err)
	}
	_ = db.Close()

	retired := filepath.Join(home, "data", "db", "gitgudbrain.db")
	if err := os.WriteFile(retired, []byte("legacy"), 0644); err != nil {
		t.Fatalf("write retired db: %v", err)
	}
	rep, err := MaintainRuntimeStorage(true)
	if err != nil || rep.LegacyDBFilesPurged != 1 {
		t.Fatalf("expected 1 retired DB file purged, got rep=%+v err=%v", rep, err)
	}
}
