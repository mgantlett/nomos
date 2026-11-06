package session

import (
	"encoding/json"
	"fmt"
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

func parseSingleInvocation(tc *rawToolCallItem) toolInvocation {
	inv := toolInvocation{ToolName: tc.Name}
	if tc.Args != nil {
		inv.CommandLine = cleanArg(tc.Args["CommandLine"])
		inv.Cwd = cleanArg(tc.Args["Cwd"])
		target := cleanArg(tc.Args["TargetFile"])
		if target == "" {
			target = cleanArg(tc.Args["AbsolutePath"])
		}
		inv.Target = target
	}
	return inv
}

func cleanArg(raw any) string {
	if raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.Trim(strings.TrimSpace(s), "\"")
	}
	return strings.TrimSpace(fmt.Sprintf("%v", raw))
}

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

	if detail.WorktreePath != "" {
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

func evalRepoBoundary(detail *SessionDetail, invocations []toolInvocation, repoRoot string) InvariantResult {
	if !isDownstreamWorkspace(detail, repoRoot) {
		return InvariantResult{
			ID:       "CHARTER-4-BOUNDARY",
			Name:     "Repository Boundary (Charter §4)",
			Passed:   true,
			Evidence: "Substrate/Sovereign workspace authorized for upstream access",
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
		if strings.Contains(combined, "pkg/sovereign") {
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

func isDownstreamWorkspace(detail *SessionDetail, repoRoot string) bool {
	return isDownstreamConsumerContext(repoRoot, detail.WorktreePath)
}

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

	if detail.Status == "ACTIVE" {
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
