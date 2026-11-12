// Package mem implements positive-path verified pattern distillation and Tier B graduation surfacing.
package mem

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

const (
	// CleanStreakCoolingThreshold defines consecutive clean task recalls required to cool hot scopes and graduation nagging.
	CleanStreakCoolingThreshold = 5
	// HotScopeMinRecurrence defines minimum directory pitfall recurrences required to sensitize FastFileRadar.
	HotScopeMinRecurrence = 3
)

// HotScopeReflex describes an active directory-scoped post-tool AST reflex (#HotScopeReflex).
type HotScopeReflex struct {
	Scope           string   `json:"scope"`
	RecurrenceDepth int      `json:"recurrence_depth"`
	CleanStreak     int      `json:"clean_streak"`
	ActiveGates     []string `json:"active_gates"`
}

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

	targetScope := DeriveDirectoryScope(repoRoot, touchedFiles)
	rankRepoPatterns(candidates, targetScope, queryHint)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return collectRecalledContents(store, repoRoot, taskKey, targetScope, candidates, recordAccess)
}

func filterActiveRepoPatterns(all []*Memory, repoID string) []*Memory {
	var candidates []*Memory
	for _, m := range all {
		if m.Kind == KindPattern && m.Status == StatusActive && strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	for _, m := range all {
		if m.Kind == KindArchitecture && m.Status == StatusActive && strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
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
		"pitfall:cognitive_debt_score",
		"pitfall:max_file_loc",
		"pitfall:function_length",
		"pitfall:file_length",
		"pitfall:docstring_density",
		"pitfall:statement_coverage",
		"pitfall:unchecked_error_hygiene",
		"pitfall:anti-shim_ast",
		"pitfall:tdd_test_pair",
		"pitfall:dead-code_reachability",
		"pitfall:go_toolchain_gate",
		"pitfall:polyglot_toolchain",
		"pitfall:intent_steel-man",
		"pitfall:intent_substrate_parity",
		"pitfall:intent_traceability",
		"pitfall:bilateral_parity",
		"pitfall:plane_separation",
		"pitfall:hermetic_worktree",
		"pitfall:zero_filesystem_alias",
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
	if strings.ToLower(strings.TrimSpace(m.RepoID)) != repoID || isEphemeralTestAssertionContent(m.Content) {
		return false
	}
	if isBuiltInStructuralASTTopic(m.Topic) || m.AccessCount >= CleanStreakCoolingThreshold {
		return false
	}
	return topicDepth >= 2 || m.AccessCount >= GraduationAccessThreshold
}

func formatGraduationCandidateLine(c *Memory, depth int) string {
	if depth < 1 {
		depth = 1
	}
	return fmt.Sprintf("[Tier A->B (recurred %dx · %dx recalled) #%d %s] Promote recurring invariant to nomos.yaml: %s",
		depth, c.AccessCount, c.ID, c.Topic, formatSnippet(c.Content, 96))
}

// ResolveHotScopeReflex inspects active directory pitfalls for post-tool AST sensitization (@intent intent.md mneme_memory_spec.md #HotScopeReflex).
func ResolveHotScopeReflex(repoRoot, relDir string) *HotScopeReflex {
	cleanRoot := strings.TrimSpace(repoRoot)
	cleanDir := strings.TrimSpace(filepath.ToSlash(relDir))
	if cleanRoot == "" || cleanDir == "" || cleanDir == workspaceScopeLabel {
		return nil
	}
	if _, err := os.Stat(config.ResolveMnemeDBPath()); err != nil {
		return nil
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
	repoID := strings.ToLower(filepath.Base(filepath.Clean(cleanRoot)))
	return buildHotScopeReflex(all, repoID, cleanDir, countPitfallTopicChain(all, repoID))
}

func buildHotScopeReflex(all []*Memory, repoID, cleanDir string, topicDepths map[string]int) *HotScopeReflex {
	seenGates := make(map[string]bool)
	var gates []string
	maxDepth := 0
	minStreak := -1

	for _, m := range all {
		if !isActiveHotScopeCandidate(m, repoID, cleanDir) {
			continue
		}
		depth := topicDepths[strings.ToLower(m.Topic)]
		if depth < HotScopeMinRecurrence || m.AccessCount >= CleanStreakCoolingThreshold {
			continue
		}
		gateSlug := extractGateSlugFromPitfallTopic(m.Topic)
		if gateSlug != "" && !seenGates[gateSlug] {
			seenGates[gateSlug] = true
			gates = append(gates, gateSlug)
		}
		if depth > maxDepth {
			maxDepth = depth
		}
		if minStreak < 0 || m.AccessCount < minStreak {
			minStreak = m.AccessCount
		}
	}
	if len(gates) == 0 {
		return nil
	}
	sort.Strings(gates)
	return &HotScopeReflex{
		Scope:           cleanDir,
		RecurrenceDepth: maxDepth,
		CleanStreak:     minStreak,
		ActiveGates:     gates,
	}
}

func isActiveHotScopeCandidate(m *Memory, repoID, cleanDir string) bool {
	if m == nil || m.Status != StatusActive || m.Kind != KindPitfall {
		return false
	}
	if strings.ToLower(strings.TrimSpace(m.RepoID)) != repoID || !isBuiltInStructuralASTTopic(m.Topic) {
		return false
	}
	return strings.EqualFold(extractScopeFromPitfallTopic(m.Topic), cleanDir)
}

