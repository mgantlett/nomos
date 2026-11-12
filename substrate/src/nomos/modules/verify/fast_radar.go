// Package verify implements the sub-15ms single-file AST and security early-warning radar.
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
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
	absPath := filePath
	if abs, err := filepath.Abs(filePath); err == nil {
		absPath = abs
	}
	res := &FastFileRadarResult{File: absPath}

	fi, err := os.Stat(absPath)
	if err != nil || fi.IsDir() {
		return res, nil
	}
	wtRoot := resolveRadarWorktreeRoot(repoRoot, absPath)
	if rootRel, rErr := filepath.Rel(wtRoot, absPath); rErr == nil && rootRel != "" && !strings.HasPrefix(rootRel, "..") {
		res.File = filepath.ToSlash(rootRel)
	} else if rootRel, rErr := filepath.Rel(repoRoot, absPath); rErr == nil && rootRel != "" && !strings.HasPrefix(rootRel, "..") {
		res.File = filepath.ToSlash(rootRel)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return res, nil
	}

	ctx := &VerifyContext{RepoRoot: repoRoot, WorktreePath: wtRoot}
	evaluateFastSecretAndEnv(wtRoot, absPath, res.File, string(data), res)
	evaluateFastForbiddenLiterals(ctx, wtRoot, absPath, res)
	evaluateFastArchitectureBoundaries(ctx, wtRoot, absPath, res)

	if !isSupportedPolyglotSourceFile(absPath) || isVerificationSuiteFile(filepath.Base(absPath)) {
		return res, nil
	}
	evaluateFastCWESinks(ctx, wtRoot, absPath, res.File, data, res)
	evaluateFastIntraPackageClones(ctx, wtRoot, absPath, data, res)
	evaluateFastASTBudgets(ctx, wtRoot, absPath, data, res)
	return res, nil
}

func evaluateFastArchitectureBoundaries(ctx *VerifyContext, wtRoot, absPath string, res *FastFileRadarResult) {
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil || len(cfg.Governance.QualityGates.ArchitectureBoundaries) == 0 {
		return
	}
	violations := EvaluateArchitectureBoundaries(wtRoot, []string{absPath}, cfg.Governance.QualityGates.ArchitectureBoundaries)
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
	if isProhibitedEnvFileName(base) && !isGitIgnoredPath(repoRoot, absPath) {
		res.Breaches = append(res.Breaches, fmt.Sprintf("unignored credential file %s detected (SecretGate)", relFile))
	}
	if isIgnoredSecretFile(absPath) {
		return
	}
	if label := matchSecretTokenLabel(content); label != "" {
		res.Breaches = append(res.Breaches, fmt.Sprintf("potential %s detected in %s (SecretGate)", label, relFile))
	}
}

func evaluateFastForbiddenLiterals(ctx *VerifyContext, wtRoot, absPath string, res *FastFileRadarResult) {
	if !isScannableSourceFile(absPath) {
		return
	}
	cfg := resolveVerifyConfig(ctx)
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

func evaluateFastASTBudgets(ctx *VerifyContext, wtRoot, absPath string, data []byte, res *FastFileRadarResult) {
	t := resolveThresholds(ctx)
	rep, err := AnalyzeFileDebt(absPath, data, t.MaxComplexity, t.MaxFuncLOC)
	if err != nil || rep == nil {
		return
	}
	res.TotalLOC = rep.TotalLOC
	res.DebtScore = rep.DebtScore

	if isBrownfieldRatchetActive(ctx) {
		if baseBytes, ok := loadBaselineFileBytes(ctx, wtRoot, absPath); ok {
			if baseRep, bErr := AnalyzeFileDebt(absPath, baseBytes, t.MaxComplexity, t.MaxFuncLOC); bErr == nil && baseRep != nil {
				evaluateRatchetedFastBudgets(rep, baseRep, t, res)
				return
			}
		}
	}

	checkBudgetMetric("file LOC", rep.TotalLOC, t.MaxFileLOC, res)
	if rep.DebtScore > 0 {
		checkBudgetMetric("cognitive debt", rep.DebtScore, t.MaxCDS, res)
	}
	for _, fn := range rep.Functions {
		checkBudgetMetric(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, t.MaxFuncLOC, res)
		checkBudgetMetric(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, t.MaxComplexity, res)
	}
}

func evaluateRatchetedFastBudgets(rep, baseRep *FileDebtReport, t Thresholds, res *FastFileRadarResult) {
	checkRatchetedMetric("file LOC", rep.TotalLOC, baseRep.TotalLOC, t.MaxFileLOC, res)
	if rep.DebtScore > 0 {
		checkRatchetedMetric("cognitive debt", rep.DebtScore, baseRep.DebtScore, t.MaxCDS, res)
	}
	baseFns := indexBaselineFunctionDebts(baseRep.Functions)
	for _, fn := range rep.Functions {
		bfn, existed := baseFns[fn.Name]
		if !existed {
			checkBudgetMetric(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, t.MaxFuncLOC, res)
			checkBudgetMetric(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, t.MaxComplexity, res)
			continue
		}
		checkRatchetedMetric(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, bfn.LOC, t.MaxFuncLOC, res)
		checkRatchetedMetric(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, bfn.Complexity, t.MaxComplexity, res)
	}
}

func indexBaselineFunctionDebts(funcs []FunctionDebt) map[string]FunctionDebt {
	byName := make(map[string]FunctionDebt, len(funcs))
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

func checkRatchetedMetric(label string, val, baseVal, limit int, res *FastFileRadarResult) {
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
	checkBudgetMetric(label, val, limit, res)
}

func checkBudgetMetric(label string, val, limit int, res *FastFileRadarResult) {
	if limit <= 0 {
		return
	}
	if val > limit {
		res.Breaches = append(res.Breaches, fmt.Sprintf("%s %d/%d (>100%% limit)", label, val, limit))
		return
	}
	if val*100 >= limit*80 {
		pct := (val * 100) / limit
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s %d/%d (%d%% budget)", label, val, limit, pct))
	}
}
