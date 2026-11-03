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

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
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

const (
	hookFieldToolCall  = "toolCall"
	hookFieldToolInput = "tool_input"
	hookFieldArgs      = "args"
	hookFieldCommand   = "command"
	hookFieldFilePath  = "file_path"
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
		extractNestedString(m, hookFieldToolInput, hookFieldPath),
		extractNestedString(m, hookFieldToolInput, "notebook_path"),
		extractNestedString(m, hookFieldArgs, hookFieldFilePath),
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

// EvaluatePreToolHook enforces Law 3 boundary isolation, Law 7 command safety,
// and Law 1 3-strike identical-failure loop circuit-breaking on PreToolUse events.
func EvaluatePreToolHook(p *HookPayload, hermeticDir string) HookDecision {
	if p == nil {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if p.CommandLine != "" {
		if dec := evaluateCommandGuard(p.CommandLine, hermeticDir); dec.Decision == hookDecisionDeny {
			return dec
		}
	}
	if dec := evaluateMCPPreToolGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateTargetFilePreToolGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateCapabilityFlowGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	return EvaluateLoopCircuitBreaker(p, hermeticDir)
}

func evaluateTargetFilePreToolGuard(p *HookPayload, hermeticDir string) HookDecision {
	if p.TargetFile == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if p.CommandLine == "" && hasUnexpandedShellPathToken(p.TargetFile) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Native file tools do not expand shell syntax; pass a concrete relative or resolved path.", p.TargetFile),
		}
	}
	if !isReadOnlyTool(p.ToolName) {
		return evaluateFileMutationGuard(p, hermeticDir)
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func evaluateMCPPreToolGuard(p *HookPayload, hermeticDir string) HookDecision {
	mcpInv, ok := capbac.ParseMCPInvocation(p.ToolName, p.TargetFile, p.ArgsDigest)
	if !ok {
		return HookDecision{Decision: hookDecisionAllow}
	}
	wsRoot := resolveCanonicalHookWorkspace(p, hermeticDir)
	policy := loadWorkspaceMCPBoundaryPolicy(wsRoot)
	dec := capbac.EvaluateMCPBoundary(wsRoot, mcpInv, policy)
	if !dec.Allowed {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   dec.Reason,
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func loadWorkspaceMCPBoundaryPolicy(wsRoot string) capbac.MCPBoundaryPolicy {
	cfg, err := config.LoadConfig(wsRoot)
	if err != nil || cfg == nil {
		return capbac.MCPBoundaryPolicy{}
	}
	return capbac.MCPBoundaryPolicy{
		AllowedServers: cfg.Governance.Boundary.AllowedMCPServers,
		BlockedServers: cfg.Governance.Boundary.BlockedMCPServers,
	}
}

func evaluateCapabilityFlowGuard(p *HookPayload, hermeticDir string) HookDecision {
	wsRoot := resolveCanonicalHookWorkspace(p, hermeticDir)
	flowDec := capbac.EvaluateWorkspaceToolFlow(wsRoot, capbac.ToolFlowInvocation{
		ToolName:    p.ToolName,
		TargetFile:  p.TargetFile,
		CommandLine: p.CommandLine,
		ArgsDigest:  p.ArgsDigest,
	})
	if !flowDec.Allowed {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   flowDec.Reason,
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func resolveCanonicalHookWorkspace(p *HookPayload, hermeticDir string) string {
	if hermeticDir != "" {
		return filepath.Clean(hermeticDir)
	}
	base := resolveHookWorkspaceDir(p, "")
	if pr, err := workspace.FindRepoRoot(base); err == nil && pr != "" {
		return filepath.Clean(pr)
	}
	return filepath.Clean(base)
}

func isReadOnlyTool(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if mcpInv, ok := capbac.ParseMCPInvocation(lower, "", ""); ok {
		return mcpInv.Mode == capbac.MCPModeRead
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
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Pass a canonical resolved path.", p.TargetFile),
		}
	}
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if isSystemRootPath(resolved) {
		return HookDecision{
			Decision: hookDecisionDeny,
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
		return HookDecision{Decision: hookDecisionAllow}
	}
	return HookDecision{
		Decision: hookDecisionDeny,
		Reason:   fmt.Sprintf("Hermetic Container Violation (Law 3): Target '%s' (resolved: '%s') escapes hermetic root '%s'.", rawTarget, resolved, canonRoot),
	}
}

func evaluateWorktreeBoundary(rawTarget, resolved, toolName string) HookDecision {
	if isPermittedWorktreePath(resolved) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if toolName == "" {
		toolName = "file_mutation"
	}
	choreHint := `nomos task chore "<title>"`
	if strings.Contains(resolved, "/intent/") || strings.HasSuffix(resolved, "/nomos.yaml") {
		choreHint = `nomos task chore "<title>" --plane intent`
	}
	return HookDecision{
		Decision: hookDecisionDeny,
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
