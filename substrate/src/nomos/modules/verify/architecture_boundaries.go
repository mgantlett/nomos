// Package verify enforces declarative polyglot architecture import boundaries
// across Go AST and Tree-sitter source files in <5ms (Law 7).
package verify

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

var (
	esFromImportRe   = regexp.MustCompile(`(?m)^\s*(?:import|export)\s+(?:[^'"]*?\s+from\s+)?['"]([^'"]+)['"]`)
	esCallImportRe   = regexp.MustCompile(`\b(?:require|import)\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	pyFromImportRe   = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_.]+)\s+import\b`)
	pyBareImportRe   = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_.]+)`)
	rustUseRe        = regexp.MustCompile(`(?m)^\s*use\s+([A-Za-z0-9_:]+)`)
	jvmDotImportRe   = regexp.MustCompile(`(?m)^\s*(?:import|using)\s+(?:static\s+)?([A-Za-z0-9_.]+)`)
	rubyRequireRe    = regexp.MustCompile(`(?m)^\s*require(?:_relative)?\s+['"]([^'"]+)['"]`)
	cIncludeImportRe = regexp.MustCompile(`(?m)^\s*#\s*include\s+[<"]([^>"]+)[>"]`)
)

// EvaluateArchitectureBoundaries checks Go AST and polyglot import statements against
// configured ArchitectureBoundaryRule entries (@intent nomos_intent_traceability_spec.md #ArchitectureBoundaryRule).
func EvaluateArchitectureBoundaries(root string, files []string, rules []config.ArchitectureBoundaryRule) []string {
	if root == "" || len(rules) == 0 {
		return nil
	}
	candidates := files
	if len(candidates) == 0 {
		candidates = listBoundaryCandidateFiles(root)
	}
	var violations []string
	for _, entry := range candidates {
		relFile, fullPath, ok := resolveBoundaryTargetFile(root, entry)
		if !ok || isVerificationSuiteFile(filepath.Base(relFile)) {
			continue
		}
		matched := matchingBoundaryRules(relFile, rules)
		if len(matched) == 0 {
			continue
		}
		imports := extractPolyglotFileImports(fullPath)
		violations = append(violations, checkImportsAgainstRules(relFile, imports, matched)...)
	}
	sort.Strings(violations)
	return violations
}

func resolveBoundaryTargetFile(root, entry string) (string, string, bool) {
	full := entry
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, filepath.FromSlash(entry))
	}
	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		return "", "", false
	}
	rel, rErr := filepath.Rel(root, full)
	if rErr != nil || strings.HasPrefix(rel, "..") {
		return "", "", false
	}
	return filepath.ToSlash(rel), full, true
}

func listBoundaryCandidateFiles(root string) []string {
	var out []string
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if isSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isSupportedPolyglotSourceFile(p) {
			if rel, rErr := filepath.Rel(root, p); rErr == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if walkErr != nil {
		return out
	}
	return out
}

func matchingBoundaryRules(relFile string, rules []config.ArchitectureBoundaryRule) []config.ArchitectureBoundaryRule {
	var matched []config.ArchitectureBoundaryRule
	for _, r := range rules {
		if strings.TrimSpace(r.From) != "" && MatchTDDExcludeGlob(r.From, relFile) {
			matched = append(matched, r)
		}
	}
	return matched
}

func checkImportsAgainstRules(relFile string, imports []string, rules []config.ArchitectureBoundaryRule) []string {
	var violations []string
	for _, imp := range imports {
		candidates := buildImportMatchCandidates(relFile, imp)
		for _, r := range rules {
			if v := evaluateRuleForImport(relFile, imp, candidates, r); v != "" {
				violations = append(violations, v)
			}
		}
	}
	return violations
}

func evaluateRuleForImport(relFile, rawImp string, candidates []string, rule config.ArchitectureBoundaryRule) string {
	label := strings.TrimSpace(rule.Name)
	if label == "" {
		label = rule.From
	}
	for _, forbid := range rule.ForbidImports {
		if matchesAnyImportCandidate(forbid, candidates) {
			return fmt.Sprintf("%s imports forbidden package %q (rule %q forbids %q)", relFile, rawImp, label, forbid)
		}
	}
	if len(rule.AllowOnly) > 0 && !matchesAnyPatternList(rule.AllowOnly, candidates) {
		return fmt.Sprintf("%s imports disallowed package %q outside allow_only %v (rule %q)", relFile, rawImp, rule.AllowOnly, label)
	}
	return ""
}

func matchesAnyPatternList(patterns, candidates []string) bool {
	for _, pat := range patterns {
		if matchesAnyImportCandidate(pat, candidates) {
			return true
		}
	}
	return false
}

func matchesAnyImportCandidate(pattern string, candidates []string) bool {
	cleanPat := strings.TrimSpace(pattern)
	if cleanPat == "" {
		return false
	}
	for _, cand := range candidates {
		if cand == cleanPat || MatchTDDExcludeGlob(cleanPat, cand) {
			return true
		}
	}
	return false
}

func buildImportMatchCandidates(relFile, rawImp string) []string {
	seen := make(map[string]bool)
	add := func(s string) {
		trimmed := strings.TrimPrefix(strings.TrimSpace(s), "/")
		if trimmed != "" {
			seen[trimmed] = true
		}
	}
	add(rawImp)
	if strings.HasPrefix(rawImp, ".") {
		resolved := path.Clean(path.Join(path.Dir(relFile), rawImp))
		add(resolved)
	}
	slashImp := strings.ReplaceAll(strings.ReplaceAll(rawImp, "::", "/"), ".", "/")
	if !strings.HasPrefix(rawImp, ".") {
		add(slashImp)
	}
	parts := strings.Split(rawImp, "/")
	for i := 1; i < len(parts); i++ {
		add(strings.Join(parts[i:], "/"))
	}
	var list []string
	for k := range seen {
		list = append(list, k)
	}
	return list
}

func extractPolyglotFileImports(fullPath string) []string {
	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext == ".go" {
		return extractGoASTImports(fullPath)
	}
	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}
	return extractLexicalImportsByExt(ext, string(raw))
}

func extractGoASTImports(fullPath string) []string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	var imports []string
	for _, spec := range node.Imports {
		if spec.Path != nil {
			imports = append(imports, strings.Trim(spec.Path.Value, "\"`"))
		}
	}
	return imports
}

func extractLexicalImportsByExt(ext, src string) []string {
	switch ext {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".svelte":
		return collectRegexCaptures(src, esFromImportRe, esCallImportRe)
	case ".py":
		return collectRegexCaptures(src, pyFromImportRe, pyBareImportRe)
	case ".rs":
		return collectRegexCaptures(src, rustUseRe)
	case ".java", ".kt", ".kts", ".cs":
		return collectRegexCaptures(src, jvmDotImportRe)
	case ".rb":
		return collectRegexCaptures(src, rubyRequireRe)
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp":
		return collectRegexCaptures(src, cIncludeImportRe)
	default:
		return nil
	}
}

func collectRegexCaptures(src string, patterns ...*regexp.Regexp) []string {
	seen := make(map[string]bool)
	var out []string
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				out = append(out, m[1])
			}
		}
	}
	return out
}

func checkArchitectureBoundariesDowngrade(trunkCfg, wtCfg *config.EcosystemConfig) string {
	if trunkCfg == nil || wtCfg == nil {
		return ""
	}
	trunkRules := trunkCfg.Governance.QualityGates.ArchitectureBoundaries
	if len(trunkRules) == 0 {
		return ""
	}
	wtRules := wtCfg.Governance.QualityGates.ArchitectureBoundaries
	if len(wtRules) < len(trunkRules) {
		return fmt.Sprintf("worktree nomos.yaml illegally removes governance.quality_gates.architecture_boundaries rule(s) (%d -> %d)", len(trunkRules), len(wtRules))
	}
	for _, tr := range trunkRules {
		if !containsEquivalentBoundaryRule(wtRules, tr) {
			return fmt.Sprintf("worktree nomos.yaml illegally weakens or deletes architecture_boundaries rule for %q", tr.From)
		}
	}
	return ""
}

func containsEquivalentBoundaryRule(wtRules []config.ArchitectureBoundaryRule, target config.ArchitectureBoundaryRule) bool {
	targetSig := serializeBoundaryRule(target)
	for _, wr := range wtRules {
		if serializeBoundaryRule(wr) == targetSig {
			return true
		}
	}
	return false
}

func serializeBoundaryRule(r config.ArchitectureBoundaryRule) string {
	forbid := append([]string(nil), r.ForbidImports...)
	allow := append([]string(nil), r.AllowOnly...)
	sort.Strings(forbid)
	sort.Strings(allow)
	return fmt.Sprintf("%s|%s|%s", strings.TrimSpace(r.From), strings.Join(forbid, ","), strings.Join(allow, ","))
}

