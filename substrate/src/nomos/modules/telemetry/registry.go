// Package telemetry provides non-blocking runtime invocation tracking,
// cognitive metrics, and intent activity auditing for Nomos.
package telemetry

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// CapabilityMetrics represents the persisted invocation metrics for a capability (#CapabilityMetrics, #CLIAuditEvent).
type CapabilityMetrics struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Plane           string `json:"plane"`
	Type            string `json:"type"`
	Enabled         bool   `json:"enabled"`
	InvocationCount int    `json:"invocation_count"`
	LastAccessedAt  string `json:"last_accessed_at"`
}

// RecordInvocationSafe records an execution event without bubbling errors or panics.
func RecordInvocationSafe(dbPath, id string) {
	if err := RecordInvocationWithDetails(dbPath, id, id, "substrate", "cli_command"); err != nil {
		return
	}
}

// RecordWorkflowInvocationSafe records a workflow or directive execution safely.
func RecordWorkflowInvocationSafe(dbPath, id, name, capType string) {
	if err := RecordInvocationWithDetails(dbPath, id, name, "intent", capType); err != nil {
		return
	}
}

// RecordInvocationWithDetails records or increments an authoritative capability invocation.
func RecordInvocationWithDetails(dbPath, id, name, plane, capType string) error {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()

	query := `
	INSERT INTO telemetry_capabilities (id, name, plane, type, enabled, invocation_count, last_accessed_at)
	VALUES (?, ?, ?, ?, 1, 1, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	ON CONFLICT(id) DO UPDATE SET
		invocation_count = invocation_count + 1,
		last_accessed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');
	`
	_, qErr := db.Exec(query, id, name, plane, capType)
	return qErr
}

// GetCapabilityMetrics retrieves all recorded capability telemetry indexed by ID.
func GetCapabilityMetrics(dbPath string) (map[string]CapabilityMetrics, error) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()

	rows, qErr := db.Query(`
		SELECT id, name, plane, type, enabled, invocation_count, COALESCE(last_accessed_at, '')
		FROM telemetry_capabilities
	`)
	if qErr != nil {
		return nil, qErr
	}
	defer func() {
		if rErr := rows.Close(); rErr != nil {
			return
		}
	}()

	return scanCapabilityMetrics(rows)
}

func scanCapabilityMetrics(rows *sql.Rows) (map[string]CapabilityMetrics, error) {
	metrics := make(map[string]CapabilityMetrics)
	for rows.Next() {
		var m CapabilityMetrics
		var enabledInt int
		if err := rows.Scan(&m.ID, &m.Name, &m.Plane, &m.Type, &enabledInt, &m.InvocationCount, &m.LastAccessedAt); err != nil {
			return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed scanning capability metrics: %v", err)
		}
		m.Enabled = (enabledInt != 0)
		metrics[m.ID] = m
	}
	if err := rows.Err(); err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "error iterating capability metrics: %v", err)
	}
	return metrics, nil
}

// RecordIntentActivitySafe records an Intent phase activity event non-blockingly.
func RecordIntentActivitySafe(dbPath string, seat, phase, taskKey, directiveID, eventType, detailsJSON string) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()

	id := fmt.Sprintf("act_%d", time.Now().UnixNano())
	if detailsJSON == "" {
		detailsJSON = "{}"
	}

	query := `
	INSERT INTO telemetry_intent_activity (id, timestamp, seat, phase, active_task, directive_id, event_type, details_json)
	VALUES (?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), ?, ?, ?, ?, ?, ?);
	`
	if _, qErr := db.Exec(query, id, seat, phase, taskKey, directiveID, eventType, detailsJSON); qErr != nil {
		return
	}
}
