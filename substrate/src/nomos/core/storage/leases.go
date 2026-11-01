// Package storage provides pure-Go SQLite connection pooling, schema initialization,
// and relational persistence primitives conforming to authoritative CUE contracts.
package storage

import (
	"database/sql"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// SaveLease records or updates a CapBAC capability lease in SQLite.
func SaveLease(db *sql.DB, leaseID, taskKey, phase string, stepBudget, tokensUsed int) error {
	if db == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "db cannot be nil")
	}
	query := `INSERT OR REPLACE INTO leases (
		lease_id, task_key, phase, step_budget, tokens_used, issued_at, valid
	) VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), 1)`

	_, err := db.Exec(query, leaseID, taskKey, phase, stepBudget, tokensUsed)
	if err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed to save lease %s: %v", leaseID, err)
	}
	return nil
}

// GetActiveLease retrieves the active capability lease for a task.
func GetActiveLease(db *sql.DB, taskKey string) (string, string, int, int, error) {
	if db == nil {
		return "", "", 0, 0, types.NewNomosError(types.ErrCodeValidationFailed, "db cannot be nil")
	}
	query := `SELECT lease_id, phase, step_budget, tokens_used
		FROM leases WHERE task_key = ? AND valid = 1
		ORDER BY rowid DESC LIMIT 1`

	var leaseID, phase string
	var stepBudget, tokensUsed int

	err := db.QueryRow(query, taskKey).Scan(&leaseID, &phase, &stepBudget, &tokensUsed)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", 0, 0, types.NewNomosError(types.ErrCodeCapBAC, "no active lease found for task %s", taskKey)
		}
		return "", "", 0, 0, types.NewNomosError(types.ErrCodeSubstrate, "failed to get active lease: %v", err)
	}
	return leaseID, phase, stepBudget, tokensUsed, nil
}

// RevokeLease marks all leases for a task key as invalid.
func RevokeLease(db *sql.DB, taskKey string) error {
	if db == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "db cannot be nil")
	}
	query := `UPDATE leases SET valid = 0 WHERE task_key = ?`
	_, err := db.Exec(query, taskKey)
	if err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed to revoke lease for task %s: %v", taskKey, err)
	}
	return nil
}
