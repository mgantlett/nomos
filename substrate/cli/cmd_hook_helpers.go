// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HookPayload normalizes incoming JSON hook events across Claude Code, Cursor, and Antigravity.
type HookPayload struct {
	ToolName       string
	TargetFile     string
	CommandLine    string
	ArgsDigest     string
	Cwd            string
	WorkspacePaths []string
	HasToolInput   bool
	IsClaudeEvent  bool
	IsPostInvoke   bool
}

var (
	readOnlyTools = map[string]bool{
		"read": true, "read_file": true, "view_file_outline": true, "view_code_item": true,
		"grep_search": true, "find_by_name": true, "list_dir": true, "glob": true, "grep": true,
		"bash": true, "run_command": true, "execute_command": true,
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
	p.IsClaudeEvent = extractStringField(m, "hook_event_name") != "" || (p.HasToolInput && hasSessionID)
	_, p.IsPostInvoke = m["invocationNum"]
	p.Cwd = extractStringField(m, "cwd")
	p.WorkspacePaths = extractStringSlice(m, "workspacePaths")
	p.ToolName = firstNonEmpty(
		extractNestedString(m, "toolCall", "name"),
		extractStringField(m, "tool_name"),
		extractStringField(m, "toolName"),
	)
	p.TargetFile = extractTargetFileField(m)
	p.CommandLine = extractCommandField(m)
	p.ArgsDigest = extractArgsDigestField(m)
	return p
}

func extractArgsDigestField(m map[string]any) string {
	for _, key := range []string{"tool_input", "args"} {
		if sub, ok := m[key]; ok {
			if raw, err := json.Marshal(sub); err == nil {
				return string(raw)
			}
		}
	}
	if tc, ok := m["toolCall"].(map[string]any); ok {
		if sub, ok := tc["args"]; ok {
			if raw, err := json.Marshal(sub); err == nil {
				return string(raw)
			}
		}
	}
	return ""
}

func extractTargetFileField(m map[string]any) string {
	return firstNonEmpty(
		extractDeepString(m, "toolCall", "args", "TargetFile"),
		extractDeepString(m, "toolCall", "args", "Path"),
		extractDeepString(m, "toolCall", "args", "file_path"),
		extractDeepString(m, "toolCall", "args", "path"),
		extractNestedString(m, "tool_input", "file_path"),
		extractNestedString(m, "tool_input", "path"),
		extractNestedString(m, "tool_input", "notebook_path"),
		extractNestedString(m, "args", "file_path"),
		extractNestedString(m, "args", "path"),
	)
}

func extractCommandField(m map[string]any) string {
	return firstNonEmpty(
		extractDeepString(m, "toolCall", "args", "CommandLine"),
		extractDeepString(m, "toolCall", "args", "command"),
		extractNestedString(m, "tool_input", "command"),
		extractNestedString(m, "args", "command"),
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

// EvaluatePreToolHook enforces Law 3 boundary isolation, Law 7 command safety,
// and Law 1 3-strike identical-failure loop circuit-breaking on PreToolUse events.
func EvaluatePreToolHook(p *HookPayload, hermeticDir string) HookDecision {
	if p == nil {
		return HookDecision{Decision: "allow"}
	}
	if p.CommandLine != "" {
		if dec := evaluateCommandGuard(p.CommandLine, hermeticDir); dec.Decision == "deny" {
			return dec
		}
	}
	if p.TargetFile != "" && p.CommandLine == "" && hasUnexpandedShellPathToken(p.TargetFile) {
		return HookDecision{
			Decision: "deny",
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Native file tools do not expand shell syntax; pass a concrete relative or resolved path.", p.TargetFile),
		}
	}
	if p.TargetFile != "" && !isReadOnlyTool(p.ToolName) {
		if dec := evaluateFileMutationGuard(p, hermeticDir); dec.Decision == "deny" {
			return dec
		}
	}
	return EvaluateLoopCircuitBreaker(p, hermeticDir)
}

func isReadOnlyTool(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if strings.HasPrefix(lower, "mcp_") {
		return true
	}
	return readOnlyTools[lower]
}

func hasUnexpandedShellPathToken(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "~") {
		return true
	}
	return strings.Contains(trimmed, "$")
}

func evaluateFileMutationGuard(p *HookPayload, hermeticDir string) HookDecision {
	if hasUnexpandedShellPathToken(p.TargetFile) {
		return HookDecision{
			Decision: "deny",
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Pass a canonical resolved path.", p.TargetFile),
		}
	}
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if isSystemRootPath(resolved) {
		return HookDecision{
			Decision: "deny",
			Reason:   fmt.Sprintf("Hermetic Boundary Violation (Law 3): System path mutation '%s' is strictly prohibited.", resolved),
		}
	}
	if hermeticDir != "" {
		return evaluateHermeticDirBoundary(p.TargetFile, resolved, hermeticDir)
	}
	return evaluateWorktreeBoundary(p.TargetFile, resolved, p.ToolName)
}

func evaluateHermeticDirBoundary(rawTarget, resolved, hermeticDir string) HookDecision {
	canonRoot := normalizeCanonicalPath(hermeticDir, "/")
	if resolved == canonRoot || strings.HasPrefix(resolved, canonRoot+"/") {
		return HookDecision{Decision: "allow"}
	}
	return HookDecision{
		Decision: "deny",
		Reason:   fmt.Sprintf("Hermetic Container Violation (Law 3): Target '%s' (resolved: '%s') escapes hermetic root '%s'.", rawTarget, resolved, canonRoot),
	}
}

func evaluateWorktreeBoundary(rawTarget, resolved, toolName string) HookDecision {
	if isPermittedWorktreePath(resolved) {
		return HookDecision{Decision: "allow"}
	}
	if toolName == "" {
		toolName = "file_mutation"
	}
	choreHint := `nomos task chore "<title>"`
	if strings.Contains(resolved, "/intent/") || strings.HasSuffix(resolved, "/nomos.yaml") {
		choreHint = `nomos task chore "<title>" --plane intent`
	}
	return HookDecision{
		Decision: "deny",
		Reason: fmt.Sprintf(
			"Hermetic Worktree Violation (Law 3): Direct trunk mutation of '%s' (resolved: '%s') via '%s' is prohibited. Provision an isolated EDIT worktree in 1 turn via '%s' (or 'nomos task start <KEY> --edit'), then edit strictly inside '/worktrees/'.",
			rawTarget, resolved, toolName, choreHint,
		),
	}
}

func isPermittedWorktreePath(resolved string) bool {
	return strings.Contains(resolved, "/worktrees/") || strings.Contains(resolved, "/.tmp/") || strings.Contains(resolved, "/brain/")
}

func isSystemRootPath(resolved string) bool {
	for _, sys := range systemRootPrefixes {
		if resolved == sys || strings.HasPrefix(resolved, sys+"/") {
			return true
		}
	}
	return resolved == "/"
}

func normalizeCanonicalPath(raw, base string) string {
	target := raw
	if !filepath.IsAbs(target) {
		if base == "" {
			base = "/"
		}
		target = filepath.Join(base, target)
	}
	cleaned := filepath.Clean(target)
	if eval, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.ToSlash(eval)
	}
	parent := filepath.Dir(cleaned)
	if evalParent, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.ToSlash(filepath.Join(evalParent, filepath.Base(cleaned)))
	}
	return filepath.ToSlash(cleaned)
}
