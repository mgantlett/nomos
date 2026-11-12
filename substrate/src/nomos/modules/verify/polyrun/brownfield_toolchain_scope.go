// Package polyrun implements brownfield toolchain scoping and baseline coverage ratcheting.
package polyrun

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveEffectiveCoverageThreshold executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ResolveEffectiveCoverageThreshold(ctx *vscope.VerifyContext, root, targetDir string, configured, curAvg float64) float64 {
	if curAvg >= configured || !IsBrownfieldGitScope(ctx, root) {
		return configured
	}
	if !vscope.IsWorktreeContext(ctx, root) {
		if len(vscope.GetModifiedFilesInScope(ctx, root)) == 0 {
			return curAvg
		}
		return configured
	}
	baseTargetDir := resolveBaselineGoTargetDir(ctx, root, targetDir)
	if baseTargetDir == "" {
		return configured
	}
	pkgs := ResolveGoTestPackages(ctx, root, targetDir)
	baseRec := executeGoPackagesCoverage(ctx.RepoRoot, baseTargetDir, pkgs)
	if baseRec != nil && baseRec.GoPassed && baseRec.AvgCoverage < configured {
		return baseRec.AvgCoverage
	}
	return configured
}

// IsBrownfieldGitScope executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func IsBrownfieldGitScope(ctx *vscope.VerifyContext, root string) bool {
	if !vscope.IsBrownfieldRatchetActive(ctx) {
		return false
	}
	if vscope.HasGitMetadata(root) {
		return true
	}
	return ctx != nil && vscope.HasGitMetadata(ctx.RepoRoot)
}

func resolveBaselineGoTargetDir(ctx *vscope.VerifyContext, root, targetDir string) string {
	if ctx == nil || ctx.RepoRoot == "" || ctx.RepoRoot == root {
		return ""
	}
	relTarget, err := filepath.Rel(root, targetDir)
	if err != nil || strings.HasPrefix(relTarget, "..") {
		return ""
	}
	baseTargetDir := filepath.Join(ctx.RepoRoot, relTarget)
	fi, sErr := os.Stat(baseTargetDir)
	if sErr != nil || !fi.IsDir() {
		return ""
	}
	return baseTargetDir
}

func executeGoPackagesCoverage(repoRoot, targetDir string, pkgs []string) *ToolchainSuiteReceipt {
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}
	baseCtx := &vscope.VerifyContext{RepoRoot: repoRoot, IsBaseline: true}
	return executeGoTestWithPackages(baseCtx, repoRoot, targetDir, "", pkgs)
}

func resolveModifiedGoPackages(ctx *vscope.VerifyContext, root, targetDir string) []string {
	if !vscope.IsBrownfieldRatchetActive(ctx) || !vscope.IsWorktreeContext(ctx, root) {
		return nil
	}
	modified := vscope.GetModifiedFilesInScope(ctx, root)
	if len(modified) == 0 {
		return nil
	}
	pkgSet := make(map[string]struct{})
	for _, rel := range modified {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		absPkg := filepath.Dir(filepath.Join(root, rel))
		if fi, err := os.Stat(absPkg); err != nil || !fi.IsDir() {
			continue
		}
		relPkg, rErr := filepath.Rel(targetDir, absPkg)
		if rErr != nil || strings.HasPrefix(relPkg, "..") {
			continue
		}
		slash := filepath.ToSlash(relPkg)
		if slash == "." || slash == "" {
			pkgSet["."] = struct{}{}
		} else {
			pkgSet["./"+slash] = struct{}{}
		}
	}
	if len(pkgSet) == 0 {
		return nil
	}
	ExpandModifiedGoCallerPackages(root, targetDir, modified, pkgSet)
	out := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ExpandModifiedGoCallerPackages executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ExpandModifiedGoCallerPackages(root, targetDir string, modified []string, pkgSet map[string]struct{}) {
	goReverse := impact.BuildGoReverseImportIndex(root)
	if len(goReverse) == 0 {
		return
	}
	seenPkgDirs := make(map[string]bool)
	for _, rel := range modified {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		pkgDir := filepath.ToSlash(filepath.Dir(rel))
		if seenPkgDirs[pkgDir] {
			continue
		}
		seenPkgDirs[pkgDir] = true
		addReverseCallerTargetPkgs(root, targetDir, pkgDir, goReverse, pkgSet)
	}
}

func addReverseCallerTargetPkgs(root, targetDir, pkgDir string, goReverse map[string][]string, pkgSet map[string]struct{}) {
	for _, callerDir := range impact.LookupGoReverseImporterDirs(root, pkgDir, goReverse) {
		absCaller := filepath.Join(root, filepath.FromSlash(callerDir))
		relPkg, err := filepath.Rel(targetDir, absCaller)
		if err != nil || strings.HasPrefix(relPkg, "..") {
			continue
		}
		slash := filepath.ToSlash(relPkg)
		if slash == "." || slash == "" {
			pkgSet["."] = struct{}{}
		} else {
			pkgSet["./"+slash] = struct{}{}
		}
	}
}

// ResolvePythonBinary executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ResolvePythonBinary(root, name string) (string, error) {
	for _, venvDir := range []string{".venv", "venv"} {
		candidate := filepath.Join(root, venvDir, "bin", name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	return exec.LookPath(name)
}

// ResolveScopedPytestTargets executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ResolveScopedPytestTargets(ctx *vscope.VerifyContext, root string) ([]string, bool) {
	if !vscope.IsBrownfieldRatchetActive(ctx) || !vscope.IsWorktreeContext(ctx, root) {
		return nil, true
	}
	modified := vscope.GetModifiedFilesInScope(ctx, root)
	if len(modified) == 0 {
		return nil, false
	}
	targets := make(map[string]struct{})
	for _, rel := range modified {
		slash := filepath.ToSlash(rel)
		base := filepath.Base(slash)
		if (strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py")) || strings.HasSuffix(base, "_test.py") {
			if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
				targets[slash] = struct{}{}
			}
			continue
		}
		if strings.HasSuffix(base, ".py") && base != "__init__.py" {
			stem := strings.TrimSuffix(base, ".py")
			for _, cand := range []string{
				filepath.Join("tests", "test_"+stem+".py"),
				filepath.Join("tests", stem+"_test.py"),
				filepath.Join("test", "test_"+stem+".py"),
				filepath.Join(filepath.Dir(rel), "test_"+stem+".py"),
			} {
				if _, err := os.Stat(filepath.Join(root, cand)); err == nil {
					targets[filepath.ToSlash(cand)] = struct{}{}
				}
			}
		}
	}
	if len(targets) == 0 {
		return nil, false
	}
	list := make([]string, 0, len(targets))
	for t := range targets {
		list = append(list, t)
	}
	sort.Strings(list)
	return list, true
}

// ResolveNomosInternalCLIDir executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ResolveNomosInternalCLIDir(ctx *vscope.VerifyContext, root string) string {
	subCLI := filepath.Join(root, "substrate", "cli")
	if _, err := os.Stat(subCLI); err == nil {
		return subCLI
	}
	if vscope.IsBrownfieldRatchetActive(ctx) {
		return ""
	}
	topCLI := filepath.Join(root, "cli")
	if _, err := os.Stat(topCLI); err == nil {
		return topCLI
	}
	return ""
}
