package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSessionDetail(t *testing.T) {
	tempBrain := t.TempDir()
	sessID := "abcdef12-3456-7890-abcd-ef1234567890"
	sessDir := filepath.Join(tempBrain, sessID)
	logDir := filepath.Join(sessDir, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create mock log dir: %v", err)
	}

	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","created_at":"2026-10-16T03:00:00Z","content":"Build the Whiteboard for SUB-808"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","created_at":"2026-10-16T03:00:10Z","status":"DONE","tool_calls":[{"name":"run_command","args":{"toolSummary":"\"Running tests\""}}]}`,
		`{"step_index":3,"type":"PLANNER_RESPONSE","created_at":"2026-10-16T03:00:20Z","status":"DONE","tool_calls":[{"name":"write_to_file","args":{"toolSummary":"\"Writing code\""}}]}`,
		`{"step_index":4,"type":"PLANNER_RESPONSE","created_at":"2026-10-16T03:00:30Z","status":"DONE","tool_calls":[{"name":"view_file","args":{"toolSummary":"\"Reading file\""}}]}`,
	}
	transcriptPath := filepath.Join(logDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write mock transcript: %v", err)
	}

	// Test 1: Full ID lookup
	detail, err := GetSessionDetail(tempBrain, "", sessID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.ID != sessID {
		t.Errorf("expected session ID %s, got %s", sessID, detail.ID)
	}
	if detail.TaskKey != "SUB-808" {
		t.Errorf("expected TaskKey SUB-808, got %s", detail.TaskKey)
	}
	if detail.ToolsSummary["run_command"] != 1 {
		t.Errorf("expected 1 run_command, got %d", detail.ToolsSummary["run_command"])
	}
	if detail.ToolsSummary["write_to_file"] != 1 {
		t.Errorf("expected 1 write_to_file, got %d", detail.ToolsSummary["write_to_file"])
	}
	if detail.ToolsSummary["view_file"] != 1 {
		t.Errorf("expected 1 view_file, got %d", detail.ToolsSummary["view_file"])
	}
	if len(detail.Events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(detail.Events))
	}
	if len(detail.RecentPrompts) != 1 {
		t.Errorf("expected 1 recent prompt, got %d", len(detail.RecentPrompts))
	}

	// Test 2: Prefix ID lookup (first 8 characters)
	prefixDetail, errPrefix := GetSessionDetail(tempBrain, "", "abcdef12")
	if errPrefix != nil {
		t.Fatalf("unexpected error on prefix lookup: %v", errPrefix)
	}
	if prefixDetail.ID != sessID {
		t.Errorf("expected matched ID %s, got %s", sessID, prefixDetail.ID)
	}

	// Test 3: Non-existent session
	_, errMissing := GetSessionDetail(tempBrain, "", "nonexistent-id")
	if errMissing == nil {
		t.Errorf("expected error for non-existent session, got nil")
	}
}
