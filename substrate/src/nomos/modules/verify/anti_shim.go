// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"context"
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

// AntiShimDTO defines the AST shape extracted by the Go substrate (#AntiShimDTO).
type AntiShimDTO struct {
	Name                 string `json:"Name"`
	BodyStatementCount   int    `json:"BodyStatementCount"`
	IsPanicShim          bool   `json:"IsPanicShim"`
	IsTrivialPassShim    bool   `json:"IsTrivialPassShim,omitempty"`
	ForbiddenLiteralShim string `json:"ForbiddenLiteralShim"`
}

// ValidateAntiShim enforces zero-trust AST anti-shim bounds (Law 2).
func ValidateAntiShim(dto AntiShimDTO) error {
	if dto.BodyStatementCount <= 0 {
		return types.NewNomosError(types.ErrCodeInvariant, "function %s is an empty stub (0 statements)", dto.Name)
	}
	if dto.IsPanicShim {
		return types.NewNomosError(types.ErrCodeInvariant, "function %s contains a panic placeholder shim", dto.Name)
	}
	if dto.IsTrivialPassShim {
		return types.NewNomosError(types.ErrCodeInvariant, "function %s is a trivial constant return shim (ERR_TRIVIAL_PASS_SHIM)", dto.Name)
	}
	if dto.ForbiddenLiteralShim != "" {
		return types.NewNomosError(types.ErrCodeInvariant, "function %s contains forbidden literal %q", dto.Name, dto.ForbiddenLiteralShim)
	}
	return nil
}

// ScanFunctionAntiShim extracts anti-shim properties from a Go AST function declaration.
func ScanFunctionAntiShim(fn *ast.FuncDecl) AntiShimDTO {
	if fn == nil || fn.Body == nil {
		return AntiShimDTO{Name: "<nil>", BodyStatementCount: 0}
	}

	dto := AntiShimDTO{
		Name:               fn.Name.Name,
		BodyStatementCount: len(fn.Body.List),
	}

	if len(fn.Body.List) == 1 {
		dto.IsPanicShim = checkSinglePanicShim(fn.Body.List[0])
		dto.IsTrivialPassShim = checkTrivialPassShim(fn)
	}
	return dto
}

func checkTrivialPassShim(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Body == nil || len(fn.Body.List) != 1 {
		return false
	}
	stmt := fn.Body.List[0]
	ret, ok := stmt.(*ast.ReturnStmt)
	if !ok || len(ret.Results) == 0 {
		return false
	}
	return isTrivialGateReturn(ret)
}

func isTrivialGateReturn(ret *ast.ReturnStmt) bool {
	if ret == nil || len(ret.Results) == 0 {
		return false
	}
	comp := extractGateCompositeLit(ret.Results[0])
	if comp == nil {
		return false
	}
	for _, elt := range comp.Elts {
		if isPassedTrueKV(elt) {
			return true
		}
	}
	return false
}

func extractGateCompositeLit(expr ast.Expr) *ast.CompositeLit {
	var comp *ast.CompositeLit
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		if c, okCast := unary.X.(*ast.CompositeLit); okCast {
			comp = c
		}
	} else if c, okCast := expr.(*ast.CompositeLit); okCast {
		comp = c
	}
	if comp == nil {
		return nil
	}
	if ident, ok := comp.Type.(*ast.Ident); ok && ident.Name == "GateResult" {
		return comp
	}
	if sel, ok := comp.Type.(*ast.SelectorExpr); ok && sel.Sel != nil && sel.Sel.Name == "GateResult" {
		return comp
	}
	return nil
}

func isPassedTrueKV(elt ast.Expr) bool {
	kv, ok := elt.(*ast.KeyValueExpr)
	if !ok {
		return false
	}
	k, kOk := kv.Key.(*ast.Ident)
	v, vOk := kv.Value.(*ast.Ident)
	return kOk && vOk && k.Name == "Passed" && v.Name == "true"
}

func checkSinglePanicShim(stmt ast.Stmt) bool {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "panic" {
		return false
	}
	if len(call.Args) == 0 {
		return true
	}
	if argIdent, isIdent := call.Args[0].(*ast.Ident); isIdent && argIdent.Name == "nil" {
		return true
	}
	lit, isLit := call.Args[0].(*ast.BasicLit)
	if !isLit {
		return false
	}
	val := strings.TrimSpace(strings.Trim(strings.ToLower(lit.Value), "\"`"))
	return val == "" ||
		strings.Contains(val, "todo") ||
		strings.Contains(val, "not implemented") ||
		strings.Contains(val, "unimplemented") ||
		strings.Contains(val, "stub")
}

// AntiShimGate verifies that source code does not contain empty stubs or panic shims.
type AntiShimGate struct{}

// Name returns the canonical display name of the anti-shim gate.
func (g *AntiShimGate) Name() string {
	return "Anti-Shim AST Invariant"
}

// Evaluate walks workspace Go and polyglot source files and validates functions against anti-shim bounds.
func (g *AntiShimGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	forbiddenShims := vscope.ResolveVerifyConfig(ctx).Governance.QualityGates.ForbiddenShims

	var violations []string

	err := vscope.WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() || !isPolyglotTargetFile(path) {
			return nil
		}
		relPath, rErr := filepath.Rel(root, path)
		if rErr != nil {
			relPath = path
		}
		var fileViolations []string
		fileViolations = append(fileViolations, scanCustomForbiddenShims(path, relPath, forbiddenShims)...)
		if !strings.HasSuffix(path, ".go") {
			fileViolations = append(fileViolations, scanPolyglotAntiShim(path, relPath)...)
		} else {
			fileViolations = append(fileViolations, scanGoFileAntiShimCached(ctx, path, relPath)...)
		}
		violations = append(violations, filterNetNewLineViolations(ctx, root, path, fileViolations, func(baseBytes []byte) []string {
			return scanTempBaselineViolations(path, baseBytes, func(tmpPath string) []string {
				out := scanCustomForbiddenShims(tmpPath, relPath, forbiddenShims)
				if !strings.HasSuffix(path, ".go") {
					return append(out, scanPolyglotAntiShim(tmpPath, relPath)...)
				}
				return append(out, scanGoFileAntiShim(token.NewFileSet(), tmpPath, relPath)...)
			})
		})...)
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "failed walking scope for anti-shim: %v", err)
	}

	if len(violations) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("detected %d anti-shim violation(s): %s", len(violations), strings.Join(violations, "; ")),
			Remedy:   "implement real algorithmic logic; eliminate panic shims, empty functions, and forbidden shim markers",
		}, nil
	}

	return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "zero anti-shim violations detected"}, nil
}

func scanGoFileAntiShimCached(ctx *vscope.VerifyContext, path, relPath string) []string {
	_, node, parseErr := vscope.MemoizedParseGoFile(ctx, path, 0)
	if parseErr != nil || node == nil {
		return nil
	}
	return collectGoFileAntiShimViolations(node, path, relPath)
}

func scanGoFileAntiShim(fset *token.FileSet, path, relPath string) []string {
	node, parseErr := parser.ParseFile(fset, path, nil, 0)
	if parseErr != nil {
		return nil
	}
	return collectGoFileAntiShimViolations(node, path, relPath)
}

func collectGoFileAntiShimViolations(node *ast.File, path, relPath string) []string {
	isGoTest := strings.HasSuffix(path, "_test.go")
	var violations []string
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		dto := ScanFunctionAntiShim(fn)
		if valErr := ValidateAntiShim(dto); valErr != nil {
			violations = append(violations, fmt.Sprintf("%s:%s", relPath, valErr.Error()))
			continue
		}
		if isGoTest && isUnconditionalGoTestSkip(fn) {
			violations = append(violations, fmt.Sprintf("%s: test_skip_reward_hack (%s)", relPath, fn.Name.Name))
		}
	}
	return violations
}

func isUnconditionalGoTestSkip(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Name == nil || !strings.HasPrefix(fn.Name.Name, "Test") || fn.Body == nil || len(fn.Body.List) == 0 {
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
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return false
	}
	switch sel.Sel.Name {
	case "Skip", "SkipNow", "Skipf":
		return true
	default:
		return false
	}
}

func scanCustomForbiddenShims(path, relPath string, markers []string) []string {
	if len(markers) == 0 {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for idx, line := range strings.Split(string(content), "\n") {
		for _, marker := range markers {
			clean := strings.TrimSpace(marker)
			if clean != "" && strings.Contains(line, clean) {
				out = append(out, fmt.Sprintf("%s:%d contains forbidden shim marker %q", relPath, idx+1, clean))
			}
		}
	}
	return out
}

func scanPolyglotAntiShim(path, relPath string) []string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	engine, err := treesitter.GetDefaultEngine()
	if err != nil {
		return nil
	}
	rep, err := engine.ParseAndEvaluate(context.Background(), path, content)
	if err != nil || rep == nil {
		return nil
	}
	var violations []string
	for _, fnName := range rep.EmptyBlocks {
		if valErr := ValidateAntiShim(AntiShimDTO{Name: fnName, BodyStatementCount: 0}); valErr != nil {
			violations = append(violations, fmt.Sprintf("%s:%s", relPath, valErr.Error()))
		}
	}
	for _, skip := range rep.SkippedTests {
		violations = append(violations, fmt.Sprintf("%s: test_skip_reward_hack (%s)", relPath, skip))
	}
	return violations
}
