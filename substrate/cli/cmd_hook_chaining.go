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
	out, err := json.MarshalIndent(existingMap, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func formatManifestJSON(raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &m); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
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

// DefaultClaudeSettingsHooksJSON returns the canonical Claude Code settings.json hook structure.
func DefaultClaudeSettingsHooksJSON() []byte {
	return []byte(`{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "nomos hook session-start",
            "timeout": 10
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash|Read|Write|Edit|MultiEdit|NotebookEdit|Glob|Grep|WebFetch|WebSearch|Task.*|mcp__.*",
        "hooks": [
          {
            "type": "command",
            "command": "nomos hook pre-tool",
            "timeout": 10
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit|NotebookEdit",
        "hooks": [
          {
            "type": "command",
            "command": "nomos hook post-tool",
            "timeout": 5
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "nomos hook stop",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`)
}
