// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func resolveSubstrateDirs(ctx *VerifyContext, root string) []string {
	dirs, explicit := resolveSubstrateTargetDirs(ctx, root)
	if explicit {
		return dirs
	}
	return dirs
}

func resolveSubstrateTargetDirs(ctx *VerifyContext, root string) ([]string, bool) {
	cfg := resolveVerifyConfig(ctx)
	if cfg != nil && len(cfg.Governance.QualityGates.SubstrateSourceRoots) > 0 {
		var dirs []string
		for _, sr := range cfg.Governance.QualityGates.SubstrateSourceRoots {
			cleanSR := filepath.Clean(sr)
			candidate := cleanSR
			if !filepath.IsAbs(cleanSR) {
				candidate = filepath.Join(root, cleanSR)
			}
			if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
				dirs = append(dirs, candidate)
			}
		}
		if len(dirs) > 0 {
			return dirs, true
		}
	}
	fallback := findSubstrateSrc(root)
	if fallback == "" {
		return nil, false
	}
	return []string{fallback}, false
}

func isSkippedTDDDirName(name string, skipCmdDir bool) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "testdata", "vendor", "node_modules":
		return true
	case "cmd":
		return skipCmdDir
	}
	return false
}

func shouldScopeTDDToModifiedPackages(ctx *VerifyContext, root string) bool {
	if ctx == nil || ctx.AllFiles || ctx.IsBaseline {
		return false
	}
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil || cfg.Governance.QualityGates.StrictMode {
		return false
	}
	return isWorktreeContext(ctx, root)
}

func findUntestedPackagesScoped(ctx *VerifyContext, root string, targetDirs []string, explicitRoots bool) ([]string, error) {
	excludes := resolveScopeExcludes(ctx, root, root)
	if shouldScopeTDDToModifiedPackages(ctx, root) {
		if modUntested, handled := evaluateModifiedPackageTDD(ctx, root, excludes, !explicitRoots); handled {
			return modUntested, nil
		}
	}

	seen := make(map[string]struct{})
	var untested []string
	for _, srcDir := range targetDirs {
		pkgs, err := walkUntestedPackages(root, srcDir, excludes, !explicitRoots)
		if err != nil {
			return nil, err
		}
		for _, p := range pkgs {
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				untested = append(untested, p)
			}
		}
	}
	sort.Strings(untested)
	return untested, nil
}

func evaluateModifiedPackageTDD(ctx *VerifyContext, root string, excludes []string, skipCmd bool) ([]string, bool) {
	modified := getModifiedFilesInScope(ctx, root)
	if len(modified) == 0 {
		return nil, false
	}
	pkgDirs := make(map[string]struct{})
	for _, rel := range modified {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		absDir := filepath.Dir(filepath.Join(root, rel))
		if shouldInspectModifiedPkgDir(root, absDir, excludes, skipCmd) {
			pkgDirs[absDir] = struct{}{}
		}
	}
	if len(pkgDirs) == 0 {
		return nil, true
	}
	var untested []string
	for dir := range pkgDirs {
		hasGo, hasTest := inspectPackageFiles(dir)
		if hasGo && !hasTest {
			untested = append(untested, relativeOrFullPath(root, dir))
		}
	}
	sort.Strings(untested)
	return untested, true
}

func shouldInspectModifiedPkgDir(root, absDir string, excludes []string, skipCmd bool) bool {
	fi, err := os.Stat(absDir)
	if err != nil || !fi.IsDir() {
		return false
	}
	if workspace.IsExcluded(absDir, true, fi.Name(), excludes) {
		return false
	}
	rel := relativeOrFullPath(root, absDir)
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part != "." && part != "" && isSkippedTDDDirName(part, skipCmd) {
			return false
		}
	}
	return true
}

func walkUntestedPackages(root, srcDir string, excludes []string, skipCmd bool) ([]string, error) {
	var untested []string
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || !info.IsDir() {
			return nil
		}
		if workspace.IsExcluded(path, true, info.Name(), excludes) {
			return filepath.SkipDir
		}
		if path != srcDir && isSkippedTDDDirName(info.Name(), skipCmd) {
			return filepath.SkipDir
		}
		hasGo, hasTest := inspectPackageFiles(path)
		if hasGo && !hasTest {
			base := srcDir
			if root != "" && srcDir != root {
				base = root
			}
			untested = append(untested, relativeOrFullPath(base, path))
		}
		return nil
	})
	return untested, err
}

func relativeOrFullPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}
