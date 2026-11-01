// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// StagedBoundariesGate verifies that no uncommitted/staged files violate worktree isolation (Law 3).
type StagedBoundariesGate struct{}

// Name returns the canonical name of the staged boundaries quality gate.
func (g *StagedBoundariesGate) Name() string {
	return "Worktree Boundaries Invariant (Law 3)"
}

// Evaluate runs ValidateUncommittedBoundaries against currently uncommitted files.
func (g *StagedBoundariesGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveBoundariesRoot(ctx)
	stagedFiles, err := getStagedFiles(root)
	if err != nil {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("failed to list uncommitted files: %v", err),
		}, nil
	}

	if len(stagedFiles) == 0 {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "no uncommitted files to check",
		}, nil
	}

	repoRoot := ctx.RepoRoot
	if repoRoot == "" {
		repoRoot = root
	}
	if valErr := validateStagedFilesAgainstBranch(repoRoot, root, stagedFiles); valErr != nil {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  valErr.Error(),
			Remedy:   "move scratch files and walkthroughs to .tmp/, and ensure mutations remain in task worktrees",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  "no staged boundary violations detected",
	}, nil
}

func resolveBoundariesRoot(ctx *VerifyContext) string {
	if ctx != nil && ctx.WorktreePath != "" {
		return ctx.WorktreePath
	}
	if ctx != nil && ctx.RepoRoot != "" {
		return ctx.RepoRoot
	}
	return "."
}

func getStagedFiles(root string) ([]string, error) {
	checkCmd := workspace.SafeGitCommand(root, "rev-parse", "--is-inside-work-tree")
	if err := checkCmd.Run(); err != nil {
		return nil, nil
	}
	cmd := workspace.SafeGitCommand(root, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var stagedFiles []string
	for _, rawLine := range strings.Split(string(out), "\n") {
		if len(rawLine) < 4 {
			continue
		}
		status := rawLine[:2]
		if strings.Contains(status, "D") {
			continue
		}
		filePath := strings.TrimSpace(rawLine[3:])
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			filePath = parts[len(parts)-1]
		}
		stagedFiles = append(stagedFiles, filePath)
	}
	return stagedFiles, nil
}

func validateStagedFilesAgainstBranch(repoRoot, worktreeRoot string, stagedFiles []string) error {
	branchCmd := workspace.SafeGitCommand(worktreeRoot, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return nil
	}
	currentBranch := strings.TrimSpace(string(branchOut))
	root := worktreeRoot
	if root == "" {
		root = repoRoot
	}
	return workspace.ValidateUncommittedBoundaries(root, currentBranch, stagedFiles)
}
