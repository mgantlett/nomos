// Package mem implements positive-path verified pattern distillation and Tier B graduation surfacing.
package mem

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// GraduationAccessThreshold is the recall count required to surface Tier A->B promotion candidates.
	GraduationAccessThreshold = 3
	maxDistilledScopes        = 2
	maxBlueprintItems         = 4
	workspaceScopeLabel       = "workspace"
)

var (
	intentAnchorPattern   = regexp.MustCompile(`#[A-Z][A-Za-z0-9_]+`)
	polyglotSymbolPattern = regexp.MustCompile(`(?m)^(?:def|class|export\s+(?:async\s+)?function|export\s+interface|export\s+type|pub\s+fn|pub\s+struct)\s+([A-Za-z_][A-Za-z0-9_]*)`)
)

type scopeBlueprint struct {
	scope   string
	files   []string
	tests   []string
	symbols []string
	intents []string
}

// DistillVerifiedPattern promotes verified 2PC worktree diffs into repo-scoped pattern blueprints (@intent: #MnemeMemory).
func DistillVerifiedPattern(repoRoot, taskKey, walkthrough string, touchedFiles []string) (int, error) {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" || len(touchedFiles) == 0 {
		return 0, nil
	}
	scopes := collectScopeBlueprints(repoRoot, touchedFiles)
	if len(scopes) == 0 {
		return 0, nil
	}

	store, err := NewStore("")
	if err != nil {
		return 0, err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	repoID := filepath.Base(filepath.Clean(repoRoot))
	summary := summarizeWalkthroughFix(walkthrough)
	upperTask := strings.ToUpper(strings.TrimSpace(taskKey))
	saved := 0

	for _, bp := range scopes {
		topic := "pattern:" + bp.scope
		rec := &Memory{
			RepoID:  repoID,
			Kind:    KindPattern,
			Topic:   topic,
			Content: formatScopeBlueprintContent(bp, upperTask, summary),
			TaskID:  upperTask,
			Status:  StatusActive,
		}
		if store.Save(rec) == nil {
			store.supersedeOlderTopicPatterns(repoID, topic, rec.ID)
			saved++
		}
	}
	return saved, nil
}

func collectScopeBlueprints(repoRoot string, touchedFiles []string) []*scopeBlueprint {
	byScope := make(map[string]*scopeBlueprint)
	var order []string

	for _, raw := range touchedFiles {
		rel := normalizePatternPath(repoRoot, raw)
		if !isDistillableFile(rel) {
			continue
		}
		scope := filepath.ToSlash(filepath.Dir(rel))
		if scope == "." || scope == "/" || scope == "" {
			scope = "root"
		}
		bp, exists := byScope[scope]
		if !exists {
			if len(order) >= maxDistilledScopes {
				continue
			}
			bp = &scopeBlueprint{scope: scope}
			byScope[scope] = bp
			order = append(order, scope)
		}
		populateBlueprintFile(repoRoot, rel, bp)
	}

	var out []*scopeBlueprint
	for _, k := range order {
		bp := byScope[k]
		if len(bp.files) > 0 || len(bp.tests) > 0 {
			out = append(out, bp)
		}
	}
	return out
}

func normalizePatternPath(repoRoot, raw string) string {
	clean := filepath.ToSlash(strings.TrimSpace(raw))
	if clean == "" {
		return ""
	}
	if repoRoot != "" && filepath.IsAbs(raw) {
		if rel, err := filepath.Rel(repoRoot, raw); err == nil {
			clean = filepath.ToSlash(rel)
		}
	}
	return clean
}

func isDistillableFile(rel string) bool {
	if rel == "" || strings.HasPrefix(rel, ".tmp/") || strings.HasPrefix(rel, "worktrees/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".go", ".py", ".ts", ".tsx", ".js", ".rs", ".cue", ".svelte":
		return true
	default:
		return false
	}
}

func isPatternTestFile(base string) bool {
	lower := strings.ToLower(base)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasPrefix(lower, "test_") ||
		strings.HasSuffix(lower, "_test.py") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.")
}

func populateBlueprintFile(repoRoot, rel string, bp *scopeBlueprint) {
	base := filepath.Base(rel)
	if isPatternTestFile(base) {
		bp.tests = appendUniqueBounded(bp.tests, base, maxBlueprintItems)
		return
	}
	bp.files = appendUniqueBounded(bp.files, base, maxBlueprintItems)
	absPath := filepath.Join(repoRoot, rel)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return
	}
	for _, sym := range extractSourceSymbols(absPath, data) {
		bp.symbols = appendUniqueBounded(bp.symbols, sym, maxBlueprintItems)
	}
	for _, anchor := range extractIntentAnchorsFromBytes(data) {
		bp.intents = appendUniqueBounded(bp.intents, anchor, maxBlueprintItems)
	}
}

func extractSourceSymbols(absPath string, data []byte) []string {
	if strings.HasSuffix(strings.ToLower(absPath), ".go") {
		return extractExportedGoNames(absPath, data)
	}
	matches := polyglotSymbolPattern.FindAllStringSubmatch(string(data), maxBlueprintItems)
	var names []string
	for _, m := range matches {
		if len(m) > 1 {
			names = appendUniqueBounded(names, m[1], maxBlueprintItems)
		}
	}
	return names
}

func extractExportedGoNames(absPath string, data []byte) []string {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, absPath, data, 0)
	if err != nil {
		return nil
	}
	var names []string
	for _, decl := range fileNode.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				names = appendUniqueBounded(names, d.Name.Name, maxBlueprintItems)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
					names = appendUniqueBounded(names, ts.Name.Name, maxBlueprintItems)
				}
			}
		}
	}
	return names
}

func extractIntentAnchorsFromBytes(data []byte) []string {
	var anchors []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, "@intent") && !strings.Contains(line, "(#") {
			continue
		}
		for _, m := range intentAnchorPattern.FindAllString(line, maxBlueprintItems) {
			anchors = appendUniqueBounded(anchors, m, maxBlueprintItems)
		}
	}
	return anchors
}

func appendUniqueBounded(list []string, item string, maxItems int) []string {
	if item == "" || len(list) >= maxItems {
		return list
	}
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

func formatScopeBlueprintContent(bp *scopeBlueprint, taskKey, summary string) string {
	var parts []string
	if len(bp.files) > 0 {
		parts = append(parts, "files=["+strings.Join(bp.files, ",")+"]")
	}
	if len(bp.symbols) > 0 {
		parts = append(parts, "symbols=["+strings.Join(bp.symbols, ",")+"]")
	}
	if len(bp.intents) > 0 {
		parts = append(parts, "intents=["+strings.Join(bp.intents, ",")+"]")
	}
	if len(bp.tests) > 0 {
		parts = append(parts, "tests=["+strings.Join(bp.tests, ",")+"]")
	}
	return fmt.Sprintf("[Pattern in %s (%s)] %s -> Verified Summary: %s",
		bp.scope, taskKey, strings.Join(parts, " "), summary)
}

func (s *Store) supersedeOlderTopicPatterns(repoID, topic string, newestID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `UPDATE memories SET status = 'superseded', superseded_by = ?
	          WHERE kind = 'pattern' AND status = 'active' AND lower(repo_id) = lower(?) AND lower(topic) = lower(?) AND id != ?`
	if _, err := s.db.Exec(query, newestID, strings.TrimSpace(repoID), strings.TrimSpace(topic), newestID); err != nil {
		return
	}
}

// RecallRepoPatterns retrieves active 2PC-verified structural blueprints partitioned to repoRoot (@intent: #MnemeMemory).
func RecallRepoPatterns(repoRoot, queryHint string, touchedFiles []string, limit int) []string {
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
		if aErr := store.RecordAccess(c.ID); aErr != nil {
			// best effort
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

// RecallGraduationCandidates surfaces high-reinforcement statutory memories eligible for Tier B nomos.yaml promotion (@intent: #MnemeMemory).
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
	if m.Kind != KindPitfall && m.Kind != KindPattern {
		return false
	}
	return strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID
}
