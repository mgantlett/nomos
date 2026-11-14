// Package prproof implements diff-scoped Go AST micro-mutation site discovery.
package prproof

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	maxMutantsPerFunc  = 4
	maxMutantsPerRun   = 24
	mutantTestTimeoutS = 20
)

var diffHunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

type lineSpan struct {
	start int
	end   int
}

type rawMutantSite struct {
	meta      DiffMutant
	modRoot   string
	pkgRel    string
	fileRel   string
	offset    int
	origBytes int
}

type mutantScanCtx struct {
	fset         *token.FileSet
	src          []byte
	fnName       string
	relFile      string
	modRoot      string
	pkgRel       string
	fileRelToMod string
	seq          *int
}

// EvaluateDiffMutations mutates single-point AST operators in ephemeral shadow copies and runs targeted tests.
// @intent nomos_intent_traceability_spec.md #DiffMutationReport
// @intent tree_sitter_ast_gate_spec.md
func EvaluateDiffMutations(targetDir, baseRef string) (*DiffMutationReport, error) {
	started := time.Now()
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed resolving repository path: %w", err)
	}
	effectiveBase := resolveEffectiveBaseRef(absTarget, baseRef)
	changedLines, dErr := collectChangedGoLineSpans(absTarget, effectiveBase)
	if dErr != nil {
		return nil, dErr
	}
	sites := collectDiffMutantSites(absTarget, changedLines)
	if len(sites) == 0 {
		return &DiffMutationReport{
			BaseRef:        effectiveBase,
			KillScorePct:   100.0,
			Passed:         true,
			DurationMillis: time.Since(started).Milliseconds(),
		}, nil
	}
	return executeShadowMutants(absTarget, effectiveBase, sites, started)
}

func resolveEffectiveBaseRef(absTarget, requested string) string {
	cand := strings.TrimSpace(requested)
	if cand != "" && gitRefExists(absTarget, cand) {
		return cand
	}
	if cfg, err := config.LoadConfig(filepath.Join(absTarget, constants.FileNomosYAML)); err == nil && cfg != nil && cfg.DefaultBranch != "" {
		if gitRefExists(absTarget, cfg.DefaultBranch) {
			return cfg.DefaultBranch
		}
	}
	for _, fallback := range []string{"main", "origin/main", "master", "HEAD~1", "HEAD"} {
		if gitRefExists(absTarget, fallback) {
			return fallback
		}
	}
	return "HEAD"
}

func gitRefExists(absTarget, ref string) bool {
	_, err := workspace.RunSafeGit(absTarget, "rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

func collectChangedGoLineSpans(absTarget, baseRef string) (map[string][]lineSpan, error) {
	raw, err := workspace.RunSafeGit(absTarget, "diff", "--unified=0", "--diff-filter=AM", baseRef, "--")
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		raw, err = workspace.RunSafeGit(absTarget, "diff", "--unified=0", "--diff-filter=AM", "HEAD", "--")
		if err != nil {
			return map[string][]lineSpan{}, nil
		}
	}
	return parseUnifiedZeroDiffGoSpans(string(raw)), nil
}

func parseUnifiedZeroDiffGoSpans(diffText string) map[string][]lineSpan {
	result := make(map[string][]lineSpan)
	currentFile := ""
	for _, line := range strings.Split(diffText, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = selectGoDiffTarget(strings.TrimPrefix(line, "+++ b/"))
			continue
		}
		if currentFile != "" {
			if sp, ok := parseHunkLineSpan(line); ok {
				result[currentFile] = append(result[currentFile], sp)
			}
		}
	}
	return result
}

func selectGoDiffTarget(raw string) string {
	cand := filepath.ToSlash(strings.TrimSpace(raw))
	if strings.HasSuffix(cand, ".go") && !strings.HasSuffix(cand, "_test.go") {
		return cand
	}
	return ""
}

func parseHunkLineSpan(line string) (lineSpan, bool) {
	m := diffHunkRe.FindStringSubmatch(line)
	if len(m) < 2 {
		return lineSpan{}, false
	}
	startLine, sErr := strconv.Atoi(m[1])
	if sErr != nil || startLine <= 0 {
		return lineSpan{}, false
	}
	count := 1
	if len(m) >= 3 && m[2] != "" {
		if parsedCount, cErr := strconv.Atoi(m[2]); cErr == nil {
			count = parsedCount
		}
	}
	if count <= 0 {
		return lineSpan{}, false
	}
	return lineSpan{start: startLine, end: startLine + count - 1}, true
}

func collectDiffMutantSites(absTarget string, changed map[string][]lineSpan) []rawMutantSite {
	var sites []rawMutantSite
	for relFile, spans := range changed {
		if len(sites) >= maxMutantsPerRun {
			break
		}
		fullPath := filepath.Join(absTarget, filepath.FromSlash(relFile))
		modRoot, pkgRel, mOK := locateEnclosingGoModule(absTarget, fullPath)
		if !mOK {
			continue
		}
		fileSites := extractFileMutantSites(modRoot, pkgRel, relFile, fullPath, spans, len(sites))
		sites = append(sites, fileSites...)
		if len(sites) > maxMutantsPerRun {
			sites = sites[:maxMutantsPerRun]
		}
	}
	return sites
}

func locateEnclosingGoModule(absTarget, fullFile string) (string, string, bool) {
	dir := filepath.Dir(fullFile)
	cur := dir
	for {
		if fi, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil && !fi.IsDir() {
			pkgRel, rErr := filepath.Rel(cur, dir)
			if rErr != nil || strings.HasPrefix(pkgRel, "..") {
				return "", "", false
			}
			if pkgRel == "." {
				return cur, ".", true
			}
			return cur, "./" + filepath.ToSlash(pkgRel), true
		}
		if cur == absTarget || filepath.Dir(cur) == cur {
			break
		}
		cur = filepath.Dir(cur)
	}
	return "", "", false
}

func extractFileMutantSites(modRoot, pkgRel, relFile, fullPath string, spans []lineSpan, startIdx int) []rawMutantSite {
	src, err := os.ReadFile(fullPath)
	if err != nil {
		return nil
	}
	fset := token.NewFileSet()
	node, pErr := parser.ParseFile(fset, fullPath, src, 0)
	if pErr != nil {
		return nil
	}
	fileRelToMod, rErr := filepath.Rel(modRoot, fullPath)
	if rErr != nil || strings.HasPrefix(fileRelToMod, "..") {
		return nil
	}
	var out []rawMutantSite
	seq := startIdx
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name == nil || fn.Name.Name == "init" {
			continue
		}
		if !overlapsAnySpan(fset.Position(fn.Pos()).Line, fset.Position(fn.End()).Line, spans) {
			continue
		}
		out = append(out, collectFuncMutants(fset, src, fn, relFile, modRoot, pkgRel, filepath.ToSlash(fileRelToMod), &seq)...)
	}
	return out
}

func overlapsAnySpan(fnStart, fnEnd int, spans []lineSpan) bool {
	for _, s := range spans {
		if s.start <= fnEnd && s.end >= fnStart {
			return true
		}
	}
	return false
}

func collectFuncMutants(fset *token.FileSet, src []byte, fn *ast.FuncDecl, relFile, modRoot, pkgRel, fileRelToMod string, seq *int) []rawMutantSite {
	mctx := &mutantScanCtx{
		fset: fset, src: src, fnName: fn.Name.Name, relFile: relFile,
		modRoot: modRoot, pkgRel: pkgRel, fileRelToMod: fileRelToMod, seq: seq,
	}
	var out []rawMutantSite
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if len(out) >= maxMutantsPerFunc || n == nil {
			return false
		}
		if site, ok := buildNodeMutant(mctx, n); ok {
			out = append(out, site)
		}
		return true
	})
	return out
}

func buildNodeMutant(mctx *mutantScanCtx, n ast.Node) (rawMutantSite, bool) {
	switch x := n.(type) {
	case *ast.BinaryExpr:
		return buildBinaryMutant(mctx, x)
	case *ast.ReturnStmt:
		return buildBoolReturnMutant(mctx, x)
	default:
		return rawMutantSite{}, false
	}
}

func buildBinaryMutant(mctx *mutantScanCtx, x *ast.BinaryExpr) (rawMutantSite, bool) {
	if isErrNilGuard(x) {
		return rawMutantSite{}, false
	}
	origOp, mutOp, ok := mapBinaryMutantOp(x.Op)
	if !ok {
		return rawMutantSite{}, false
	}
	pos := mctx.fset.Position(x.OpPos)
	if !matchesSourceBytes(mctx.src, pos.Offset, origOp) {
		return rawMutantSite{}, false
	}
	*mctx.seq = *mctx.seq + 1
	return rawMutantSite{
		meta:      DiffMutant{ID: fmt.Sprintf("MUT-%03d", *mctx.seq), File: mctx.relFile, Function: mctx.fnName, Line: pos.Line, OriginalOp: origOp, MutatedOp: mutOp},
		modRoot:   mctx.modRoot,
		pkgRel:    mctx.pkgRel,
		fileRel:   mctx.fileRelToMod,
		offset:    pos.Offset,
		origBytes: len(origOp),
	}, true
}

func buildBoolReturnMutant(mctx *mutantScanCtx, x *ast.ReturnStmt) (rawMutantSite, bool) {
	for _, res := range x.Results {
		id, isID := res.(*ast.Ident)
		if !isID || (id.Name != "true" && id.Name != "false") {
			continue
		}
		mutVal := "false"
		if id.Name == "false" {
			mutVal = "true"
		}
		pos := mctx.fset.Position(id.Pos())
		if !matchesSourceBytes(mctx.src, pos.Offset, id.Name) {
			continue
		}
		*mctx.seq = *mctx.seq + 1
		return rawMutantSite{
			meta:      DiffMutant{ID: fmt.Sprintf("MUT-%03d", *mctx.seq), File: mctx.relFile, Function: mctx.fnName, Line: pos.Line, OriginalOp: id.Name, MutatedOp: mutVal},
			modRoot:   mctx.modRoot,
			pkgRel:    mctx.pkgRel,
			fileRel:   mctx.fileRelToMod,
			offset:    pos.Offset,
			origBytes: len(id.Name),
		}, true
	}
	return rawMutantSite{}, false
}

func isErrNilGuard(b *ast.BinaryExpr) bool {
	if b.Op != token.EQL && b.Op != token.NEQ {
		return false
	}
	leftID, lOK := b.X.(*ast.Ident)
	rightID, rOK := b.Y.(*ast.Ident)
	if !lOK || !rOK {
		return false
	}
	return (leftID.Name == "nil" && strings.Contains(strings.ToLower(rightID.Name), "err")) ||
		(rightID.Name == "nil" && strings.Contains(strings.ToLower(leftID.Name), "err"))
}

func mapBinaryMutantOp(op token.Token) (string, string, bool) {
	switch op {
	case token.GTR:
		return ">", ">=", true
	case token.GEQ:
		return ">=", ">", true
	case token.LSS:
		return "<", "<=", true
	case token.LEQ:
		return "<=", "<", true
	case token.EQL:
		return "==", "!=", true
	case token.NEQ:
		return "!=", "==", true
	default:
		return "", "", false
	}
}

func matchesSourceBytes(src []byte, offset int, expected string) bool {
	if offset < 0 || offset+len(expected) > len(src) {
		return false
	}
	return string(src[offset:offset+len(expected)]) == expected
}
