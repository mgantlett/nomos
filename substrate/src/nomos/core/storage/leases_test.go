package storage

import (
	"path/filepath"
	"testing"
)

func TestLeases(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_leases.db")

	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing db: %v", closeErr)
		}
	}()

	// First insert task to satisfy foreign key constraint
	insertTask := `INSERT INTO tasks (key, plane, title, status) VALUES ('SUB-900', 'substrate', 'Lease Test Task', 'TODO')`
	if _, err := db.Exec(insertTask); err != nil {
		t.Fatalf("failed inserting task: %v", err)
	}

	leaseID := "cap_lease_11112222333344445555666677778888"
	if err := SaveLease(db, leaseID, "SUB-900", "PLAN", 100, 5); err != nil {
		t.Fatalf("SaveLease failed: %v", err)
	}

	id, phase, budget, used, err := GetActiveLease(db, "SUB-900")
	if err != nil {
		t.Fatalf("GetActiveLease failed: %v", err)
	}
	if id != leaseID || phase != "PLAN" || budget != 100 || used != 5 {
		t.Errorf("got (%s, %s, %d, %d), want (%s, PLAN, 100, 5)", id, phase, budget, used, leaseID)
	}

	// Revoke lease
	if err := RevokeLease(db, "SUB-900"); err != nil {
		t.Fatalf("RevokeLease failed: %v", err)
	}

	// Get active lease should now fail
	if _, _, _, _, err := GetActiveLease(db, "SUB-900"); err == nil {
		t.Errorf("expected error after lease revocation")
	}
}
