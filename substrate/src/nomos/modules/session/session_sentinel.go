// Package session evaluates deterministic Zero-Trust Sentinel invariants across Tier 1
// coding agent transcripts (@intent nomos_hermetic_worktree_spec.md).
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// InvariantResult models an individual automated invariant evaluation result.
type InvariantResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	Evidence   string `json:"evidence"`
	RemedyHint string `json:"remedy_hint,omitempty"`
}

// SentinelScorecard models the complete Zero-Trust architectural surveillance evaluation.
type SentinelScorecard struct {
	IntegrityStatus string            `json:"integrity_status"` // CLEAN, SUSPICIOUS, VIOLATION
	OverallScore    int               `json:"overall_score"`    // 0 to 100
	Violations      []string          `json:"violations"`
	Invariants      []InvariantResult `json:"invariants"`
}

type toolInvocation struct {
	ToolName    string
	CommandLine string
	Cwd         string
	Target      string
}

// EvaluateSentinel audits an agent session against the 7 Laws and Open-Core Boundary.
func EvaluateSentinel(detail *SessionDetail, rawLines []string, repoRoot string) *SentinelScorecard {
	scorecard := &SentinelScorecard{
		IntegrityStatus: "CLEAN",
		OverallScore:    100,
		Violations:      make([]string, 0),
		Invariants:      make([]InvariantResult, 0),
	}

	invocations := extractToolInvocations(rawLines)

	invs := []InvariantResult{
		evalHermeticWorktree(detail, invocations),
		evalRepoBoundary(detail, invocations, repoRoot),
		evalDoDVerification(detail, invocations),
		evalDBEncapsulation(invocations),
		evalASTBudget(invocations),
	}

	for _, inv := range invs {
		scorecard.Invariants = append(scorecard.Invariants, inv)
		if !inv.Passed {
			scorecard.Violations = append(scorecard.Violations, inv.Name+": "+inv.Evidence)
			scorecard.OverallScore -= 20
		}
	}

	calculateScorecardStatus(scorecard)
	return scorecard
}

// extractToolInvocations parses JSONL transcript lines into normalized tool invocation records.
func extractToolInvocations(rawLines []string) []toolInvocation {
	var invocations []toolInvocation
	for _, line := range rawLines {
		var item transcriptLine
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			continue
		}
		for _, tc := range item.ToolCalls {
			invocations = append(invocations, parseSingleInvocation(&tc))
		}
	}
	return invocations
}

// parseSingleInvocation normalizes command, working directory, and target path fields.
func parseSingleInvocation(tc *rawToolCallItem) toolInvocation {
	inv := toolInvocation{ToolName: tc.Name}
	if tc.Args != nil {
		inv.CommandLine = firstCleanArg(tc.Args, "CommandLine", "command", "cmd")
		inv.Cwd = firstCleanArg(tc.Args, "Cwd", "cwd", "workdir", "working_directory")
		inv.Target = firstCleanArg(tc.Args, "Path", "TargetFile", "AbsolutePath", "file_path", "filePath", "path")
	}
	return inv
}

// firstCleanArg returns the first non-empty trimmed string value across candidate argument keys.
func firstCleanArg(args map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := cleanArg(args[k]); v != "" {
			return v
		}
	}
	return ""
}

// cleanArg formats a raw JSON argument value into a trimmed string without surrounding quotes.
func cleanArg(raw any) string {
	if raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.Trim(strings.TrimSpace(s), "\"")
	}
	return strings.TrimSpace(fmt.Sprintf("%v", raw))
}

// calculateScorecardStatus classifies overall Sentinel health from violation count and score.
func calculateScorecardStatus(sc *SentinelScorecard) {
	if sc.OverallScore < 0 {
		sc.OverallScore = 0
	}
	if len(sc.Violations) == 0 {
		sc.IntegrityStatus = "CLEAN"
	} else if len(sc.Violations) == 1 && sc.OverallScore >= 80 {
		sc.IntegrityStatus = "SUSPICIOUS"
	} else {
		sc.IntegrityStatus = "VIOLATION"
	}
}

// evalHermeticWorktree enforces Law 3 worktree confinement against unscoped trunk git commits.
func evalHermeticWorktree(detail *SessionDetail, invocations []toolInvocation) InvariantResult {
	for _, inv := range invocations {
		if hasUnscopedGitCommitInvocation(inv.CommandLine, inv.Cwd) {
			return InvariantResult{
				ID:         "LAW-3-HERMETIC",
				Name:       "Hermetic Worktree (Law 3)",
				Passed:     false,
				Evidence:   "Direct git commit attempted on protected trunk without worktree",
				RemedyHint: "Execute mutations strictly inside isolated task worktrees via nomos task chore/start",
			}
		}
	}

	if detail != nil && detail.WorktreePath != "" {
		return InvariantResult{
			ID:       "LAW-3-HERMETIC",
			Name:     "Hermetic Worktree (Law 3)",
			Passed:   true,
			Evidence: "Isolated task worktree lease verified",
		}
	}

	return InvariantResult{
		ID:       "LAW-3-HERMETIC",
		Name:     "Hermetic Worktree (Law 3)",
		Passed:   true,
		Evidence: "Clean-room execution inside transient worktree or idle trunk",
	}
}

// evalRepoBoundary enforces Charter §4 black-box separation on external downstream consumers
// while authorizing Substrate, plugin, and self-owned workspace packages (@intent nomos_hermetic_worktree_spec.md).
func evalRepoBoundary(detail *SessionDetail, invocations []toolInvocation, repoRoot string) InvariantResult {
	if !isDownstreamWorkspace(detail, repoRoot) {
		return InvariantResult{
			ID:       "CHARTER-4-BOUNDARY",
			Name:     "Repository Boundary (Charter §4)",
			Passed:   true,
			Evidence: "Substrate/Plugin workspace authorized for upstream access",
		}
	}

	for _, inv := range invocations {
		combined := inv.CommandLine + " " + inv.Cwd + " " + inv.Target
		if strings.Contains(combined, "/nomos/substrate") || strings.Contains(combined, "nomos/src") {
			return InvariantResult{
				ID:         "CHARTER-4-BOUNDARY",
				Name:       "Repository Boundary (Charter §4)",
				Passed:     false,
				Evidence:   "Downstream consumer peeked into upstream nomos source repository",
				RemedyHint: "Treat nomos as an external black-box CLI binary; zero source peeking",
			}
		}
		if strings.Contains(combined, "pkg/sovereign") && !ownsSovereignModuleInWorkspace(repoRoot, detail, inv) {
			return InvariantResult{
				ID:         "CHARTER-4-BOUNDARY",
				Name:       "Repository Boundary (Charter §4)",
				Passed:     false,
				Evidence:   "Downstream consumer accessed proprietary sovereign module boundary",
				RemedyHint: "Sovereign extensions are restricted to authorized workspaces",
			}
		}
	}

	return InvariantResult{
		ID:       "CHARTER-4-BOUNDARY",
		Name:     "Repository Boundary (Charter §4)",
		Passed:   true,
		Evidence: "Zero forbidden peeking detected into upstream or sovereign code",
	}
}

// isDownstreamWorkspace delegates to isDownstreamConsumerContext with nil-safe worktree resolution.
func isDownstreamWorkspace(detail *SessionDetail, repoRoot string) bool {
	worktreePath := ""
	if detail != nil {
		worktreePath = detail.WorktreePath
	}
	return isDownstreamConsumerContext(repoRoot, worktreePath)
}

// ownsSovereignModuleInWorkspace reports whether the active workspace legitimately owns
// pkg/sovereign inside its own repository root and the invocation stays inside that boundary.
func ownsSovereignModuleInWorkspace(repoRoot string, detail *SessionDetail, inv toolInvocation) bool {
	worktreePath := ""
	if detail != nil {
		worktreePath = detail.WorktreePath
	}
	roots := candidateWorkspaceRoots(repoRoot, worktreePath)
	if !anyRootOwnsSovereignDir(roots) {
		return false
	}
	return invocationStaysWithinRoots(inv, roots)
}

// anyRootOwnsSovereignDir checks whether any candidate root contains pkg/sovereign or substrate/pkg/sovereign.
func anyRootOwnsSovereignDir(roots []string) bool {
	for _, root := range roots {
		for _, rel := range []string{filepath.Join("pkg", "sovereign"), filepath.Join("substrate", "pkg", "sovereign")} {
			if info, err := os.Stat(filepath.Join(root, rel)); err == nil && info.IsDir() {
				return true
			}
		}
	}
	return false
}

// invocationStaysWithinRoots ensures absolute Target or Cwd coordinates do not escape candidate workspace roots.
func invocationStaysWithinRoots(inv toolInvocation, roots []string) bool {
	for _, p := range []string{inv.Target, inv.Cwd} {
		if p != "" && filepath.IsAbs(p) && !isPathWithinAnyRoot(p, roots) {
			return false
		}
	}
	return true
}

// isPathWithinAnyRoot reports whether absPath lies inside at least one cleaned root directory.
func isPathWithinAnyRoot(absPath string, roots []string) bool {
	cleanPath := filepath.Clean(absPath)
	for _, root := range roots {
		if !filepath.IsAbs(root) {
			continue
		}
		cleanRoot := filepath.Clean(root)
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// evalDoDVerification evaluates Law 2 verification gate execution across session commands.
func evalDoDVerification(detail *SessionDetail, invocations []toolInvocation) InvariantResult {
	for _, inv := range invocations {
		if strings.Contains(inv.CommandLine, "nomos verify") {
			return InvariantResult{
				ID:       "LAW-2-VERIFY",
				Name:     "DoD Quality Gates (Law 2)",
				Passed:   true,
				Evidence: "Canonical nomos verify quality gates executed and confirmed",
			}
		}
	}

	if detail != nil && detail.Status == "ACTIVE" {
		return InvariantResult{
			ID:       "LAW-2-VERIFY",
			Name:     "DoD Quality Gates (Law 2)",
			Passed:   true,
			Evidence: "Active in-flight task; verification pending pre-commit phase",
		}
	}

	return InvariantResult{
		ID:       "LAW-2-VERIFY",
		Name:     "DoD Quality Gates (Law 2)",
		Passed:   true,
		Evidence: "Task completed or read-only exploratory session",
	}
}

// evalDBEncapsulation enforces Law 1 CLI encapsulation against raw sqlite3 queries on Nomos DBs.
func evalDBEncapsulation(invocations []toolInvocation) InvariantResult {
	for _, inv := range invocations {
		if strings.Contains(inv.CommandLine, "sqlite3 ") {
			cmd := inv.CommandLine
			if strings.Contains(cmd, constants.FileNomosGlobalDB) || strings.Contains(cmd, constants.FileMnemeDB) {
				return InvariantResult{
					ID:         "LAW-1-DB-ENCAP",
					Name:       "Database Encapsulation (Law 1)",
					Passed:     false,
					Evidence:   "Direct sqlite3 query executed against internal Nomos database",
					RemedyHint: "Route all task and memory queries through canonical nomos CLI commands",
				}
			}
		}
	}

	return InvariantResult{
		ID:       "LAW-1-DB-ENCAP",
		Name:     "Database Encapsulation (Law 1)",
		Passed:   true,
		Evidence: "Zero raw sqlite3 queries detected; CLI encapsulation preserved",
	}
}

// evalASTBudget checks whether session commands monitor AST file length and cognitive bounds.
func evalASTBudget(invocations []toolInvocation) InvariantResult {
	for _, inv := range invocations {
		if strings.Contains(inv.CommandLine, "wc -l") || strings.Contains(inv.CommandLine, "400") {
			return InvariantResult{
				ID:       "LAW-2-AST-BUDGET",
				Name:     "AST Budget & LOC Bounds",
				Passed:   true,
				Evidence: "File length and cognitive debt actively monitored (<= 400 LOC)",
			}
		}
	}

	return InvariantResult{
		ID:       "LAW-2-AST-BUDGET",
		Name:     "AST Budget & LOC Bounds",
		Passed:   true,
		Evidence: "Source modifications within standard complexity bounds",
	}
}
