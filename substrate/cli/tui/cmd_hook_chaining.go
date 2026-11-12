// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// MergeHookManifest additively merges Nomos hook definitions into an existing
// JSON hook manifest or settings.json without clobbering existing hooks.
func MergeHookManifest(existingJSON, incomingJSON []byte) ([]byte, error) {
	trimmedExisting := bytes.TrimSpace(existingJSON)
	if len(trimmedExisting) == 0 {
		return formatManifestJSON(incomingJSON)
	}
	var existingMap map[string]any
	if err := json.Unmarshal(trimmedExisting, &existingMap); err != nil {
		return nil, fmt.Errorf("failed parsing existing hook manifest: %w", err)
	}
	var incomingMap map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(incomingJSON), &incomingMap); err != nil {
		return nil, fmt.Errorf("failed parsing incoming hook manifest: %w", err)
	}
	mergeHookObjectMaps(existingMap, incomingMap)
	return MarshalManifestNoHTML(existingMap)
}

func formatManifestJSON(raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &m); err != nil {
		return nil, err
	}
	return MarshalManifestNoHTML(m)
}

// MarshalManifestNoHTML executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func MarshalManifestNoHTML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mergeHookObjectMaps(dst, src map[string]any) {
	for k, srcVal := range src {
		dstVal, exists := dst[k]
		if !exists {
			dst[k] = srcVal
			continue
		}
		dstSubMap, dstIsMap := dstVal.(map[string]any)
		srcSubMap, srcIsMap := srcVal.(map[string]any)
		if dstIsMap && srcIsMap {
			mergeHookObjectMaps(dstSubMap, srcSubMap)
			continue
		}
		dstSlice, dstIsSlice := dstVal.([]any)
		srcSlice, srcIsSlice := srcVal.([]any)
		if dstIsSlice && srcIsSlice {
			dst[k] = mergeHookSlices(dstSlice, srcSlice)
		}
	}
}

func mergeHookSlices(existing, incoming []any) []any {
	seen := make(map[string]bool, len(existing))
	out := make([]any, 0, len(existing)+len(incoming))
	for _, item := range existing {
		sig := canonicalHookSignature(item)
		seen[sig] = true
		out = append(out, item)
	}
	for _, item := range incoming {
		sig := canonicalHookSignature(item)
		if !seen[sig] {
			seen[sig] = true
			out = append(out, item)
		}
	}
	return out
}

func canonicalHookSignature(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

const (
	HookEntryTypeCommand = "command"
	// AgentsDirToken names the shared multi-agent directory token (@intent task_lifecycle_cli_spec.md).
	AgentsDirToken = ".agents"
	// HooksFileName names the canonical JSON hooks manifest filename (@intent task_lifecycle_cli_spec.md).
	HooksFileName = "hooks.json"
)

type AgentHookEventSpec struct {
	Event   string
	Matcher string
	Subcmd  string
	Timeout int
}

// PosixHookShellCommand executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func PosixHookShellCommand(subcmd string) string {
	return fmt.Sprintf(
		`export PATH="$PATH:${GOBIN:+$GOBIN:}$HOME/go/bin:$HOME/.local/bin:$HOME/bin"; if command -v nomos >/dev/null 2>&1; then exec nomos hook %s; fi; printf '{}'`,
		subcmd,
	)
}

// BuildAgentHookSettingsJSON executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func BuildAgentHookSettingsJSON(specs []AgentHookEventSpec) []byte {
	hooksMap := make(map[string]any, len(specs))
	for _, sp := range specs {
		inner := map[string]any{
			"type":               HookEntryTypeCommand,
			HookEntryTypeCommand: PosixHookShellCommand(sp.Subcmd),
			"timeout":            sp.Timeout,
		}
		entry := map[string]any{"hooks": []any{inner}}
		if sp.Matcher != "" {
			entry["matcher"] = sp.Matcher
		}
		hooksMap[sp.Event] = []any{entry}
	}
	out, err := MarshalManifestNoHTML(map[string]any{"hooks": hooksMap})
	if err != nil {
		return nil
	}
	return out
}

// DefaultClaudeSettingsHooksJSON returns the canonical Claude Code settings.json hook structure.
func DefaultClaudeSettingsHooksJSON() []byte {
	return BuildAgentHookSettingsJSON([]AgentHookEventSpec{
		{Event: "SessionStart", Subcmd: "session-start", Timeout: 10},
		{Event: "PreToolUse", Matcher: "Bash|Read|Write|Edit|MultiEdit|NotebookEdit|Glob|Grep|WebFetch|WebSearch|Task.*|mcp__.*", Subcmd: "pre-tool", Timeout: 10},
		{Event: "PostToolUse", Matcher: "Bash|Read|Write|Edit|MultiEdit|NotebookEdit|Glob|Grep", Subcmd: "post-tool", Timeout: 5},
		{Event: "Stop", Subcmd: "stop", Timeout: 10},
	})
}
