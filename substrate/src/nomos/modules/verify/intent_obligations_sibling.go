// Package verify implements sibling worktree obligation isolation across concurrent
// INT-*/SUB-* task pairs so independent in-flight tasks never deadlock each other.
package verify

import (
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
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
		if !isEligibleSiblingRecord(trunkRoot, scanRoot, cleanKey, rec) {
			continue
		}
		for sp := range listTwinCommitSpecFiles(trunkRoot, strings.TrimSpace(rec.TwinKey)) {
			if !ownSpecs[sp] {
				deferred[sp] = true
			}
		}
	}
	return deferred
}

// collectActiveSiblingTwinDeltas extracts exact commit-delta TC-* vectors, structs, and struct fields
// introduced by pending sibling INT-* twins even when they share a canonical domain spec with activeSubKey
// (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func collectActiveSiblingTwinDeltas(scanRoot, activeSubKey string) *impact.SiblingDelta {
	cleanKey := strings.TrimSpace(activeSubKey)
	if !strings.HasPrefix(cleanKey, "SUB-") {
		return nil
	}
	trunkRoot := resolvePrimaryRepoRoot(scanRoot)
	tasks := loadRepoScopedTasks(trunkRoot)
	if len(tasks) == 0 {
		return nil
	}
	return computePendingSiblingDeltasFromTasks(trunkRoot, scanRoot, cleanKey, tasks)
}

func computePendingSiblingDeltasFromTasks(trunkRoot, scanRoot, activeSubKey string, tasks []task.TaskRecord) *impact.SiblingDelta {
	ownTwin := findOwnTwinKey(activeSubKey, tasks)
	delta := impact.NewSiblingDelta()
	for _, rec := range tasks {
		twin := strings.TrimSpace(rec.TwinKey)
		if twin == ownTwin || !isEligibleSiblingRecord(trunkRoot, scanRoot, activeSubKey, rec) {
			continue
		}
		accumulateTwinCommitDelta(trunkRoot, twin, delta)
	}
	if ownTwin != "" {
		ownDelta := impact.NewSiblingDelta()
		accumulateTwinCommitDelta(trunkRoot, ownTwin, ownDelta)
		subtractOwnDelta(delta, ownDelta)
	}
	return delta
}

func isEligibleSiblingRecord(trunkRoot, scanRoot, activeSubKey string, rec task.TaskRecord) bool {
	if rec.Key == activeSubKey || !strings.HasPrefix(rec.Key, "SUB-") {
		return false
	}
	twin := strings.TrimSpace(rec.TwinKey)
	return strings.HasPrefix(twin, "INT-") && isSiblingSubPendingForWorktree(trunkRoot, scanRoot, rec)
}

func findOwnTwinKey(activeSubKey string, tasks []task.TaskRecord) string {
	for _, rec := range tasks {
		if rec.Key == activeSubKey && strings.HasPrefix(strings.TrimSpace(rec.TwinKey), "INT-") {
			return strings.TrimSpace(rec.TwinKey)
		}
	}
	return ""
}

func accumulateTwinCommitDelta(repoRoot, twinKey string, into *impact.SiblingDelta) {
	sha := findTaskCommitSHA(repoRoot, twinKey)
	if sha == "" {
		return
	}
	for specRel := range listTwinCommitSpecFiles(repoRoot, twinKey) {
		beforeMD := readGitBlobString(repoRoot, sha+"~1:"+specRel)
		afterMD := readGitBlobString(repoRoot, sha+":"+specRel)
		if afterMD != "" {
			impact.ComputeMarkdownSpecDelta(beforeMD, afterMD, into)
		}
	}
}

func subtractOwnDelta(sibling, own *impact.SiblingDelta) {
	for tc := range own.AddedTCs {
		delete(sibling.AddedTCs, tc)
	}
	for st := range own.AddedStructs {
		delete(sibling.AddedStructs, st)
	}
	for st, fields := range own.AddedFields {
		for f := range fields {
			delete(sibling.AddedFields[st], f)
		}
	}
}

func findTaskCommitSHA(repoRoot, taskKey string) string {
	out, err := workspace.SafeGitCommand(repoRoot, "log", "--fixed-strings", "--grep=["+taskKey+"]", "-n", "1", "--format=%H").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readGitBlobString(repoRoot, specRev string) string {
	out, err := workspace.SafeGitCommand(repoRoot, "show", specRev).Output()
	if err != nil {
		return ""
	}
	return string(out)
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
	if own := findOwnTwinKey(activeSubKey, tasks); own != "" {
		return listTwinCommitSpecFiles(trunkRoot, own)
	}
	return nil
}

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
	sha := findTaskCommitSHA(trunkRoot, subKey)
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
