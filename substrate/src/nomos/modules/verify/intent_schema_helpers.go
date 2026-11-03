// Package verify implements Intent frontmatter validation and AST traceability resolution (Law 4).
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

	"gopkg.in/yaml.v3"
)

var (
	intentGoTypeAnchorRe = regexp.MustCompile(`@intent[^\n]*?(#[A-Za-z0-9_]+)`)
	hashTypeTokenRe      = regexp.MustCompile(`#[A-Za-z0-9_]+`)
	specFileTokenRe      = regexp.MustCompile(`\b([A-Za-z0-9_-]+\.md)\b`)
)

type intentFrontmatter struct {
	Title  string `yaml:"title"`
	Plane  string `yaml:"plane"`
	Status string `yaml:"status"`
}

var (
	validIntentPlanes = map[string]bool{
		"intent":    true,
		"substrate": true,
	}
	validIntentStatuses = map[string]bool{
		"active":     true,
		"superseded": true,
		"deprecated": true,
		"archived":   true,
	}
)

func validateIntentDocuments(intentDir string, requireFrontmatter bool) (int, []string) {
	var docCount int
	var issues []string

	_ = filepath.Walk(intentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		docCount++
		rel, rErr := filepath.Rel(intentDir, path)
		if rErr != nil {
			rel = filepath.Base(path)
		}
		rel = filepath.ToSlash(rel)
		if docErr := auditSingleIntentMarkdown(path, requireFrontmatter); docErr != "" {
			issues = append(issues, fmt.Sprintf("%s: %s", rel, docErr))
		}
		return nil
	})
	return docCount, issues
}

func auditSingleIntentMarkdown(path string, requireFrontmatter bool) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("unreadable document (%v)", err)
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		if requireFrontmatter {
			return "missing leading YAML frontmatter delimiter (---)"
		}
		if hasUnclosedCodeFence(normalized) {
			return "unclosed Markdown fenced code block detected"
		}
		return ""
	}
	endIdx := strings.Index(normalized[4:], "\n---\n")
	if endIdx < 0 {
		return "missing closing YAML frontmatter delimiter (---)"
	}
	fmRaw := normalized[4 : 4+endIdx]
	body := normalized[4+endIdx+5:]

	var fm intentFrontmatter
	if yErr := yaml.Unmarshal([]byte(fmRaw), &fm); yErr != nil {
		return fmt.Sprintf("malformed YAML frontmatter (%v)", yErr)
	}
	if strings.TrimSpace(fm.Title) == "" {
		return "YAML frontmatter missing required non-empty 'title'"
	}
	if fm.Plane != "" || requireFrontmatter {
		plane := strings.ToLower(strings.TrimSpace(fm.Plane))
		if !validIntentPlanes[plane] {
			return fmt.Sprintf("invalid frontmatter plane %q (expected intent or substrate)", fm.Plane)
		}
	}
	if fm.Status != "" || requireFrontmatter {
		status := strings.ToLower(strings.TrimSpace(fm.Status))
		if !validIntentStatuses[status] {
			return fmt.Sprintf("invalid frontmatter status %q (expected active, superseded, deprecated, or archived)", fm.Status)
		}
	}
	if hasUnclosedCodeFence(body) {
		return "unclosed Markdown fenced code block detected"
	}
	return ""
}

func hasUnclosedCodeFence(body string) bool {
	inFence := false
	fenceLen := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		ticks := countLeadingBackticks(trimmed)
		if ticks < 3 {
			continue
		}
		rest := strings.TrimSpace(trimmed[ticks:])
		if !inFence {
			inFence = true
			fenceLen = ticks
		} else if ticks >= fenceLen && rest == "" {
			inFence = false
			fenceLen = 0
		}
	}
	return inFence
}

func countLeadingBackticks(s string) int {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	return n
}

func verifyIntentASTAnchors(ctx *VerifyContext, root, targetDir string) []string {
	definedTypes := collectDefinedGoTypeAnchors(root, targetDir)
	definedSpecs := collectIntentSpecBasenames(ctx, root)

	fset := token.NewFileSet()
	var unresolved []string
	WalkTargetFiles(ctx, targetDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			unresolved = append(unresolved, scanFileIntentASTAnchors(fset, targetDir, path, definedTypes, definedSpecs)...)
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
	intentDir := filepath.Join(root, "intent")
	if _, err := os.Stat(intentDir); os.IsNotExist(err) && ctx != nil && ctx.RepoRoot != "" {
		intentDir = filepath.Join(ctx.RepoRoot, "intent")
	}
	if fi, err := os.Stat(intentDir); err != nil || !fi.IsDir() {
		return nil
	}
	specs := make(map[string]bool)
	_ = filepath.Walk(intentDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
			specs[filepath.Base(path)] = true
		}
		return nil
	})
	return specs
}
