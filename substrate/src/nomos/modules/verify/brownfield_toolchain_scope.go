// Package verify implements brownfield toolchain scoping and baseline coverage ratcheting.
package verify

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func resolveEffectiveCoverageThreshold(ctx *VerifyContext, root, targetDir string, configured, curAvg float64) float64 {
	if curAvg >= configured || !isBrownfieldGitScope(ctx, root) {
		return configured
	}
	if !isWorktreeContext(ctx, root) {
		if len(getModifiedFilesInScope(ctx, root)) == 0 {
			return curAvg
		}
		return configured
	}
	baseTargetDir := resolveBaselineGoTargetDir(ctx, root, targetDir)
	if baseTargetDir == "" {
		return configured
	}
	pkgs := resolveGoTestPackages(ctx, root, targetDir)
	baseRec := executeGoPackagesCoverage(ctx.RepoRoot, baseTargetDir, pkgs)
	if baseRec != nil && baseRec.GoPassed && baseRec.AvgCoverage < configured {
		return baseRec.AvgCoverage
	}
	return configured
}

func isBrownfieldGitScope(ctx *VerifyContext, root string) bool {
	if !isBrownfieldRatchetActive(ctx) {
		return false
	}
	if hasGitMetadata(root) {
		return true
	}
	return ctx != nil && hasGitMetadata(ctx.RepoRoot)
}

func resolveBaselineGoTargetDir(ctx *VerifyContext, root, targetDir string) string {
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
	baseCtx := &VerifyContext{RepoRoot: repoRoot, IsBaseline: true}
	return executeGoTestWithPackages(baseCtx, repoRoot, targetDir, "", pkgs)
}

func resolveModifiedGoPackages(ctx *VerifyContext, root, targetDir string) []string {
	if !isBrownfieldRatchetActive(ctx) || !isWorktreeContext(ctx, root) {
		return nil
	}
	modified := getModifiedFilesInScope(ctx, root)
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
	expandModifiedGoCallerPackages(root, targetDir, modified, pkgSet)
	out := make([]string, 0, len(pkgSet))
	for p := range pkgSet {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func expandModifiedGoCallerPackages(root, targetDir string, modified []string, pkgSet map[string]struct{}) {
	goReverse := buildGoReverseImportIndex(root)
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
	for _, callerDir := range lookupGoReverseImporterDirs(root, pkgDir, goReverse) {
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

func resolvePythonBinary(root, name string) (string, error) {
	for _, venvDir := range []string{".venv", "venv"} {
		candidate := filepath.Join(root, venvDir, "bin", name)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, nil
		}
	}
	return exec.LookPath(name)
}

func resolveScopedPytestTargets(ctx *VerifyContext, root string) ([]string, bool) {
	if !isBrownfieldRatchetActive(ctx) || !isWorktreeContext(ctx, root) {
		return nil, true
	}
	modified := getModifiedFilesInScope(ctx, root)
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

func resolveNomosInternalCLIDir(ctx *VerifyContext, root string) string {
	subCLI := filepath.Join(root, "substrate", "cli")
	if _, err := os.Stat(subCLI); err == nil {
		return subCLI
	}
	if isBrownfieldRatchetActive(ctx) {
		return ""
	}
	topCLI := filepath.Join(root, "cli")
	if _, err := os.Stat(topCLI); err == nil {
		return topCLI
	}
	return ""
}
