package session

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDiscoverSessions(t *testing.T) {
	tempBrain := t.TempDir()
	sessID := "12345678-1234-1234-1234-123456789abc"
	sessDir := filepath.Join(tempBrain, sessID)
	logDir := filepath.Join(sessDir, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create mock log dir: %v", err)
	}

	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Please implement SUB-805"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","content":"Working on SUB-805 in worktrees/nomos-SUB-805","tool_calls":[{"name":"run_command"}]}`,
	}
	transcriptPath := filepath.Join(logDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write mock transcript: %v", err)
	}

	sessions, err := DiscoverSessions(tempBrain, "")
	if err != nil {
		t.Fatalf("unexpected error discovering sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]
	if s.ID != sessID {
		t.Errorf("expected session ID %s, got %s", sessID, s.ID)
	}
	if s.TaskKey != "SUB-805" {
		t.Errorf("expected TaskKey SUB-805, got %s", s.TaskKey)
	}
	if s.TurnCount != 1 {
		t.Errorf("expected TurnCount 1, got %d", s.TurnCount)
	}
	if s.ToolCallCount != 1 {
		t.Errorf("expected ToolCallCount 1, got %d", s.ToolCallCount)
	}
	if s.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", s.Status)
	}
}

func TestGenerateMermaidVisual(t *testing.T) {
	// Empty sessions test
	emptyGraph := GenerateMermaidVisual(nil)
	if !strings.Contains(emptyGraph, "No active Nomos sessions detected") {
		t.Errorf("expected empty state message in graph, got: %s", emptyGraph)
	}

	// Populated sessions test
	sessions := []AgentSession{
		{
			ID:            "4929f603-d051-47e0-84d9-c4af4c3279bf",
			AgentType:     "AGY-IDE (Tier 1)",
			TaskKey:       "SUB-805",
			WorktreePath:  "/tmp/worktrees/nomos-SUB-805",
			Phase:         "EDIT",
			TurnCount:     12,
			ToolCallCount: 45,
			LastActiveAt:  time.Now(),
			Status:        "ACTIVE",
			RecentSummary: "Implement nomos session CLI",
		},
		{
			ID:            "3694dd7b-5bc0-4533-9005-98a8176fa541",
			AgentType:     "AGY-IDE (Tier 1)",
			TaskKey:       "SUB-803",
			WorktreePath:  "",
			Phase:         "DONE",
			TurnCount:     8,
			ToolCallCount: 20,
			LastActiveAt:  time.Now().Add(-30 * time.Minute),
			Status:        "IDLE",
			RecentSummary: "Sync KaTeX formulas",
		},
	}

	graph := GenerateMermaidVisual(sessions)
	if !strings.HasPrefix(graph, "flowchart TD") {
		t.Errorf("expected graph to start with flowchart TD, got: %s", graph)
	}
	if !strings.Contains(graph, "Session 4929f603") {
		t.Errorf("expected graph to contain short ID 4929f603, got: %s", graph)
	}
	if !strings.Contains(graph, "Task: SUB-805 (EDIT)") {
		t.Errorf("expected graph to contain task SUB-805, got: %s", graph)
	}
	if !strings.Contains(graph, "Worktree: nomos-SUB-805") {
		t.Errorf("expected graph to link worktree nomos-SUB-805, got: %s", graph)
	}
}

func TestReadTailLines(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	lines, err := readTailLines(tmpFile, 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) == 0 {
		t.Fatalf("expected lines, got none")
	}
}

func TestGenerateSessionThreadsMermaid(t *testing.T) {
	// Nil detail test
	empty := GenerateSessionThreadsMermaid(nil)
	if !strings.Contains(empty, "No session details available") {
		t.Errorf("expected empty message, got: %s", empty)
	}

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:            "49f670c2-d758-4533-802c-38ba851eebc1",
			AgentType:     "AGY-IDE (Tier 1)",
			TaskKey:       "SUB-1034",
			Phase:         "REVIEW",
			TurnCount:     2,
			ToolCallCount: 5,
		},
		Events: []SessionEvent{
			{Type: "USER_REQUEST", Title: "proceed with Option A first then B,C,D"},
			{Type: "TOOL_CALL", Title: "run_command", Tool: "run_command"},
			{Type: "RECEIPT", Title: "Merged commit dcbe86bb"},
		},
	}

	res := GenerateSessionThreadsMermaid(detail)
	if !strings.HasPrefix(res, "graph TD") {
		t.Errorf("expected graph TD header, got: %s", res)
	}
	if !strings.Contains(res, "proceed with Option A") {
		t.Errorf("expected prompt in graph, got: %s", res)
	}
	if !strings.Contains(res, "dcbe86bb") {
		t.Errorf("expected receipt in graph, got: %s", res)
	}
}

func TestCleanUserPromptContent(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			input:    "<USER_REQUEST>\nproceed with Option A first then B,C,D\n</USER_REQUEST>",
			expected: "proceed with Option A first then B,C,D",
		},
		{
			input:    "# Resuming from a compaction\n\n# User Requests\n1. execute mission\n",
			expected: "execute mission",
		},
		{
			input:    "### Title\n\nJust do it",
			expected: "Just do it",
		},
	}

	for _, tc := range cases {
		got := cleanUserPromptContent(tc.input)
		if got != tc.expected {
			t.Errorf("cleanUserPromptContent(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestTranscriptTail_NoFalsePositiveSub1(t *testing.T) {
	tempBrain := t.TempDir()
	sessID := "87654321-4321-4321-4321-cba987654321"
	sessDir := filepath.Join(tempBrain, sessID)
	logDir := filepath.Join(sessDir, ".system_generated", "logs")
	_ = os.MkdirAll(logDir, 0755)

	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Please check the open source checklist"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"run_command"}]}`,
		`{"step_index":3,"type":"RUN_COMMAND","content":"nomos task list\n│ SUB-651 │ Task │ ...\n│ SUB-1 │ DONE │ V3 Dual-Core Foundation Scaffolding │"}`,
	}
	transcriptPath := filepath.Join(logDir, "transcript.jsonl")
	_ = os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")), 0644)

	sessions, err := DiscoverSessions(tempBrain, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].TaskKey == "SUB-1" {
		t.Errorf("expected TaskKey NOT to be false-positive SUB-1 from terminal stdout, got %s", sessions[0].TaskKey)
	}
}

func TestSortSessionsByActive_StableTieBreaker(t *testing.T) {
	now := time.Now()
	sessions := []AgentSession{
		{ID: "session-z", LastActiveAt: now},
		{ID: "session-a", LastActiveAt: now},
		{ID: "session-m", LastActiveAt: now},
	}
	sortSessionsByActive(sessions)
	if sessions[0].ID != "session-a" || sessions[1].ID != "session-m" || sessions[2].ID != "session-z" {
		t.Errorf("expected sessions sorted deterministically by ID on equal timestamp, got %s, %s, %s",
			sessions[0].ID, sessions[1].ID, sessions[2].ID)
	}
}

func TestScanFullTranscriptMetrics(t *testing.T) {
	tempDir := t.TempDir()
	transcriptPath := filepath.Join(tempDir, "transcript.jsonl")
	var lines []string
	for i := 0; i < 5; i++ {
		lines = append(lines, `{"step_index":`+strconv.Itoa(i*2)+`,"type":"USER_INPUT","content":"prompt"}`)
		lines = append(lines, `{"step_index":`+strconv.Itoa(i*2+1)+`,"type":"PLANNER_RESPONSE","tool_calls":[{"name":"view_file"}]}`)
	}
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	turns, tools, err := scanFullTranscriptMetrics(transcriptPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if turns != 5 {
		t.Errorf("expected 5 turns, got %d", turns)
	}
	if tools != 5 {
		t.Errorf("expected 5 tools, got %d", tools)
	}
}


