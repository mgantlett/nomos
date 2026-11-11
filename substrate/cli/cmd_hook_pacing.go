// Package cli implements read-only exploration spiral pacing advisories across active task worktrees and idle trunk sessions.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	readSpiralWarnThreshold = 12
	readSpiralCritThreshold = 20
	pacingTracesSubdir      = "pacing_traces"
)

var exploratoryReadToolNames = map[string]bool{
	"read": true, "read_file": true, "view_file": true,
	"view_file_outline": true, "view_code_item": true,
	"grep": true, "grep_search": true, "search_file_content": true,
	"glob": true, "find_by_name": true,
	"list_dir": true, "list_directory": true, "list": true, "ls": true, "lsp": true,
}

var readOnlyShellPrefixes = []string{
	"git grep", "grep ", "rg ", "find ", "cat ", "head ", "tail ", "sed -n", "nl ",
}

// evaluateReadSpiralAdvisory tracks consecutive read/search calls without a source edit
// and emits non-blocking pacing nudges at configured thresholds.
func evaluateReadSpiralAdvisory(p *HookPayload, hermeticDir, style string) string {
	prof := resolveHookAdvisoryProfile(0, 0, 0, style)
	return evaluateReadSpiralAdvisoryWithProfile(p, hermeticDir, prof)
}

// evaluateReadSpiralAdvisoryWithProfile evaluates consecutive exploratory read spirals
// across active worktrees and session-keyed trunk traces (@intent mneme_memory_spec.md).
func evaluateReadSpiralAdvisoryWithProfile(p *HookPayload, hermeticDir string, profile HookAdvisoryProfile) string {
	if p == nil || p.IsPostInvoke {
		return ""
	}
	path, traceRoot, isTrunk := resolvePacingTraceTarget(p, hermeticDir)
	if path == "" {
		return ""
	}
	tr := loadOrInitHookSessionTrace(path, traceRoot)
	if isPacingSourceMutation(p) || (isTrunk && isTrunkTaskTransition(p)) {
		resetReadSpiralCounters(&tr)
		saveHookSessionTrace(path, tr)
		return ""
	}
	if !isExploratoryReadCall(p) {
		return ""
	}
	warnAt, critAt := resolveReadSpiralBounds(profile)
	tr.ConsecutiveReadCalls++
	msg := advanceReadSpiralWarning(&tr, warnAt, critAt, isTrunk, profile.AdvisoryStyle)
	saveHookSessionTrace(path, tr)
	return msg
}

func resolvePacingTraceTarget(p *HookPayload, hermeticDir string) (string, string, bool) {
	if wtRoot := resolveWorktreeTraceRoot(p, hermeticDir); wtRoot != "" {
		return resolveTraceStateFilePath(wtRoot), wtRoot, false
	}
	path, trunkRoot := resolveTrunkSessionTracePath(p, hermeticDir)
	return path, trunkRoot, path != ""
}

func resolveReadSpiralBounds(profile HookAdvisoryProfile) (int, int) {
	warnAt := profile.ReadSpiralWarn
	if warnAt <= 0 {
		warnAt = readSpiralWarnThreshold
	}
	critAt := profile.ReadSpiralCrit
	if critAt <= warnAt {
		critAt = readSpiralCritThreshold
	}
	return warnAt, critAt
}

func advanceReadSpiralWarning(tr *hookSessionTrace, warnAt, critAt int, isTrunk bool, style string) string {
	if tr.ConsecutiveReadCalls >= critAt && !tr.WarnedReadSpiral20 {
		tr.WarnedReadSpiral12 = true
		tr.WarnedReadSpiral20 = true
		if isTrunk {
			return formatTrunkReadSpiralAdvisory(tr.ConsecutiveReadCalls, true, style)
		}
		return formatReadSpiralAdvisory(tr.ConsecutiveReadCalls, true, style)
	}
	if tr.ConsecutiveReadCalls >= warnAt && !tr.WarnedReadSpiral12 {
		tr.WarnedReadSpiral12 = true
		if isTrunk {
			return formatTrunkReadSpiralAdvisory(tr.ConsecutiveReadCalls, false, style)
		}
		return formatReadSpiralAdvisory(tr.ConsecutiveReadCalls, false, style)
	}
	return ""
}

// resolveTrunkSessionTracePath resolves a session-scoped pacing trace file under NOMOS_HOME/cache/pacing_traces
// when operating on an idle Nomos-governed trunk with a non-empty session identifier.
func resolveTrunkSessionTracePath(p *HookPayload, hermeticDir string) (string, string) {
	if p == nil || hermeticDir != "" {
		return "", ""
	}
	sessID := strings.TrimSpace(p.SessionID)
	if sessID == "" {
		return "", ""
	}
	trunkRoot := resolveGovernedTrunkRoot(p)
	if trunkRoot == "" {
		return "", ""
	}
	tracesDir := filepath.Join(config.GetNomosHome(), constants.DirCache, pacingTracesSubdir)
	if mkErr := os.MkdirAll(tracesDir, 0700); mkErr != nil {
		return "", ""
	}
	fi, stErr := os.Lstat(tracesDir)
	if stErr != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return "", ""
	}
	sum := sha256.Sum256([]byte(trunkRoot + "|" + sessID))
	fileName := fmt.Sprintf("trunk-session-%s.json", hex.EncodeToString(sum[:8]))
	return filepath.Join(tracesDir, fileName), trunkRoot
}

func resolveGovernedTrunkRoot(p *HookPayload) string {
	baseCwd := resolveHookWorkspaceDir(p, "")
	if strings.TrimSpace(baseCwd) == "" {
		return ""
	}
	canonBase := normalizeCanonicalPath(baseCwd, "/")
	repoRoot := canonBase
	if pr, err := workspace.FindRepoRoot(canonBase); err == nil && pr != "" {
		repoRoot = filepath.ToSlash(filepath.Clean(pr))
	}
	if !hasNomosWorkspaceMarker(filepath.FromSlash(repoRoot)) {
		return ""
	}
	return repoRoot
}

func hasNomosWorkspaceMarker(dir string) bool {
	cand := filepath.Join(dir, constants.FileNomosYAML)
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return true
	}
	localHome := filepath.Join(dir, constants.DirNomosHome)
	if fi, err := os.Stat(localHome); err == nil && fi.IsDir() {
		return true
	}
	return false
}

func isTrunkTaskTransition(p *HookPayload) bool {
	if strings.TrimSpace(p.CommandLine) == "" {
		return false
	}
	lower := strings.ToLower(p.CommandLine)
	return hasAnySubstring(lower, "nomos task chore", "nomos task start", "nomos task pair", "nomos task create")
}

func resetReadSpiralCounters(tr *hookSessionTrace) {
	tr.ConsecutiveReadCalls = 0
	tr.WarnedReadSpiral12 = false
	tr.WarnedReadSpiral20 = false
}

func isPacingSourceMutation(p *HookPayload) bool {
	if p.CommandLine == "" {
		return p.TargetFile != "" && !isReadOnlyTool(p.ToolName)
	}
	lower := strings.ToLower(strings.TrimSpace(p.CommandLine))
	return hasAnySubstring(lower, "sed -i", "git apply", "patch ")
}

func isExploratoryReadCall(p *HookPayload) bool {
	if p.CommandLine == "" {
		name := strings.ToLower(strings.TrimSpace(p.ToolName))
		return exploratoryReadToolNames[name]
	}
	if isRecognizedTestCommand(p.CommandLine) {
		return false
	}
	return isReadOnlyInspectionShellCmd(p.CommandLine)
}

func isReadOnlyInspectionShellCmd(cmdLine string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(cmdLine))
	if trimmed == "" || strings.Contains(trimmed, ">") {
		return false
	}
	for _, prefix := range readOnlyShellPrefixes {
		if trimmed == strings.TrimSpace(prefix) || strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func formatReadSpiralAdvisory(readCalls int, urgent bool, style string) string {
	if style == advisoryStyleTerse {
		return fmt.Sprintf("🧭 [Nomos Pacing] %d read calls without edit: apply a minimal fix and run tests now.", readCalls)
	}
	if urgent {
		return fmt.Sprintf(
			"🧭 [Nomos Pacing Advisory] %d consecutive read/search calls without a source edit: stop passive reading, apply your best minimal fix hypothesis now, and run tests to verify.",
			readCalls,
		)
	}
	return fmt.Sprintf(
		"🧭 [Nomos Pacing Advisory] %d consecutive read/search calls without a source edit: formulate a minimal draft edit now and validate iteratively with tests.",
		readCalls,
	)
}

func formatTrunkReadSpiralAdvisory(readCalls int, urgent bool, style string) string {
	if style == advisoryStyleTerse {
		return fmt.Sprintf("🧭 [Nomos Trunk Pacing] %d trunk read calls: synthesize findings or start an isolated worktree ('nomos task chore \"<title>\"').", readCalls)
	}
	if urgent {
		return fmt.Sprintf(
			"🧭 [Nomos Trunk Pacing Advisory] %d consecutive exploratory reads on trunk: stop passive exploration and either synthesize your findings now or provision an isolated worktree via 'nomos task chore \"<title>\"' (or 'nomos task start <KEY> --edit').",
			readCalls,
		)
	}
	return fmt.Sprintf(
		"🧭 [Nomos Trunk Pacing Advisory] %d consecutive exploratory reads on trunk: converge on a concrete answer or provision an isolated worktree via 'nomos task chore \"<title>\"' before reading further.",
		readCalls,
	)
}
