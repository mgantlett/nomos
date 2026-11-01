// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

var regexTaskSlug = regexp.MustCompile(`(SUB|INT)-\d+`)

// ValidateWorktreeBoundary ensures that mutations during a task execute strictly within
// an isolated worktree when on a protected branch (Law 3).
func ValidateWorktreeBoundary(repoRoot, currentBranch, targetFile string) error {
	return ValidateWorktreeTaskBoundary(repoRoot, currentBranch, targetFile, os.Getenv("NOMOS_ACTIVE_TASK"))
}

// ValidateWorktreeTaskBoundary ensures worktree isolation and verifies worktreeTaskKey == activeTaskKey (Law 3, Bug 0.5).
func ValidateWorktreeTaskBoundary(repoRoot, currentBranch, targetFile, activeTaskKey string) error {
	cleanRoot := filepath.Clean(repoRoot)
	cleanTarget := filepath.Clean(targetFile)

	if !strings.HasPrefix(cleanTarget, cleanRoot) {
		return nil
	}

	wtDir := filepath.Join(cleanRoot, constants.DirWorktrees)
	// Inside a worktree is permitted, provided task matches if activeTaskKey is specified
	if strings.Contains(cleanTarget, wtDir) {
		if activeTaskKey != "" {
			rel, err := filepath.Rel(wtDir, cleanTarget)
			if err == nil && !strings.HasPrefix(rel, "..") {
				parts := strings.Split(rel, string(filepath.Separator))
				if len(parts) > 0 {
					wtSlug := parts[0]
					match := regexTaskSlug.FindString(wtSlug)
					if match != "" && !strings.EqualFold(match, activeTaskKey) {
						return types.NewNomosError(types.ErrCodeBoundary, "worktree task mismatch: worktree %q targets %s, but active task is %s (Bug 0.5)", wtSlug, match, activeTaskKey)
					}
				}
			}
		}
		return nil
	}

	// If target is in root repo and branch is protected, fail closed (Law 3)
	if IsProtectedBranch(currentBranch) {
		return types.NewNomosError(types.ErrCodeBoundary, "worktree boundary violation: direct mutation of root repository on protected trunk branch is strictly prohibited (Law 3)")
	}

	return nil
}

// ValidateUncommittedBoundaries checks that none of the mutated files violate worktree isolation boundaries.
func ValidateUncommittedBoundaries(repoRoot, currentBranch string, uncommittedFiles []string) error {
	return ValidateUncommittedTaskBoundaries(repoRoot, currentBranch, uncommittedFiles, os.Getenv("NOMOS_ACTIVE_TASK"))
}

// ValidateUncommittedTaskBoundaries checks that none of the mutated files violate worktree isolation boundaries or task assignment.
func ValidateUncommittedTaskBoundaries(repoRoot, currentBranch string, uncommittedFiles []string, activeTaskKey string) error {
	for _, f := range uncommittedFiles {
		if err := validateFileBoundary(repoRoot, currentBranch, f, activeTaskKey); err != nil {
			return err
		}
	}
	return nil
}

func validateFileBoundary(repoRoot, currentBranch, f, activeTaskKey string) error {
	base := filepath.Base(f)
	dir := filepath.Dir(f)
	if (strings.Contains(base, "walkthrough") || strings.Contains(base, "scratch")) && dir == "." {
		return types.NewNomosError(types.ErrCodeBoundary, "worktree boundary violation: scratch files and walkthroughs must go in .tmp/ (e.g., .tmp/%s)", base)
	}
	target := filepath.Join(repoRoot, f)
	if rootInfo, err := os.Stat(repoRoot); err == nil && rootInfo.IsDir() {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return nil
		}
	}
	return ValidateWorktreeTaskBoundary(repoRoot, currentBranch, target, activeTaskKey)
}
