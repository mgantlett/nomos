// Package vscope implements configurable verify_scope and tdd_exclude matching for TDDGate.
package vscope

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

var nonLogicConfigSuffixes = []string{
	".config.ts", ".config.js", ".config.mjs", ".config.cjs", ".config.mts", ".config.cts",
}

// ResolveEffectiveVerifyScope executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func ResolveEffectiveVerifyScope(ctx *VerifyContext) string {
	if ctx != nil && ctx.AllFiles {
		return config.VerifyScopeRepo
	}
	cfg := ResolveVerifyConfig(ctx)
	if cfg == nil {
		return config.VerifyScopeDiff
	}
	return cfg.Governance.QualityGates.EffectiveVerifyScope()
}

// IsNonLogicToolConfigFile executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsNonLogicToolConfigFile(lowerName string) bool {
	for _, suf := range nonLogicConfigSuffixes {
		if strings.HasSuffix(lowerName, suf) {
			return true
		}
	}
	return false
}

// IsTDDExcludedRel executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsTDDExcludedRel(ctx *VerifyContext, root, rel string) bool {
	cfg := ResolveVerifyConfig(ctx)
	if cfg == nil || len(cfg.Governance.QualityGates.TDDExclude) == 0 {
		return false
	}
	normRel := NormalizeSlashPath(rel)
	if normRel == "" && root != "" {
		return false
	}
	for _, pattern := range cfg.Governance.QualityGates.TDDExclude {
		if matchTDDExcludePattern(pattern, normRel) {
			return true
		}
	}
	return false
}

// FilterNonExcludedImplFiles executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func FilterNonExcludedImplFiles(ctx *VerifyContext, root, dir string, implFiles []string) []string {
	if len(implFiles) == 0 {
		return nil
	}
	cfg := ResolveVerifyConfig(ctx)
	if cfg == nil || len(cfg.Governance.QualityGates.TDDExclude) == 0 {
		return implFiles
	}
	var kept []string
	for _, name := range implFiles {
		absFile := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, absFile)
		if err != nil {
			kept = append(kept, name)
			continue
		}
		if !IsTDDExcludedRel(ctx, root, filepath.ToSlash(rel)) {
			kept = append(kept, name)
		}
	}
	return kept
}

// NormalizeSlashPath executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func NormalizeSlashPath(p string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(p)))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimSuffix(clean, "/")
	if clean == "." {
		return ""
	}
	return clean
}

// MatchTDDExcludeGlob evaluates workspace-relative glob patterns supporting "**", "*", and "?" (@intent: #QualityGatesConfig).
func MatchTDDExcludeGlob(pattern, relPath string) bool {
	return matchTDDExcludePattern(pattern, NormalizeSlashPath(relPath))
}

func matchTDDExcludePattern(pattern, normRel string) bool {
	normPat := NormalizeSlashPath(pattern)
	if normPat == "" || normRel == "" {
		return false
	}
	if normRel == normPat || strings.HasPrefix(normRel, normPat+"/") {
		return true
	}
	patSegs := strings.Split(normPat, "/")
	relSegs := strings.Split(normRel, "/")
	return matchGlobSegments(patSegs, relSegs)
}

func matchGlobSegments(patSegs, relSegs []string) bool {
	if len(patSegs) == 0 {
		return len(relSegs) == 0
	}
	if patSegs[0] == "**" {
		if matchGlobSegments(patSegs[1:], relSegs) {
			return true
		}
		return len(relSegs) > 0 && matchGlobSegments(patSegs, relSegs[1:])
	}
	if len(relSegs) == 0 {
		return false
	}
	matched, err := path.Match(patSegs[0], relSegs[0])
	if err != nil || !matched {
		return false
	}
	return matchGlobSegments(patSegs[1:], relSegs[1:])
}
