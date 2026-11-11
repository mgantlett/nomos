// Package verify implements sibling worktree obligation isolation across concurrent
// INT-*/SUB-* task pairs so independent in-flight tasks never deadlock each other.
package verify

import (
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

// ResolveActiveWorktreeSpecRoots scopes specification paths when running inside an
// isolated worktree, skipping Substrate AST parity inside INT-* worktrees and deferring
// specs mutated exclusively by another in-flight sibling SUB-* task's INT-* twin
// (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func ResolveActiveWorktreeSpecRoots(scanRoot string, specRoots []string) []string {
	if extractWorktreePrefixedKey(scanRoot, "INT-") != "" {
		return []string{}
	}
	subKey := extractWorktreeSubTaskKey(scanRoot)
	if subKey == "" {
		return specRoots
	}
	return ResolveTaskContractSpecRoots(scanRoot, subKey, specRoots)
}

// ResolveTaskContractSpecRoots returns the contract specification files applicable to taskKey,
// excluding specs owned exclusively by another pending sibling SUB-* task's INT-* twin
// (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func ResolveTaskContractSpecRoots(scanRoot, taskKey string, specRoots []string) []string {
	deferred := collectActiveSiblingTwinSpecs(scanRoot, taskKey)
	filtered, hasDeferred := filterDeferredSpecRoots(scanRoot, specRoots, deferred)
	if !hasDeferred {
		return specRoots
	}
	return filtered
}

// extractWorktreeSubTaskKey extracts a canonical SUB-<N> key strictly when scanRoot
// resides inside an isolated '/worktrees/' directory.
func extractWorktreeSubTaskKey(scanRoot string) string {
	return extractWorktreePrefixedKey(scanRoot, "SUB-")
}

func extractWorktreePrefixedKey(scanRoot, prefix string) string {
	slash := filepath.ToSlash(filepath.Clean(scanRoot))
	if !strings.Contains(slash, "/worktrees/") {
		return ""
	}
	baseUpper := strings.ToUpper(filepath.Base(slash))
	idx := strings.LastIndex(baseUpper, prefix)
	if idx < 0 {
		return ""
	}
	cand := baseUpper[idx:]
	if err := task.ValidateTaskKey(cand); err != nil {
		return ""
	}
	return cand
}

// collectActiveSiblingTwinSpecs returns relative Markdown specification paths touched by
// other pending sibling SUB-* twins that are disjoint from activeSubKey's own twin.
func collectActiveSiblingTwinSpecs(scanRoot, activeSubKey string) map[string]bool {
	cleanKey := strings.TrimSpace(activeSubKey)
	if !strings.HasPrefix(cleanKey, "SUB-") {
		return nil
	}
	trunkRoot := resolvePrimaryRepoRoot(scanRoot)
	tasks := loadRepoScopedTasks(trunkRoot)
	if len(tasks) == 0 {
		return nil
	}
	ownSpecs := resolveOwnTwinSpecs(trunkRoot, cleanKey, tasks)
	deferred := make(map[string]bool)
	for _, rec := range tasks {
		if rec.Key == cleanKey || !strings.HasPrefix(rec.Key, "SUB-") {
			continue
		}
		twin := strings.TrimSpace(rec.TwinKey)
		if !strings.HasPrefix(twin, "INT-") || !isSiblingSubPendingForWorktree(trunkRoot, scanRoot, rec) {
			continue
		}
		for sp := range listTwinCommitSpecFiles(trunkRoot, twin) {
			if !ownSpecs[sp] {
				deferred[sp] = true
			}
		}
	}
	return deferred
}

func resolvePrimaryRepoRoot(scanRoot string) string {
	if pr, err := workspace.FindRepoRoot(scanRoot); err == nil && pr != "" {
		return pr
	}
	return scanRoot
}

func loadRepoScopedTasks(trunkRoot string) []task.TaskRecord {
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil || store == nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// best-effort read-only store close
		}
	}()
	project := task.ResolveRepoProject(trunkRoot)
	return task.FilterTasksByProject(store.ListTasks(), project, true)
}

func resolveOwnTwinSpecs(trunkRoot, activeSubKey string, tasks []task.TaskRecord) map[string]bool {
	for _, rec := range tasks {
		if rec.Key == activeSubKey && strings.HasPrefix(strings.TrimSpace(rec.TwinKey), "INT-") {
			return listTwinCommitSpecFiles(trunkRoot, strings.TrimSpace(rec.TwinKey))
		}
	}
	return nil
}

// isSiblingSubPendingForWorktree reports whether sibling SUB-* task rec is still pending
// (TODO, READY, PLAN, EDIT, REVIEW) or completed on trunk after scanRoot's branch diverged.
func isSiblingSubPendingForWorktree(trunkRoot, scanRoot string, rec task.TaskRecord) bool {
	if rec.Status != task.StatusDone && rec.Status != task.StatusAborted {
		return true
	}
	if rec.Status == task.StatusDone && strings.Contains(filepath.ToSlash(scanRoot), "/worktrees/") {
		return !worktreeContainsTaskCommit(trunkRoot, scanRoot, rec.Key)
	}
	return false
}

func worktreeContainsTaskCommit(trunkRoot, scanRoot, subKey string) bool {
	out, err := workspace.SafeGitCommand(trunkRoot, "log", "--fixed-strings", "--grep=["+subKey+"]", "-n", "1", "--format=%H").Output()
	if err != nil {
		return true
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return true
	}
	return workspace.SafeGitCommand(scanRoot, "merge-base", "--is-ancestor", sha, constants.ValHead).Run() == nil
}

func listTwinCommitSpecFiles(repoRoot, twinKey string) map[string]bool {
	out, err := workspace.SafeGitCommand(repoRoot, "log", "--fixed-strings", "--grep=["+twinKey+"]", "-n", "1", "--name-only", "--format=").Output()
	if err != nil {
		return nil
	}
	specs := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		clean := filepath.ToSlash(strings.TrimSpace(line))
		if clean != "" && strings.HasSuffix(clean, ".md") {
			specs[clean] = true
		}
	}
	return specs
}

// filterDeferredSpecRoots expands specRoots to individual Markdown files when at least one
// file matches deferred, returning the remaining non-deferred spec file paths.
func filterDeferredSpecRoots(scanRoot string, specRoots []string, deferred map[string]bool) ([]string, bool) {
	if len(deferred) == 0 {
		return specRoots, false
	}
	allFiles := resolveObligationSpecFiles(scanRoot, specRoots)
	kept := make([]string, 0, len(allFiles))
	skippedAny := false
	for _, f := range allFiles {
		if deferred[toRelSlash(scanRoot, f)] {
			skippedAny = true
			continue
		}
		kept = append(kept, f)
	}
	if !skippedAny {
		return specRoots, false
	}
	return kept, true
}
