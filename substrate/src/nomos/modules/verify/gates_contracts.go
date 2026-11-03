// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// FileMetric records lines of code and cognitive debt score for a file (#FileMetric).
type FileMetric struct {
	Name string `json:"name"`
	LOC  int    `json:"loc"`
	CDS  int    `json:"cds"`
}

// FunctionMetric records complexity and LOC for an individual function (#FunctionMetric).
type FunctionMetric struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Complexity int    `json:"complexity"`
	LOC        int    `json:"loc"`
}

// Thresholds captures the numerical quality bounds for gates (#Thresholds).
type Thresholds struct {
	MinCoverage               float64 `json:"min_coverage"`
	MinDocstringDensity        int     `json:"min_docstring_density"`
	MinTraceabilityDensity     int     `json:"min_traceability_density"`
	MaxCDS                    int     `json:"max_cds"`
	MaxComplexity             int     `json:"max_complexity"`
	MaxFuncLOC                int     `json:"max_func_loc"`
	MaxFileLOC                int     `json:"max_file_loc"`
	MaxFuncParams             int     `json:"max_func_params"`
	MinCloneLines             int     `json:"min_clone_lines"`
	MaxMagicStringDuplication int     `json:"max_magic_string_duplication"`
}

// DefaultThresholds returns canonical statutory quality gate thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinCoverage:               55.0,
		MinDocstringDensity:       10,
		MinTraceabilityDensity:    50,
		MaxCDS:                    45,
		MaxComplexity:             15,
		MaxFuncLOC:                60,
		MaxFileLOC:                400,
		MaxFuncParams:             8,
		MinCloneLines:             18,
		MaxMagicStringDuplication: 2,
	}
}

// resolveThresholds merges statutory DefaultThresholds with active nomos.yaml overrides.
func resolveThresholds(ctx *VerifyContext) Thresholds {
	out := DefaultThresholds()
	out.MaxMagicStringDuplication = 10
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil {
		return out
	}
	ct := cfg.Governance.QualityGates.Thresholds
	if ct.MaxFileLOC > 0 {
		out.MaxFileLOC = ct.MaxFileLOC
	}
	if ct.MaxFuncLOC > 0 {
		out.MaxFuncLOC = ct.MaxFuncLOC
	}
	if ct.MaxCyclomaticComplexity > 0 {
		out.MaxComplexity = ct.MaxCyclomaticComplexity
	}
	if ct.MaxCognitiveDebtScore > 0 {
		out.MaxCDS = ct.MaxCognitiveDebtScore
	}
	if ct.MinCommentDensityPct >= 0 {
		out.MinDocstringDensity = ct.MinCommentDensityPct
	}
	if ct.MinTestCoveragePct >= 0 {
		out.MinCoverage = float64(ct.MinTestCoveragePct)
	}
	if ct.MinCloneLines > 0 {
		out.MinCloneLines = ct.MinCloneLines
	}
	if ct.MaxFuncParams > 0 {
		out.MaxFuncParams = ct.MaxFuncParams
	}
	if ct.MaxMagicStringDuplication > 0 {
		out.MaxMagicStringDuplication = ct.MaxMagicStringDuplication
	}
	return out
}

func scanProhibitedCommands(root string, prohibited []string) []string {
	if len(prohibited) == 0 {
		return nil
	}
	banned := make(map[string]bool, len(prohibited))
	for _, p := range prohibited {
		if clean := strings.TrimSpace(p); clean != "" {
			banned[clean] = true
		}
	}
	cliDirs := []string{filepath.Join(root, "substrate", "cli"), filepath.Join(root, "cli")}
	fset := token.NewFileSet()
	var violations []string
	for _, dir := range cliDirs {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			violations = append(violations, scanFileProhibitedCobraUse(fset, path, banned)...)
			return nil
		})
	}
	return violations
}

func scanFileProhibitedCobraUse(fset *token.FileSet, path string, banned map[string]bool) []string {
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	var out []string
	ast.Inspect(node, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != "Use" {
			return true
		}
		lit, ok := kv.Value.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		fields := strings.Fields(strings.Trim(lit.Value, "`\""))
		if len(fields) > 0 && banned[fields[0]] {
			out = append(out, fmt.Sprintf("%s: registers prohibited CLI command %q", path, fields[0]))
		}
		return true
	})
	return out
}

// EvaluateCoverageViolations checks if test coverage meets minimum threshold.
func EvaluateCoverageViolations(coverage, minCoverage float64) []string {
	if coverage < minCoverage {
		return []string{fmt.Sprintf("coverage %.1f%% failed test execution or is below threshold %.1f%%", coverage, minCoverage)}
	}
	return nil
}

// EvaluateCDSViolations checks if files exceed maximum cognitive debt.
func EvaluateCDSViolations(files []FileMetric, maxCDS int) []string {
	var violations []string
	for _, f := range files {
		if f.CDS > maxCDS {
			violations = append(violations, fmt.Sprintf("%s has cognitive debt %d (exceeds max %d)", f.Name, f.CDS, maxCDS))
		}
	}
	return violations
}

// EvaluateDocstringViolations checks docstring density threshold.
func EvaluateDocstringViolations(density, minDensity, documented, total int, missing []string) []string {
	if density < minDensity {
		if len(missing) > 0 {
			return []string{fmt.Sprintf("docstring density is %d%% (%d/%d), minimum required is %d%%\nMissing:\n    - %s", density, documented, total, minDensity, strings.Join(missing, "\n    - "))}
		}
		return []string{fmt.Sprintf("docstring density is %d%% (%d/%d), minimum required is %d%%", density, documented, total, minDensity)}
	}
	return nil
}


// EvaluateComplexityViolations checks cyclomatic complexity and function LOC.
func EvaluateComplexityViolations(funcs []FunctionMetric, maxComplexity, maxFuncLOC int) []string {
	var violations []string
	for _, fn := range funcs {
		if fn.Complexity > maxComplexity {
			violations = append(violations, fmt.Sprintf("%s:%d (%s) cyclomatic complexity %d exceeds max %d", fn.File, fn.Line, fn.Name, fn.Complexity, maxComplexity))
		}
		if fn.LOC > maxFuncLOC {
			violations = append(violations, fmt.Sprintf("%s:%d (%s) LOC %d exceeds max %d", fn.File, fn.Line, fn.Name, fn.LOC, maxFuncLOC))
		}
	}
	return violations
}

// EvaluateFileLOCViolations checks file LOC limits.
func EvaluateFileLOCViolations(files []FileMetric, maxFileLOC int) []string {
	var violations []string
	for _, f := range files {
		if f.LOC > maxFileLOC {
			violations = append(violations, fmt.Sprintf("%s has %d LOC (exceeds max %d)", f.Name, f.LOC, maxFileLOC))
		}
	}
	return violations
}

// EvaluateZeroFilesystemAliasViolations reports any detected symlink paths.
func EvaluateZeroFilesystemAliasViolations(symlinks []string) []string {
	if len(symlinks) > 0 {
		return []string{fmt.Sprintf("detected %d filesystem symlink(s); symlinks are strictly prohibited under Law 6: %s", len(symlinks), strings.Join(symlinks, "; "))}
	}
	return nil
}

// EvaluatePlaneViolations checks if a task mutated files across prohibited plane boundaries.
// It defaults to standard "intent" and "substrate" roots.
func EvaluatePlaneViolations(taskKey string, modifiedFiles []string) []string {
	return EvaluatePlaneViolationsWithRoots(taskKey, modifiedFiles, nil, nil)
}

// EvaluatePlaneViolationsWithRoots evaluates plane boundaries dynamically using declared roots (Law 5).
func EvaluatePlaneViolationsWithRoots(taskKey string, modifiedFiles []string, intentRoots, substrateRoots []string) []string {
	if len(intentRoots) == 0 {
		intentRoots = []string{"intent"}
	}
	if len(substrateRoots) == 0 {
		substrateRoots = []string{"substrate"}
	}

	var violations []string
	isSub := strings.HasPrefix(taskKey, "SUB-")
	isInt := strings.HasPrefix(taskKey, "INT-")

	matchesRoot := func(clean string, roots []string) bool {
		for _, r := range roots {
			rootClean := strings.Trim(filepath.ToSlash(r), "/")
			if rootClean == "" {
				continue
			}
			if clean == rootClean || strings.HasPrefix(clean, rootClean+"/") {
				return true
			}
		}
		return false
	}

	for _, f := range modifiedFiles {
		clean := strings.TrimPrefix(filepath.ToSlash(f), "/")
		if isSub && matchesRoot(clean, intentRoots) {
			violations = append(violations, fmt.Sprintf("Law 7 violation: Substrate task %s cannot modify intent plane files (detected: %s)", taskKey, f))
		}
		if isInt && matchesRoot(clean, substrateRoots) {
			violations = append(violations, fmt.Sprintf("Law 7 violation: Intent task %s cannot modify substrate plane files (detected: %s)", taskKey, f))
		}
	}
	return violations
}

// PlaneSeparationRemedy provides deterministic, actionable JIT instructions for Law 7 plane separation recovery.
func PlaneSeparationRemedy(taskKey string) string {
	targetPlane, oppPlane := "substrate", "intent"
	if strings.HasPrefix(taskKey, "INT-") {
		targetPlane, oppPlane = "intent", "substrate"
	}
	return fmt.Sprintf("Law 7 strictly forbids cross-plane mutations. Protocol Recovery Steps:\n"+
		"  1. Revert cross-plane edits in active %s worktree: git checkout -- <file>\n"+
		"  2. Provision a twin task in the %s plane: nomos task create \"<title>\" --plane %s\n"+
		"  3. Execute and sync the twin task in its isolated worktree: nomos task start <KEY>",
		targetPlane, oppPlane, oppPlane)
}


