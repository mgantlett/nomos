package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndReadSessionName(t *testing.T) {
	tempBrain := t.TempDir()
	sessID := "f9745978-78c4-4ffa-a21a-0375b0ae9998"
	sessDir := filepath.Join(tempBrain, sessID)
	if err := os.MkdirAll(sessDir, 0755); err != nil {
		t.Fatalf("failed to create temp sess dir: %v", err)
	}

	// 1. Initial read should be empty
	if name := readSessionCustomName(sessDir); name != "" {
		t.Errorf("expected empty name, got %q", name)
	}

	// 2. Save session name
	meta, err := SaveSessionName(tempBrain, sessID, "Cache Layer Refactor")
	if err != nil {
		t.Fatalf("SaveSessionName failed: %v", err)
	}
	if meta.Name != "Cache Layer Refactor" {
		t.Errorf("expected name %q, got %q", "Cache Layer Refactor", meta.Name)
	}
	if meta.ID != sessID {
		t.Errorf("expected id %q, got %q", sessID, meta.ID)
	}

	// 3. Read back via readSessionCustomName
	readBack := readSessionCustomName(sessDir)
	if readBack != "Cache Layer Refactor" {
		t.Errorf("expected readBack %q, got %q", "Cache Layer Refactor", readBack)
	}

	// 4. Overwrite session name
	_, err = SaveSessionName(tempBrain, sessID, "Updated Session Name")
	if err != nil {
		t.Fatalf("SaveSessionName overwrite failed: %v", err)
	}
	if updated := readSessionCustomName(sessDir); updated != "Updated Session Name" {
		t.Errorf("expected %q, got %q", "Updated Session Name", updated)
	}
}
