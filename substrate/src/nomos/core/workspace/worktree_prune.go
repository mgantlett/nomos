// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// PruneReport summarizes orphaned worktree directories, merged feature branches, and temporary walkthroughs swept.
type PruneReport struct {
	RemovedDirs      []string `json:"removed_dirs"`
	SkippedActive    []string `json:"skipped_active"`
	RemovedBranches  []string `json:"removed_branches"`
	RemovedTempFiles []string `json:"removed_temp_files"`
	DryRun           bool     `json:"dry_run"`
}

// PruneOrphanWorktrees sweeps orphaned worktrees/* directories, prunes git worktree metadata,
// deletes merged feature/* branches, and cleans stale .tmp/walkthrough-* files.
// Any worktree whose task key is present in activeKeys (or is an active git worktree with uncommitted changes) is preserved.
func (m *Manager) PruneOrphanWorktrees(dryRun bool, activeKeys map[string]bool) (*PruneReport, error) {
	rep := &PruneReport{DryRun: dryRun}
	if activeKeys == nil {
		activeKeys = map[string]bool{}
	}

	registeredPaths := m.registeredWorktreePathSet()
	entries, err := os.ReadDir(m.WorktreesDir())
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			m.evaluateWorktreeDirEntry(entry.Name(), registeredPaths, activeKeys, dryRun, rep)
		}
	}

	if !dryRun {
		if pErr := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "prune", "--expire", "now").Run(); pErr != nil {
			// best-effort git worktree prune
		}
		if bErr := SafeGitCommand(m.RepoRoot, constants.DirConfig, "core.bare", "false").Run(); bErr != nil {
			// best-effort bare flag guard
		}
	}

	rep.RemovedBranches = m.pruneMergedFeatureBranches(dryRun, activeKeys)
	rep.RemovedTempFiles = m.pruneStaleWalkthroughFiles(dryRun, activeKeys)
	return rep, nil
}

func (m *Manager) registeredWorktreePathSet() map[string]bool {
	set := map[string]bool{}
	sessions, err := m.ListWorktrees()
	if err != nil {
		return set
	}
	for _, s := range sessions {
		set[filepath.Clean(s.WorktreePath)] = true
	}
	return set
}

func (m *Manager) evaluateWorktreeDirEntry(
	dirName string,
	registeredPaths map[string]bool,
	activeKeys map[string]bool,
	dryRun bool,
	rep *PruneReport,
) {
	fullPath := filepath.Clean(filepath.Join(m.WorktreesDir(), dirName))
	taskKey := extractTaskKeyFromPath(dirName)
	baseKey := extractBaseKey(taskKey)

	if (taskKey != "" && activeKeys[taskKey]) || (baseKey != "" && activeKeys[baseKey]) {
		rep.SkippedActive = append(rep.SkippedActive, dirName)
		return
	}

	if registeredPaths[fullPath] {
		if hasDirtyGitState(fullPath) {
			rep.SkippedActive = append(rep.SkippedActive, dirName)
			return
		}
		if !dryRun && taskKey != "" {
			if err := m.RemoveWorktree(taskKey); err != nil {
				return
			}
		}
		rep.RemovedDirs = append(rep.RemovedDirs, dirName)
		return
	}

	if hasUnregisteredDirtyGitTree(fullPath) {
		rep.SkippedActive = append(rep.SkippedActive, dirName)
		return
	}
	if !dryRun {
		if err := os.RemoveAll(fullPath); err != nil {
			return
		}
	}
	rep.RemovedDirs = append(rep.RemovedDirs, dirName)
}

func hasUnregisteredDirtyGitTree(fullPath string) bool {
	if _, err := os.Stat(filepath.Join(fullPath, ".git")); err != nil {
		return false
	}
	return hasDirtyGitState(fullPath)
}

func hasDirtyGitState(wtPath string) bool {
	out, err := SafeGitCommand(wtPath, "status", "--porcelain").Output()
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(out)) != ""
}

func (m *Manager) pruneMergedFeatureBranches(dryRun bool, activeKeys map[string]bool) []string {
	out, err := SafeGitCommand(m.RepoRoot, "branch", "--merged", constants.ValMain).Output()
	if err != nil {
		return nil
	}
	var removed []string
	for _, rawLine := range strings.Split(string(out), "\n") {
		branch := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rawLine), "*"))
		if !strings.HasPrefix(branch, "feature/") {
			continue
		}
		taskKey := strings.TrimPrefix(branch, "feature/")
		baseKey := extractBaseKey(taskKey)
		if activeKeys[taskKey] || activeKeys[baseKey] {
			continue
		}
		if !dryRun {
			if err := SafeGitCommand(m.RepoRoot, "branch", "-d", branch).Run(); err != nil {
				continue
			}
		}
		removed = append(removed, branch)
	}
	return removed
}

func (m *Manager) pruneStaleWalkthroughFiles(dryRun bool, activeKeys map[string]bool) []string {
	tmpDir := filepath.Join(m.RepoRoot, constants.DirTmp)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil
	}
	var removed []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "walkthrough-") {
			continue
		}
		keyPart := strings.TrimSuffix(strings.TrimPrefix(name, "walkthrough-"), ".md")
		baseKey := extractBaseKey(keyPart)
		if activeKeys[keyPart] || activeKeys[baseKey] {
			continue
		}
		if !dryRun {
			if err := os.Remove(filepath.Join(tmpDir, name)); err != nil {
				continue
			}
		}
		removed = append(removed, name)
	}
	return removed
}
