// Package cli implements one-time JIT directory-scoped Mneme pitfall recall on first package edits.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

var worktreeTaskKeyTokenRe = regexp.MustCompile(`(?i)(?:^|-)((?:SUB|INT)-\d+)(?:$|-)`)

type scopeMutationPair struct {
	relDir  string
	relFile string
}

// evaluateJITDirectoryPitfallAdvisory recalls matching directory-scoped pitfalls from Nomos Mneme
// at most once per directory in a worktree session trace (@intent mneme_memory_spec.md).
func evaluateJITDirectoryPitfallAdvisory(p *HookPayload, hermeticDir string) string {
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return ""
	}
	tracePath := resolveTraceStateFilePath(wtRoot)
	if tracePath == "" {
		return ""
	}
	canonWt := normalizeCanonicalPath(wtRoot, "/")
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	tr := loadOrInitHookSessionTrace(tracePath, wtRoot)
	pairs, dirty := collectUnseenMutationScopes(p, &tr, canonWt, baseCwd)
	if dirty {
		saveHookSessionTrace(tracePath, tr)
	}
	if len(pairs) == 0 {
		return ""
	}
	if _, err := os.Stat(config.ResolveMnemeDBPath()); err != nil {
		return ""
	}
	repoRoot := resolveWorktreePrimaryRepoRoot(wtRoot)
	taskKey := extractWorktreeTaskKeyFromRoot(wtRoot)
	var lines []string
	for _, pair := range pairs {
		for _, item := range recallScopedDirectoryPitfalls(repoRoot, taskKey, pair.relFile, pair.relDir) {
			lines = append(lines, fmt.Sprintf("⚡ [Nomos JIT Pitfall · %s] %s", pair.relDir, item))
		}
	}
	return strings.Join(lines, "\n")
}

// collectUnseenMutationScopes records newly mutated worktree directories in hookSessionTrace
// so subsequent edits in the same package never repeat Mneme queries or spam advisories.
func collectUnseenMutationScopes(p *HookPayload, tr *hookSessionTrace, canonWt, baseCwd string) ([]scopeMutationPair, bool) {
	targets := p.TargetFiles
	if len(targets) == 0 && p.TargetFile != "" {
		targets = []string{p.TargetFile}
	}
	var unseen []scopeMutationPair
	dirty := false
	for _, rawTarget := range targets {
		resolved := normalizeCanonicalPath(rawTarget, baseCwd)
		if !strings.HasPrefix(resolved, canonWt+"/") {
			continue
		}
		relFile := strings.TrimPrefix(resolved, canonWt+"/")
		if isIgnoredPitfallScopeFile(relFile) {
			continue
		}
		relDir := mem.DeriveDirectoryScope("", []string{relFile})
		if relDir == "" || containsTraceItem(tr.RecalledPitfallDirs, relDir) {
			continue
		}
		tr.RecalledPitfallDirs = appendUniqueBounded(tr.RecalledPitfallDirs, relDir, 64)
		dirty = true
		unseen = append(unseen, scopeMutationPair{relDir: relDir, relFile: relFile})
	}
	return unseen, dirty
}

func isIgnoredPitfallScopeFile(relFile string) bool {
	if relFile == "" || strings.HasPrefix(relFile, ".") {
		return true
	}
	return strings.Contains(relFile, "/.tmp/")
}

func containsTraceItem(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// recallScopedDirectoryPitfalls invokes RecallRepoPitfallsForTask when an active directory-scoped
// pitfall matches relDir and filters results strictly to that directory scope.
func recallScopedDirectoryPitfalls(repoRoot, taskKey, relFile, relDir string) []string {
	matchedSet := collectMatchingScopedPitfallSet(repoRoot, relDir)
	if len(matchedSet) == 0 {
		return nil
	}
	recalled := mem.RecallRepoPitfallsForTask(repoRoot, taskKey, []string{relFile}, 2)
	var scoped []string
	for _, item := range recalled {
		if matchedSet[item] {
			scoped = append(scoped, item)
		}
	}
	return scoped
}

func collectMatchingScopedPitfallSet(repoRoot, relDir string) map[string]bool {
	store, err := mem.NewStore("")
	if err != nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			return
		}
	}()
	all, err := store.ListAll()
	if err != nil {
		return nil
	}
	repoID := strings.ToLower(filepath.Base(filepath.Clean(repoRoot)))
	topicSuffix := ":" + strings.ToLower(strings.TrimSpace(relDir))
	inBracketTag := " in " + relDir + "]"
	matched := make(map[string]bool)
	for _, m := range all {
		if !isActivePitfallForRepo(m, repoID) {
			continue
		}
		lowerTopic := strings.ToLower(strings.TrimSpace(m.Topic))
		if strings.HasSuffix(lowerTopic, topicSuffix) || strings.Contains(m.Content, inBracketTag) {
			matched[m.Content] = true
		}
	}
	return matched
}

func isActivePitfallForRepo(m *mem.Memory, repoID string) bool {
	if m == nil || m.Status != mem.StatusActive {
		return false
	}
	if m.Kind != mem.KindPitfall && m.Kind != mem.KindDeadEnd {
		return false
	}
	return strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID
}

func resolveWorktreePrimaryRepoRoot(wtRoot string) string {
	if pr, err := workspace.FindRepoRoot(wtRoot); err == nil && pr != "" {
		return filepath.Clean(pr)
	}
	slashWt := filepath.ToSlash(wtRoot)
	const marker = "/worktrees/"
	if idx := strings.Index(slashWt, marker); idx > 0 {
		return filepath.FromSlash(slashWt[:idx])
	}
	return filepath.Clean(wtRoot)
}

func extractWorktreeTaskKeyFromRoot(wtRoot string) string {
	base := filepath.Base(filepath.Clean(wtRoot))
	matches := worktreeTaskKeyTokenRe.FindAllStringSubmatch(base, -1)
	if len(matches) == 0 {
		return ""
	}
	last := matches[len(matches)-1]
	if len(last) < 2 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(last[1]))
}
