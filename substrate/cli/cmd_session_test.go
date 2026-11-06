package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionCmd_ListAndVisualAndShow(t *testing.T) {
	tempBrain := t.TempDir()
	sessID := "11111111-2222-3333-4444-555555555555"
	sessDir := filepath.Join(tempBrain, sessID)
	logDir := filepath.Join(sessDir, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed creating log dir: %v", err)
	}

	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Build feature SUB-777"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","content":"Running command","tool_calls":[{"name":"run_command","args":{"CommandLine":"nomos verify"}}]}`,
	}
	tPath := filepath.Join(logDir, "transcript.jsonl")
	if err := os.WriteFile(tPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed writing transcript: %v", err)
	}

	// 1. Test session list --json
	cmdList := NewSessionCmd()
	bufList := new(bytes.Buffer)
	cmdList.SetOut(bufList)
	cmdList.SetArgs([]string{"list", "--brain", tempBrain, "--json"})
	if err := cmdList.Execute(); err != nil {
		t.Fatalf("session list failed: %v", err)
	}

	// 2. Test session name
	cmdName := NewSessionCmd()
	bufName := new(bytes.Buffer)
	cmdName.SetOut(bufName)
	cmdName.SetArgs([]string{"name", "Release Architecture Lead", "--id", sessID, "--brain", tempBrain, "--json"})
	if err := cmdName.Execute(); err != nil {
		t.Fatalf("session name failed: %v", err)
	}

	// 3. Test session visual --json
	cmdVisual := NewSessionCmd()
	bufVisual := new(bytes.Buffer)
	cmdVisual.SetOut(bufVisual)
	cmdVisual.SetArgs([]string{"visual", "--brain", tempBrain, "--json"})
	if err := cmdVisual.Execute(); err != nil {
		t.Fatalf("session visual failed: %v", err)
	}

	// 4. Test session show --json
	cmdShow := NewSessionCmd()
	bufShow := new(bytes.Buffer)
	cmdShow.SetOut(bufShow)
	cmdShow.SetArgs([]string{"show", sessID, "--brain", tempBrain, "--json"})
	if err := cmdShow.Execute(); err != nil {
		t.Fatalf("session show failed: %v", err)
	}

	// Verify session show outputs JSON with detail
	metaPath := filepath.Join(sessDir, "session_meta.json")
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("expected session_meta.json to be created: %v", err)
	}
	var meta struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil || meta.Name != "Release Architecture Lead" {
		t.Errorf("expected session name %q, got %q", "Release Architecture Lead", meta.Name)
	}

	// 5. Test session export (JSONL and --json)
	cmdExport := NewSessionCmd()
	bufExport := new(bytes.Buffer)
	cmdExport.SetOut(bufExport)
	cmdExport.SetArgs([]string{"export", "--brain", tempBrain, "--limit", "5", "--json"})
	if err := cmdExport.Execute(); err != nil {
		t.Fatalf("session export failed: %v", err)
	}
	if !strings.Contains(bufExport.String(), sessID) {
		t.Fatalf("expected exported JSON to contain %s, got %s", sessID, bufExport.String())
	}
}
