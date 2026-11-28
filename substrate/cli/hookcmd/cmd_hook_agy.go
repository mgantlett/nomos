// Package hookcmd implements Antigravity 2.0 and Cursor hook normalization, scaffolding, and stdio JSON responses.
// @intent intent.md
// @intent nomos_hermetic_worktree_spec.md
package hookcmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/hookcmd/shellguard"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/tui"
)

// Canonical Cursor and agent hook manifest path tokens.
const (
	CursorDirToken = ".cursor"
	AgentsDirToken = ".agents"
	HooksFileName  = "hooks.json"

	cursorEventSessionStart = "sessionStart"
	cursorEventPreToolUse   = "preToolUse"
	cursorEventBeforeShell  = "beforeShellExecution"
	cursorEventPostToolUse  = "postToolUse"
	cursorEventAfterEdit    = "afterFileEdit"
	cursorEventStop         = "stop"
	cursorMaxStopLoops      = 3
)

var cursorExclusiveEvents = map[string]bool{
	"beforeshellexecution": true,
	"afterfileedit":        true,
	"beforereadfile":       true,
	"beforemcpexecution":   true,
	"aftershellexecution":  true,
	"aftermcpexecution":    true,
	"beforesubmitprompt":   true,
}

// isAntigravityHookEnvelope returns true when the JSON hook map carries Antigravity 2.0 fields.
func isAntigravityHookEnvelope(m map[string]any) bool {
	for _, key := range []string{hookFieldToolCall, "invocationNum", "terminationReason", "conversationId", "stepIdx"} {
		if _, exists := m[key]; exists {
			return true
		}
	}
	return false
}

// extractEffectiveCwd resolves per-call working directory overrides ahead of the session-level cwd.
func extractEffectiveCwd(m map[string]any) string {
	return firstNonEmpty(
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "cwd"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "Cwd"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "workdir"),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, "working_directory"),
		extractNestedString(m, hookFieldToolInput, "cwd"),
		extractNestedString(m, hookFieldToolInput, "workdir"),
		extractNestedString(m, hookFieldToolInput, "working_directory"),
		extractNestedString(m, hookFieldToolArgs, "cwd"),
		extractNestedString(m, hookFieldToolArgs, "workdir"),
		extractNestedString(m, hookFieldArgs, "cwd"),
		extractNestedString(m, hookFieldArgs, "workdir"),
		extractStringField(m, "cwd"),
	)
}

// extractAllTargetFiles collects the primary target path, multi-file array fields, and apply_patch headers.
func extractAllTargetFiles(m map[string]any, primary string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(raw string) {
		if trimmed := strings.TrimSpace(raw); trimmed != "" && trimmed != "null" && !seen[trimmed] {
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	add(primary)
	for _, item := range collectNestedPathSlices(m) {
		add(item)
	}
	patch := firstNonEmpty(
		extractNestedString(m, hookFieldToolInput, hookFieldPatchText),
		extractNestedString(m, hookFieldToolArgs, hookFieldPatchText),
		extractNestedString(m, hookFieldArgs, hookFieldPatchText),
		extractDeepString(m, hookFieldToolCall, hookFieldArgs, hookFieldPatchText),
	)
	if patch == "" {
		return out
	}
	for _, line := range strings.Split(patch, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"*** Update File:", "*** Add File:", "*** Delete File:", "*** Move to:"} {
			if strings.HasPrefix(trimmed, prefix) {
				add(strings.TrimPrefix(trimmed, prefix))
			}
		}
	}
	return out
}

// collectNestedPathSlices extracts multi-file target path arrays across nested IDE payload containers.
func collectNestedPathSlices(m map[string]any) []string {
	sliceKeys := []string{"paths", "Paths", "file_paths", "filePaths", "files", "TargetFiles"}
	var all []string
	for _, rootKey := range []string{hookFieldToolInput, hookFieldToolArgs, hookFieldArgs} {
		if sub, ok := m[rootKey].(map[string]any); ok {
			for _, sk := range sliceKeys {
				all = append(all, extractStringSlice(sub, sk)...)
			}
		}
	}
	if tc, ok := m[hookFieldToolCall].(map[string]any); ok {
		if sub, ok := tc[hookFieldArgs].(map[string]any); ok {
			for _, sk := range sliceKeys {
				all = append(all, extractStringSlice(sub, sk)...)
			}
		}
	}
	return all
}

// resolveAntigravityPendingPath computes the session-scoped buffer path for pending Antigravity advisories.
func resolveAntigravityPendingPath(p *shellguard.HookPayload, hermeticDir string) string {
	if p == nil {
		return ""
	}
	seed := strings.TrimSpace(p.SessionID)
	if seed == "" {
		seed = filepath.Clean(resolveHookWorkspaceDir(p, hermeticDir))
	}
	hookDir := resolveHookStateCacheDir()
	if hookDir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(seed))
	return filepath.Join(hookDir, fmt.Sprintf("nomos-hook-agy-adv-%s.txt", hex.EncodeToString(sum[:8])))
}

// queueAntigravityAdvisory buffers non-empty PostToolUse advisories for injection at PostInvocation.
func queueAntigravityAdvisory(p *shellguard.HookPayload, hermeticDir, msg string) {
	trimmed := strings.TrimSpace(msg)
	path := resolveAntigravityPendingPath(p, hermeticDir)
	if trimmed == "" || path == "" {
		return
	}
	var existing string
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
		if raw, rErr := os.ReadFile(path); rErr == nil {
			existing = strings.TrimSpace(string(raw))
		}
	}
	combined := trimmed
	if existing != "" && !strings.Contains(existing, trimmed) {
		combined = existing + "\n" + trimmed
	} else if existing != "" {
		combined = existing
	}
	if wErr := os.WriteFile(path, []byte(combined), 0600); wErr != nil {
		return
	}
}

// popAntigravityAdvisory drains buffered PostToolUse advisories once PostInvocation fires.
func popAntigravityAdvisory(p *shellguard.HookPayload, hermeticDir string) string {
	path := resolveAntigravityPendingPath(p, hermeticDir)
	if path == "" {
		return ""
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	data, rErr := os.ReadFile(path)
	if rmErr := os.Remove(path); rmErr != nil && rErr != nil {
		return ""
	}
	if rErr != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// DefaultCursorHooksJSON returns the canonical Cursor .cursor/hooks.json (version 1) hook manifest.
func DefaultCursorHooksJSON() []byte {
	specs := []tui.AgentHookEventSpec{
		{Event: cursorEventSessionStart, Subcmd: "session-start", Timeout: 10},
		{Event: cursorEventPreToolUse, Subcmd: "pre-tool", Timeout: 10},
		{Event: cursorEventBeforeShell, Subcmd: "pre-tool", Timeout: 10},
		{Event: cursorEventPostToolUse, Subcmd: "post-tool", Timeout: 5},
		{Event: cursorEventAfterEdit, Subcmd: "post-tool", Timeout: 5},
		{Event: cursorEventStop, Subcmd: cursorEventStop, Timeout: 10},
	}
	hooksMap := make(map[string]any, len(specs))
	for _, sp := range specs {
		entry := map[string]any{
			"type":                   tui.HookEntryTypeCommand,
			tui.HookEntryTypeCommand: tui.PosixHookShellCommand(sp.Subcmd),
			"timeout":                sp.Timeout,
		}
		if sp.Matcher != "" {
			entry["matcher"] = sp.Matcher
		}
		hooksMap[sp.Event] = []any{entry}
	}
	out, err := tui.MarshalManifestNoHTML(map[string]any{"version": 1, "hooks": hooksMap})
	if err != nil {
		return nil
	}
	return out
}

// ResolveCursorManifestPath resolves project (.cursor/hooks.json) or user (~/.cursor/hooks.json) manifest paths.
// @intent task_lifecycle_cli_spec.md
func ResolveCursorManifestPath(userScope bool, baseDir string) string {
	if userScope {
		if envDir := strings.TrimSpace(os.Getenv("CURSOR_CONFIG_DIR")); envDir != "" {
			return filepath.Join(envDir, HooksFileName)
		}
	}
	return filepath.Join(baseDir, CursorDirToken, HooksFileName)
}

// isCursorHookEnvelope returns true when the incoming JSON payload carries Cursor hook coordinates.
func isCursorHookEnvelope(m map[string]any) bool {
	if isAntigravityHookEnvelope(m) {
		return false
	}
	eventName := strings.TrimSpace(extractStringField(m, "hook_event_name"))
	if cursorExclusiveEvents[strings.ToLower(eventName)] {
		return true
	}
	for _, key := range []string{"workspace_roots", "generation_id", "cursor_version", "edits"} {
		if _, exists := m[key]; exists {
			return true
		}
	}
	switch eventName {
	case cursorEventSessionStart, cursorEventPreToolUse, cursorEventPostToolUse:
		return true
	case cursorEventStop:
		_, hasConvID := m["conversation_id"]
		_, hasSessID := m["session_id"]
		return hasConvID && !hasSessID
	default:
		return false
	}
}

// normalizeCursorPayload populates workspace_roots, command, file_path, edits, and stop state for Cursor events.
func normalizeCursorPayload(p *shellguard.HookPayload, m map[string]any, eventName string) {
	if len(p.WorkspacePaths) == 0 {
		p.WorkspacePaths = extractStringSlice(m, "workspace_roots")
	}
	if !p.IsCursorEvent {
		return
	}
	p.IsClaudeEvent, p.IsGeminiEvent = false, false
	if p.Cwd == "" && len(p.WorkspacePaths) > 0 {
		p.Cwd = p.WorkspacePaths[0]
	}
	lowerEvent := strings.ToLower(strings.TrimSpace(eventName))
	if lowerEvent == "beforeshellexecution" && p.ToolName == "" {
		p.ToolName = "bash"
	} else if lowerEvent == "afterfileedit" && p.ToolName == "" {
		p.ToolName = "Edit"
	}
	if p.ArgsDigest == "" {
		p.ArgsDigest = buildCursorArgsDigest(m)
	}
	normalizeCursorStopStatus(p, m)
}

// normalizeCursorStopStatus maps Cursor stop loop counts and termination statuses onto shellguard.HookPayload.
func normalizeCursorStopStatus(p *shellguard.HookPayload, m map[string]any) {
	if status := strings.ToLower(extractStringField(m, "status")); status == "aborted" || status == "error" {
		p.TerminationReason = strings.ToUpper(status)
	}
	if count, ok := m["loop_count"].(float64); ok && int(count) >= cursorMaxStopLoops {
		p.StopHookActive = true
	}
}

// buildCursorArgsDigest serializes Cursor tool arguments into a deterministic digest string.
func buildCursorArgsDigest(m map[string]any) string {
	digestMap := make(map[string]any)
	for _, key := range []string{hookFieldCommand, hookFieldFilePath, "edits"} {
		if val, ok := m[key]; ok {
			digestMap[key] = val
		}
	}
	if len(digestMap) == 0 {
		return ""
	}
	raw, err := json.Marshal(digestMap)
	if err != nil {
		return ""
	}
	return string(raw)
}

// extractCursorEditsContent concatenates new_string blocks across Cursor afterFileEdit edits slices.
func extractCursorEditsContent(m map[string]any) string {
	rawEdits, ok := m["edits"].([]any)
	if !ok {
		if sub, subOK := m[hookFieldToolInput].(map[string]any); subOK {
			rawEdits, ok = sub["edits"].([]any)
		}
	}
	if !ok || len(rawEdits) == 0 {
		return ""
	}
	var parts []string
	for _, item := range rawEdits {
		if entry, isMap := item.(map[string]any); isMap {
			if s := extractStringField(entry, "new_string"); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// writeCursorSessionStartResponse emits Cursor's native sessionStart additional_context JSON payload.
func writeCursorSessionStartResponse(w io.Writer, banner string) error {
	return writeJSONMap(w, map[string]any{"additional_context": banner})
}

// writeCursorPreToolResponse emits Cursor's native permission decision JSON for preToolUse and beforeShellExecution.
func writeCursorPreToolResponse(w io.Writer, dec shellguard.HookDecision) error {
	if dec.Decision != hookDecisionDeny {
		return writeJSONMap(w, map[string]any{"permission": hookDecisionAllow})
	}
	return writeJSONMap(w, map[string]any{"permission": hookDecisionDeny, "user_message": dec.Reason, "agent_message": dec.Reason})
}

// writeCursorPostToolResponse emits Cursor's native postToolUse additional_context JSON payload.
func writeCursorPostToolResponse(w io.Writer, msg string) error {
	return writeJSONMap(w, map[string]any{"additional_context": msg})
}

// writeCursorStopResponse emits Cursor's native stop followup_message JSON payload when blocking exit.
func writeCursorStopResponse(w io.Writer, reason string) error {
	return writeJSONMap(w, map[string]any{"followup_message": reason})
}
