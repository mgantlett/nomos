// Package verify exports static AST parity sensors consumed by zero-init brownfield X-Ray scans.
package verify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

// DefaultXrayMagicStringThreshold matches MagicStringDuplicationGate's statutory ceiling (Cell 5.1, Law 5).
const DefaultXrayMagicStringThreshold = 10

// CountFileUncheckedErrors counts swallowed or discarded error results in a single non-test source file (@intent tree_sitter_ast_gate_spec.md #XrayReport).
func CountFileUncheckedErrors(relSlash string, data []byte) int {
	if len(data) == 0 {
		return 0
	}
	if strings.HasSuffix(relSlash, astdebt.ExtGo) {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, relSlash, data, 0)
		if err != nil || node == nil {
			return 0
		}
		var violations []string
		checkUncheckedErrorsInFile(node, fset, relSlash, &violations)
		return len(violations)
	}
	ext := strings.ToLower(filepath.Ext(relSlash))
	return len(astdebt.CheckPolyglotUncheckedErrors(ext, relSlash, data))
}

// CountFileSecretAndPathLeaks counts hardcoded secret tokens and workstation path literals in a single source file (@intent tree_sitter_ast_gate_spec.md #XrayReport).
func CountFileSecretAndPathLeaks(relSlash string, data []byte) int {
	if len(data) == 0 {
		return 0
	}
	leaks := 0
	if !isIgnoredSecretFile(relSlash) && impact.MatchSecretTokenLabel(string(data)) != "" {
		leaks++
	}
	if !strings.HasSuffix(relSlash, "gates_governance.go") {
		leaks += countHostPathLiteralsInBytes(relSlash, data, defaultForbiddenLiterals())
	}
	return leaks
}

func countHostPathLiteralsInBytes(relSlash string, data []byte, patterns []string) int {
	if len(patterns) == 0 {
		return 0
	}
	if strings.HasSuffix(relSlash, astdebt.ExtGo) {
		return countGoStringPathLiterals(relSlash, data, patterns)
	}
	return countPolyglotLinePathLiterals(string(data), patterns)
}

func countGoStringPathLiterals(relSlash string, data []byte, patterns []string) int {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, relSlash, data, 0)
	if err != nil || node == nil {
		return 0
	}
	hits := 0
	ast.Inspect(node, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val := strings.Trim(lit.Value, "`\"")
		for _, p := range patterns {
			if strings.Contains(val, p) {
				hits++
			}
		}
		return true
	})
	return hits
}

func countPolyglotLinePathLiterals(text string, patterns []string) int {
	hits := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(line, p) {
				hits++
			}
		}
	}
	return hits
}

// CountCorpusMagicStringGroups counts package-scoped repeated string literal clusters exceeding maxDup (@intent tree_sitter_ast_gate_spec.md #XrayReport).
func CountCorpusMagicStringGroups(sources map[string][]byte, maxDup int) int {
	if len(sources) == 0 {
		return 0
	}
	if maxDup <= 0 {
		maxDup = DefaultXrayMagicStringThreshold
	}
	paths := make([]string, 0, len(sources))
	for rel := range sources {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	byDir := make(map[string]map[string]map[string]int)
	fset := token.NewFileSet()
	for _, rel := range paths {
		dirKey := filepath.ToSlash(filepath.Dir(rel))
		if byDir[dirKey] == nil {
			byDir[dirKey] = make(map[string]map[string]int)
		}
		data := sources[rel]
		if strings.HasSuffix(rel, astdebt.ExtGo) {
			collectGoStringLiteralsFromSrc(fset, rel, data, byDir[dirKey])
			continue
		}
		collectPolyglotStringLiteralsFromBytes(rel, data, byDir[dirKey])
	}
	return countExceedingStringClusters(byDir, maxDup)
}

func countExceedingStringClusters(byDir map[string]map[string]map[string]int, maxDup int) int {
	groups := 0
	for _, pkgMaps := range byDir {
		for _, pkgMap := range pkgMaps {
			for _, cnt := range pkgMap {
				if cnt > maxDup {
					groups++
				}
			}
		}
	}
	return groups
}

func evaluateBrownfieldArityRatchet(ctx *vscope.VerifyContext, root string, funcs []astdebt.FunctionMetric, thresh astdebt.Thresholds) []string {
	if !vscope.IsBrownfieldRatchetActive(ctx) || !vscope.IsWorktreeContext(ctx, root) {
		return nil
	}
	seen := make(map[string]bool)
	var files []string
	for _, fn := range funcs {
		if fn.File != "" && !seen[fn.File] {
			seen[fn.File] = true
			files = append(files, fn.File)
		}
	}
	sort.Strings(files)
	var violations []string
	for _, rel := range files {
		violations = append(violations, evaluateSingleFileArityRatchet(ctx, root, rel, thresh.MaxFuncParams)...)
	}
	return violations
}

func evaluateSingleFileArityRatchet(ctx *vscope.VerifyContext, root, rel string, maxParams int) []string {
	if maxParams <= 0 {
		maxParams = astdebt.MaxFunctionParams
	}
	absPath := filepath.Join(root, rel)
	src, err := os.ReadFile(absPath)
	if err != nil || len(src) == 0 {
		return nil
	}
	allowed := maxParams
	if baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath); ok {
		baseMaxP, _, _, baseDeepN := astdebt.SummarizeFileArityAndNesting(rel, baseBytes, maxParams, astdebt.MaxFunctionNesting)
		_ = baseDeepN
		allowed = maxInt(maxParams, baseMaxP)
	}
	return astdebt.EvaluateParameterArityViolations(rel, src, allowed)
}
