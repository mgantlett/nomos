// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
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
	return prescribePolyglotFunctionSeams(src, funcLine)
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

func prescribePolyglotFunctionSeams(src []byte, funcLine int) []string {
	lines := strings.Split(string(src), "\n")
	startIdx := funcLine - 1
	if startIdx < 0 || startIdx >= len(lines) {
		return nil
	}
	bodyLines, endIdx := slicePolyglotFuncBody(lines, startIdx)
	bodyText := strings.Join(bodyLines, "\n")

	var seams []string
	fallbacks := strings.Count(bodyText, "||") + strings.Count(bodyText, "??") + strings.Count(bodyText, " ? ")
	if fallbacks >= 5 {
		seams = append(seams, fmt.Sprintf("[SEAM: %d inline fallback/ternary branches (|| / ?? / ?:) -> replace with typed default object / Object.assign]", fallbacks))
	}
	span := endIdx - startIdx + 1
	if span >= 20 {
		mid := startIdx + span/2
		seams = append(seams, fmt.Sprintf("[SEAM lines %d-%d (%d LOC body) -> split around line %d into cohesive helper functions]", funcLine, endIdx+1, span, mid+1))
	}
	return seams
}

func slicePolyglotFuncBody(lines []string, startIdx int) ([]string, int) {
	depth := 0
	started := false
	maxIdx := startIdx + 120
	if maxIdx > len(lines) {
		maxIdx = len(lines)
	}
	for i := startIdx; i < maxIdx; i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
				started = true
			} else if ch == '}' {
				depth--
			}
		}
		if started && depth <= 0 {
			return lines[startIdx : i+1], i
		}
	}
	return lines[startIdx:maxIdx], maxIdx - 1
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
	out, err := workspace.RunSafeGit(root, "show", "main:"+cleanRel)
	if err != nil || len(out) == 0 {
		return ""
	}
	mainLOC := strings.Count(string(out), "\n") + 1
	mainCDS := 0
	if rep, dErr := AnalyzeFileDebt(cleanRel, out, maxComp, maxFuncLOC); dErr == nil && rep != nil {
		mainCDS = rep.DebtScore
	}
	msg := fmt.Sprintf("[BASELINE main: %d LOC, %d CDS | ΔLOC %+d, ΔCDS %+d]", mainLOC, mainCDS, currentLOC-mainLOC, currentCDS-mainCDS)
	if mainLOC > 400 || mainCDS > 45 {
		targetLOC := mainLOC - 30
		if targetLOC < 400 {
			targetLOC = 400
		}
		msg += fmt.Sprintf(" [BOY-SCOUT RATCHET: legacy monolith; extract helper module to reduce <= %d LOC]", targetLOC)
	}
	return msg
}

var tsParamSigRegex = regexp.MustCompile(`(?m)(?:function\s+(\w+)\s*\(([^)]*)\)|const\s+(\w+)\s*=\s*(?:async\s*)?\(([^)]*)\)\s*=>)`)

// EvaluateParameterArityViolations enforces the maximum parameter count per function to prevent low-cohesion splits.
// @intent: #FunctionMetric
func EvaluateParameterArityViolations(relPath string, src []byte, maxParams int) []string {
	if maxParams <= 0 || len(src) == 0 {
		return nil
	}
	if strings.HasSuffix(relPath, ".go") {
		return scanGoParamViolations(relPath, src, maxParams)
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".svelte" {
		return scanTSParamViolations(relPath, src, maxParams)
	}
	return nil
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
			line := fset.Position(fn.Pos()).Line
			violations = append(violations, fmt.Sprintf("%s:%d (%s) parameter count %d exceeds max %d -> [SEAM: encapsulate parameters into a cohesive domain struct]", relPath, line, fn.Name.Name, count, maxParams))
		}
	}
	return violations
}

func scanTSParamViolations(relPath string, src []byte, maxParams int) []string {
	text := string(src)
	matches := tsParamSigRegex.FindAllStringSubmatchIndex(text, -1)
	var violations []string
	for _, m := range matches {
		name, paramStr := extractTSMatchNameAndParams(text, m)
		if name == "" || paramStr == "" {
			continue
		}
		count := len(strings.Split(paramStr, ","))
		if count > maxParams {
			line := strings.Count(text[:m[0]], "\n") + 1
			violations = append(violations, fmt.Sprintf("%s:%d (%s) parameter count %d exceeds max %d -> [SEAM: encapsulate parameters into an options interface/object]", relPath, line, name, count, maxParams))
		}
	}
	return violations
}

func extractTSMatchNameAndParams(text string, m []int) (string, string) {
	if m[2] >= 0 && m[4] >= 0 {
		return text[m[2]:m[3]], strings.TrimSpace(text[m[4]:m[5]])
	}
	if m[6] >= 0 && m[8] >= 0 {
		return text[m[6]:m[7]], strings.TrimSpace(text[m[8]:m[9]])
	}
	return "", ""
}
