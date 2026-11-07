// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
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
	Fingerprint       string `json:"fingerprint"`
	WorkspaceDigest   string `json:"workspace_digest"`
	ConsecutiveCount  int    `json:"consecutive_count"`
	RunnerFamily      string `json:"runner_family,omitempty"`
	NoEditRunnerCount int    `json:"no_edit_runner_count,omitempty"`
}

// EvaluateLoopCircuitBreaker enforces the identical-failure and no-edit runner circuit-breaker
// when mutating or shell tool calls repeat without workspace mtime progress.
func EvaluateLoopCircuitBreaker(p *HookPayload, hermeticDir string) HookDecision {
	return EvaluateLoopCircuitBreakerWithStrikes(p, hermeticDir, resolveHookAdvisoryProfile(0, 0, 0, "").LoopStrikes)
}

// EvaluateLoopCircuitBreakerWithStrikes enforces a bounded N-strike identical-failure
// and flag-permuted no-edit verification runner circuit-breaker.
func EvaluateLoopCircuitBreakerWithStrikes(p *HookPayload, hermeticDir string, strikes int) HookDecision {
	if p == nil || !isLoopTrackedInvocation(p) {
		return HookDecision{Decision: "allow"}
	}
	wsDir := resolveLoopTrackingDir(p, hermeticDir)
	fp := computeInvocationFingerprint(p)
	wsDigest := computeWorkspaceMtimeDigest(wsDir)
	runner := detectVerificationRunnerFamily(p.CommandLine)
	statePath := resolveLoopStateFilePath(wsDir)

	prev := loadLoopBreakerState(statePath)
	next := advanceLoopBreakerState(prev, fp, wsDigest, runner)
	saveLoopBreakerState(statePath, next)

	threshold := maxLoopStrikesWithThreshold(p, strikes)
	if next.ConsecutiveCount >= threshold {
		return HookDecision{
			Decision: "deny",
			Reason:   formatLoopBreakerReason(threshold),
		}
	}
	if runner != "" && next.NoEditRunnerCount >= threshold {
		return HookDecision{
			Decision: "deny",
			Reason:   formatNoEditRunnerBreakerReason(runner, threshold),
		}
	}
	return HookDecision{Decision: "allow"}
}

func advanceLoopBreakerState(prev loopBreakerState, fp, wsDigest, runner string) loopBreakerState {
	next := loopBreakerState{
		Fingerprint:      fp,
		WorkspaceDigest:  wsDigest,
		ConsecutiveCount: 1,
		RunnerFamily:     runner,
	}
	if runner != "" {
		next.NoEditRunnerCount = 1
	}
	if prev.WorkspaceDigest != wsDigest {
		return next
	}
	if prev.Fingerprint == fp {
		next.ConsecutiveCount = prev.ConsecutiveCount + 1
	}
	if runner != "" && prev.RunnerFamily == runner {
		next.NoEditRunnerCount = prev.NoEditRunnerCount + 1
	}
	return next
}

func detectVerificationRunnerFamily(cmdLine string) string {
	lower := strings.ToLower(strings.TrimSpace(cmdLine))
	if lower == "" {
		return ""
	}
	switch {
	case strings.Contains(lower, "pytest") || strings.Contains(lower, "py.test"):
		return "pytest"
	case strings.Contains(lower, "unittest"):
		return "unittest"
	case strings.Contains(lower, "go test"):
		return "go-test"
	case strings.Contains(lower, "cargo test"):
		return "cargo-test"
	case strings.Contains(lower, "vitest") || strings.Contains(lower, "jest") ||
		strings.Contains(lower, "npm test") || strings.Contains(lower, "pnpm test") ||
		strings.Contains(lower, "yarn test") || strings.Contains(lower, "bun test"):
		return "js-test"
	case strings.Contains(lower, "ctest"):
		return "ctest"
	default:
		return ""
	}
}

func formatLoopBreakerReason(threshold int) string {
	if threshold == defaultLoopStrikes {
		return loopBreakerReason
	}
	return fmt.Sprintf(
		"Loop Circuit-Breaker (Law 1): Identical tool call repeated %d consecutive times without workspace state change. Alter parameters or inspect workspace state before retrying.",
		threshold,
	)
}

func formatNoEditRunnerBreakerReason(runner string, threshold int) string {
	return fmt.Sprintf(
		"Loop Circuit-Breaker (Law 1): Verification runner '%s' executed %d consecutive times without any workspace file modification. Edit source or inspect failure root cause before re-running.",
		runner, threshold,
	)
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

func maxLoopStrikesWithThreshold(p *HookPayload, strikes int) int {
	cmd := strings.ToLower(strings.TrimSpace(p.CommandLine))
	if cmd != "" {
		for _, tok := range []string{"status", "poll", "wait", "ps ", "jobs"} {
			if strings.Contains(cmd, tok) {
				return 12
			}
		}
	}
	if strikes <= 0 {
		return defaultLoopStrikes
	}
	return clampAdvisoryInt(strikes, minLoopStrikes, maxLoopStrikesBound)
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
	hookDir := resolveHookStateCacheDir()
	if hookDir == "" {
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
