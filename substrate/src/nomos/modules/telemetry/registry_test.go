package telemetry_test

import (
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
)

func TestRecordInvocationAndMetrics(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_telemetry.db")

	err := telemetry.RecordInvocationWithDetails(dbPath, "cli-handshake", "handshake", "substrate", "cli_command")
	if err != nil {
		t.Fatalf("failed recording initial invocation: %v", err)
	}

	err = telemetry.RecordInvocationWithDetails(dbPath, "cli-handshake", "handshake", "substrate", "cli_command")
	if err != nil {
		t.Fatalf("failed recording second invocation: %v", err)
	}

	telemetry.RecordWorkflowInvocationSafe(dbPath, "wf-handshake", "Nomos Handshake", "human_workflow")

	metrics, err := telemetry.GetCapabilityMetrics(dbPath)
	if err != nil {
		t.Fatalf("failed getting capability metrics: %v", err)
	}

	cliMetric, ok := metrics["cli-handshake"]
	if !ok {
		t.Fatalf("expected cli-handshake in metrics")
	}
	if cliMetric.InvocationCount != 2 {
		t.Errorf("expected invocation_count=2, got %d", cliMetric.InvocationCount)
	}
	if cliMetric.LastAccessedAt == "" {
		t.Errorf("expected non-empty last_accessed_at")
	}

	wfMetric, ok := metrics["wf-handshake"]
	if !ok {
		t.Fatalf("expected wf-handshake in metrics")
	}
	if wfMetric.InvocationCount != 1 {
		t.Errorf("expected invocation_count=1, got %d", wfMetric.InvocationCount)
	}
}

func TestRecordInvocationSafe_NoPanic(t *testing.T) {
	// Must not panic on invalid DB path
	telemetry.RecordInvocationSafe("/dev/null/invalid/path/db.sqlite", "cli-test")
	telemetry.RecordWorkflowInvocationSafe("/dev/null/invalid/path/db.sqlite", "wf-test", "Test", "workflow")
	telemetry.RecordIntentActivitySafe("/dev/null/invalid/path/db.sqlite", "antigravity", "PLAN", "SUB-1", "plan-01", "directive_dispatched", "{}")
}
