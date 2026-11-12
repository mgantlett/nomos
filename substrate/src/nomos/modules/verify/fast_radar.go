// Package verify implements the sub-15ms single-file AST and security early-warning radar.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

const (
	defaultRadarWarnPct  = 80
	hotScopeRadarWarnPct = 65
)

// FastFileRadarResult reports single-file AST budget proximity and immediate breaches (#QualityGatesConfig).
type FastFileRadarResult struct {
	File      string   `json:"file"`
	TotalLOC  int      `json:"total_loc"`
	DebtScore int      `json:"debt_score"`
	Warnings  []string `json:"warnings,omitempty"`
	Breaches  []string `json:"breaches,omitempty"`
}

// EvaluateFastFileRadar analyzes a single file in <15ms for >=80% AST budget warnings and immediate breaches (@intent: #QualityGatesConfig).
func EvaluateFastFileRadar(repoRoot, filePath string) (*FastFileRadarResult, error) {
	absPath := resolveAbsRadarPath(filePath)
	res := &FastFileRadarResult{File: absPath}

	fi, err := os.Stat(absPath)
	if err != nil || fi.IsDir() {
		return res, nil
	}
	wtRoot := resolveRadarWorktreeRoot(repoRoot, absPath)
	res.File = resolveRelativeRadarFile(repoRoot, wtRoot, absPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return res, nil
	}

	ctx := &vscope.VerifyContext{RepoRoot: repoRoot, WorktreePath: wtRoot}
	evaluateFastSecretAndEnv(wtRoot, absPath, res.File, string(data), res)
	evaluateFastForbiddenLiterals(ctx, wtRoot, absPath, res)
	evaluateFastArchitectureBoundaries(ctx, wtRoot, absPath, res)

	if !vscope.IsSupportedPolyglotSourceFile(absPath) || impact.IsVerificationSuiteFile(filepath.Base(absPath)) {
		return res, nil
	}
	evaluateFastCWESinks(ctx, wtRoot, absPath, res.File, data, res)
	evaluateFastIntraPackageClones(ctx, wtRoot, absPath, data, res)
	warnPct := applyHotScopeRadarReflex(ctx, repoRoot, wtRoot, absPath, data, res)
	evaluateFastASTBudgets(ctx, wtRoot, absPath, data, warnPct, res)
	return res, nil
}

func resolveAbsRadarPath(filePath string) string {
	if abs, err := filepath.Abs(filePath); err == nil {
		return abs
	}
	return filePath
}

func resolveRelativeRadarFile(repoRoot, wtRoot, absPath string) string {
	if rootRel, err := filepath.Rel(wtRoot, absPath); err == nil && rootRel != "" && !strings.HasPrefix(rootRel, "..") {
		return filepath.ToSlash(rootRel)
	}
	if rootRel, err := filepath.Rel(repoRoot, absPath); err == nil && rootRel != "" && !strings.HasPrefix(rootRel, "..") {
		return filepath.ToSlash(rootRel)
	}
	return absPath
}

func applyHotScopeRadarReflex(ctx *vscope.VerifyContext, repoRoot, wtRoot, absPath string, data []byte, res *FastFileRadarResult) int {
	relDir := mem.DeriveDirectoryScope("", []string{res.File})
	if mem.ResolveHotScopeReflex(resolveRadarPrimaryRoot(repoRoot, wtRoot), relDir) == nil {
		return defaultRadarWarnPct
	}
	evaluateFastHotScopeProbes(ctx, absPath, res.File, data, res)
	return hotScopeRadarWarnPct
}

func evaluateFastArchitectureBoundaries(ctx *vscope.VerifyContext, wtRoot, absPath string, res *FastFileRadarResult) {
	cfg := vscope.ResolveVerifyConfig(ctx)
	if cfg == nil || len(cfg.Governance.QualityGates.ArchitectureBoundaries) == 0 {
		return
	}
	violations := impact.EvaluateArchitectureBoundaries(wtRoot, []string{absPath}, cfg.Governance.QualityGates.ArchitectureBoundaries)
	res.Breaches = append(res.Breaches, violations...)
}

func resolveRadarWorktreeRoot(repoRoot, absPath string) string {
	if wt, err := workspace.FindWorkingTreeRoot(filepath.Dir(absPath)); err == nil && wt != "" {
		return wt
	}
	if repoRoot != "" {
		return repoRoot
	}
	return filepath.Dir(absPath)
}

func evaluateFastSecretAndEnv(repoRoot, absPath, relFile, content string, res *FastFileRadarResult) {
	base := filepath.Base(absPath)
	if impact.IsProhibitedEnvFileName(base) && !impact.IsGitIgnoredPath(repoRoot, absPath) {
		res.Breaches = append(res.Breaches, fmt.Sprintf("unignored credential file %s detected (SecretGate)", relFile))
	}
	if isIgnoredSecretFile(absPath) {
		return
	}
	if label := impact.MatchSecretTokenLabel(content); label != "" {
		res.Breaches = append(res.Breaches, fmt.Sprintf("potential %s detected in %s (SecretGate)", label, relFile))
	}
}

func evaluateFastForbiddenLiterals(ctx *vscope.VerifyContext, wtRoot, absPath string, res *FastFileRadarResult) {
	if !isScannableSourceFile(absPath) {
		return
	}
	cfg := vscope.ResolveVerifyConfig(ctx)
	patterns := cfg.Governance.QualityGates.ForbiddenLiterals
	if len(patterns) == 0 && !cfg.Governance.QualityGates.HasForbiddenLiteralsOverride {
		patterns = defaultForbiddenLiterals()
	}
	var clean []string
	for _, p := range patterns {
		if trimmed := strings.Trim(p, "\"'"); trimmed != "" {
			clean = append(clean, trimmed)
		}
	}
	if len(clean) == 0 {
		return
	}
	hits := scanFileForLiterals(wtRoot, absPath, clean)
	hits = filterRatchetedForbiddenLiterals(ctx, wtRoot, absPath, clean, hits)
	res.Breaches = append(res.Breaches, hits...)
}

func evaluateFastASTBudgets(ctx *vscope.VerifyContext, wtRoot, absPath string, data []byte, warnPct int, res *FastFileRadarResult) {
	t := resolveThresholds(ctx)
	rep, err := astdebt.AnalyzeFileDebt(absPath, data, t.MaxComplexity, t.MaxFuncLOC)
	if err != nil || rep == nil {
		return
	}
	res.TotalLOC = rep.TotalLOC
	res.DebtScore = rep.DebtScore

	if vscope.IsBrownfieldRatchetActive(ctx) {
		if baseBytes, ok := loadBaselineFileBytes(ctx, wtRoot, absPath); ok {
			if baseRep, bErr := astdebt.AnalyzeFileDebt(absPath, baseBytes, t.MaxComplexity, t.MaxFuncLOC); bErr == nil && baseRep != nil {
				evaluateRatchetedFastBudgets(rep, baseRep, t, warnPct, res)
				return
			}
		}
	}

	checkBudgetMetricWithPct("file LOC", rep.TotalLOC, t.MaxFileLOC, warnPct, res)
	if rep.DebtScore > 0 {
		checkBudgetMetricWithPct("cognitive debt", rep.DebtScore, t.MaxCDS, warnPct, res)
	}
	for _, fn := range rep.Functions {
		checkBudgetMetricWithPct(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, t.MaxFuncLOC, warnPct, res)
		checkBudgetMetricWithPct(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, t.MaxComplexity, warnPct, res)
	}
}

func evaluateRatchetedFastBudgets(rep, baseRep *astdebt.FileDebtReport, t astdebt.Thresholds, warnPct int, res *FastFileRadarResult) {
	checkRatchetedMetricWithPct("file LOC", rep.TotalLOC, baseRep.TotalLOC, t.MaxFileLOC, warnPct, res)
	if rep.DebtScore > 0 {
		checkRatchetedMetricWithPct("cognitive debt", rep.DebtScore, baseRep.DebtScore, t.MaxCDS, warnPct, res)
	}
	baseFns := indexBaselineFunctionDebts(baseRep.Functions)
	for _, fn := range rep.Functions {
		bfn, existed := baseFns[fn.Name]
		if !existed {
			checkBudgetMetricWithPct(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, t.MaxFuncLOC, warnPct, res)
			checkBudgetMetricWithPct(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, t.MaxComplexity, warnPct, res)
			continue
		}
		checkRatchetedMetricWithPct(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, bfn.LOC, t.MaxFuncLOC, warnPct, res)
		checkRatchetedMetricWithPct(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, bfn.Complexity, t.MaxComplexity, warnPct, res)
	}
}

func indexBaselineFunctionDebts(funcs []astdebt.FunctionDebt) map[string]astdebt.FunctionDebt {
	byName := make(map[string]astdebt.FunctionDebt, len(funcs))
	for _, bfn := range funcs {
		prev, exists := byName[bfn.Name]
		if !exists {
			byName[bfn.Name] = bfn
			continue
		}
		prev.LOC = maxInt(prev.LOC, bfn.LOC)
		prev.Complexity = maxInt(prev.Complexity, bfn.Complexity)
		byName[bfn.Name] = prev
	}
	return byName
}

func checkRatchetedMetricWithPct(label string, val, baseVal, limit, warnPct int, res *FastFileRadarResult) {
	if limit <= 0 {
		return
	}
	ceil := maxInt(limit, baseVal)
	if val > ceil {
		res.Breaches = append(res.Breaches, fmt.Sprintf("%s %d/%d (exceeds brownfield baseline %d)", label, val, ceil, baseVal))
		return
	}
	if baseVal > limit {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s %d (brownfield baseline %d, delta %+d)", label, val, baseVal, val-baseVal))
		return
	}
	checkBudgetMetricWithPct(label, val, limit, warnPct, res)
}

func checkBudgetMetricWithPct(label string, val, limit, warnPct int, res *FastFileRadarResult) {
	if limit <= 0 {
		return
	}
	if val > limit {
		res.Breaches = append(res.Breaches, fmt.Sprintf("%s %d/%d (>100%% limit)", label, val, limit))
		return
	}
	if warnPct <= 0 {
		warnPct = defaultRadarWarnPct
	}
	if val*100 >= limit*warnPct {
		pct := (val * 100) / limit
		if warnPct < defaultRadarWarnPct {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s %d/%d (%d%% hot-scope >=%d%% budget)", label, val, limit, pct, warnPct))
			return
		}
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s %d/%d (%d%% budget)", label, val, limit, pct))
	}
}

func resolveRadarPrimaryRoot(repoRoot, wtRoot string) string {
	if pr, err := workspace.FindRepoRoot(wtRoot); err == nil && pr != "" {
		return pr
	}
	if repoRoot != "" {
		if pr, err := workspace.FindRepoRoot(repoRoot); err == nil && pr != "" {
			return pr
		}
		return repoRoot
	}
	return wtRoot
}

func evaluateFastHotScopeProbes(ctx *vscope.VerifyContext, absPath, relFile string, data []byte, res *FastFileRadarResult) {
	maxParams := resolveThresholds(ctx).MaxFuncParams
	if maxParams <= 0 {
		maxParams = astdebt.MaxFunctionParams
	}
	res.Breaches = append(res.Breaches, astdebt.EvaluateParameterArityViolations(relFile, data, maxParams)...)
	ext := strings.ToLower(filepath.Ext(absPath))
	if ext == ".go" {
		fset := token.NewFileSet()
		if node, err := parser.ParseFile(fset, absPath, data, 0); err == nil {
			checkUncheckedErrorsInFile(node, fset, relFile, &res.Breaches)
		}
		res.Breaches = append(res.Breaches, scanGoFileAntiShim(token.NewFileSet(), absPath, relFile)...)
		return
	}
	res.Breaches = append(res.Breaches, astdebt.CheckPolyglotUncheckedErrors(ext, relFile, data)...)
	res.Breaches = append(res.Breaches, scanPolyglotAntiShim(absPath, relFile)...)
}
