// Package verify implements executable TC-* obligation binding and specification
// struct signature AST parity verification (Law 2, Law 4, and Law 7).
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	obligationVectorRe  = regexp.MustCompile(`TC-[A-Z0-9]+-[0-9]+\b`)
	underscoreVectorRe  = regexp.MustCompile(`TC_([A-Z0-9]+)_([0-9]+)\b`)
	goFencedBlockSpecRe = regexp.MustCompile("(?s)```go\\s*\\n(.*?)```")
)

// VerifyExecutableObligations checks that TC-* vectors declared in Markdown tables
// across specPaths appear in repository verification suites (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func VerifyExecutableObligations(root string, specPaths []string) []string {
	resolvedSpecs := resolveObligationSpecFiles(root, specPaths)
	if len(resolvedSpecs) == 0 {
		return nil
	}
	boundVectors := collectVerifiedVectorRefs(root)
	var issues []string
	for _, specFile := range resolvedSpecs {
		relSpec := toRelSlash(root, specFile)
		for _, vec := range extractSpecTableVectors(specFile) {
			if !boundVectors[vec] {
				issues = append(issues, fmt.Sprintf("%s -> %s", relSpec, vec))
			}
		}
	}
	return issues
}

// VerifySpecTypeSignatures checks Go and polyglot contract types in fenced specification code blocks
// against compiled Go AST and polyglot source declarations (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func VerifySpecTypeSignatures(root, targetDir string, specPaths []string) []string {
	resolvedSpecs := resolveObligationSpecFiles(root, specPaths)
	if len(resolvedSpecs) == 0 {
		return nil
	}
	var issues []string
	if targetDir != "" {
		compiledStructs := collectCompiledStructFields(root, targetDir)
		if len(compiledStructs) > 0 {
			for _, specFile := range resolvedSpecs {
				relSpec := toRelSlash(root, specFile)
				specStructs := extractSpecGoStructFields(specFile)
				issues = append(issues, diffSpecStructsAgainstAST(relSpec, specStructs, compiledStructs)...)
			}
		}
	}
	issues = append(issues, verifyPolyglotSpecSignatures(root, resolvedSpecs)...)
	return issues
}

func diffSpecStructsAgainstAST(relSpec string, specStructs, compiledStructs map[string]map[string]bool) []string {
	var names []string
	for name := range specStructs {
		names = append(names, name)
	}
	sort.Strings(names)

	var issues []string
	for _, typeName := range names {
		actualFields, exists := compiledStructs[typeName]
		if !exists {
			issues = append(issues, fmt.Sprintf("%s: missing compiled Go struct %s", relSpec, typeName))
			continue
		}
		issues = append(issues, findMissingStructFields(relSpec, typeName, specStructs[typeName], actualFields)...)
	}
	return issues
}

func findMissingStructFields(relSpec, typeName string, expected, actual map[string]bool) []string {
	var fields []string
	for field := range expected {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	var missing []string
	for _, field := range fields {
		if !actual[field] {
			missing = append(missing, fmt.Sprintf("%s: struct %s missing declared field %s", relSpec, typeName, field))
		}
	}
	return missing
}

func resolveObligationSpecFiles(root string, specPaths []string) []string {
	if specPaths != nil && len(specPaths) == 0 {
		return nil
	}
	roots := specPaths
	if len(roots) == 0 {
		roots = []string{filepath.Join("intent", "architecture", "specs")}
	}
	seen := make(map[string]bool)
	var files []string
	for _, entry := range roots {
		full := entry
		if !filepath.IsAbs(full) {
			full = filepath.Join(root, entry)
		}
		files = collectMarkdownSpecs(full, seen, files)
	}
	sort.Strings(files)
	return files
}

func collectMarkdownSpecs(target string, seen map[string]bool, acc []string) []string {
	fi, err := os.Stat(target)
	if err != nil {
		return acc
	}
	if !fi.IsDir() {
		if strings.HasSuffix(target, ".md") && !seen[target] {
			seen[target] = true
			acc = append(acc, target)
		}
		return acc
	}
	walkErr := filepath.Walk(target, func(path string, info os.FileInfo, wErr error) error {
		if wErr == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, "_spec.md") && !seen[path] {
			seen[path] = true
			acc = append(acc, path)
		}
		return nil
	})
	if walkErr != nil {
		return acc
	}
	return acc
}

func extractSpecTableVectors(specFile string) []string {
	raw, err := os.ReadFile(specFile)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var ordered []string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		for _, tok := range obligationVectorRe.FindAllString(trimmed, -1) {
			if !seen[tok] {
				seen[tok] = true
				ordered = append(ordered, tok)
			}
		}
	}
	return ordered
}

func collectVerifiedVectorRefs(root string) map[string]bool {
	refs := make(map[string]bool)
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if impact.IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if impact.IsVerificationSuiteFile(info.Name()) {
			scanFileVectorRefs(path, refs)
		}
		return nil
	})
	if walkErr != nil {
		return refs
	}
	return refs
}

func scanFileVectorRefs(path string, refs map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text := string(raw)
	for _, tok := range obligationVectorRe.FindAllString(text, -1) {
		refs[tok] = true
	}
	for _, m := range underscoreVectorRe.FindAllStringSubmatch(text, -1) {
		if len(m) == 3 {
			refs[fmt.Sprintf("TC-%s-%s", m[1], m[2])] = true
		}
	}
}

func collectCompiledStructFields(root, targetDir string) map[string]map[string]bool {
	structs := make(map[string]map[string]bool)
	fset := token.NewFileSet()
	scanDir := targetDir
	subDir := filepath.Join(root, "substrate")
	if fi, err := os.Stat(subDir); err == nil && fi.IsDir() {
		scanDir = subDir
	}
	walkErr := filepath.Walk(scanDir, func(path string, info os.FileInfo, err error) error {
		if isScannableGoSource(path, info, err) {
			node, pErr := parser.ParseFile(fset, path, nil, 0)
			if pErr == nil {
				recordASTStructFields(node, structs)
			}
		}
		return nil
	})
	if walkErr != nil {
		return structs
	}
	return structs
}

func extractSpecGoStructFields(specFile string) map[string]map[string]bool {
	raw, err := os.ReadFile(specFile)
	if err != nil {
		return nil
	}
	structs := make(map[string]map[string]bool)
	fset := token.NewFileSet()
	for _, match := range goFencedBlockSpecRe.FindAllStringSubmatch(string(raw), -1) {
		if len(match) < 2 {
			continue
		}
		src := strings.TrimSpace(match[1])
		if !strings.HasPrefix(src, "package ") {
			src = "package specstub\n" + src
		}
		if node, pErr := parser.ParseFile(fset, "", src, 0); pErr == nil {
			recordASTStructFields(node, structs)
		}
	}
	return structs
}

func recordASTStructFields(node *ast.File, out map[string]map[string]bool) {
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || !ts.Name.IsExported() {
				continue
			}
			st, isStruct := ts.Type.(*ast.StructType)
			if !isStruct || st.Fields == nil {
				continue
			}
			recordSingleStructType(ts.Name.Name, st, out)
		}
	}
}

func recordSingleStructType(typeName string, st *ast.StructType, out map[string]map[string]bool) {
	fields, exists := out[typeName]
	if !exists {
		fields = make(map[string]bool)
		out[typeName] = fields
	}
	for _, f := range st.Fields.List {
		for _, ident := range f.Names {
			if ident != nil && ident.IsExported() {
				fields[ident.Name] = true
			}
		}
	}
}

func toRelSlash(root, fullPath string) string {
	if rel, err := filepath.Rel(root, fullPath); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(fullPath)
}
