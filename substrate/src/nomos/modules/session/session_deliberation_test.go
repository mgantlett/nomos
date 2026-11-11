package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-SES-07
func TestDeliberationMetrics_FullTranscriptRecoveryAndExportParity(t *testing.T) {
	tempBrain := t.TempDir()
	mnemePath := filepath.Join(t.TempDir(), "mneme_delib.db")
	t.Setenv("NOMOS_MNEME_DB", mnemePath)

	sessID := "cccccccc-1111-2222-3333-444444444444"
	logDir := filepath.Join(tempBrain, sessID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir logDir: %v", err)
	}

	fullThinking := strings.Repeat("carbon-deliberation-", 200) // 4000 bytes
	compactLines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Implement SUB-2191"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","thinking":"short preview...","truncated_fields":["thinking"],"tool_calls":[{"name":"read_file","args":{"Path":"a.go"}},{"name":"grep_search","args":{"Query":"Foo"}},{"name":"run_command","args":{"CommandLine":"git status -s"}}]}`,
		`{"step_index":3,"type":"PLANNER_RESPONSE","thinking":"ready to patch","tool_calls":[{"name":"edit_file","args":{"Path":"a.go"}},{"name":"run_command","args":{"CommandLine":"nomos verify"}}]}`,
	}
	fullLines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Implement SUB-2191"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","thinking":"` + fullThinking + `"}`,
		`{"step_index":3,"type":"PLANNER_RESPONSE","thinking":"ready to patch"}`,
	}

	if err := os.WriteFile(filepath.Join(logDir, "transcript.jsonl"), []byte(strings.Join(compactLines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write compact transcript: %v", err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "transcript_full.jsonl"), []byte(strings.Join(fullLines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write full transcript: %v", err)
	}

	expectedBytes := len(fullThinking) + len("ready to patch")
	detail, err := GetSessionDetail(tempBrain, "/workspaces/nomos", sessID[:8])
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.Deliberation == nil {
		t.Fatalf("expected non-nil Deliberation metrics")
	}
	if detail.Deliberation.ThinkingBytes != expectedBytes {
		t.Fatalf("TC-SES-07: expected recovered ThinkingBytes=%d, got %d", expectedBytes, detail.Deliberation.ThinkingBytes)
	}
	if detail.Deliberation.ThinkingStepCount != 2 {
		t.Fatalf("TC-SES-07: expected ThinkingStepCount=2, got %d", detail.Deliberation.ThinkingStepCount)
	}
	if detail.Deliberation.MaxReadStreak != 3 {
		t.Fatalf("TC-SES-07: expected MaxReadStreak=3, got %d", detail.Deliberation.MaxReadStreak)
	}
	if detail.Deliberation.MutationCallCount != 1 {
		t.Fatalf("TC-SES-07: expected MutationCallCount=1, got %d", detail.Deliberation.MutationCallCount)
	}
	if detail.Deliberation.DeliberationToMutationRatio != float64(expectedBytes) {
		t.Fatalf("TC-SES-07: expected DeliberationToMutationRatio=%v, got %v", float64(expectedBytes), detail.Deliberation.DeliberationToMutationRatio)
	}

	records, err := ExportSessionDataset(tempBrain, t.TempDir(), 5)
	if err != nil || len(records) != 1 {
		t.Fatalf("ExportSessionDataset failed: len=%d err=%v", len(records), err)
	}
	if records[0].Deliberation == nil || records[0].Deliberation.ThinkingBytes != expectedBytes {
		t.Fatalf("TC-SES-07: expected dataset export Deliberation.ThinkingBytes=%d, got %+v", expectedBytes, records[0].Deliberation)
	}
}

// TestComputeDeliberationFromEvents_ZeroMutationDenominatorFallback verifies read-only transcripts use denominator 1.
func TestComputeDeliberationFromEvents_ZeroMutationDenominatorFallback(t *testing.T) {
	events := []SessionEvent{
		{Type: "THINKING", Detail: strings.Repeat("x", 120)},
		{Type: "TOOL_CALL", Tool: "Read", Detail: "main.go"},
		{Type: "TOOL_CALL", Tool: "Grep", Detail: " Verify "},
	}
	dm := ComputeDeliberationFromEvents(events)
	if dm.ThinkingBytes != 120 || dm.ThinkingStepCount != 1 {
		t.Fatalf("unexpected thinking counters: %+v", dm)
	}
	if dm.MaxReadStreak != 2 || dm.MutationCallCount != 0 || dm.DeliberationToMutationRatio != 120 {
		t.Fatalf("unexpected streak/ratio counters: %+v", dm)
	}
}
