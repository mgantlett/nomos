// Package astdebt implements 11-language function arity and control-flow nesting metrics.
package astdebt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxFunctionNesting defines the statutory upper bound on control-flow nesting depth (#Thresholds).
const MaxFunctionNesting = 5

var (
	polyglotControlLineRe = regexp.MustCompile(`^(?:if|else\s+if|elif|for|while|switch|select|match|try|with|foreach)\b`)
	pyRbDefLineRe         = regexp.MustCompile(`^(?:async\s+)?def\s+[A-Za-z_]`)
)

// SummarizeFileArityAndNesting computes per-file maximum parameter arity, maximum control-flow nesting depth,
// and counts of functions breaching maxParams or maxNesting (@intent tree_sitter_ast_gate_spec.md #XrayReport).
func SummarizeFileArityAndNesting(relPath string, src []byte, maxParams, maxNesting int) (int, int, int, int) {
	if len(src) == 0 {
		return 0, 0, 0, 0
	}
	if maxParams <= 0 {
		maxParams = MaxFunctionParams
	}
	if maxNesting <= 0 {
		maxNesting = MaxFunctionNesting
	}
	if strings.HasSuffix(relPath, ExtGo) {
		return summarizeGoArityAndNesting(relPath, src, maxParams, maxNesting)
	}
	return summarizePolyglotArityAndNesting(relPath, src, maxParams, maxNesting)
}

func summarizeGoArityAndNesting(relPath string, src []byte, maxParams, maxNesting int) (int, int, int, int) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil || node == nil {
		return 0, 0, 0, 0
	}
	obsParams, obsNesting, highArity, deepNest := 0, 0, 0, 0
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		obsParams, highArity = recordMetricBound(countGoFuncParams(fn), maxParams, obsParams, highArity)
		obsNesting, deepNest = recordMetricBound(measureGoStmtSliceNesting(fn.Body.List, 0), maxNesting, obsNesting, deepNest)
	}
	return obsParams, obsNesting, highArity, deepNest
}

func recordMetricBound(val, limit, curMax, curBreaches int) (int, int) {
	if val > curMax {
		curMax = val
	}
	if val > limit {
		curBreaches++
	}
	return curMax, curBreaches
}

func measureGoStmtSliceNesting(stmts []ast.Stmt, depth int) int {
	maxDepth := depth
	for _, s := range stmts {
		if d := measureGoStmtNesting(s, depth); d > maxDepth {
			maxDepth = d
		}
	}
	return maxDepth
}

func measureGoStmtNesting(stmt ast.Stmt, depth int) int {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		return measureGoStmtSliceNesting(s.List, depth)
	case *ast.IfStmt:
		return measureGoIfNesting(s, depth)
	case *ast.ForStmt:
		return measureGoBlockPtrNesting(s.Body, depth+1)
	case *ast.RangeStmt:
		return measureGoBlockPtrNesting(s.Body, depth+1)
	case *ast.SwitchStmt:
		return measureGoClauseBlockNesting(s.Body, depth+1)
	case *ast.TypeSwitchStmt:
		return measureGoClauseBlockNesting(s.Body, depth+1)
	case *ast.SelectStmt:
		return measureGoClauseBlockNesting(s.Body, depth+1)
	case *ast.LabeledStmt:
		return measureGoStmtNesting(s.Stmt, depth)
	default:
		return depth
	}
}

func measureGoIfNesting(s *ast.IfStmt, depth int) int {
	maxDepth := measureGoBlockPtrNesting(s.Body, depth+1)
	if s.Else == nil {
		return maxDepth
	}
	elseDepth := depth + 1
	if _, isElseIf := s.Else.(*ast.IfStmt); isElseIf {
		elseDepth = depth
	}
	if d := measureGoStmtNesting(s.Else, elseDepth); d > maxDepth {
		maxDepth = d
	}
	return maxDepth
}

func measureGoBlockPtrNesting(b *ast.BlockStmt, nextDepth int) int {
	if b == nil {
		return nextDepth
	}
	return measureGoStmtSliceNesting(b.List, nextDepth)
}

func measureGoClauseBlockNesting(b *ast.BlockStmt, nextDepth int) int {
	if b == nil {
		return nextDepth
	}
	maxDepth := nextDepth
	for _, item := range b.List {
		switch cl := item.(type) {
		case *ast.CaseClause:
			if d := measureGoStmtSliceNesting(cl.Body, nextDepth); d > maxDepth {
				maxDepth = d
			}
		case *ast.CommClause:
			if d := measureGoStmtSliceNesting(cl.Body, nextDepth); d > maxDepth {
				maxDepth = d
			}
		}
	}
	return maxDepth
}

func summarizePolyglotArityAndNesting(relPath string, src []byte, maxParams, maxNesting int) (int, int, int, int) {
	ext := strings.ToLower(filepath.Ext(relPath))
	text := string(src)
	obsParams, highArity := measurePolyglotParamStats(ext, text, maxParams)
	obsNest, deepNest := measurePolyglotNestingStats(ext, text, maxNesting)
	return obsParams, obsNest, highArity, deepNest
}

func measurePolyglotParamStats(ext, text string, maxParams int) (int, int) {
	re := selectParamHeaderRegex(ext)
	if re == nil {
		return 0, 0
	}
	obsMax, breaches := 0, 0
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if count, ok := parseMatchedSignatureArity(ext, text, m); ok {
			obsMax, breaches = recordMetricBound(count, maxParams, obsMax, breaches)
		}
	}
	return obsMax, breaches
}

func parseMatchedSignatureArity(ext, text string, m []int) (int, bool) {
	name := extractFirstCapturedGroup(text, m)
	openParen := m[1] - 1
	if name == "" || nonFuncControlKeywords[name] || openParen < 0 || text[openParen] != '(' {
		return 0, false
	}
	if IsJSFamilyExt(ext) && !isFollowedByFuncBodyOrArrow(text, openParen) {
		return 0, false
	}
	paramRaw, _, ok := extractBalancedParenSpanWithEnd(text, openParen)
	if !ok {
		return 0, false
	}
	return countTopLevelParams(ext, paramRaw), true
}

func measurePolyglotNestingStats(ext, text string, maxNesting int) (int, int) {
	if ext == ExtPy || ext == ExtRB || ext == ExtRake {
		return measureIndentControlNesting(text, maxNesting)
	}
	return measureBraceControlNesting(text, maxNesting)
}

type indentNestTracker struct {
	obsMax     int
	breaches   int
	curFuncMax int
	baseIndent int
	stack      []int
}

func (t *indentNestTracker) flush(limit int) {
	if t.baseIndent < 0 {
		return
	}
	t.obsMax, t.breaches = recordMetricBound(t.curFuncMax, limit, t.obsMax, t.breaches)
	t.curFuncMax = 0
	t.stack = t.stack[:0]
}

func (t *indentNestTracker) stepLine(rawLine string, limit int) {
	trimmed := strings.TrimSpace(rawLine)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return
	}
	indent := len(rawLine) - len(strings.TrimLeft(rawLine, " \t"))
	if pyRbDefLineRe.MatchString(trimmed) && (t.baseIndent < 0 || indent <= t.baseIndent) {
		t.flush(limit)
		t.baseIndent = indent
		return
	}
	if t.baseIndent < 0 {
		return
	}
	t.stack = trimIndentStack(t.stack, indent)
	if polyglotControlLineRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "elif") {
		t.stack = append(t.stack, indent)
		if len(t.stack) > t.curFuncMax {
			t.curFuncMax = len(t.stack)
		}
	}
}

func trimIndentStack(stack []int, indent int) []int {
	for len(stack) > 0 && indent <= stack[len(stack)-1] {
		stack = stack[:len(stack)-1]
	}
	return stack
}

func measureIndentControlNesting(text string, maxNesting int) (int, int) {
	tr := &indentNestTracker{baseIndent: -1}
	for _, rawLine := range strings.Split(text, "\n") {
		tr.stepLine(rawLine, maxNesting)
	}
	tr.flush(maxNesting)
	return tr.obsMax, tr.breaches
}

type braceNestTracker struct {
	obsMax        int
	breaches      int
	braceDepth    int
	curMax        int
	controlBraces []int
}

func (t *braceNestTracker) stepLine(rawLine string, limit int) {
	trimmed := strings.TrimSpace(rawLine)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
		return
	}
	opens := strings.Count(trimmed, "{")
	closes := strings.Count(trimmed, "}")
	if isBraceControlHeader(trimmed) && opens > closes {
		t.controlBraces = append(t.controlBraces, t.braceDepth+1)
		if len(t.controlBraces) > t.curMax {
			t.curMax = len(t.controlBraces)
		}
	}
	t.braceDepth += opens - closes
	if t.braceDepth <= 0 {
		t.obsMax, t.breaches = recordMetricBound(t.curMax, limit, t.obsMax, t.breaches)
		t.braceDepth, t.curMax, t.controlBraces = 0, 0, t.controlBraces[:0]
		return
	}
	t.controlBraces = trimControlBraces(t.controlBraces, t.braceDepth)
}

func isBraceControlHeader(trimmed string) bool {
	return polyglotControlLineRe.MatchString(trimmed) && !strings.HasPrefix(trimmed, "else if")
}

func trimControlBraces(braces []int, depth int) []int {
	for len(braces) > 0 && depth < braces[len(braces)-1] {
		braces = braces[:len(braces)-1]
	}
	return braces
}

func measureBraceControlNesting(text string, maxNesting int) (int, int) {
	tr := &braceNestTracker{}
	for _, rawLine := range strings.Split(text, "\n") {
		tr.stepLine(rawLine, maxNesting)
	}
	return tr.obsMax, tr.breaches
}
