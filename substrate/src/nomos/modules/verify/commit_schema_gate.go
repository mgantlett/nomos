// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const remedyAmendCommit = "amend the commit via `git commit --amend` or use `nomos task commit`"

var prefixRegex = regexp.MustCompile(`^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]`)

// CommitSchemaGate enforces the Dual-Layer Hybrid AI schema and Canonical Singularity Law on the HEAD commit (#CommitMessage, #CommitValidationReport).
type CommitSchemaGate struct{}

// Name returns the canonical name of the Commit Schema quality gate.
func (g *CommitSchemaGate) Name() string {
	return "Commit Schema Invariant Gate"
}

// Evaluate parses the HEAD commit message to ensure format compliance.
func (g *CommitSchemaGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx != nil && ctx.IsBaseline {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "baseline trunk verification mode; historical commit schema check bypassed",
		}, nil
	}
	root := resolveBoundariesRoot(ctx)
	if isPreCommitExempt(root) {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "worktree branch has zero commits relative to trunk; exempt from HEAD commit schema gate",
		}, nil
	}

	cmd := workspace.SafeGitCommand(root, "log", "-1", "--format=%B")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no commit history to evaluate"}, nil
	}

	commitMsg := strings.TrimSpace(string(out))
	if commitMsg == "" {
		return &GateResult{GateName: g.Name(), Passed: false, Message: "HEAD commit message is empty", Remedy: remedyAmendCommit}, nil
	}

	return checkCommitCompliance(g.Name(), commitMsg)
}

func isPreCommitExempt(worktreePath string) bool {
	if worktreePath == "" {
		return false
	}
	currentBranchCmd := workspace.SafeGitCommand(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, branchErr := currentBranchCmd.CombinedOutput()
	if branchErr != nil {
		return false
	}
	branchName := strings.TrimSpace(string(branchOut))
	if workspace.IsProtectedBranch(branchName) || branchName == "" || branchName == "HEAD" {
		return false
	}
	for _, t := range workspace.ProtectedTrunkBranches {
		checkCmd := workspace.SafeGitCommand(worktreePath, "rev-parse", "--verify", t)
		if err := checkCmd.Run(); err == nil {
			revCmd := workspace.SafeGitCommand(worktreePath, "rev-list", t+"..HEAD")
			revOut, revErr := revCmd.CombinedOutput()
			if revErr == nil && strings.TrimSpace(string(revOut)) == "" {
				return true
			}
		}
	}
	return false
}

func checkCommitCompliance(gateName, commitMsg string) (*GateResult, error) {
	lines := strings.Split(commitMsg, "\n")
	firstLine := strings.TrimSpace(lines[0])

	if isMergeCommit(firstLine) {
		return &GateResult{GateName: gateName, Passed: true, Message: "merge commit is exempt from task ID prefix invariant"}, nil
	}

	if !prefixRegex.MatchString(firstLine) {
		return &GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  fmt.Sprintf("HEAD commit message missing valid [TaskID] prefix on first line: %q", firstLine),
			Remedy:   remedyAmendCommit,
		}, nil
	}

	lowerMsg := strings.ToLower(commitMsg)
	hasBlastRadius := strings.Contains(lowerMsg, "**blast radius & contracts:**")
	hasResolution := strings.Contains(lowerMsg, "**resolution mechanics:**")
	hasRationale := strings.Contains(lowerMsg, "**architectural rationale:**")

	if !hasBlastRadius && !hasResolution && !hasRationale {
		return &GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  "HEAD commit message missing dual-layer headings (**Architectural Rationale:**, **Blast Radius & Contracts:**, or **Resolution Mechanics:**)",
			Remedy:   remedyAmendCommit,
		}, nil
	}

	return &GateResult{GateName: gateName, Passed: true, Message: "HEAD commit schema is properly formatted"}, nil
}

func isMergeCommit(firstLine string) bool {
	return strings.HasPrefix(firstLine, "Merge pull request ") ||
		strings.HasPrefix(firstLine, "Merge branch ") ||
		strings.HasPrefix(firstLine, "Merge remote-tracking branch ")
}
