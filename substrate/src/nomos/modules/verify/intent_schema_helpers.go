// Package verify implements Intent AST traceability resolution (Law 4).
package verify

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

var (
	intentGoTypeAnchorRe = regexp.MustCompile(`@intent[^\n]*?(#[A-Za-z0-9_]+)`)
	hashTypeTokenRe      = regexp.MustCompile(`#[A-Za-z0-9_]+`)
	specFileTokenRe      = regexp.MustCompile(`\b([A-Za-z0-9_-]+\.md)\b`)
)

func verifyIntentASTAnchors(ctx *VerifyContext, root, targetDir string) []string {
	definedTypes := collectDefinedGoTypeAnchors(root, targetDir)
	collectPolyglotDefinedTypeAnchors(root, definedTypes)
	definedSpecs := collectIntentSpecBasenames(ctx, root)

	fset := token.NewFileSet()
	var unresolved []string
	WalkTargetFiles(ctx, targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || isVerificationSuiteFile(info.Name()) {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			unresolved = append(unresolved, scanFileIntentASTAnchors(fset, targetDir, path, definedTypes, definedSpecs)...)
		} else if isSupportedPolyglotSourceFile(path) {
			unresolved = append(unresolved, scanPolyglotFileIntentAnchors(root, path, definedTypes, definedSpecs)...)
		}
		return nil
	})
	return unresolved
}

func scanFileIntentASTAnchors(fset *token.FileSet, targetDir, path string, definedTypes, definedSpecs map[string]bool) []string {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	rel, rErr := filepath.Rel(targetDir, path)
	if rErr != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	var unresolved []string
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}
		unresolved = append(unresolved, checkDocIntentTokens(rel, fn.Name.Name, fn.Doc.Text(), definedTypes, definedSpecs)...)
	}
	return unresolved
}

func checkDocIntentTokens(rel, fnName, docText string, definedTypes, definedSpecs map[string]bool) []string {
	var unresolved []string
	for _, line := range strings.Split(docText, "\n") {
		idx := strings.Index(line, "@intent")
		if idx < 0 {
			continue
		}
		segment := line[idx:]
		for _, tok := range hashTypeTokenRe.FindAllString(segment, -1) {
			if !definedTypes[tok] {
				unresolved = append(unresolved, fmt.Sprintf("%s (%s -> %s)", rel, fnName, tok))
			}
		}
		if len(definedSpecs) > 0 {
			for _, m := range specFileTokenRe.FindAllStringSubmatch(segment, -1) {
				if len(m) > 1 && !definedSpecs[m[1]] {
					unresolved = append(unresolved, fmt.Sprintf("%s (%s -> %s)", rel, fnName, m[1]))
				}
			}
		}
	}
	return unresolved
}

func collectDefinedGoTypeAnchors(root, targetDir string) map[string]bool {
	defined := make(map[string]bool)
	fset := token.NewFileSet()
	scanDirs := []string{targetDir}
	subDir := filepath.Join(root, "substrate")
	if fi, err := os.Stat(subDir); err == nil && fi.IsDir() {
		scanDirs = []string{subDir}
	}

	for _, dir := range scanDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if isScannableGoSource(path, info, err) {
				extractTypeNamesFromGoFile(fset, path, defined)
			}
			return nil
		})
	}
	return defined
}

func isScannableGoSource(path string, info os.FileInfo, err error) bool {
	if err != nil || info == nil || info.IsDir() {
		return false
	}
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func extractTypeNamesFromGoFile(fset *token.FileSet, path string, defined map[string]bool) {
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return
	}
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name != nil {
				defined["#"+ts.Name.Name] = true
			}
		}
	}
}

func collectIntentSpecBasenames(ctx *VerifyContext, root string) map[string]bool {
	cfg := resolveVerifyConfig(ctx)
	roots := []string{"intent"}
	if cfg != nil && len(cfg.Governance.QualityGates.IntentSourceRoots) > 0 {
		roots = cfg.Governance.QualityGates.IntentSourceRoots
	}
	specs := make(map[string]bool)
	for _, r := range roots {
		intentDir := filepath.Join(root, filepath.FromSlash(r))
		if _, err := os.Stat(intentDir); os.IsNotExist(err) && ctx != nil && ctx.RepoRoot != "" {
			intentDir = filepath.Join(ctx.RepoRoot, filepath.FromSlash(r))
		}
		if fi, err := os.Stat(intentDir); err == nil && fi.IsDir() {
			if walkErr := filepath.Walk(intentDir, func(path string, info os.FileInfo, wErr error) error {
				if wErr == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
					specs[filepath.Base(path)] = true
				}
				return nil
			}); walkErr != nil {
				continue
			}
		}
	}
	if len(specs) == 0 {
		return nil
	}
	return specs
}
