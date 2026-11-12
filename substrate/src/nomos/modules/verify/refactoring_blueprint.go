// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// MaxFunctionParams defines the maximum parameter arity before requiring a cohesive struct (#Thresholds).
const MaxFunctionParams = 8

// PrescribeFunctionSeams analyzes a violating function and returns deterministic AST extraction cut-lines.
// @intent: #FunctionMetric
func PrescribeFunctionSeams(filePath string, src []byte, funcName string, funcLine int) []string {
	if len(src) == 0 {
		return nil
	}
	if strings.HasSuffix(filePath, ".go") {
		return prescribeGoFunctionSeams(filePath, src, funcName)
	}
	return prescribePolyglotFunctionSeams(filePath, src, funcLine)
}

func prescribeGoFunctionSeams(filePath string, src []byte, funcName string) []string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, 0)
	if err != nil {
		return nil
	}
	fn := findGoFuncDecl(node, funcName)
	if fn == nil || fn.Body == nil {
		return nil
	}

	var seams []string
	if params := countGoFuncParams(fn); params > MaxFunctionParams {
		seams = append(seams, fmt.Sprintf("[SEAM (parameter arity %d > %d): encapsulate parameters into a cohesive domain struct]", params, MaxFunctionParams))
	}
	seams = append(seams, extractGoStatementSeams(fn, fset)...)
	if boolOps := countGoLogicalOps(fn.Body); boolOps >= 5 {
		seams = append(seams, fmt.Sprintf("[SEAM: %d compound boolean branches (&&/||) -> extract predicate helper or lookup map]", boolOps))
	}
	return seams
}

func findGoFuncDecl(node *ast.File, funcName string) *ast.FuncDecl {
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			return fn
		}
	}
	return nil
}

func countGoFuncParams(fn *ast.FuncDecl) int {
	if fn == nil || fn.Type == nil || fn.Type.Params == nil {
		return 0
	}
	count := 0
	for _, field := range fn.Type.Params.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	return count
}

func extractGoStatementSeams(fn *ast.FuncDecl, fset *token.FileSet) []string {
	var seams []string
	for _, stmt := range fn.Body.List {
		kind, comp := classifyGoBlockStmt(stmt)
		if kind == "" {
			continue
		}
		start := fset.Position(stmt.Pos()).Line
		end := fset.Position(stmt.End()).Line
		span := end - start + 1
		if comp >= 3 || span >= 12 {
			inputs := collectStmtInputs(stmt)
			inStr := ""
			if len(inputs) > 0 {
				inStr = fmt.Sprintf(": inputs(%s)", strings.Join(inputs, ", "))
			}
			seams = append(seams, fmt.Sprintf("[SEAM lines %d-%d (%s, +%d complexity, %d LOC)%s -> extract pure helper]", start, end, kind, comp, span, inStr))
		}
	}
	return seams
}

func classifyGoBlockStmt(stmt ast.Stmt) (string, int) {
	kind := ""
	switch stmt.(type) {
	case *ast.ForStmt:
		kind = "for loop"
	case *ast.RangeStmt:
		kind = "range loop"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		kind = "switch dispatch"
	case *ast.IfStmt:
		kind = "conditional branch"
	default:
		return "", 0
	}
	comp := 0
	ast.Inspect(stmt, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			comp++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				comp++
			}
		}
		return true
	})
	return kind, comp
}

func collectStmtInputs(stmt ast.Stmt) []string {
	seen := make(map[string]bool)
	var list []string
	ast.Inspect(stmt, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name == "_" || ident.Name == "nil" || ident.Name == "true" || ident.Name == "false" || ident.Name == "err" {
			return true
		}
		if len(ident.Name) > 1 && ident.Name[0] >= 'a' && ident.Name[0] <= 'z' && !seen[ident.Name] {
			seen[ident.Name] = true
			list = append(list, ident.Name)
		}
		return true
	})
	if len(list) > 4 {
		list = list[:4]
	}
	return list
}

func countGoLogicalOps(body *ast.BlockStmt) int {
	ops := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if bin, ok := n.(*ast.BinaryExpr); ok && (bin.Op == token.LAND || bin.Op == token.LOR) {
			ops++
		}
		return true
	})
	return ops
}

// PrescribeFileDecomposition generates a baseline delta and ranks the top 3 extraction candidates in a file.
// @intent: #FileMetric
func PrescribeFileDecomposition(root, relPath string, src []byte, currentLOC, currentCDS, maxComp, maxFuncLOC int) string {
	var parts []string
	if delta := computeGitBaselineDelta(root, relPath, currentLOC, currentCDS, maxComp, maxFuncLOC); delta != "" {
		parts = append(parts, delta)
	}
	rep, err := AnalyzeFileDebt(relPath, src, maxComp, maxFuncLOC)
	if err == nil && rep != nil && len(rep.Functions) > 0 {
		fns := make([]FunctionDebt, len(rep.Functions))
		copy(fns, rep.Functions)
		sort.Slice(fns, func(i, j int) bool {
			return (fns[i].Complexity*4 + fns[i].LOC) > (fns[j].Complexity*4 + fns[j].LOC)
		})
		limit := 3
		if len(fns) < limit {
			limit = len(fns)
		}
		for i := 0; i < limit; i++ {
			fn := fns[i]
			parts = append(parts, fmt.Sprintf("[EXTRACT %d: %s() lines %d-%d (%d LOC, complexity %d) -> move to sibling module]", i+1, fn.Name, fn.StartLine, fn.EndLine, fn.LOC, fn.Complexity))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n      " + strings.Join(parts, "\n      ")
}

func computeGitBaselineDelta(root, relPath string, currentLOC, currentCDS, maxComp, maxFuncLOC int) string {
	if root == "" || relPath == "" {
		return ""
	}
	cleanRel := filepath.ToSlash(relPath)
	baseRef := resolveBaselineCommitRef(&VerifyContext{RepoRoot: root, WorktreePath: root}, root)
	if baseRef == "" {
		baseRef = "main"
	}
	out, err := workspace.RunSafeGit(root, "show", baseRef+":"+cleanRel)
	if err != nil || len(out) == 0 {
		return ""
	}
	mainLOC := strings.Count(string(out), "\n") + 1
	mainCDS := 0
	if rep, dErr := AnalyzeFileDebt(cleanRel, out, maxComp, maxFuncLOC); dErr == nil && rep != nil {
		mainCDS = rep.DebtScore
	}
	msg := fmt.Sprintf("[BASELINE %s: %d LOC, %d CDS | ΔLOC %+d, ΔCDS %+d]", shortRefLabel(baseRef), mainCDSOrLOCLabel(mainLOC), mainCDS, currentLOC-mainLOC, currentCDS-mainCDS)
	if mainLOC > 400 || mainCDS > 45 {
		msg += fmt.Sprintf(" [RATCHET: restore ΔLOC <= 0 and ΔCDS <= 0 relative to baseline (%d LOC, %d CDS)]", mainLOC, mainCDS)
	}
	return msg
}

func shortRefLabel(ref string) string {
	if len(ref) > 8 {
		return ref[:8]
	}
	return ref
}

func mainCDSOrLOCLabel(loc int) int {
	return loc
}

// EvaluateParameterArityViolations enforces the maximum parameter count per function to prevent low-cohesion splits.
// @intent software_physics_engine_spec.md tree_sitter_ast_gate_spec.md #FunctionMetric
func EvaluateParameterArityViolations(relPath string, src []byte, maxParams int) []string {
	if maxParams <= 0 || len(src) == 0 {
		return nil
	}
	if strings.HasSuffix(relPath, ".go") {
		return scanGoParamViolations(relPath, src, maxParams)
	}
	return scanPolyglotParamViolations(relPath, src, maxParams)
}

func scanGoParamViolations(relPath string, src []byte, maxParams int) []string {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil {
		return nil
	}
	var violations []string
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if count := countGoFuncParams(fn); count > maxParams {
			startLine := fset.Position(fn.Pos()).Line
			endLine := fset.Position(fn.End()).Line
			groupMin := (count - maxParams) + 1
			span := formatFileSpan(relPath, startLine, endLine)
			violations = append(violations, fmt.Sprintf("%s (%s) parameter count %d exceeds max %d -> [1-TURN RECIPE: group >=%d parameters into a %sOptions struct (%s)]",
				span, fn.Name.Name, count, maxParams, groupMin, fn.Name.Name, span))
		}
	}
	return violations
}
