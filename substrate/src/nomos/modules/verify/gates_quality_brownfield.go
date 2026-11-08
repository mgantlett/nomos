// Package verify implements brownfield scoping for TDD, coverage, security, and broken-wire gates.
package verify

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	if !isBrownfieldRatchetActive(ctx) {
		return false
	}
	if isWorktreeContext(ctx, root) {
		return true
	}
	return hasGitMetadata(root)
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
		if hasGitMetadata(root) || (ctx != nil && hasGitMetadata(ctx.RepoRoot)) {
			return nil, true
		}
		return nil, false
	}
	goDirs := make(map[string]struct{})
	var polyFiles []string
	for _, rel := range modified {
		if !isPolyglotTargetFile(rel) {
			continue
		}
		absDir := filepath.Dir(filepath.Join(root, rel))
		if !shouldInspectModifiedPkgDir(root, absDir, excludes, skipCmd) {
			continue
		}
		if strings.HasSuffix(rel, ".go") {
			goDirs[absDir] = struct{}{}
		} else if isPolyglotTDDTrackedExt(filepath.Ext(rel)) {
			polyFiles = append(polyFiles, rel)
		}
	}
	var untested []string
	for dir := range goDirs {
		hasGo, hasTest := inspectPackageFiles(dir)
		if hasGo && !hasTest {
			untested = append(untested, relativeOrFullPath(root, dir))
		}
	}
	untested = append(untested, findUntestedPolyglotModules(root, modified, polyFiles)...)
	sort.Strings(untested)
	return untested, true
}

func isPolyglotTDDTrackedExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".py", ".ts", ".tsx", ".js", ".jsx", ".rs":
		return true
	}
	return false
}

func findUntestedPolyglotModules(root string, modified, polyFiles []string) []string {
	if len(polyFiles) == 0 || !hasPolyglotTestManifest(root) {
		return nil
	}
	if hasModifiedTestFile(modified) || hasRepositoryPolyglotTests(root) {
		return nil
	}
	seen := make(map[string]struct{})
	var missing []string
	for _, rel := range polyFiles {
		absPath := filepath.Join(root, rel)
		if strings.HasSuffix(rel, ".rs") && hasInlineRustTestModule(absPath) {
			continue
		}
		dirRel := filepath.ToSlash(filepath.Dir(rel))
		if _, ok := seen[dirRel]; !ok {
			seen[dirRel] = struct{}{}
			missing = append(missing, dirRel)
		}
	}
	return missing
}

func hasPolyglotTestManifest(root string) bool {
	for _, m := range []string{"pyproject.toml", "pytest.ini", "tox.ini", "Cargo.toml"} {
		if _, err := os.Stat(filepath.Join(root, m)); err == nil {
			return true
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		return strings.Contains(string(data), "\"test\"")
	}
	return false
}

func hasModifiedTestFile(modified []string) bool {
	for _, rel := range modified {
		if isPolyglotTestFilePath(rel) {
			return true
		}
	}
	return false
}

func isPolyglotTestFilePath(rel string) bool {
	slash := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(slash)
	if strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py") || strings.HasSuffix(base, "_test.go") {
		return true
	}
	for _, suf := range []string{".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx", ".test.js", ".spec.js"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	return strings.HasPrefix(slash, "tests/") || strings.HasPrefix(slash, "test/") || strings.Contains(slash, "/__tests__/")
}

func hasRepositoryPolyglotTests(root string) bool {
	for _, d := range []string{"tests", "test", "__tests__", "spec"} {
		if fi, err := os.Stat(filepath.Join(root, d)); err == nil && fi.IsDir() {
			return true
		}
	}
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || found {
			return nil
		}
		if d.IsDir() {
			if path != root && isSkippableSecurityDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isPolyglotTestFilePath(d.Name()) {
			found = true
		}
		return nil
	})
	return found
}

func hasInlineRustTestModule(absPath string) bool {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "#[cfg(test)]") || strings.Contains(string(data), "#[test]")
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

func scanSecretsScoped(ctx *VerifyContext, root string) ([]string, error) {
	if !isBrownfieldRatchetActive(ctx) || (ctx != nil && ctx.AllFiles) || !hasGitMetadata(root) {
		return scanSecrets(root)
	}
	leaks := scanUnignoredEnvFiles(root)
	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		if isIgnoredSecretFile(path) || (isProhibitedEnvFileName(info.Name()) && isGitIgnoredPath(root, path)) {
			return nil
		}
		if data, rErr := os.ReadFile(path); rErr == nil {
			if label := matchSecretTokenLabel(string(data)); label != "" {
				leaks = append(leaks, relativeOrFullPath(root, path)+" ("+label+")")
			}
		}
		return nil
	})
	return leaks, err
}

func scanBrokenLinksScoped(ctx *VerifyContext, root string) ([]string, error) {
	if !isBrownfieldRatchetActive(ctx) || (ctx != nil && ctx.AllFiles) || !hasGitMetadata(root) {
		return scanBrokenLinks(root)
	}
	var broken []string
	linkRegex := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		if data, rErr := os.ReadFile(path); rErr == nil {
			broken = append(broken, checkLinksInContent(root, path, string(data), linkRegex)...)
		}
		return nil
	})
	return broken, err
}
