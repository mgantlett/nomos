// Package xray implements Go AST skip and anti-shim helpers for zero-init scanning.
package xray

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

func countExplicitPolyglotStubs(lang string, data []byte) int {
	switch lang {
	case "go":
		return countMatchingGoFuncs(data, func(fn *ast.FuncDecl) bool {
			dto := verify.ScanFunctionAntiShim(fn)
			return dto.IsPanicShim || dto.IsTrivialPassShim
		})
	case "python":
		return len(explicitPyStubRe.FindAll(data, -1))
	case "rust":
		return len(explicitRustStubRe.FindAll(data, -1))
	case "typescript", "svelte":
		return len(explicitTSStubRe.FindAll(data, -1))
	default:
		return 0
	}
}

func countSkippedTests(lang string, data []byte) int {
	if lang != "go" {
		return len(treesitter.DetectSkippedTests(lang, data))
	}
	return countMatchingGoFuncs(data, isUnconditionalGoSkipFunc)
}

func countMatchingGoFuncs(data []byte, match func(*ast.FuncDecl) bool) int {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "scan.go", data, 0)
	if err != nil {
		return 0
	}
	count := 0
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Body != nil && match(fn) {
			count++
		}
	}
	return count
}

func isUnconditionalGoSkipFunc(fn *ast.FuncDecl) bool {
	if fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") || len(fn.Body.List) == 0 {
		return false
	}
	exprStmt, ok := fn.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel || sel.Sel == nil {
		return false
	}
	return sel.Sel.Name == "Skip" || sel.Sel.Name == "Skipf" || sel.Sel.Name == "SkipNow"
}
