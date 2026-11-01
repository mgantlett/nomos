// Package storage implements automatic retention, WAL checkpointing, freelist VACUUM, and log rotation for Nomos runtime data.
package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

const (
	defaultMaxJournalRows    = 5000
	defaultMaxTelemetryRows  = 5000
	defaultMaxExpiredLeases  = 200
	defaultMinVacuumBytes    = 2 * 1024 * 1024
	defaultMaxLogBytes       = 10 * 1024 * 1024
	defaultTailKeepBytes     = 1 * 1024 * 1024
	maintenanceInterval      = 24 * time.Hour
	legacyDirMaxAge          = 24 * time.Hour
	stampFileName            = "last_maintenance.stamp"
	dirLogsName              = "logs"
)

// RuntimeMaintainReport summarizes bounded retention and log rotation actions performed on Nomos runtime storage.
type RuntimeMaintainReport struct {
	JournalPruned         int  `json:"journal_pruned"`
	TelemetryPruned       int  `json:"telemetry_pruned"`
	ExpiredLeasesPruned   int  `json:"expired_leases_pruned"`
	LegacyDBFilesPurged   int  `json:"legacy_db_files_purged"`
	LogsRotated           int  `json:"logs_rotated"`
	LegacyLogDirsPurged   int  `json:"legacy_log_dirs_purged"`
	Vacuumed              bool `json:"vacuumed"`
	WALCheckpointed       bool `json:"wal_checkpointed"`
}

func maintenanceStampPath() string {
	return filepath.Join(config.GetNomosHome(), constants.DirCache, stampFileName)
}

// ShouldRunPeriodicMaintenance returns true when forced or when the 24-hour maintenance window has elapsed.
func ShouldRunPeriodicMaintenance(force bool) bool {
	if force {
		return true
	}
	fi, err := os.Stat(maintenanceStampPath())
	if err != nil {
		return true
	}
	return time.Since(fi.ModTime()) >= maintenanceInterval
}

func touchMaintenanceStamp() {
	stamp := maintenanceStampPath()
	if os.MkdirAll(filepath.Dir(stamp), 0755) != nil {
		return
	}
	_ = os.WriteFile(stamp, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0644)
}

// MaintainRuntimeStorage bounds SQLite telemetry tables, reclaims freelist pages, checkpoints WAL files, and rotates logs.
func MaintainRuntimeStorage(force bool) (*RuntimeMaintainReport, error) {
	if !ShouldRunPeriodicMaintenance(force) {
		return &RuntimeMaintainReport{}, nil
	}

	rep := &RuntimeMaintainReport{}
	db, err := OpenDB("")
	if err == nil {
		rep.JournalPruned = capJournalTable(db, defaultMaxJournalRows)
		rep.TelemetryPruned = capTelemetryTable(db, defaultMaxTelemetryRows)
		rep.ExpiredLeasesPruned = capExpiredLeases(db, defaultMaxExpiredLeases)
		normalizeLegacyTimestamps(db)
		rep.Vacuumed = VacuumIfFragmented(db, defaultMinVacuumBytes)
		rep.WALCheckpointed = checkpointGlobalDB(db)
		_ = db.Close()
	}

	dbDir := config.GetNomosDBDir()
	logsDir := filepath.Join(config.GetNomosHome(), constants.DirData, dirLogsName)
	rep.LogsRotated = RotateDirectoryLogs(logsDir, defaultMaxLogBytes, defaultTailKeepBytes)
	rep.LegacyLogDirsPurged = purgeLegacyDBLogDirs(filepath.Join(dbDir, dirLogsName))
	rep.LegacyDBFilesPurged = purgeRetiredDBArtifacts(dbDir)
	touchMaintenanceStamp()
	return rep, nil
}

func capJournalTable(db *sql.DB, maxRows int) int {
	res, err := db.Exec(
		`DELETE FROM journal WHERE id NOT IN (SELECT id FROM journal ORDER BY id DESC LIMIT ?)`,
		maxRows,
	)
	if err != nil {
		return 0
	}
	n, rErr := res.RowsAffected()
	if rErr != nil {
		return 0
	}
	return int(n)
}

func capTelemetryTable(db *sql.DB, maxRows int) int {
	res, err := db.Exec(
		`DELETE FROM telemetry_intent_activity WHERE rowid NOT IN (SELECT rowid FROM telemetry_intent_activity ORDER BY timestamp DESC LIMIT ?)`,
		maxRows,
	)
	if err != nil {
		return 0
	}
	n, rErr := res.RowsAffected()
	if rErr != nil {
		return 0
	}
	return int(n)
}

func capExpiredLeases(db *sql.DB, maxRetained int) int {
	res, err := db.Exec(
		`DELETE FROM leases WHERE valid = 0 AND rowid NOT IN (SELECT rowid FROM leases WHERE valid = 0 ORDER BY rowid DESC LIMIT ?)`,
		maxRetained,
	)
	if err != nil {
		return 0
	}
	n, rErr := res.RowsAffected()
	if rErr != nil {
		return 0
	}
	return int(n)
}

func normalizeLegacyTimestamps(db *sql.DB) {
	statements := []string{
		`UPDATE tasks SET created_at = replace(created_at, ' ', 'T') || 'Z' WHERE length(created_at) = 19 AND substr(created_at, 11, 1) = ' '`,
		`UPDATE tasks SET updated_at = replace(updated_at, ' ', 'T') || 'Z' WHERE length(updated_at) = 19 AND substr(updated_at, 11, 1) = ' '`,
		`UPDATE leases SET issued_at = replace(issued_at, ' ', 'T') || 'Z' WHERE length(issued_at) = 19 AND substr(issued_at, 11, 1) = ' '`,
		`UPDATE journal SET timestamp = replace(timestamp, ' ', 'T') || 'Z' WHERE length(timestamp) = 19 AND substr(timestamp, 11, 1) = ' '`,
		`UPDATE telemetry_capabilities SET last_accessed_at = replace(last_accessed_at, ' ', 'T') || 'Z' WHERE length(last_accessed_at) = 19 AND substr(last_accessed_at, 11, 1) = ' '`,
		`UPDATE telemetry_intent_activity SET timestamp = replace(timestamp, ' ', 'T') || 'Z' WHERE length(timestamp) = 19 AND substr(timestamp, 11, 1) = ' '`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			continue
		}
	}
}

// VacuumIfFragmented executes SQLite VACUUM when reclaimable freelist pages meet or exceed minReclaimBytes.
func VacuumIfFragmented(db *sql.DB, minReclaimBytes int64) bool {
	if db == nil || minReclaimBytes <= 0 {
		return false
	}
	var freePages, pageSize int64
	if err := db.QueryRow(`PRAGMA freelist_count;`).Scan(&freePages); err != nil || freePages <= 0 {
		return false
	}
	if err := db.QueryRow(`PRAGMA page_size;`).Scan(&pageSize); err != nil || pageSize <= 0 {
		return false
	}
	if freePages*pageSize < minReclaimBytes {
		return false
	}
	if _, err := db.Exec(`VACUUM;`); err != nil {
		return false
	}
	return true
}

func checkpointGlobalDB(db *sql.DB) bool {
	_, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE); PRAGMA optimize;`)
	return err == nil
}

func purgeRetiredDBArtifacts(dbDir string) int {
	retiredPrefixes := []string{
		"gitgudbrain.db",
		"nomos_embeddings.db",
		"cache.db",
		"nomos_global.db.bak",
	}
	entries, err := os.ReadDir(dbDir)
	if err != nil {
		return 0
	}
	purged := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if isRetiredDBFile(entry.Name(), retiredPrefixes) {
			if os.Remove(filepath.Join(dbDir, entry.Name())) == nil {
				purged++
			}
		}
	}
	return purged
}

func isRetiredDBFile(name string, prefixes []string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range prefixes {
		if lower == prefix || lower == prefix+"-wal" || lower == prefix+"-shm" {
			return true
		}
	}
	return false
}

// RotateDirectoryLogs rotates any .log file in logDir exceeding maxBytes, keeping tailBytes in place and writing a .1 archive.
func RotateDirectoryLogs(logDir string, maxBytes, tailBytes int64) int {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 0
	}
	rotated := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		fullPath := filepath.Join(logDir, e.Name())
		if rotateSingleLogFile(fullPath, maxBytes, tailBytes) {
			rotated++
		}
	}
	return rotated
}

func rotateSingleLogFile(logPath string, maxBytes, tailBytes int64) bool {
	fi, err := os.Stat(logPath)
	if err != nil || fi.Size() <= maxBytes {
		return false
	}
	data, rErr := os.ReadFile(logPath)
	if rErr != nil {
		return false
	}
	_ = os.WriteFile(logPath+".1", data, 0644)
	tail := data
	if int64(len(data)) > tailBytes && tailBytes > 0 {
		tail = data[int64(len(data))-tailBytes:]
	}
	return os.WriteFile(logPath, tail, 0644) == nil
}

func purgeLegacyDBLogDirs(legacyDir string) int {
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-legacyDirMaxAge)
	purged := 0
	for _, e := range entries {
		fi, iErr := e.Info()
		if iErr != nil || !fi.ModTime().Before(cutoff) {
			continue
		}
		if os.RemoveAll(filepath.Join(legacyDir, e.Name())) == nil {
			purged++
		}
	}
	if remaining, rErr := os.ReadDir(legacyDir); rErr == nil && len(remaining) == 0 {
		_ = os.Remove(legacyDir)
	}
	return purged
}
