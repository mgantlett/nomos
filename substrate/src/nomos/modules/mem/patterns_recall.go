// Package mem implements positive-path verified pattern distillation and Tier B graduation surfacing.
package mem

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// RecallRepoPatterns retrieves active 2PC-verified structural blueprints partitioned to repoRoot and records active task access (@intent: #Memory).
func RecallRepoPatterns(repoRoot, queryHint string, touchedFiles []string, limit int) []string {
	return recallRepoPatternsWithTask(repoRoot, "", queryHint, touchedFiles, limit, true)
}

// RecallRepoPatternsReadOnly retrieves active 2PC-verified structural blueprints without incrementing reinforcement counts (@intent: #Memory).
func RecallRepoPatternsReadOnly(repoRoot, queryHint string, touchedFiles []string, limit int) []string {
	return recallRepoPatternsWithTask(repoRoot, "", queryHint, touchedFiles, limit, false)
}

func recallRepoPatternsWithTask(repoRoot, taskKey, queryHint string, touchedFiles []string, limit int, recordAccess bool) []string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	store, err := NewStore("")
	if err != nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	all, err := store.ListAll()
	if err != nil {
		return nil
	}
	repoID := strings.ToLower(filepath.Base(filepath.Clean(repoRoot)))
	candidates := filterActiveRepoPatterns(all, repoID)
	if len(candidates) == 0 {
		return nil
	}

	rankRepoPatterns(candidates, DeriveDirectoryScope(repoRoot, touchedFiles), queryHint)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return collectRecalledContents(store, repoRoot, taskKey, candidates, recordAccess)
}

func filterActiveRepoPatterns(all []*Memory, repoID string) []*Memory {
	var candidates []*Memory
	for _, m := range all {
		if m.Kind == KindPattern && m.Status == StatusActive && strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
			candidates = append(candidates, m)
		}
	}
	return candidates
}

func rankRepoPatterns(candidates []*Memory, targetScope, queryHint string) {
	var queryEmb []float32
	trimmedHint := strings.TrimSpace(queryHint)
	if trimmedHint != "" {
		if emb, err := GenerateEmbedding(trimmedHint); err == nil {
			queryEmb = emb
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		iScope := targetScope != workspaceScopeLabel && strings.Contains(candidates[i].Topic, targetScope)
		jScope := targetScope != workspaceScopeLabel && strings.Contains(candidates[j].Topic, targetScope)
		if iScope != jScope {
			return iScope
		}
		if len(queryEmb) > 0 {
			iSim := CombinedScore(trimmedHint, queryEmb, candidates[i].Content, candidates[i].Embedding)
			jSim := CombinedScore(trimmedHint, queryEmb, candidates[j].Content, candidates[j].Embedding)
			if iSim != jSim {
				return iSim > jSim
			}
		}
		return candidates[i].ID > candidates[j].ID
	})
}

// RecallGraduationCandidates surfaces high-reinforcement statutory pitfall traps eligible for Tier B nomos.yaml promotion (@intent: #Memory).
func RecallGraduationCandidates(repoRoot string, limit int) []string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	store, err := NewStore("")
	if err != nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	all, err := store.ListAll()
	if err != nil {
		return nil
	}
	repoID := strings.ToLower(filepath.Base(filepath.Clean(repoRoot)))
	topicDepths := countPitfallTopicChain(all, repoID)
	var eligible []*Memory
	for _, m := range all {
		if isGraduationEligibleWithDepth(m, repoID, topicDepths[strings.ToLower(m.Topic)]) {
			eligible = append(eligible, m)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		di := topicDepths[strings.ToLower(eligible[i].Topic)]
		dj := topicDepths[strings.ToLower(eligible[j].Topic)]
		if di != dj {
			return di > dj
		}
		if eligible[i].AccessCount != eligible[j].AccessCount {
			return eligible[i].AccessCount > eligible[j].AccessCount
		}
		return eligible[i].ID > eligible[j].ID
	})
	if len(eligible) > limit {
		eligible = eligible[:limit]
	}
	out := make([]string, 0, len(eligible))
	for _, c := range eligible {
		depth := topicDepths[strings.ToLower(c.Topic)]
		out = append(out, formatGraduationCandidateLine(c, depth))
	}
	return out
}

func countPitfallTopicChain(all []*Memory, repoID string) map[string]int {
	counts := make(map[string]int)
	for _, m := range all {
		if m == nil || m.Kind != KindPitfall || m.Status == StatusTombstoned {
			continue
		}
		if strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
			counts[strings.ToLower(m.Topic)]++
		}
	}
	return counts
}

func isBuiltInStructuralASTTopic(topic string) bool {
	lower := strings.ToLower(strings.TrimSpace(topic))
	prefixes := []string{
		"pitfall:cognitive_complexity",
		"pitfall:function_length",
		"pitfall:file_length",
		"pitfall:docstring_density",
		"pitfall:statement_coverage",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func isGraduationEligibleWithDepth(m *Memory, repoID string, topicDepth int) bool {
	if m == nil || m.Status != StatusActive || m.Kind != KindPitfall {
		return false
	}
	if strings.ToLower(strings.TrimSpace(m.RepoID)) != repoID {
		return false
	}
	if isBuiltInStructuralASTTopic(m.Topic) {
		return topicDepth >= 2
	}
	return topicDepth >= 2 || m.AccessCount >= GraduationAccessThreshold
}

func formatGraduationCandidateLine(c *Memory, depth int) string {
	if depth < 1 {
		depth = 1
	}
	if isBuiltInStructuralASTTopic(c.Topic) {
		return fmt.Sprintf("[Tier A->Axiom (recurred %dx · %dx recalled) #%d %s] Recurring structural trap - promote package axiom or dismiss: %s",
			depth, c.AccessCount, c.ID, c.Topic, formatSnippet(c.Content, 96))
	}
	return fmt.Sprintf("[Tier A->B (recurred %dx · %dx recalled) #%d %s] Promote recurring invariant to nomos.yaml: %s",
		depth, c.AccessCount, c.ID, c.Topic, formatSnippet(c.Content, 96))
}

