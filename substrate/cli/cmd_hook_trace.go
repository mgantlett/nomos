// Package cli implements per-worktree hook execution trace persistence and
// deterministic Receipt-Trace Parity validation during atomic 2PC task sync.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
)

// hookSessionTrace summarizes verified tool and shell activity inside an active task worktree.
type hookSessionTrace struct {
	WorktreeRoot         string   `json:"worktree_root"`
	ToolCallsCount       int      `json:"tool_calls_count"`
	ShellCommandsCount   int      `json:"shell_commands_count"`
	TestCommandsCount    int      `json:"test_commands_count"`
	DeniedInterceptions  int      `json:"denied_interceptions"`
	UnverifiedCalls      int      `json:"unverified_calls,omitempty"`
	ConsecutiveReadCalls int      `json:"consecutive_read_calls,omitempty"`
	DirtyUnverifiedEdits bool     `json:"dirty_unverified_edits,omitempty"`
	WarnedChurn75        bool     `json:"warned_churn_75,omitempty"`
	WarnedChurn150       bool     `json:"warned_churn_150,omitempty"`
	WarnedReadSpiral12   bool     `json:"warned_read_spiral_12,omitempty"`
	WarnedReadSpiral20   bool     `json:"warned_read_spiral_20,omitempty"`
	MutatedFiles         []string `json:"mutated_files,omitempty"`
}

var standaloneVerifyCmdRe = regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?(?:\S+/)?nomos\s+(?:verify|task\s+review|task\s+sync)(?:\s|$)`)

// recordPreToolSessionTrace records allowed or denied PreToolUse activity scoped to an active worktree.
func recordPreToolSessionTrace(p *HookPayload, hermeticDir string, denied bool) {
	if p == nil || p.IsPostInvoke {
		return
	}
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return
	}
	path := resolveTraceStateFilePath(wtRoot)
	if path == "" {
		return
	}
	tr := loadOrInitHookSessionTrace(path, wtRoot)
	if denied {
		tr.DeniedInterceptions++
		saveHookSessionTrace(path, tr)
		return
	}
	tr.ToolCallsCount++
	if isNomosVerificationCommand(p.CommandLine) {
		resetWorktreeChurnCounters(&tr)
	}
	recordTracePayloadDetails(&tr, p, wtRoot)
	saveHookSessionTrace(path, tr)
}

// evaluateWorktreeChurnAdvisory tracks unverified PostToolUse calls inside an active task worktree
// and emits non-blocking advisories at 75 and 150 calls since the last 'nomos verify' run.
func evaluateWorktreeChurnAdvisory(p *HookPayload, hermeticDir, style string) string {
	if p == nil || p.IsPostInvoke {
		return ""
	}
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return ""
	}
	path := resolveTraceStateFilePath(wtRoot)
	if path == "" {
		return ""
	}
	tr := loadOrInitHookSessionTrace(path, wtRoot)
	if isNomosVerificationCommand(p.CommandLine) {
		if hasFailedVerificationResponse(p.ToolResponseText) {
			tr.DirtyUnverifiedEdits = true
		} else {
			resetWorktreeChurnCounters(&tr)
		}
		saveHookSessionTrace(path, tr)
		return ""
	}
	tr.UnverifiedCalls++
	var msg string
	if tr.UnverifiedCalls >= 150 && !tr.WarnedChurn150 {
		tr.WarnedChurn75 = true
		tr.WarnedChurn150 = true
		msg = formatWorktreeChurnAdvisory(tr.UnverifiedCalls, style)
	} else if tr.UnverifiedCalls >= 75 && !tr.WarnedChurn75 {
		tr.WarnedChurn75 = true
		msg = formatWorktreeChurnAdvisory(tr.UnverifiedCalls, style)
	}
	saveHookSessionTrace(path, tr)
	return msg
}

// markWorktreeVerificationFailed preserves DirtyUnverifiedEdits when 'nomos verify' fails in-process.
func markWorktreeVerificationFailed(worktreeDir string) {
	wtRoot := extractEnclosingWorktreeRoot(normalizeCanonicalPath(worktreeDir, "/"))
	if wtRoot == "" {
		return
	}
	path := resolveTraceStateFilePath(wtRoot)
	if path == "" {
		return
	}
	tr := loadOrInitHookSessionTrace(path, wtRoot)
	tr.DirtyUnverifiedEdits = true
	saveHookSessionTrace(path, tr)
}

func resetWorktreeChurnCounters(tr *hookSessionTrace) {
	tr.UnverifiedCalls = 0
	tr.DirtyUnverifiedEdits = false
	tr.WarnedChurn75 = false
	tr.WarnedChurn150 = false
}

func isNomosVerificationCommand(cmdLine string) bool {
	trimmed := strings.TrimSpace(cmdLine)
	if trimmed == "" || !standaloneVerifyCmdRe.MatchString(trimmed) || strings.ContainsAny(trimmed, ";|") {
		return false
	}
	lower := strings.ToLower(trimmed)
	return !hasAnySubstring(lower, "--impact", "--fast-file", "--help", " -h")
}

func hasFailedVerificationResponse(resp string) bool {
	lower := strings.ToLower(strings.TrimSpace(resp))
	if lower == "" {
		return false
	}
	return hasAnySubstring(lower,
		"verification failed",
		"dod verification failed",
		"exited with code 1",
		"exit status 1",
	)
}

// recordPostToolSessionTrace records PostToolUse file mutations or shell runs when PreToolUse was not invoked.
func recordPostToolSessionTrace(p *HookPayload, hermeticDir string) {
	if p == nil || p.IsPostInvoke {
		return
	}
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return
	}
	path := resolveTraceStateFilePath(wtRoot)
	if path == "" {
		return
	}
	tr := loadOrInitHookSessionTrace(path, wtRoot)
	if tr.ToolCallsCount == 0 {
		tr.ToolCallsCount = 1
	}
	recordTracePayloadDetails(&tr, p, wtRoot)
	if isNomosVerificationCommand(p.CommandLine) && hasFailedVerificationResponse(p.ToolResponseText) {
		tr.DirtyUnverifiedEdits = true
	}
	saveHookSessionTrace(path, tr)
}

func recordTracePayloadDetails(tr *hookSessionTrace, p *HookPayload, wtRoot string) {
	if cmd := strings.TrimSpace(p.CommandLine); cmd != "" {
		tr.ShellCommandsCount++
		if isRecognizedTestCommand(cmd) {
			tr.TestCommandsCount++
		}
	}
	if p.TargetFile == "" || p.CommandLine != "" || isReadOnlyTool(p.ToolName) {
		return
	}
	targets := p.TargetFiles
	if len(targets) == 0 {
		targets = []string{p.TargetFile}
	}
	for _, rawTarget := range targets {
		resolved := normalizeCanonicalPath(rawTarget, wtRoot)
		if !strings.HasPrefix(resolved, wtRoot+"/") {
			continue
		}
		rel := strings.TrimPrefix(resolved, wtRoot+"/")
		if rel == "" || strings.HasPrefix(rel, ".tmp/") {
			continue
		}
		tr.MutatedFiles = appendUniqueBounded(tr.MutatedFiles, rel, 64)
		tr.DirtyUnverifiedEdits = true
	}
}

func isRecognizedTestCommand(cmdLine string) bool {
	lower := strings.ToLower(cmdLine)
	for _, tok := range []string{
		"go test", "pytest", "py.test", "cargo test", "npm test",
		"pnpm test", "yarn test", "vitest", "jest", "bun test",
		"unittest", "ctest", "nomos verify", "nomos task review",
	} {
		if strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

func appendUniqueBounded(items []string, candidate string, limit int) []string {
	if candidate == "" || len(items) >= limit {
		return items
	}
	for _, existing := range items {
		if existing == candidate {
			return items
		}
	}
	return append(items, candidate)
}

// resolveWorktreeTraceRoot extracts the enclosing '/worktrees/<name>' root when operating in a task worktree.
func resolveWorktreeTraceRoot(p *HookPayload, hermeticDir string) string {
	candidates := []string{hermeticDir, p.TargetFile, p.Cwd}
	candidates = append(candidates, p.WorkspacePaths...)
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	for _, raw := range candidates {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		norm := normalizeCanonicalPath(raw, baseCwd)
		if wt := extractEnclosingWorktreeRoot(norm); wt != "" {
			return wt
		}
	}
	return ""
}

func extractEnclosingWorktreeRoot(slashPath string) string {
	const marker = "/worktrees/"
	idx := strings.Index(slashPath, marker)
	if idx < 0 {
		return ""
	}
	rest := slashPath[idx+len(marker):]
	if rest == "" {
		return ""
	}
	seg := rest
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		seg = rest[:slash]
	}
	if seg == "" {
		return ""
	}
	return slashPath[:idx+len(marker)+len(seg)]
}

func resolveTraceStateFilePath(wtRoot string) string {
	canon := normalizeCanonicalPath(wtRoot, "/")
	if canon == "" {
		return ""
	}
	hookDir := resolveHookStateCacheDir()
	if hookDir == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(canon))
	return filepath.Join(hookDir, fmt.Sprintf("nomos-hook-trace-%s.json", hex.EncodeToString(sum[:8])))
}

func loadOrInitHookSessionTrace(path, wtRoot string) hookSessionTrace {
	if existing := loadHookSessionTraceByFile(path); existing != nil {
		return *existing
	}
	return hookSessionTrace{WorktreeRoot: wtRoot}
}

func loadHookSessionTrace(wtPath string) *hookSessionTrace {
	if strings.TrimSpace(wtPath) == "" {
		return nil
	}
	return loadHookSessionTraceByFile(resolveTraceStateFilePath(wtPath))
}

func loadHookSessionTraceByFile(path string) *hookSessionTrace {
	if path == "" {
		return nil
	}
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tr hookSessionTrace
	if uErr := json.Unmarshal(data, &tr); uErr != nil {
		return nil
	}
	return &tr
}

func saveHookSessionTrace(path string, tr hookSessionTrace) {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return
	}
	data, err := json.Marshal(tr)
	if err != nil {
		return
	}
	if wErr := capbac.WriteStateFileAtomically(path, data); wErr != nil {
		return
	}
}

// clearHookSessionTrace removes transient worktree hook trace state after 2PC release or abort.
func clearHookSessionTrace(wtPath string) {
	if path := resolveTraceStateFilePath(wtPath); path != "" {
		removeBestEffort(path)
	}
}
