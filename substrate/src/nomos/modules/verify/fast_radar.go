// Package verify implements the sub-15ms single-file AST and security early-warning radar.
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if rootRel, rErr := filepath.Rel(repoRoot, absPath); rErr == nil && rootRel != "" && !strings.HasPrefix(rootRel, "..") {
		res.File = filepath.ToSlash(rootRel)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return res, nil
	}

	ctx := &VerifyContext{RepoRoot: repoRoot, WorktreePath: filepath.Dir(absPath)}
	evaluateFastSecretAndEnv(repoRoot, absPath, res.File, string(data), res)
	evaluateFastForbiddenLiterals(ctx, repoRoot, absPath, res)

	if !isSupportedPolyglotSourceFile(absPath) || strings.HasSuffix(absPath, "_test.go") {
		return res, nil
	}
	evaluateFastASTBudgets(ctx, absPath, data, res)
	return res, nil
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

func evaluateFastForbiddenLiterals(ctx *VerifyContext, repoRoot, absPath string, res *FastFileRadarResult) {
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
	for _, hit := range scanFileForLiterals(repoRoot, absPath, clean) {
		res.Breaches = append(res.Breaches, hit)
	}
}

func evaluateFastASTBudgets(ctx *VerifyContext, absPath string, data []byte, res *FastFileRadarResult) {
	t := resolveThresholds(ctx)
	rep, err := AnalyzeFileDebt(absPath, data, t.MaxComplexity, t.MaxFuncLOC)
	if err != nil || rep == nil {
		return
	}
	res.TotalLOC = rep.TotalLOC
	res.DebtScore = rep.DebtScore

	checkBudgetMetric("file LOC", rep.TotalLOC, t.MaxFileLOC, res)
	if rep.DebtScore > 0 {
		checkBudgetMetric("cognitive debt", rep.DebtScore, t.MaxCDS, res)
	}
	for _, fn := range rep.Functions {
		checkBudgetMetric(fmt.Sprintf("func %s LOC", fn.Name), fn.LOC, t.MaxFuncLOC, res)
		checkBudgetMetric(fmt.Sprintf("func %s complexity", fn.Name), fn.Complexity, t.MaxComplexity, res)
	}
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
