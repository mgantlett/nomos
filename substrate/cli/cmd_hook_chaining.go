// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

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
	return marshalManifestNoHTML(existingMap)
}

func formatManifestJSON(raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &m); err != nil {
		return nil, err
	}
	return marshalManifestNoHTML(m)
}

func marshalManifestNoHTML(v any) ([]byte, error) {
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

const hookEntryTypeCommand = "command"

type agentHookEventSpec struct {
	event   string
	matcher string
	subcmd  string
	timeout int
}

func posixHookShellCommand(subcmd string) string {
	return fmt.Sprintf(
		`export PATH="$PATH:${GOBIN:+$GOBIN:}$HOME/go/bin:$HOME/.local/bin:$HOME/bin"; if command -v nomos >/dev/null 2>&1; then exec nomos hook %s; fi; printf '{}'`,
		subcmd,
	)
}

func buildAgentHookSettingsJSON(specs []agentHookEventSpec) []byte {
	hooksMap := make(map[string]any, len(specs))
	for _, sp := range specs {
		inner := map[string]any{
			"type":          hookEntryTypeCommand,
			hookEntryTypeCommand: posixHookShellCommand(sp.subcmd),
			"timeout":       sp.timeout,
		}
		entry := map[string]any{"hooks": []any{inner}}
		if sp.matcher != "" {
			entry["matcher"] = sp.matcher
		}
		hooksMap[sp.event] = []any{entry}
	}
	out, err := marshalManifestNoHTML(map[string]any{"hooks": hooksMap})
	if err != nil {
		return nil
	}
	return out
}

// DefaultClaudeSettingsHooksJSON returns the canonical Claude Code settings.json hook structure.
func DefaultClaudeSettingsHooksJSON() []byte {
	return buildAgentHookSettingsJSON([]agentHookEventSpec{
		{event: "SessionStart", subcmd: "session-start", timeout: 10},
		{event: "PreToolUse", matcher: "Bash|Read|Write|Edit|MultiEdit|NotebookEdit|Glob|Grep|WebFetch|WebSearch|Task.*|mcp__.*", subcmd: "pre-tool", timeout: 10},
		{event: "PostToolUse", matcher: "Write|Edit|MultiEdit|NotebookEdit", subcmd: "post-tool", timeout: 5},
		{event: "Stop", subcmd: "stop", timeout: 10},
	})
}
