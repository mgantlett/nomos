// Package cli implements native Cursor lifecycle hook scaffolding, envelope
// detection, stdin normalization, and stdio JSON response formatting.
// @intent intent.md
// @intent nomos_hermetic_worktree_spec.md
package cli

import (
	"encoding/json"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	cursorDirToken = ".cursor"
	agentsDirToken = ".agents"
	hooksFileName  = "hooks.json"

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
	out, err := tui.MarshalManifestNoHTML(map[string]any{
		"version": 1,
		"hooks":   hooksMap,
	})
	if err != nil {
		return nil
	}
	return out
}

// resolveCursorManifestPath resolves project (.cursor/hooks.json) or user (~/.cursor/hooks.json) manifest paths.
func resolveCursorManifestPath(userScope bool, baseDir string) string {
	if userScope {
		if envDir := strings.TrimSpace(os.Getenv("CURSOR_CONFIG_DIR")); envDir != "" {
			return filepath.Join(envDir, hooksFileName)
		}
	}
	return filepath.Join(baseDir, cursorDirToken, hooksFileName)
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
func normalizeCursorPayload(p *HookPayload, m map[string]any, eventName string) {
	if len(p.WorkspacePaths) == 0 {
		p.WorkspacePaths = extractStringSlice(m, "workspace_roots")
	}
	if !p.IsCursorEvent {
		return
	}
	p.IsClaudeEvent = false
	p.IsGeminiEvent = false
	if p.Cwd == "" && len(p.WorkspacePaths) > 0 {
		p.Cwd = p.WorkspacePaths[0]
	}
	lowerEvent := strings.ToLower(strings.TrimSpace(eventName))
	if lowerEvent == "beforeshellexecution" && p.ToolName == "" {
		p.ToolName = "bash"
	}
	if lowerEvent == "afterfileedit" && p.ToolName == "" {
		p.ToolName = "Edit"
	}
	if p.ArgsDigest == "" {
		p.ArgsDigest = buildCursorArgsDigest(m)
	}
	normalizeCursorStopStatus(p, m)
}

func normalizeCursorStopStatus(p *HookPayload, m map[string]any) {
	status := strings.ToLower(extractStringField(m, "status"))
	if status == "aborted" || status == "error" {
		p.TerminationReason = strings.ToUpper(status)
	}
	if count, ok := m["loop_count"].(float64); ok && int(count) >= cursorMaxStopLoops {
		p.StopHookActive = true
	}
}

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
		entry, isMap := item.(map[string]any)
		if !isMap {
			continue
		}
		if s := extractStringField(entry, "new_string"); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// writeCursorSessionStartResponse emits Cursor's native sessionStart additional_context JSON payload.
func writeCursorSessionStartResponse(w io.Writer, banner string) error {
	return writeJSONMap(w, map[string]any{
		"additional_context": banner,
	})
}

// writeCursorPreToolResponse emits Cursor's native permission decision JSON for preToolUse and beforeShellExecution.
func writeCursorPreToolResponse(w io.Writer, dec HookDecision) error {
	if dec.Decision != hookDecisionDeny {
		return writeJSONMap(w, map[string]any{
			"permission": hookDecisionAllow,
		})
	}
	return writeJSONMap(w, map[string]any{
		"permission":    hookDecisionDeny,
		"user_message":  dec.Reason,
		"agent_message": dec.Reason,
	})
}

// writeCursorPostToolResponse emits Cursor's native postToolUse additional_context JSON payload.
func writeCursorPostToolResponse(w io.Writer, msg string) error {
	return writeJSONMap(w, map[string]any{
		"additional_context": msg,
	})
}

// writeCursorStopResponse emits Cursor's native stop followup_message JSON payload when blocking exit.
func writeCursorStopResponse(w io.Writer, reason string) error {
	return writeJSONMap(w, map[string]any{
		"followup_message": reason,
	})
}
