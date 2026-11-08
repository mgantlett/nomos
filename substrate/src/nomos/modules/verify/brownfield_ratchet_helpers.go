// Package verify implements CDS, clone, shim, docstring, and error-hygiene ratchets for brownfield mode.
package verify

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

var lineNumberStripRe = regexp.MustCompile(`:[0-9]+`)

func collectPolyglotComplexityFromBytes(path, rel string, content []byte, out *[]FunctionMetric) {
	engine, err := treesitter.GetDefaultEngine()
	if err != nil {
		return
	}
	rep, err := engine.ParseAndEvaluate(context.Background(), path, content)
	if err != nil || rep == nil {
		return
	}
	for _, fn := range rep.Functions {
		*out = append(*out, FunctionMetric{
			File:       rel,
			Line:       fn.Line,
			Name:       fn.Name,
			Complexity: fn.Complexity,
			LOC:        fn.LOC,
		})
	}
}

func evaluateRatchetedCDSViolations(ctx *VerifyContext, root string, metrics []FileMetric, sources map[string][]byte, thresh Thresholds) []string {
	if !isBrownfieldRatchetActive(ctx) {
		return EnrichCDSViolations(root, metrics, sources, thresh, isWorktreeContext(ctx, root))
	}
	adjMetrics := make([]FileMetric, 0, len(metrics))
	changedSources := make(map[string][]byte)
	baseSources := make(map[string][]byte)

	for _, m := range metrics {
		absPath := filepath.Join(root, m.Name)
		baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath)
		if !ok {
			adjMetrics = append(adjMetrics, m)
			changedSources[m.Name] = sources[m.Name]
			continue
		}
		if !bytes.Equal(sources[m.Name], baseBytes) {
			changedSources[m.Name] = sources[m.Name]
			baseSources[m.Name] = baseBytes
		}
		if baseRep, err := AnalyzeFileDebt(absPath, baseBytes, thresh.MaxComplexity, thresh.MaxFuncLOC); err == nil && baseRep != nil {
			if m.CDS <= maxInt(thresh.MaxCDS, baseRep.DebtScore) {
				m.CDS = thresh.MaxCDS
			}
		}
		adjMetrics = append(adjMetrics, m)
	}

	violations := EnrichCDSViolations(root, adjMetrics, nil, thresh, false)
	if isWorktreeContext(ctx, root) && len(changedSources) > 0 {
		violations = append(violations, detectNetNewCloneViolations(changedSources, baseSources, thresh)...)
	}
	return violations
}

func detectNetNewCloneViolations(changedSources, baseSources map[string][]byte, thresh Thresholds) []string {
	minLines := thresh.MinCloneLines
	if minLines <= 0 {
		minLines = DefaultMinCloneLines
	}
	baseHashes := make(map[string]bool)
	for _, cg := range DetectStructuralClones(baseSources, minLines) {
		baseHashes[cg.Hash] = true
	}
	var netNew []CloneGroup
	for _, cg := range DetectStructuralClones(changedSources, minLines) {
		if !baseHashes[cg.Hash] {
			netNew = append(netNew, cg)
		}
	}
	return FormatCloneViolations(netNew)
}

func filterNetNewLineViolations(ctx *VerifyContext, root, absPath string, curViolations []string, scanBase func(baseBytes []byte) []string) []string {
	if len(curViolations) == 0 || !isBrownfieldRatchetActive(ctx) {
		return curViolations
	}
	baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath)
	if !ok {
		return curViolations
	}
	baseCounts := make(map[string]int)
	for _, v := range scanBase(baseBytes) {
		baseCounts[normalizeViolationKey(v)]++
	}
	var netNew []string
	for _, v := range curViolations {
		key := normalizeViolationKey(v)
		if baseCounts[key] > 0 {
			baseCounts[key]--
			continue
		}
		netNew = append(netNew, v)
	}
	return netNew
}

func normalizeViolationKey(v string) string {
	return lineNumberStripRe.ReplaceAllString(v, "")
}

func scanTempBaselineViolations(absPath string, baseBytes []byte, scanPath func(tempPath string) []string) []string {
	tmpFile, err := os.CreateTemp("", "nomos-base-*"+filepath.Ext(absPath))
	if err != nil {
		return nil
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if rErr := os.Remove(tmpPath); rErr != nil && !os.IsNotExist(rErr) {
			return
		}
	}()
	if _, wErr := tmpFile.Write(baseBytes); wErr != nil {
		_ = tmpFile.Close()
		return nil
	}
	_ = tmpFile.Close()
	return scanPath(tmpPath)
}

func countRatchetedDocstringsForFile(ctx *VerifyContext, srcDir, path, rel string) (int, int, []string, bool) {
	if !isBrownfieldRatchetActive(ctx) {
		return 0, 0, nil, false
	}
	baseBytes, ok := loadBaselineFileBytes(ctx, srcDir, path)
	if !ok {
		return 0, 0, nil, false
	}
	baseExportedDoc := extractExportedDocMap(path, baseBytes)
	curBytes, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, nil, true
	}
	curExportedDoc := extractExportedDocMap(path, curBytes)
	var total, documented int
	var missing []string
	for name, curDoc := range curExportedDoc {
		baseDoc, existed := baseExportedDoc[name]
		if existed && !baseDoc {
			continue
		}
		total++
		if curDoc {
			documented++
		} else {
			missing = append(missing, fmt.Sprintf("%s (%s)", rel, name))
		}
	}
	return total, documented, missing, true
}

func extractExportedDocMap(path string, content []byte) map[string]bool {
	res := make(map[string]bool)
	if strings.HasSuffix(path, ".go") {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return res
		}
		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
				res[fn.Name.Name] = fn.Doc != nil && len(fn.Doc.List) > 0
			}
		}
		return res
	}
	engine, err := treesitter.GetDefaultEngine()
	if err != nil {
		return res
	}
	rep, pErr := engine.ParseAndEvaluate(context.Background(), path, content)
	if pErr != nil || rep == nil {
		return res
	}
	for _, fn := range rep.Functions {
		res[fn.Name] = fn.Documented
	}
	return res
}

func collectBaselineStringLiterals(ctx *VerifyContext, targetDir, path string, fset *token.FileSet, baseCounts map[string]map[string]int) {
	baseBytes, ok := loadBaselineFileBytes(ctx, targetDir, path)
	if !ok {
		return
	}
	if !strings.HasSuffix(path, ".go") {
		collectPolyglotStringLiteralsFromBytes(path, baseBytes, baseCounts)
		return
	}
	collectGoStringLiteralsFromSrc(fset, path, baseBytes, baseCounts)
}

func filterRatchetedForbiddenLiterals(ctx *VerifyContext, root, path string, patterns, cur []string) []string {
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		rel = path
	}
	return filterNetNewLineViolations(ctx, root, path, cur, func(baseBytes []byte) []string {
		return scanTempBaselineViolations(path, baseBytes, func(tmpPath string) []string {
			raw := scanFileForLiterals(filepath.Dir(tmpPath), tmpPath, patterns)
			tmpBase := filepath.Base(tmpPath)
			for i, item := range raw {
				raw[i] = strings.Replace(item, tmpBase, rel, 1)
			}
			return raw
		})
	})
}
