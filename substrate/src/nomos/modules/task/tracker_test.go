package task

import (
	"strings"
	"testing"
)

func TestValidateReleaseNote(t *testing.T) {
	valid := ReleaseNotePayload{
		TaskKey:            "SUB-1196",
		CommitSHA:          "64c133d08f21962dc069ba29f2bbca186bccc813",
		Timestamp:          "2026-10-23T15:00:00Z",
		Phase:              "SYNC",
		DoDGatesPassed:     42,
		CapBACTokenID:      "token_123",
		WalkthroughContent: "# Walkthrough",
		SHA256Receipt:      "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}

	if err := ValidateReleaseNote(valid); err != nil {
		t.Fatalf("expected valid release note to pass: %v", err)
	}

	// Short commit SHA
	badSHA := valid
	badSHA.CommitSHA = "12345"
	if err := ValidateReleaseNote(badSHA); err == nil {
		t.Errorf("expected short commit SHA to fail validation")
	}

	// Gates < 42
	badGates := valid
	badGates.DoDGatesPassed = 41
	if err := ValidateReleaseNote(badGates); err == nil {
		t.Errorf("expected < 42 DoD gates to fail validation")
	}

	// Wrong phase
	badPhase := valid
	badPhase.Phase = "EDIT"
	if err := ValidateReleaseNote(badPhase); err == nil {
		t.Errorf("expected non-SYNC phase to fail validation")
	}
}

func TestComputeReleaseReceipt(t *testing.T) {
	receipt := ComputeReleaseReceipt("SUB-1", "64c133d08f21962dc069ba29f2bbca186bccc813", "2026-10-23T15:00:00Z", "tok_abc")
	if len(receipt) != 64 {
		t.Errorf("expected 64-char sha256 receipt, got %d chars", len(receipt))
	}
}

func TestInspection(t *testing.T) {
	taskRec := TaskRecord{
		Key:         "SUB-1196",
		Title:       "Clean-room task subsystem",
		Status:      StatusEdit,
		Plane:       "substrate",
		Tier:        Tier1,
		Description: "Detailed description",
	}

	rec := BuildInspectionRecord(taskRec, []string{"SUB-1195"}, []string{"SUB-1197"}, "/worktrees/nomos-SUB-1196", "feature/SUB-1196")
	card := FormatInspectionCard(rec)

	if !strings.Contains(card, "# Task: SUB-1196") {
		t.Errorf("expected card to contain task title header")
	}
	if !strings.Contains(card, "**Blocked By**: [SUB-1195]") {
		t.Errorf("expected card to list prerequisite")
	}
}

func TestMemoryTaskStore(t *testing.T) {
	store := NewMemoryTaskStore()

	t1 := TaskRecord{
		Key:        "SUB-1",
		Title:      "Initial Root Task",
		Plane:      "substrate",
		SchemaType: SchemaSubstrate,
		Status:     StatusDone,
		Tier:       Tier1,
	}
	completedAt := "2026-10-23T15:00:00Z"
	t1.CompletedAt = &completedAt

	if err := store.CreateTask(t1); err != nil {
		t.Fatalf("failed creating task t1: %v", err)
	}

	t2 := TaskRecord{
		Key:          "SUB-2",
		Title:        "Dependent Child Task",
		Plane:        "substrate",
		SchemaType:   SchemaSubstrate,
		Status:       StatusReady,
		Tier:         Tier1,
		Dependencies: []string{"SUB-1"},
	}
	if err := store.CreateTask(t2); err != nil {
		t.Fatalf("failed creating task t2: %v", err)
	}

	wavefront, err := store.GetWavefront()
	if err != nil {
		t.Fatalf("unexpected error getting wavefront: %v", err)
	}

	// Since t1 is DONE, t2 should be unblocked and present on wavefront!
	if len(wavefront) != 1 || wavefront[0].Key != "SUB-2" {
		t.Errorf("expected SUB-2 on wavefront, got %+v", wavefront)
	}
}
