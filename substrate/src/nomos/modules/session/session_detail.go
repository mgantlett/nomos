package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SessionEvent captures a discrete causal milestone or action within a session (#SessionEvent).
type SessionEvent struct {
	Type      string `json:"type"` // USER_REQUEST, TOOL_CALL, PLAN, RECEIPT
	Timestamp string `json:"timestamp,omitempty"`
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	Tool      string `json:"tool,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// SessionDetail captures the deep causal inspection telemetry of an agent session (#SessionDetail).
type SessionDetail struct {
	AgentSession
	ToolsSummary  map[string]int     `json:"tools_summary"`
	Events        []SessionEvent     `json:"events"`
	RecentPrompts []string           `json:"recent_prompts,omitempty"`
	Sentinel      *SentinelScorecard `json:"sentinel,omitempty"`
	Trajectory    *SessionTrajectory `json:"trajectory,omitempty"`
	Persona       string             `json:"persona,omitempty"`
}

type transcriptLine struct {
	Type      string            `json:"type"`
	CreatedAt string            `json:"created_at"`
	Status    string            `json:"status"`
	Content   string            `json:"content"`
	Thinking  string            `json:"thinking,omitempty"`
	ToolCalls []rawToolCallItem `json:"tool_calls"`
}

type rawToolCallItem struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// GetSessionDetail discovers and deeply parses an individual agent session by ID across all providers.
func GetSessionDetail(rootOverride, repoRoot, sessionID string) (*SessionDetail, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}
	activeWorktrees := discoverActiveWorktrees(repoRoot)
	providers := DefaultProviders(rootOverride)

	for _, p := range providers {
		detail, err := p.GetDetail(sessionID, repoRoot, activeWorktrees)
		if err == nil && detail != nil {
			return detail, nil
		}
	}
	return nil, fmt.Errorf("session %q not found across supported Tier 1 agent providers", sessionID)
}

func detectPersona(sess *AgentSession, repoRoot string) string {
	lower := strings.ToLower(repoRoot + " " + sess.WorktreePath + " " + sess.RecentSummary)
	if isDownstreamConsumerContext(repoRoot, sess.WorktreePath) {
		return "CONSUMER_MODE"
	}
	if strings.Contains(lower, "audit") || strings.Contains(lower, "open-core") {
		return "AUDITOR_MODE"
	}
	return "SUBSTRATE_ARCHITECT"
}

func isDownstreamConsumerContext(repoRoot, worktreePath string) bool {
	if repoRoot != "" {
		cmdDir := filepath.Join(repoRoot, "substrate", "cmd", "nomos")
		if _, err := os.Stat(cmdDir); err == nil {
			return false
		}
		if filepath.Base(repoRoot) != "nomos" {
			return true
		}
	}
	if worktreePath != "" && !strings.Contains(filepath.ToSlash(worktreePath), "/nomos-") {
		return true
	}
	return false
}

func populateDetailFromLines(detail *SessionDetail, lines []string) {
	for _, raw := range lines {
		var item transcriptLine
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}
		processDetailItem(detail, &item)
	}
	trimDetailHistory(detail)
}

func processDetailItem(detail *SessionDetail, item *transcriptLine) {
	if item.Type == "USER_INPUT" {
		recordUserEvent(detail, item)
		return
	}
	if item.Thinking != "" {
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "THINKING",
			Timestamp: item.CreatedAt,
			Title:     "Agent Deliberation",
			Detail:    item.Thinking,
		})
	}
	for _, tc := range item.ToolCalls {
		recordToolCall(detail, item, &tc)
	}
}

func recordUserEvent(detail *SessionDetail, item *transcriptLine) {
	title := cleanSummary(item.Content)
	fullPrompt := cleanFullPrompt(item.Content)
	if title != "" {
		detail.RecentPrompts = append(detail.RecentPrompts, title)
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "USER_REQUEST",
			Timestamp: item.CreatedAt,
			Title:     title,
			Detail:    fullPrompt,
		})
	}
}

func cleanFullPrompt(content string) string {
	clean := strings.TrimSpace(content)
	if strings.Contains(clean, "<USER_REQUEST>") {
		start := strings.Index(clean, "<USER_REQUEST>") + len("<USER_REQUEST>")
		end := strings.Index(clean, "</USER_REQUEST>")
		if end > start {
			clean = clean[start:end]
		}
	}
	clean = strings.TrimSpace(clean)
	if len(clean) > 800 {
		return clean[:797] + "..."
	}
	return clean
}

func cleanSummary(content string) string {
	clean := cleanFullPrompt(content)
	lines := strings.Split(clean, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "<") {
			if len(l) > 80 {
				return l[:77] + "..."
			}
			return l
		}
	}
	if len(clean) > 80 {
		return clean[:77] + "..."
	}
	return clean
}

func recordToolCall(detail *SessionDetail, item *transcriptLine, tc *rawToolCallItem) {
	toolName := tc.Name
	if toolName == "" {
		return
	}
	detail.ToolsSummary[toolName]++

	if isSignificantTool(toolName) {
		title := extractToolTitle(tc)
		toolDetail := extractToolDetail(tc)
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "TOOL_CALL",
			Timestamp: item.CreatedAt,
			Title:     title,
			Detail:    toolDetail,
			Tool:      toolName,
			IsError:   item.Status == "ERROR",
		})
	}
}

var significantToolNames = map[string]bool{
	"run_command": true, "write_to_file": true, "replace_file_content": true, "multi_replace_file_content": true,
	"edit_file": true, "write_file": true, "read_file": true, "find_by_name": true, "list_dir": true,
	"search_web": true, "invoke_subagent": true, "apply_patch": true,
	"browser_subagent": true, "generate_image": true, "grep_search": true, "view_file": true, "read_url_content": true, "ask_question": true,
	"Bash": true, "Read": true, "Write": true, "Edit": true, "MultiEdit": true, "Grep": true, "Glob": true, "WebFetch": true, "WebSearch": true,
	"bash": true, "read": true, "write": true, "edit": true, "grep": true, "glob": true,
}

func extractToolDetail(tc *rawToolCallItem) string {
	if tc.Args == nil {
		return ""
	}
	priorityKeys := []string{
		"CommandLine", "command", "Path", "TargetFile", "AbsolutePath",
		"file_path", "filePath", "path", "Query", "query", "pattern", "Url", "url", "Prompt",
	}
	for _, k := range priorityKeys {
		if val, ok := tc.Args[k]; ok && val != nil {
			s := strings.Trim(fmt.Sprintf("%v", val), "\"")
			if len(s) > 160 {
				return s[:157] + "..."
			}
			return s
		}
	}
	return ""
}

func extractToolTitle(tc *rawToolCallItem) string {
	if tc.Args != nil {
		valToString := func(v any) string {
			if s, ok := v.(string); ok {
				return strings.Trim(s, "\"")
			}
			return fmt.Sprintf("%v", v)
		}
		if summary, ok := tc.Args["toolSummary"]; ok && summary != nil {
			return valToString(summary)
		}
		if action, ok := tc.Args["toolAction"]; ok && action != nil {
			return valToString(action)
		}
	}
	return tc.Name
}

func isSignificantTool(name string) bool {
	return significantToolNames[name]
}

func trimDetailHistory(detail *SessionDetail) {
	if len(detail.Events) > 100 {
		detail.Events = detail.Events[len(detail.Events)-100:]
	}
	if len(detail.RecentPrompts) > 10 {
		detail.RecentPrompts = detail.RecentPrompts[len(detail.RecentPrompts)-10:]
	}
}
