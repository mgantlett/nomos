// Package verify implements 11-language TDD paired-test verification across strict, ratchet, diff, and off modes.
package verify

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

var tddImplIgnoredNames = map[string]bool{
	"__init__.py": true, "mod.rs": true, "lib.rs": true, "main.rs": true,
	"index.ts": true, "index.js": true, "types.ts": true, "constants.ts": true,
}

func resolveEffectiveTDDMode(ctx *VerifyContext) string {
	cfg := resolveVerifyConfig(ctx)
	if cfg != nil && strings.TrimSpace(cfg.Governance.QualityGates.TDDMode) != "" && (ctx == nil || !ctx.Strict) {
		return cfg.Governance.QualityGates.EffectiveTDDMode()
	}
	if ctx != nil && ctx.Strict {
		return config.TDDModeStrict
	}
	if cfg != nil {
		return cfg.Governance.QualityGates.EffectiveTDDMode()
	}
	return config.TDDModeStrict
}

func resolveEffectiveCoverageMode(ctx *VerifyContext) string {
	cfg := resolveVerifyConfig(ctx)
	if cfg != nil && strings.TrimSpace(cfg.Governance.QualityGates.CoverageMode) != "" && (ctx == nil || !ctx.Strict) {
		return cfg.Governance.QualityGates.EffectiveCoverageMode()
	}
	if cfg != nil && (ctx == nil || !ctx.Strict) {
		return cfg.Governance.QualityGates.EffectiveCoverageMode()
	}
	return config.CoverageModeLive
}

func hasGoPairedTestsForModified(root, absDir string, modified []string) bool {
	for _, rel := range modified {
		if !strings.HasSuffix(rel, extGo) || strings.HasSuffix(rel, suffixGoTest) {
			continue
		}
		if filepath.Clean(filepath.Dir(filepath.Join(root, rel))) != filepath.Clean(absDir) {
			continue
		}
		if !hasPairedPolyglotTest(root, rel) {
			return false
		}
	}
	return true
}

func isPolyglotImplFileForTDD(name string) bool {
	lower := strings.ToLower(name)
	if isPolyglotTestFilePath(lower) || strings.HasSuffix(lower, ".d.ts") || tddImplIgnoredNames[lower] {
		return false
	}
	return isPolyglotTDDTrackedExt(filepath.Ext(lower))
}

func inspectPolyglotDirForStrictTDD(root, dir string) (bool, bool) {
	if !hasPolyglotTestManifest(root) && !hasRepositoryPolyglotTests(root) {
		return false, false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, false
	}
	implFiles, hasLocalTest := classifyPolyglotDirEntries(entries)
	if len(implFiles) == 0 {
		return false, false
	}
	if hasLocalTest || hasAnyPairedPolyglotImpl(root, dir, implFiles) {
		return true, true
	}
	return true, false
}

func classifyPolyglotDirEntries(entries []os.DirEntry) ([]string, bool) {
	var implFiles []string
	hasLocalTest := false
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if name == "__tests__" || name == dirTests || name == dirTest {
				hasLocalTest = true
			}
			continue
		}
		if strings.HasSuffix(name, extGo) {
			continue
		}
		if isPolyglotTestFilePath(name) {
			hasLocalTest = true
		} else if isPolyglotImplFileForTDD(name) {
			implFiles = append(implFiles, name)
		}
	}
	return implFiles, hasLocalTest
}

func hasAnyPairedPolyglotImpl(root, dir string, implFiles []string) bool {
	for _, name := range implFiles {
		absFile := filepath.Join(dir, name)
		if strings.HasSuffix(name, extRS) && hasInlineRustTestModule(absFile) {
			return true
		}
		if rel, rErr := filepath.Rel(root, absFile); rErr == nil && hasPairedPolyglotTest(root, filepath.ToSlash(rel)) {
			return true
		}
	}
	return false
}

func hasPairedPolyglotTest(root, rel string) bool {
	status := resolvePairedTestStatus(root, rel)
	return strings.HasPrefix(status, pairedTestPrefix)
}

func evaluateBrownfieldPolyglotTDD(ctx *VerifyContext, root string, modified, polyFiles []string) []string {
	if len(polyFiles) == 0 {
		return nil
	}
	mode := resolveEffectiveTDDMode(ctx)
	hasModTest := hasModifiedTestFile(modified)
	hasRepoSuite := hasRepositoryPolyglotTests(root)
	hasManifest := hasPolyglotTestManifest(root)

	seen := make(map[string]struct{})
	var missing []string
	for _, rel := range polyFiles {
		if isPolyglotFileSatisfyingBrownfieldTDD(ctx, root, rel, mode, hasModTest, hasRepoSuite, hasManifest) {
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

func isPolyglotFileSatisfyingBrownfieldTDD(ctx *VerifyContext, root, rel, mode string, hasModTest, hasRepoSuite, hasManifest bool) bool {
	if !isPolyglotImplFileForTDD(filepath.Base(rel)) {
		return true
	}
	absPath := filepath.Join(root, rel)
	if strings.HasSuffix(rel, extRS) && hasInlineRustTestModule(absPath) {
		return true
	}
	if hasPairedPolyglotTest(root, rel) || hasModTest {
		return true
	}
	if mode == config.TDDModeDiff {
		return false
	}
	_, existedOnBaseline := loadBaselineFileBytes(ctx, root, absPath)
	if existedOnBaseline && (hasRepoSuite || !hasManifest) {
		return true
	}
	return !existedOnBaseline && !hasManifest && !hasRepoSuite
}
