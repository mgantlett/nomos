// Package cli implements Antigravity 2.0 hook envelope normalization and PostInvocation advisory delivery.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" && trimmed != "null" && !seen[trimmed] {
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

func collectNestedPathSlices(m map[string]any) []string {
	sliceKeys := []string{"paths", "Paths", "file_paths", "filePaths", "files", "TargetFiles"}
	var all []string
	for _, rootKey := range []string{hookFieldToolInput, hookFieldArgs} {
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

func resolveAntigravityPendingPath(p *HookPayload, hermeticDir string) string {
	if p == nil {
		return ""
	}
	seed := strings.TrimSpace(p.SessionID)
	if seed == "" {
		seed = filepath.Clean(resolveHookWorkspaceDir(p, hermeticDir))
	}
	sum := sha256.Sum256([]byte(seed))
	hookDir := resolveHookStateCacheDir()
	if hookDir == "" {
		return ""
	}
	return filepath.Join(hookDir, fmt.Sprintf("nomos-hook-agy-adv-%s.txt", hex.EncodeToString(sum[:8])))
}

// queueAntigravityAdvisory buffers non-empty PostToolUse advisories for injection at PostInvocation.
func queueAntigravityAdvisory(p *HookPayload, hermeticDir, msg string) {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return
	}
	path := resolveAntigravityPendingPath(p, hermeticDir)
	if path == "" {
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
	_ = os.WriteFile(path, []byte(combined), 0600)
}

// popAntigravityAdvisory drains buffered PostToolUse advisories once PostInvocation fires.
func popAntigravityAdvisory(p *HookPayload, hermeticDir string) string {
	path := resolveAntigravityPendingPath(p, hermeticDir)
	if path == "" {
		return ""
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	data, rErr := os.ReadFile(path)
	_ = os.Remove(path)
	if rErr != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
