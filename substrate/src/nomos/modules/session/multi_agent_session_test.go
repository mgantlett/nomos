package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMultiAgentSession_Discovery verifies that the Nomos session engine
// polymorphically discovers all 4 officially supported Tier 1 coding agents:
// AGY-IDE, AGY-CLI, Claude Code, and OpenCode in a clean-room testbed.
func TestMultiAgentSession_Discovery(t *testing.T) {
	tempRoot := t.TempDir()

	// 1. Setup synthetic AGY-IDE session
	ideBrain := filepath.Join(tempRoot, "gemini-ide", "brain")
	ideID := "11111111-1111-1111-1111-111111111111"
	setupMockBrainSession(t, ideBrain, ideID, "SUB-801", "Implement AGY-IDE task", "run_command")

	// 2. Setup synthetic AGY-CLI session
	cliBrain := filepath.Join(tempRoot, "gemini-cli", "brain")
	cliID := "22222222-2222-2222-2222-222222222222"
	setupMockBrainSession(t, cliBrain, cliID, "SUB-802", "Run AGY-CLI command", "run_command")

	// 3. Setup synthetic Claude Code session
	claudeRoot := filepath.Join(tempRoot, "claude")
	claudeID := "33333333-3333-3333-3333-333333333333"
	setupMockClaudeSession(t, claudeRoot, claudeID, "SUB-803", "Build Claude Code feature", "Bash")

	// 4. Setup synthetic OpenCode session
	opencodeRoot := filepath.Join(tempRoot, "opencode", "storage")
	opencodeID := "ses_44444444444444444444444444"
	setupMockOpenCodeSession(t, opencodeRoot, opencodeID, "SUB-804", "Develop OpenCode module", "edit_file")

	activeWorktrees := map[string]string{
		"SUB-801": "/workspaces/nomos/worktrees/nomos-SUB-801",
		"SUB-802": "/workspaces/nomos/worktrees/nomos-SUB-802",
		"SUB-803": "/workspaces/nomos/worktrees/nomos-SUB-803",
		"SUB-804": "/workspaces/nomos/worktrees/nomos-SUB-804",
	}

	providers := []SessionProvider{
		NewProviderAGYIDE(ideBrain),
		NewProviderAGYCLI(cliBrain),
		NewProviderClaudeCode(claudeRoot),
		NewProviderOpenCode(opencodeRoot),
	}

	var allSessions []AgentSession
	for _, p := range providers {
		sessions, err := p.Discover("", activeWorktrees)
		if err != nil {
			t.Fatalf("provider %s failed: %v", p.AgentType(), err)
		}
		allSessions = append(allSessions, sessions...)
	}

	if len(allSessions) != 4 {
		t.Fatalf("expected 4 discovered sessions across all providers, got %d", len(allSessions))
	}

	sessionMap := make(map[string]AgentSession)
	for _, s := range allSessions {
		sessionMap[s.ID] = s
	}

	// Verify AGY-IDE
	sIDE, ok := sessionMap[ideID]
	if !ok {
		t.Fatalf("AGY-IDE session %s not discovered", ideID)
	}
	if sIDE.AgentType != "AGY-IDE (Tier 1)" {
		t.Errorf("expected AGY-IDE agent type, got %s", sIDE.AgentType)
	}
	if sIDE.TaskKey != "SUB-801" {
		t.Errorf("expected TaskKey SUB-801, got %s", sIDE.TaskKey)
	}

	// Verify AGY-CLI
	sCLI, ok := sessionMap[cliID]
	if !ok {
		t.Fatalf("AGY-CLI session %s not discovered", cliID)
	}
	if sCLI.AgentType != "AGY-CLI (Tier 1)" {
		t.Errorf("expected AGY-CLI agent type, got %s", sCLI.AgentType)
	}
	if sCLI.TaskKey != "SUB-802" {
		t.Errorf("expected TaskKey SUB-802, got %s", sCLI.TaskKey)
	}

	// Verify Claude Code
	sCC, ok := sessionMap[claudeID]
	if !ok {
		t.Fatalf("Claude Code session %s not discovered", claudeID)
	}
	if sCC.AgentType != "Claude Code (Tier 1)" {
		t.Errorf("expected Claude Code agent type, got %s", sCC.AgentType)
	}
	if sCC.TaskKey != "SUB-803" {
		t.Errorf("expected TaskKey SUB-803, got %s", sCC.TaskKey)
	}

	// Verify OpenCode
	sOC, ok := sessionMap[opencodeID]
	if !ok {
		t.Fatalf("OpenCode session %s not discovered", opencodeID)
	}
	if sOC.AgentType != "OpenCode (Tier 1)" {
		t.Errorf("expected OpenCode agent type, got %s", sOC.AgentType)
	}
	if sOC.TaskKey != "SUB-804" {
		t.Errorf("expected TaskKey SUB-804, got %s", sOC.TaskKey)
	}

	// Verify Mermaid Graph includes all 4
	visual := GenerateMermaidVisual(allSessions)
	if !strings.Contains(visual, "11111111") || !strings.Contains(visual, "22222222") ||
		!strings.Contains(visual, "33333333") || !strings.Contains(visual, "ses_4444") {
		t.Errorf("Mermaid visual missing session short IDs:\n%s", visual)
	}
}

// TestMultiAgentSession_DeepDetailInspection tests causal timeline and sentinel evaluation across all agents.
func TestMultiAgentSession_DeepDetailInspection(t *testing.T) {
	tempRoot := t.TempDir()

	claudeRoot := filepath.Join(tempRoot, "claude")
	claudeID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	setupMockClaudeSession(t, claudeRoot, claudeID, "SUB-999", "Execute pre-flight audit", "Bash")

	pClaude := NewProviderClaudeCode(claudeRoot)
	detail, err := pClaude.GetDetail(claudeID, "", nil)
	if err != nil {
		t.Fatalf("failed getting Claude Code session detail: %v", err)
	}

	if detail.AgentType != "Claude Code (Tier 1)" {
		t.Errorf("expected Claude Code agent type, got %s", detail.AgentType)
	}
	if detail.ToolsSummary["Bash"] != 1 {
		t.Errorf("expected 1 Bash tool call, got %d", detail.ToolsSummary["Bash"])
	}
	if len(detail.Events) < 2 {
		t.Errorf("expected at least 2 causal events (USER_REQUEST, TOOL_CALL), got %d", len(detail.Events))
	}
	if detail.Sentinel == nil {
		t.Fatalf("expected Sentinel scorecard evaluation, got nil")
	}
	if detail.Trajectory == nil || len(detail.Trajectory.Stages) != 7 {
		t.Fatalf("expected 7-stage SessionTrajectory, got %v", detail.Trajectory)
	}
}

// TestMultiAgentSession_SessionNaming tests metadata assignment across all providers.
func TestMultiAgentSession_SessionNaming(t *testing.T) {
	tempRoot := t.TempDir()

	opencodeRoot := filepath.Join(tempRoot, "opencode", "storage")
	opencodeID := "ses_nametest_12345678"
	setupMockOpenCodeSession(t, opencodeRoot, opencodeID, "SUB-100", "Initial title", "grep")

	pOpenCode := NewProviderOpenCode(opencodeRoot)
	meta, err := pOpenCode.SaveName(opencodeID, "Architectural Session V4")
	if err != nil {
		t.Fatalf("SaveName failed for OpenCode: %v", err)
	}
	if meta.Name != "Architectural Session V4" {
		t.Errorf("expected name %q, got %q", "Architectural Session V4", meta.Name)
	}

	// Verify discovery reflects updated title
	sessions, err := pOpenCode.Discover("", nil)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("failed discovering OpenCode session after rename: %v", err)
	}
	if sessions[0].Name != "Architectural Session V4" {
		t.Errorf("discovered session did not reflect renamed title: %q", sessions[0].Name)
	}
}

func setupMockBrainSession(t *testing.T, brainDir, sessID, taskKey, prompt, toolName string) {
	logDir := filepath.Join(brainDir, sessID, ".system_generated", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create mock brain log dir: %v", err)
	}
	lines := []string{
		`{"step_index":1,"type":"USER_INPUT","content":"Please work on ` + taskKey + `: ` + prompt + `"}`,
		`{"step_index":2,"type":"PLANNER_RESPONSE","content":"Running ` + toolName + `","tool_calls":[{"name":"` + toolName + `","args":{"CommandLine":"nomos verify"}}]}`,
	}
	transcriptPath := filepath.Join(logDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("failed to write mock brain transcript: %v", err)
	}
}

func setupMockClaudeSession(t *testing.T, claudeRoot, sessID, taskKey, prompt, toolName string) {
	projectDir := filepath.Join(claudeRoot, "projects", "-workspaces-nomos")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create claude project dir: %v", err)
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	records := []map[string]any{
		{
			"type":      "user",
			"sessionId": sessID,
			"gitBranch": "feature/" + taskKey,
			"cwd":       "/workspaces/nomos/worktrees/nomos-" + taskKey,
			"timestamp": ts,
			"message": map[string]any{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Task " + taskKey + ": " + prompt},
				},
			},
		},
		{
			"type":      "assistant",
			"sessionId": sessID,
			"gitBranch": "feature/" + taskKey,
			"cwd":       "/workspaces/nomos/worktrees/nomos-" + taskKey,
			"timestamp": ts,
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{
						"type": "tool_use",
						"name": toolName,
						"input": map[string]any{
							"command": "nomos verify",
						},
					},
				},
			},
		},
	}

	var sb strings.Builder
	for _, rec := range records {
		bytes, _ := json.Marshal(rec)
		sb.Write(bytes)
		sb.WriteByte('\n')
	}

	filePath := filepath.Join(projectDir, sessID+".jsonl")
	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("failed to write claude jsonl: %v", err)
	}
}

func setupMockOpenCodeSession(t *testing.T, storageRoot, sessID, taskKey, title, toolName string) {
	sessDir := filepath.Join(storageRoot, "session", "project-alpha")
	msgDir := filepath.Join(storageRoot, "message", sessID)
	partDir1 := filepath.Join(storageRoot, "part", "msg_001")
	partDir2 := filepath.Join(storageRoot, "part", "msg_002")

	for _, d := range []string{sessDir, msgDir, partDir1, partDir2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create opencode mock dir %s: %v", d, err)
		}
	}

	sessDoc := openCodeSessionDoc{
		ID:        sessID,
		Slug:      "mock-slug",
		ProjectID: "project-alpha",
		Directory: "/workspaces/nomos/worktrees/nomos-" + taskKey,
		Title:     title,
	}
	sessDoc.Time.Created = time.Now().Add(-10 * time.Minute).UnixMilli()
	sessDoc.Time.Updated = time.Now().UnixMilli()

	sBytes, _ := json.MarshalIndent(sessDoc, "", "  ")
	_ = os.WriteFile(filepath.Join(sessDir, sessID+".json"), sBytes, 0644)

	// Message 1: User
	msg1 := openCodeMessageDoc{
		ID:        "msg_001",
		SessionID: sessID,
		Role:      "user",
	}
	msg1.Time.Created = sessDoc.Time.Created
	m1Bytes, _ := json.MarshalIndent(msg1, "", "  ")
	_ = os.WriteFile(filepath.Join(msgDir, "msg_001.json"), m1Bytes, 0644)

	// Part 1: User text
	part1 := openCodePartDoc{
		ID:        "prt_001",
		SessionID: sessID,
		MessageID: "msg_001",
		Type:      "text",
		Text:      "Please build " + taskKey,
	}
	p1Bytes, _ := json.MarshalIndent(part1, "", "  ")
	_ = os.WriteFile(filepath.Join(partDir1, "prt_001.json"), p1Bytes, 0644)

	// Message 2: Assistant
	msg2 := openCodeMessageDoc{
		ID:        "msg_002",
		SessionID: sessID,
		Role:      "assistant",
	}
	msg2.Time.Created = sessDoc.Time.Updated
	m2Bytes, _ := json.MarshalIndent(msg2, "", "  ")
	_ = os.WriteFile(filepath.Join(msgDir, "msg_002.json"), m2Bytes, 0644)

	// Part 2: Tool call
	part2 := openCodePartDoc{
		ID:        "prt_002",
		SessionID: sessID,
		MessageID: "msg_002",
		Type:      "tool_call",
		Call: map[string]any{
			"tool": toolName,
			"path": "main.go",
		},
	}
	p2Bytes, _ := json.MarshalIndent(part2, "", "  ")
	_ = os.WriteFile(filepath.Join(partDir2, "prt_002.json"), p2Bytes, 0644)
}
