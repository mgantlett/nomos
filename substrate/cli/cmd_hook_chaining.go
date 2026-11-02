// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const loopBreakerReason = "Loop Circuit-Breaker (Law 1): Identical tool call repeated 3 consecutive times without workspace state change. Alter parameters or inspect workspace state before retrying."

type loopBreakerState struct {
	Fingerprint      string `json:"fingerprint"`
	WorkspaceDigest  string `json:"workspace_digest"`
	ConsecutiveCount int    `json:"consecutive_count"`
}

// MergeHookManifest additively merges Nomos hook definitions into an existing
// .agents/hooks.json or .claude/settings.json manifest without clobbering existing hooks.
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
        "matcher": "Bash|Read|Write|Edit|MultiEdit|NotebookEdit|Glob|Grep|Task.*|mcp__.*",
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
    ]
  }
}
`)
}

// EvaluateLoopCircuitBreaker enforces a 3-strike identical-failure circuit-breaker
// when the same mutating or shell tool call repeats without workspace mtime progress.
func EvaluateLoopCircuitBreaker(p *HookPayload, hermeticDir string) HookDecision {
	if p == nil || !isLoopTrackedInvocation(p) {
		return HookDecision{Decision: "allow"}
	}
	wsDir := resolveLoopTrackingDir(p, hermeticDir)
	fp := computeInvocationFingerprint(p)
	wsDigest := computeWorkspaceMtimeDigest(wsDir)
	statePath := resolveLoopStateFilePath(wsDir)

	prev := loadLoopBreakerState(statePath)
	next := loopBreakerState{
		Fingerprint:      fp,
		WorkspaceDigest:  wsDigest,
		ConsecutiveCount: 1,
	}
	if prev.Fingerprint == fp && prev.WorkspaceDigest == wsDigest {
		next.ConsecutiveCount = prev.ConsecutiveCount + 1
	}
	saveLoopBreakerState(statePath, next)

	if next.ConsecutiveCount >= maxLoopStrikes(p) {
		return HookDecision{
			Decision: "deny",
			Reason:   loopBreakerReason,
		}
	}
	return HookDecision{Decision: "allow"}
}

func isLoopTrackedInvocation(p *HookPayload) bool {
	if strings.TrimSpace(p.CommandLine) != "" {
		return true
	}
	if strings.TrimSpace(p.TargetFile) != "" {
		return true
	}
	return strings.TrimSpace(p.ArgsDigest) != ""
}

func maxLoopStrikes(p *HookPayload) int {
	cmd := strings.ToLower(strings.TrimSpace(p.CommandLine))
	if cmd != "" {
		for _, tok := range []string{"status", "poll", "wait", "ps ", "jobs"} {
			if strings.Contains(cmd, tok) {
				return 12
			}
		}
	}
	return 3
}

func resolveLoopTrackingDir(p *HookPayload, hermeticDir string) string {
	if hermeticDir != "" {
		return hermeticDir
	}
	if p.TargetFile != "" && filepath.IsAbs(p.TargetFile) {
		return filepath.Dir(p.TargetFile)
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

func computeInvocationFingerprint(p *HookPayload) string {
	raw := strings.ToLower(strings.TrimSpace(p.ToolName)) + "|" +
		strings.TrimSpace(p.TargetFile) + "|" +
		strings.TrimSpace(p.CommandLine) + "|" +
		strings.TrimSpace(p.ArgsDigest)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:16])
}

func resolveLoopStateFilePath(wsDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(wsDir)))
	name := fmt.Sprintf("nomos-hook-loop-%s.json", hex.EncodeToString(sum[:8]))
	baseDir, err := os.UserCacheDir()
	if err != nil || baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), fmt.Sprintf("nomos-cache-%d", os.Getuid()))
	}
	hookDir := filepath.Join(baseDir, "nomos", "hooks")
	if mkErr := os.MkdirAll(hookDir, 0700); mkErr != nil {
		return ""
	}
	fi, statErr := os.Lstat(hookDir)
	if statErr != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	return filepath.Join(hookDir, name)
}

func loadLoopBreakerState(path string) loopBreakerState {
	var st loopBreakerState
	if path == "" {
		return st
	}
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if uErr := json.Unmarshal(data, &st); uErr != nil {
		return loopBreakerState{}
	}
	return st
}

func saveLoopBreakerState(path string, st loopBreakerState) {
	if path == "" {
		return
	}
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	if wErr := os.WriteFile(path, data, 0600); wErr != nil {
		return
	}
}

func computeWorkspaceMtimeDigest(rootDir string) string {
	cleanRoot := filepath.Clean(rootDir)
	var records []string
	maxFiles := 256
	walkErr := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if len(records) >= maxFiles {
			return fs.SkipAll
		}
		if info, iErr := d.Info(); iErr == nil {
			rel, rErr := filepath.Rel(cleanRoot, path)
			if rErr != nil {
				rel = path
			}
			records = append(records, fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano()))
		}
		return nil
	})
	if walkErr != nil {
		return "walk-err"
	}
	sort.Strings(records)
	sum := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(sum[:16])
}
