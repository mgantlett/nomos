// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

// DefaultMinCloneLines defines the minimum function LOC required to evaluate Type-2 structural clones.
const DefaultMinCloneLines = 18

type cloneBlockCandidate struct {
	file      string
	startLine int
	endLine   int
	loc       int
	hash      string
}

// DetectStructuralClones identifies Type-2 duplicated AST function structures across scanned source files.
// @intent: #CloneGroup
func DetectStructuralClones(files map[string][]byte, minLines int) []CloneGroup {
	if minLines <= 0 {
		minLines = DefaultMinCloneLines
	}
	buckets := make(map[string][]cloneBlockCandidate)

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, relPath := range paths {
		src := files[relPath]
		candidates := extractFileCloneCandidates(relPath, src, minLines)
		for _, c := range candidates {
			buckets[c.hash] = append(buckets[c.hash], c)
		}
	}

	return buildCloneGroups(buckets)
}

func extractFileCloneCandidates(relPath string, src []byte, minLines int) []cloneBlockCandidate {
	if strings.HasSuffix(relPath, ".go") {
		return extractGoCloneCandidates(relPath, src, minLines)
	}
	lang := treesitter.DetectLanguage(relPath)
	if lang == "typescript" || lang == "svelte" || lang == "python" || lang == "rust" || lang == "c" || lang == "cpp" {
		return extractPolyglotCloneCandidates(lang, relPath, src, minLines)
	}
	return nil
}

func extractGoCloneCandidates(relPath string, src []byte, minLines int) []cloneBlockCandidate {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, relPath, src, 0)
	if err != nil {
		return nil
	}
	var out []cloneBlockCandidate
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		loc := end - start + 1
		if loc < minLines {
			continue
		}
		skel := buildGoASTSkeleton(fn.Body)
		if len(skel) < 24 {
			continue
		}
		out = append(out, cloneBlockCandidate{
			file:      filepath.ToSlash(relPath),
			startLine: start,
			endLine:   end,
			loc:       loc,
			hash:      hashSkeleton("go:" + skel),
		})
	}
	return out
}

func buildGoASTSkeleton(body *ast.BlockStmt) string {
	var tokens []string
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if tag := goNodeStructuralTag(n); tag != "" {
			tokens = append(tokens, tag)
		}
		return true
	})
	return strings.Join(tokens, ";")
}

func goNodeStructuralTag(n ast.Node) string {
	switch x := n.(type) {
	case *ast.IfStmt:
		return "IF"
	case *ast.ForStmt:
		return "FOR"
	case *ast.RangeStmt:
		return "RANGE"
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return "SWITCH"
	case *ast.CaseClause:
		return "CASE"
	case *ast.ReturnStmt:
		return fmt.Sprintf("RET:%d", len(x.Results))
	case *ast.AssignStmt:
		return fmt.Sprintf("ASN:%s", x.Tok.String())
	case *ast.BinaryExpr:
		return fmt.Sprintf("BIN:%s", x.Op.String())
	case *ast.CallExpr:
		return formatGoCallTag(x)
	default:
		return ""
	}
}

func formatGoCallTag(call *ast.CallExpr) string {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return fmt.Sprintf("CALL:.%s/%d", sel.Sel.Name, len(call.Args))
	}
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return fmt.Sprintf("CALL:%s/%d", ident.Name, len(call.Args))
	}
	return fmt.Sprintf("CALL:?/%d", len(call.Args))
}

var (
	polyglotStringLitRe = regexp.MustCompile(`(?:"[^"\\]*(?:\\.[^"\\]*)*"|'[^'\\]*(?:\\.[^'\\]*)*')`)
	polyglotNumberLitRe = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	polyglotSpaceRe     = regexp.MustCompile(`\s+`)
)

func extractPolyglotCloneCandidates(lang, relPath string, src []byte, minLines int) []cloneBlockCandidate {
	funcs, err := treesitter.ExtractFunctions(lang, relPath, src)
	if err != nil || len(funcs) == 0 {
		return nil
	}
	lines := strings.Split(string(src), "\n")
	var out []cloneBlockCandidate
	for _, fn := range funcs {
		if fn.LOC < minLines {
			continue
		}
		startIdx := fn.Line - 1
		endIdx := startIdx + fn.LOC
		if startIdx < 0 || endIdx > len(lines) {
			continue
		}
		skel := normalizePolyglotLines(lines[startIdx:endIdx])
		if len(skel) < 40 {
			continue
		}
		out = append(out, cloneBlockCandidate{
			file:      filepath.ToSlash(relPath),
			startLine: fn.Line,
			endLine:   fn.Line + fn.LOC - 1,
			loc:       fn.LOC,
			hash:      hashSkeleton(lang + ":" + skel),
		})
	}
	return out
}

func normalizePolyglotLines(lines []string) string {
	if len(lines) <= 1 {
		return ""
	}
	var cleaned []string
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		norm := polyglotStringLitRe.ReplaceAllString(trimmed, "$STR")
		norm = polyglotNumberLitRe.ReplaceAllString(norm, "$NUM")
		norm = polyglotSpaceRe.ReplaceAllString(norm, " ")
		cleaned = append(cleaned, norm)
	}
	return strings.Join(cleaned, "|")
}

func hashSkeleton(skel string) string {
	sum := sha256.Sum256([]byte(skel))
	return hex.EncodeToString(sum[:8])
}

func buildCloneGroups(buckets map[string][]cloneBlockCandidate) []CloneGroup {
	var groups []CloneGroup
	for hash, list := range buckets {
		if len(list) < 2 {
			continue
		}
		occs := make([]CloneOccurrence, 0, len(list))
		maxLOC := 0
		for _, item := range list {
			if item.loc > maxLOC {
				maxLOC = item.loc
			}
			occs = append(occs, CloneOccurrence{
				File:      item.file,
				StartLine: item.startLine,
				EndLine:   item.endLine,
			})
		}
		groups = append(groups, CloneGroup{
			Hash:        hash,
			LineCount:   maxLOC,
			Occurrences: occs,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Hash < groups[j].Hash
	})
	return groups
}

// FormatCloneViolations renders structural clone groups into actionable Law 6 violation strings.
// @intent: #CloneGroup
func FormatCloneViolations(clones []CloneGroup) []string {
	var violations []string
	for _, cg := range clones {
		if len(cg.Occurrences) < 2 {
			continue
		}
		locs := make([]string, 0, len(cg.Occurrences))
		for _, occ := range cg.Occurrences {
			locs = append(locs, fmt.Sprintf("%s:%d-%d", occ.File, occ.StartLine, occ.EndLine))
		}
		violations = append(violations, fmt.Sprintf(
			"structural AST clone [%s] (%d lines duplicated across %d locations: %s) -> [SEAM: consolidate duplicate logic into a single shared canonical function (Law 6)]",
			cg.Hash[:8], cg.LineCount, len(cg.Occurrences), strings.Join(locs, ", "),
		))
	}
	return violations
}

// EnrichCDSViolations appends file decomposition blueprints and structural clone diagnostics to CDS violations.
// @intent: #FileDebtReport
func EnrichCDSViolations(root string, metrics []FileMetric, sources map[string][]byte, thresh Thresholds, checkClones bool) []string {
	var violations []string
	for _, f := range metrics {
		if f.CDS > thresh.MaxCDS {
			msg := fmt.Sprintf("%s has cognitive debt %d (exceeds max %d)", f.Name, f.CDS, thresh.MaxCDS)
			msg += PrescribeFileDecomposition(root, f.Name, sources[f.Name], f.LOC, f.CDS, thresh.MaxComplexity, thresh.MaxFuncLOC)
			violations = append(violations, msg)
		}
	}
	if checkClones && len(sources) > 0 {
		minLines := thresh.MinCloneLines
		if minLines <= 0 {
			minLines = DefaultMinCloneLines
		}
		clones := DetectStructuralClones(sources, minLines)
		violations = append(violations, FormatCloneViolations(clones)...)
	}
	return violations
}

// EnrichFileLOCViolations appends baseline deltas and top extraction candidate functions to LOC violations.
// @intent: #FileMetric
func EnrichFileLOCViolations(root string, metrics []FileMetric, thresh Thresholds) []string {
	var violations []string
	for _, f := range metrics {
		if f.LOC > thresh.MaxFileLOC {
			msg := fmt.Sprintf("%s has %d LOC (exceeds max %d)", f.Name, f.LOC, thresh.MaxFileLOC)
			if src, err := os.ReadFile(filepath.Join(root, f.Name)); err == nil {
				msg += PrescribeFileDecomposition(root, f.Name, src, f.LOC, 0, thresh.MaxComplexity, thresh.MaxFuncLOC)
			}
			violations = append(violations, msg)
		}
	}
	return violations
}

// EnrichComplexityViolations appends AST seam cut-lines and parameter arity checks to complexity violations.
// @intent: #FunctionMetric
func EnrichComplexityViolations(root string, funcs []FunctionMetric, thresh Thresholds, checkParams bool) []string {
	var violations []string
	seenFiles := make(map[string][]byte)
	for _, fn := range funcs {
		src, ok := seenFiles[fn.File]
		if !ok {
			src, _ = os.ReadFile(filepath.Join(root, fn.File))
			seenFiles[fn.File] = src
		}
		if fn.Complexity > thresh.MaxComplexity {
			msg := fmt.Sprintf("%s:%d (%s) cyclomatic complexity %d exceeds max %d", fn.File, fn.Line, fn.Name, fn.Complexity, thresh.MaxComplexity)
			if seams := PrescribeFunctionSeams(fn.File, src, fn.Name, fn.Line); len(seams) > 0 {
				msg += " -> " + strings.Join(seams, " ")
			}
			violations = append(violations, msg)
		}
		if fn.LOC > thresh.MaxFuncLOC {
			msg := fmt.Sprintf("%s:%d (%s) LOC %d exceeds max %d", fn.File, fn.Line, fn.Name, fn.LOC, thresh.MaxFuncLOC)
			if seams := PrescribeFunctionSeams(fn.File, src, fn.Name, fn.Line); len(seams) > 0 {
				msg += " -> " + strings.Join(seams, " ")
			}
			violations = append(violations, msg)
		}
	}
	if checkParams {
		maxParams := thresh.MaxFuncParams
		if maxParams <= 0 {
			maxParams = MaxFunctionParams
		}
		for rel, src := range seenFiles {
			violations = append(violations, EvaluateParameterArityViolations(rel, src, maxParams)...)
		}
	}
	return violations
}

