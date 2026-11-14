// Package mtpo extracts Go and polyglot AST symbol read/write sets in sub-5ms.
package mtpo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	polyExportRe = regexp.MustCompile(`(?m)^\s*(?:export\s+(?:async\s+)?(?:function|class|interface|type|const)\s+|pub\s+(?:async\s+)?(?:fn|struct|enum|trait)\s+|def\s+)([A-Za-z_][A-Za-z0-9_]*)`)
	polyImportRe = regexp.MustCompile(`(?m)^\s*(?:import\s+.*?from\s+['"]([^'"]+)['"]|from\s+([A-Za-z0-9_.]+)\s+import|use\s+([A-Za-z0-9_:]+))`)
	polyCallRe   = regexp.MustCompile(`\b([A-Z][A-Za-z0-9_]{2,})\s*\(`)
)

// ExtractFileASTSets extracts exported declarations, referenced symbols, and package imports from fullPath.
func ExtractFileASTSets(fullPath, relFile string) (writeSyms, readSyms, imports []string) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, nil, nil
	}
	ext := strings.ToLower(filepath.Ext(fullPath))
	if ext == ".go" {
		return extractGoFileASTSets(fullPath, data)
	}
	return extractPolyglotFileASTSets(relFile, string(data))
}

func extractGoFileASTSets(fullPath string, src []byte) ([]string, []string, []string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fullPath, src, 0)
	if err != nil || node == nil {
		return nil, nil, nil
	}
	pkgName := node.Name.Name
	aliasPkg := make(map[string]string)
	importsSet := make(map[string]bool)
	for _, imp := range node.Imports {
		impPath := strings.Trim(imp.Path.Value, `"`)
		importsSet[impPath] = true
		localName := path.Base(impPath)
		if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
			localName = imp.Name.Name
		}
		aliasPkg[localName] = path.Base(impPath)
	}
	writeSet := collectGoExportedDecls(node, pkgName)
	readSet := collectGoReferencedSymbols(node, pkgName, aliasPkg, writeSet)
	return sortedKeys(writeSet), sortedKeys(readSet), sortedKeys(importsSet)
}

func collectGoExportedDecls(node *ast.File, pkgName string) map[string]bool {
	writeSet := make(map[string]bool)
	isTestFile := strings.HasSuffix(pkgName, "_test")
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !isTestFile && d.Name != nil && d.Name.IsExported() {
				writeSet[pkgName+"."+d.Name.Name] = true
			}
		case *ast.GenDecl:
			recordExportedGenDecl(d, pkgName, isTestFile, writeSet)
		}
	}
	return writeSet
}

func recordExportedGenDecl(d *ast.GenDecl, pkgName string, isTestFile bool, writeSet map[string]bool) {
	if isTestFile {
		return
	}
	for _, sp := range d.Specs {
		switch s := sp.(type) {
		case *ast.TypeSpec:
			if s.Name != nil && s.Name.IsExported() {
				writeSet[pkgName+"."+s.Name.Name] = true
			}
		case *ast.ValueSpec:
			for _, id := range s.Names {
				if id != nil && id.IsExported() {
					writeSet[pkgName+"."+id.Name] = true
				}
			}
		}
	}
}

func collectGoReferencedSymbols(node *ast.File, pkgName string, aliasPkg map[string]string, writeSet map[string]bool) map[string]bool {
	readSet := make(map[string]bool)
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && x.Sel != nil && x.Sel.IsExported() {
				targetPkg, known := aliasPkg[id.Name]
				if known {
					sym := targetPkg + "." + x.Sel.Name
					if !writeSet[sym] {
						readSet[sym] = true
					}
				}
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && id.IsExported() {
				sym := strings.TrimSuffix(pkgName, "_test") + "." + id.Name
				if !writeSet[sym] {
					readSet[sym] = true
				}
			}
		}
		return true
	})
	return readSet
}

func extractPolyglotFileASTSets(relFile, text string) ([]string, []string, []string) {
	modPrefix := strings.TrimSuffix(filepath.Base(relFile), filepath.Ext(relFile))
	writeSet := make(map[string]bool)
	readSet := make(map[string]bool)
	importSet := make(map[string]bool)
	for _, m := range polyExportRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 {
			writeSet[modPrefix+"."+m[1]] = true
		}
	}
	for _, m := range polyImportRe.FindAllStringSubmatch(text, -1) {
		for i := 1; i < len(m); i++ {
			if m[i] != "" {
				importSet[m[i]] = true
			}
		}
	}
	for _, m := range polyCallRe.FindAllStringSubmatch(text, -1) {
		if len(m) >= 2 && !writeSet[modPrefix+"."+m[1]] {
			readSet[m[1]] = true
		}
	}
	return sortedKeys(writeSet), sortedKeys(readSet), sortedKeys(importSet)
}

func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mergeSortedStringSets(existing, added []string) []string {
	m := make(map[string]bool, len(existing)+len(added))
	for _, s := range existing {
		if s != "" {
			m[s] = true
		}
	}
	for _, s := range added {
		if s != "" {
			m[s] = true
		}
	}
	return sortedKeys(m)
}
