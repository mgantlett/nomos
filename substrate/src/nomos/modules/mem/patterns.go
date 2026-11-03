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

// DistillVerifiedPattern promotes verified 2PC worktree diffs into repo-scoped pattern blueprints (@intent: #Memory).
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
	case ".go", ".py", ".ts", ".tsx", ".js", ".rs", ".svelte":
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
