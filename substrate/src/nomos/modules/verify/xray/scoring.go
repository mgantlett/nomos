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
}

func buildHotspotCandidate(fm fileEvalMetrics, th treesitter.EngineThresholds) *XrayHotspot {
	var reasons []string
	if fm.loc > th.MaxFileLOC {
		reasons = append(reasons, fmt.Sprintf("file_loc=%d>%d", fm.loc, th.MaxFileLOC))
	}
	if fm.maxComp > th.MaxComplexity {
		reasons = append(reasons, fmt.Sprintf("complexity=%d>%d", fm.maxComp, th.MaxComplexity))
	}
	if fm.maxFnLOC > th.MaxFuncLOC {
		reasons = append(reasons, fmt.Sprintf("func_loc=%d>%d", fm.maxFnLOC, th.MaxFuncLOC))
	}
	if fm.stubs > 0 {
		reasons = append(reasons, fmt.Sprintf("stubs=%d", fm.stubs))
	}
	if fm.sinks > 0 {
		reasons = append(reasons, fmt.Sprintf("cwe_sinks=%d", fm.sinks))
	}
	return &XrayHotspot{
		File: fm.relSlash, Language: fm.lang, LOC: fm.loc, CDS: fm.cds,
		MaxComplexity: fm.maxComp, MaxFuncLOC: fm.maxFnLOC, Stubs: fm.stubs, Reasons: reasons,
	}
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
	raw := rep.CloneGroups*8 +
		rep.MonolithFiles*7 +
		rep.OversizedFuncs*4 +
		rep.EmptyStubs*6 +
		rep.SkippedTests*5 +
		rep.SecuritySinks*12
	if raw > 100 {
		return 100
	}
	return raw
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
	return h.CDS*3 + h.MaxComplexity*4 + h.Clones*25 + h.Stubs*20 + h.LOC/10
}

func selectTopHotspots(byFile map[string]*XrayHotspot, limit int) []XrayHotspot {
	candidates := make([]XrayHotspot, 0, len(byFile))
	for _, hs := range byFile {
		if len(hs.Reasons) > 0 || hs.CDS > 20 {
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
