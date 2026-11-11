package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderAGYIDE_Antigravity20AndLegacyBrainDiscovery(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ANTIGRAVITY_APP_DATA_DIR", "")

	agy20Brain := filepath.Join(homeDir, ".gemini", "antigravity", "brain")
	legacyBrain := filepath.Join(homeDir, ".gemini", "antigravity-ide", "brain")

	id20 := "aaaaaaaa-1111-2222-3333-444444444444"
	idLegacy := "bbbbbbbb-1111-2222-3333-444444444444"
	setupMockBrainSession(t, agy20Brain, id20, "SUB-2179", "Antigravity 2.0 session", "edit_file")
	setupMockBrainSession(t, legacyBrain, idLegacy, "SUB-2180", "Legacy AGY-IDE session", "replace_file_content")

	p := NewProviderAGYIDE("")
	sessions, err := p.Discover("/workspaces/custom-repo", nil)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions across antigravity and antigravity-ide brain roots, got %d", len(sessions))
	}
	for _, s := range sessions {
		if s.Repo != "custom-repo" {
			t.Fatalf("expected repoRoot fallback 'custom-repo', got %q", s.Repo)
		}
	}

	detail, err := p.GetDetail(id20[:8], "/workspaces/custom-repo", nil)
	if err != nil {
		t.Fatalf("GetDetail on AGY 2.0 session failed: %v", err)
	}
	if detail.ID != id20 {
		t.Fatalf("expected session ID %s, got %s", id20, detail.ID)
	}
}

func TestMultiHarnessSentinel_ClaudeParallelToolsAndLargeTranscriptLine(t *testing.T) {
	detail := &SessionDetail{
		AgentSession: AgentSession{ID: "sess-multi", Status: "ACTIVE"},
		ToolsSummary: make(map[string]int),
	}
	p := NewProviderClaudeCode("")
	assistantJSON := `[{"type":"thinking","thinking":"Inspecting sqlite3 guardrail"},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"sqlite3 .nomos/mneme.db 'SELECT 1'"}},{"type":"tool_use","id":"t2","name":"Read","input":{"file_path":"app_test.go"}}]`
	rec := claudeRecord{
		Type:      "assistant",
		Timestamp: "2026-11-10T12:00:00Z",
		Message:   claudeMessage{Role: "assistant", Content: []byte(assistantJSON)},
	}
	lines := p.buildDetailEvents([]claudeRecord{rec}, detail, "/workspaces/consumer-app")
	if len(lines) != 1 {
		t.Fatalf("expected 1 consolidated sentinel record, got %d", len(lines))
	}
	if detail.Sentinel == nil || detail.Sentinel.IntegrityStatus == "CLEAN" {
		t.Fatalf("expected parallel Claude tool_use #1 sqlite3 violation to be caught by Sentinel, got %+v", detail.Sentinel)
	}
	for _, st := range detail.Trajectory.Stages {
		if st.ID == "ORACLE" && st.Status == "COMPLETED" {
			t.Fatalf("reading app_test.go must not falsely mark ORACLE stage COMPLETED")
		}
	}

	// Verify >1 MiB JSONL line streams cleanly without bufio.ErrTooLong
	bigFile := filepath.Join(t.TempDir(), "transcript.jsonl")
	bigLine := `{"type":"USER_INPUT","content":"` + strings.Repeat("x", 1300*1024) + `"}` + "\n"
	if err := os.WriteFile(bigFile, []byte(bigLine), 0600); err != nil {
		t.Fatal(err)
	}
	turns, _, sErr := scanFullTranscriptMetrics(bigFile)
	if sErr != nil || turns != 1 {
		t.Fatalf("expected 1 turn on 1.3 MiB line without chunk-boundary corruption, got turns=%d err=%v", turns, sErr)
	}
	tail, tErr := readTailLines(bigFile, 2*1024*1024)
	if tErr != nil || len(tail) != 1 {
		t.Fatalf("expected readTailLines to handle >1 MiB line, got len=%d err=%v", len(tail), tErr)
	}
}
