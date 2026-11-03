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
	return recallRepoPatterns(repoRoot, queryHint, touchedFiles, limit, true)
}

// RecallRepoPatternsReadOnly retrieves active 2PC-verified structural blueprints without incrementing reinforcement counts (@intent: #Memory).
func RecallRepoPatternsReadOnly(repoRoot, queryHint string, touchedFiles []string, limit int) []string {
	return recallRepoPatterns(repoRoot, queryHint, touchedFiles, limit, false)
}

func recallRepoPatterns(repoRoot, queryHint string, touchedFiles []string, limit int, recordAccess bool) []string {
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
	var candidates []*Memory
	for _, m := range all {
		if m.Kind == KindPattern && m.Status == StatusActive && strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	rankRepoPatterns(candidates, DeriveDirectoryScope(repoRoot, touchedFiles), queryHint)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if recordAccess {
			if aErr := store.RecordAccess(c.ID); aErr != nil {
				// best effort
			}
		}
		out = append(out, c.Content)
	}
	return out
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
	var eligible []*Memory
	for _, m := range all {
		if isGraduationEligible(m, repoID) {
			eligible = append(eligible, m)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
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
		out = append(out, fmt.Sprintf("[Tier A->B (%dx recalled) #%d %s] Promote recurring invariant to nomos.yaml: %s",
			c.AccessCount, c.ID, c.Topic, formatSnippet(c.Content, 96)))
	}
	return out
}

func isGraduationEligible(m *Memory, repoID string) bool {
	if m == nil || m.Status != StatusActive || m.AccessCount < GraduationAccessThreshold {
		return false
	}
	if m.Kind != KindPitfall {
		return false
	}
	return strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID
}

