// Package verify implements test complexity ratcheting and Go local-symbol TDD coupling gates.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const maxTestComplexity = 25

func evaluateTestComplexityViolations(ctx *vscope.VerifyContext, root string) []string {
	var testFuncs []astdebt.FunctionMetric
	fset := token.NewFileSet()

	walkErr := vscope.WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || vscope.IsExcludedPolyglotDir(filepath.ToSlash(path)) {
			return nil
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		if !isPolyglotTestFilePath(rel) {
			return nil
		}
		if strings.HasSuffix(path, astdebt.ExtGo) {
			if node, pErr := parser.ParseFile(fset, path, nil, 0); pErr == nil {
				collectFunctionMetrics(node, fset, rel, &testFuncs)
			}
			return nil
		}
		collectPolyglotComplexity(path, rel, &testFuncs)
		return nil
	})
	if walkErr != nil {
		return nil
	}

	breaches := filterRatchetedTestComplexity(ctx, root, testFuncs)
	out := make([]string, 0, len(breaches))
	for _, fn := range breaches {
		out = append(out, fmt.Sprintf("%s:%d test function %s cyclomatic complexity %d exceeds max %d (decompose into modular table-driven subtests)",
			filepath.ToSlash(fn.File), fn.Line, fn.Name, fn.Complexity, maxTestComplexity))
	}
	return out
}

func filterRatchetedTestComplexity(ctx *vscope.VerifyContext, root string, funcs []astdebt.FunctionMetric) []astdebt.FunctionMetric {
	ratchet := vscope.IsBrownfieldRatchetActive(ctx)
	baseCache := make(map[string]map[string]astdebt.FunctionMetric)
	var out []astdebt.FunctionMetric
	for _, fn := range funcs {
		if fn.Complexity <= maxTestComplexity {
			continue
		}
		if !ratchet {
			out = append(out, fn)
			continue
		}
		baseMap, loaded := baseCache[fn.File]
		if !loaded {
			absPath := filepath.Join(root, fn.File)
			if baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath); ok {
				baseMap = parseBaselineFunctions(absPath, fn.File, baseBytes)
			}
			baseCache[fn.File] = baseMap
		}
		if baseMap != nil && fn.Complexity <= maxInt(maxTestComplexity, baseMap[fn.Name].Complexity) {
			continue
		}
		out = append(out, fn)
	}
	return out
}

// hasCoupledGoPackageTests verifies that *_test.go files in dir reference at least one
// local package symbol when production .go files declare testable functions or types.
func hasCoupledGoPackageTests(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	fset := token.NewFileSet()
	declared := make(map[string]bool)
	var testFiles []*ast.File

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), astdebt.ExtGo) {
			continue
		}
		parsed, pErr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if pErr != nil {
			// Defer syntax error reporting to GoToolchainGate rather than double-flagging TDD.
			return true
		}
		if strings.HasSuffix(e.Name(), astdebt.SuffixGoTest) {
			testFiles = append(testFiles, parsed)
			continue
		}
		collectDeclaredGoSymbols(parsed, declared)
	}
	if len(testFiles) == 0 {
		return false
	}
	if len(declared) == 0 {
		return true
	}
	return anyGoTestReferencesDeclared(testFiles, declared)
}

func collectDeclaredGoSymbols(node *ast.File, declared map[string]bool) {
	for _, d := range node.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			if decl.Name != nil && decl.Name.Name != "main" && decl.Name.Name != "init" && decl.Name.Name != "_" {
				declared[decl.Name.Name] = true
			}
		case *ast.GenDecl:
			collectGenDeclSymbols(decl, declared)
		}
	}
}

func collectGenDeclSymbols(decl *ast.GenDecl, declared map[string]bool) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name != nil && s.Name.Name != "_" {
				declared[s.Name.Name] = true
			}
		case *ast.ValueSpec:
			for _, n := range s.Names {
				if n != nil && n.Name != "_" {
					declared[n.Name] = true
				}
			}
		}
	}
}

func anyGoTestReferencesDeclared(testFiles []*ast.File, declared map[string]bool) bool {
	matched := false
	for _, tf := range testFiles {
		for _, decl := range tf.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				if matched {
					return false
				}
				if id, ok := n.(*ast.Ident); ok && declared[id.Name] {
					matched = true
					return false
				}
				return true
			})
			if matched {
				return true
			}
		}
	}
	return false
}
