// Package xray implements structural debt scoring and hotspot ranking.
package xray

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

func summarizeFunctionBounds(funcs []treesitter.FunctionMetric, th treesitter.EngineThresholds) (int, int, int) {
	maxComp, maxFnLOC, oversized := 0, 0, 0
	for _, fn := range funcs {
		if fn.Complexity > maxComp {
			maxComp = fn.Complexity
		}
		if fn.LOC > maxFnLOC {
			maxFnLOC = fn.LOC
		}
		if fn.Complexity > th.MaxComplexity || fn.LOC > th.MaxFuncLOC {
			oversized++
		}
	}
	return maxComp, maxFnLOC, oversized
}

func updateBaselineEnvelope(env *XrayBaselineEnvelope, fm fileEvalMetrics) {
	if fm.loc > env.ObservedMaxFileLOC {
		env.ObservedMaxFileLOC = fm.loc
	}
	if fm.cds > env.ObservedMaxCDS {
		env.ObservedMaxCDS = fm.cds
	}
	if fm.maxComp > env.ObservedMaxComplexity {
		env.ObservedMaxComplexity = fm.maxComp
	}
	if fm.maxFnLOC > env.ObservedMaxFuncLOC {
		env.ObservedMaxFuncLOC = fm.maxFnLOC
	}
	if fm.maxParams > env.ObservedMaxParams {
		env.ObservedMaxParams = fm.maxParams
	}
	if fm.maxNesting > env.ObservedMaxNesting {
		env.ObservedMaxNesting = fm.maxNesting
	}
}

func buildHotspotCandidate(fm fileEvalMetrics, th treesitter.EngineThresholds) *XrayHotspot {
	reasons := collectHotspotReasons(fm, th)
	return &XrayHotspot{
		File: fm.relSlash, Language: fm.lang, LOC: fm.loc, CDS: fm.cds,
		MaxComplexity: fm.maxComp, MaxFuncLOC: fm.maxFnLOC,
		MaxParams: fm.maxParams, MaxNesting: fm.maxNesting,
		Stubs: fm.stubs, UncheckedErrors: fm.unchecked, SecretLeaks: fm.secretLeak,
		Reasons: reasons,
	}
}

func collectHotspotReasons(fm fileEvalMetrics, th treesitter.EngineThresholds) []string {
	var reasons []string
	reasons = appendThresholdReason(reasons, "file_loc", fm.loc, th.MaxFileLOC)
	reasons = appendThresholdReason(reasons, "complexity", fm.maxComp, th.MaxComplexity)
	reasons = appendThresholdReason(reasons, "func_loc", fm.maxFnLOC, th.MaxFuncLOC)
	reasons = appendThresholdReason(reasons, "params", fm.maxParams, astdebt.MaxFunctionParams)
	reasons = appendThresholdReason(reasons, "nesting", fm.maxNesting, astdebt.MaxFunctionNesting)
	reasons = appendCountReason(reasons, "stubs", fm.stubs)
	reasons = appendCountReason(reasons, "swallowed_errors", fm.unchecked)
	reasons = appendCountReason(reasons, "secret_leaks", fm.secretLeak)
	reasons = appendCountReason(reasons, "cwe_sinks", fm.sinks)
	return reasons
}

func appendThresholdReason(reasons []string, key string, val, limit int) []string {
	if val > limit {
		return append(reasons, fmt.Sprintf("%s=%d>%d", key, val, limit))
	}
	return reasons
}

func appendCountReason(reasons []string, key string, val int) []string {
	if val > 0 {
		return append(reasons, fmt.Sprintf("%s=%d", key, val))
	}
	return reasons
}

func applyCloneCountsToHotspots(groups []astdebt.CloneGroup, hotspots map[string]*XrayHotspot) {
	for _, g := range groups {
		seenInGroup := make(map[string]bool)
		for _, occ := range g.Occurrences {
			slash := filepath.ToSlash(occ.File)
			if seenInGroup[slash] {
				continue
			}
			seenInGroup[slash] = true
			if hs, ok := hotspots[slash]; ok {
				hs.Clones++
			}
		}
	}
	for _, hs := range hotspots {
		if hs.Clones > 0 {
			hs.Reasons = append(hs.Reasons, fmt.Sprintf("clones=%d", hs.Clones))
		}
	}
}

func computeHangoverScore(rep *XrayReport) int {
	if rep.FilesScanned == 0 {
		return 0
	}
	faults := rep.CloneGroups*10 +
		rep.MonolithFiles*8 +
		rep.OversizedFuncs*4 +
		rep.HighArityFuncs*3 +
		rep.DeepNestingFuncs*3 +
		rep.EmptyStubs*7 +
		rep.SkippedTests*6 +
		rep.UncheckedErrors*5 +
		rep.SecretPathLeaks*12 +
		rep.MagicStringGroups*4 +
		rep.SecuritySinks*14
	if faults == 0 {
		return 0
	}
	scale := 1.0 + float64(rep.FilesScanned)/50.0
	normalized := int(float64(faults)/scale + 0.5)
	if normalized < 1 {
		normalized = 1
	}
	if normalized > 100 {
		return 100
	}
	return normalized
}

func computeHangoverGrade(score int) string {
	switch {
	case score <= 10:
		return "A"
	case score <= 25:
		return "B"
	case score <= 50:
		return "C"
	case score <= 75:
		return "D"
	default:
		return "F"
	}
}

func hotspotRankWeight(h *XrayHotspot) int {
	return h.CDS*3 + h.MaxComplexity*4 + h.Clones*25 + h.Stubs*20 +
		h.UncheckedErrors*15 + h.SecretLeaks*30 + h.LOC/10
}

func selectTopHotspots(byFile map[string]*XrayHotspot, limit int) []XrayHotspot {
	candidates := make([]XrayHotspot, 0, len(byFile))
	for _, hs := range byFile {
		if len(hs.Reasons) > 0 {
			candidates = append(candidates, *hs)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		wi, wj := hotspotRankWeight(&candidates[i]), hotspotRankWeight(&candidates[j])
		if wi != wj {
			return wi > wj
		}
		return candidates[i].File < candidates[j].File
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}
