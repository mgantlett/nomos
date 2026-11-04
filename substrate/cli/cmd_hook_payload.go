// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// HookPayload normalizes incoming JSON hook events across Claude Code, Gemini CLI, Cursor, and Antigravity.
type HookPayload struct {
	ToolName       string
	TargetFile     string
	CommandLine    string
	ArgsDigest     string
	Cwd            string
	WorkspacePaths []string
	HasToolInput   bool
	IsClaudeEvent  bool
	IsGeminiEvent  bool
	IsPostInvoke   bool
}

var (
	readOnlyTools = map[string]bool{
		"read": true, "read_file": true, "view_file_outline": true, "view_code_item": true,
		"grep_search": true, "find_by_name": true, "list_dir": true, "list_directory": true,
		"glob": true, "grep": true, "search_file_content": true,
		"bash": true, "run_command": true, "execute_command": true, "run_shell_command": true,
	}
	geminiSpecificTools = map[string]bool{
		"run_shell_command": true, "write_file": true, "replace": true,
		"list_directory": true, "search_file_content": true, "google_web_search": true,
	}
	systemRootPrefixes = []string{"/etc", "/usr", "/bin", "/sbin", "/var", "/proc", "/sys", "/dev", "/boot"}
)

func parseHookPayload(raw []byte) *HookPayload {
	p := &HookPayload{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return p
	}
	var m map[string]any
	if err := json.Unmarshal(trimmed, &m); err != nil {
		return p
	}
	_, p.HasToolInput = m["tool_input"]
	_, hasSessionID := m["session_id"]
	_, p.IsPostInvoke = m["invocationNum"]
	p.Cwd = extractStringField(m, "cwd")
	p.WorkspacePaths = extractStringSlice(m, "workspacePaths")
	p.ToolName = firstNonEmpty(
		extractNestedString(m, "toolCall", "name"),
		extractStringField(m, "tool_name"),
		extractStringField(m, "toolName"),
	)
	p.IsClaudeEvent, p.IsGeminiEvent = classifyHookEventFamily(
		extractStringField(m, "hook_event_name"),
		p.ToolName,
		p.HasToolInput,
		hasSessionID,
	)
	p.TargetFile = extractTargetFileField(m)
	p.CommandLine = extractCommandField(m)
	p.ArgsDigest = extractArgsDigestField(m)
	return p
}

func classifyHookEventFamily(eventName, toolName string, hasToolInput, hasSessionID bool) (bool, bool) {
	lowerEvent := strings.ToLower(strings.TrimSpace(eventName))
	lowerTool := strings.ToLower(strings.TrimSpace(toolName))
	if lowerEvent == "beforetool" || lowerEvent == "aftertool" {
		return false, true
	}
	if lowerEvent == "" && geminiSpecificTools[lowerTool] {
		return false, true
	}
	if lowerEvent == "pretooluse" || lowerEvent == "posttooluse" || lowerEvent == "sessionstart" {
		return true, false
	}
	if lowerEvent == "" && hasToolInput && hasSessionID {
		return true, false
	}
	return false, false
}

const (
	hookFieldToolCall  = "toolCall"
	hookFieldToolInput = "tool_input"
	hookFieldArgs      = "args"
	hookFieldCommand   = "command"
	hookFieldFilePath  = "file_path"
	hookFieldDirPath   = "dir_path"
	hookFieldPath      = "path"
)

func extractArgsDigestField(m map[string]any) string {
	for _, key := range []string{hookFieldToolInput, hookFieldArgs} {
		if sub, ok := m[key]; ok {
			if raw, err := json.Marshal(sub); err == nil {
				return string(raw)
			}
		}
	}
	if tc, ok := m[hookFieldToolCall].(map[string]any); ok {
		if sub, ok := tc[hookFieldArgs]; ok {
			if raw, err := json.Marshal(sub); err == nil {
				return string(raw)
			}
		}
	}
	return ""
}

func extractTargetFileField(m map[string]any) string {
	return firstNonEmpty(
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "TargetFile"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "Path"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, hookFieldFilePath),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, hookFieldPath),
		extractNestedString(m, hookFieldToolInput, hookFieldFilePath),
		extractNestedString(m, hookFieldToolInput, hookFieldDirPath),
		extractNestedString(m, hookFieldToolInput, hookFieldPath),
		extractNestedString(m, hookFieldToolInput, "notebook_path"),
		extractNestedString(m, hookFieldArgs, hookFieldFilePath),
		extractNestedString(m, hookFieldArgs, hookFieldDirPath),
		extractNestedString(m, hookFieldArgs, hookFieldPath),
	)
}

func extractCommandField(m map[string]any) string {
	return firstNonEmpty(
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "CommandLine"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, hookFieldCommand),
		extractNestedString(m, hookFieldToolInput, hookFieldCommand),
		extractNestedString(m, hookFieldArgs, hookFieldCommand),
	)
}

func extractStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func extractNestedString(m map[string]any, k1, k2 string) string {
	sub, ok := m[k1].(map[string]any)
	if !ok {
		return ""
	}
	return extractStringField(sub, k2)
}

func extractDeepString(m map[string]any, k1, k2, k3 string) string {
	sub, ok := m[k1].(map[string]any)
	if !ok {
		return ""
	}
	return extractNestedString(sub, k2, k3)
}

func extractStringSlice(m map[string]any, key string) []string {
	arr, ok := m[key].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range arr {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" && v != "null" {
			return v
		}
	}
	return ""
}

func resolveHookWorkspaceDir(p *HookPayload, hermeticDir string) string {
	if hermeticDir != "" {
		return hermeticDir
	}
	if len(p.WorkspacePaths) > 0 {
		return p.WorkspacePaths[0]
	}
	if p.Cwd != "" {
		return p.Cwd
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}
