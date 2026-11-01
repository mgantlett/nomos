package task

import (
	"testing"
)

func TestValidateTaskKey(t *testing.T) {
	validKeys := []string{"INT-1", "SUB-100", "INT-725-test-slug", "SUB-1196"}
	for _, k := range validKeys {
		if err := ValidateTaskKey(k); err != nil {
			t.Errorf("expected valid key %s, got: %v", k, err)
		}
	}

	invalidKeys := []string{"", "TASK-1", "TSK-10", "INT-", "sub-10", "GSI-1"}
	for _, k := range invalidKeys {
		if err := ValidateTaskKey(k); err == nil {
			t.Errorf("expected invalid key %s to fail validation", k)
		}
	}
}

func TestFormatTaskKey(t *testing.T) {
	key, err := FormatTaskKey("INT", 42)
	if err != nil || key != "INT-42" {
		t.Errorf("expected INT-42, got %s (err: %v)", key, err)
	}

	key, err = FormatTaskKey("sub", 100)
	if err != nil || key != "SUB-100" {
		t.Errorf("expected SUB-100, got %s (err: %v)", key, err)
	}

	if _, err := FormatTaskKey("INVALID", 1); err == nil {
		t.Errorf("expected error for invalid prefix")
	}

	if _, err := FormatTaskKey("INT", -1); err == nil {
		t.Errorf("expected error for negative number")
	}
}

func TestValidateTaskRecord(t *testing.T) {
	completedAt := "2026-10-23T15:00:00Z"
	valid := TaskRecord{
		Key:         "INT-100",
		Plane:       "intent",
		SchemaType:  SchemaIntent,
		Title:       "Valid Title",
		Status:      StatusDone,
		Tier:        Tier1,
		CompletedAt: &completedAt,
	}

	if err := ValidateTaskRecord(&valid); err != nil {
		t.Fatalf("expected valid task record to pass: %v", err)
	}

	// Plane mismatch
	mismatch := valid
	mismatch.Plane = "substrate"
	if err := ValidateTaskRecord(&mismatch); err == nil {
		t.Errorf("expected plane mismatch to fail validation")
	}

	// Done without completed_at
	noComplete := valid
	noComplete.CompletedAt = nil
	if err := ValidateTaskRecord(&noComplete); err == nil {
		t.Errorf("expected status DONE without completed_at to fail validation")
	}

	// Short title
	shortTitle := valid
	shortTitle.Title = "Hi"
	if err := ValidateTaskRecord(&shortTitle); err == nil {
		t.Errorf("expected short title to fail validation")
	}
}
