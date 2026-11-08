// Package handshake implements JIT session orientation telemetry and brownfield topology discovery.
package handshake

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

const (
	modeBrownfieldRatchet = "brownfield_ratchet"
	hotspotMinLines       = 300
	hotspotMinBytes       = 6000
	maxSourceRoots        = 8
	maxSharedModules      = 6
	maxHotspotFiles       = 5
)

// BrownfieldHotspot identifies a high-LOC legacy source file governed by monotonic ratcheting.
type BrownfieldHotspot struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
}

// BrownfieldTopologyBrief provides a compact cold-start repository map for brownfield sessions.
type BrownfieldTopologyBrief struct {
	GovernanceMode string              `json:"governance_mode"`
	TDDMode        string              `json:"tdd_mode"`
	CoverageMode   string              `json:"coverage_mode"`
	Languages      []string            `json:"languages"`
	TestRunners    []string            `json:"test_runners"`
	SourceRoots    []string            `json:"source_roots"`
	TestRoots      []string            `json:"test_roots,omitempty"`
	SharedModules  []string            `json:"shared_modules,omitempty"`
	Hotspots       []BrownfieldHotspot `json:"hotspots,omitempty"`
}

type topologyAccumulator struct {
	langCounts    map[string]int
	dirCounts     map[string]int
	testRoots     map[string]bool
	hasRootSource bool
	hasColocTests bool
	sharedMods    []string
	hotspots      []BrownfieldHotspot
}

// SynthesizeBrownfieldTopology builds a compact cold-start repository map when strict_mode is false.
func SynthesizeBrownfieldTopology(treeRoot string, cfg *config.EcosystemConfig) *BrownfieldTopologyBrief {
	if treeRoot == "" || cfg == nil || cfg.Governance.QualityGates.StrictMode {
		return nil
	}
	acc := &topologyAccumulator{
		langCounts: make(map[string]int),
		dirCounts:  make(map[string]int),
		testRoots:  make(map[string]bool),
	}
	walkErr := filepath.WalkDir(treeRoot, func(path string, d fs.DirEntry, err error) error {
		return visitTopologyNode(treeRoot, path, d, err, acc)
	})
	if walkErr != nil {
		return nil
	}
	langs := rankCountsDesc(acc.langCounts, 4)
	if len(langs) == 0 {
		return nil
	}
	return &BrownfieldTopologyBrief{
		GovernanceMode: modeBrownfieldRatchet,
		TDDMode:        cfg.Governance.QualityGates.EffectiveTDDMode(),
		CoverageMode:   cfg.Governance.QualityGates.EffectiveCoverageMode(),
		Languages:      langs,
		TestRunners:    detectBrownfieldTestRunners(treeRoot, langs),
		SourceRoots:    selectSourceRoots(acc),
		TestRoots:      selectTestRoots(acc),
		SharedModules:  finalizeSharedModules(acc.sharedMods),
		Hotspots:       finalizeHotspots(acc.hotspots),
	}
}

func resolveHandshakeGovernance(treeRoot, primaryRoot string) (bool, *BrownfieldTopologyBrief) {
	cfg, err := config.LoadConfig(treeRoot)
	if (err != nil || cfg == nil) && primaryRoot != "" && primaryRoot != treeRoot {
		if fallbackCfg, fErr := config.LoadConfig(primaryRoot); fErr == nil {
			cfg = fallbackCfg
		}
	}
	if cfg == nil || cfg.Governance.QualityGates.StrictMode {
		return true, nil
	}
	return false, SynthesizeBrownfieldTopology(treeRoot, cfg)
}

func visitTopologyNode(root, path string, d fs.DirEntry, err error, acc *topologyAccumulator) error {
	if err != nil || d == nil {
		return nil
	}
	if d.IsDir() {
		if path != root && isSkippedTopologyDir(d.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		return nil
	}
	slashRel := filepath.ToSlash(rel)
	lang := classifyTopologyLanguage(d.Name())
	if lang == "" {
		return nil
	}
	if isTopologyTestFile(slashRel, d.Name()) {
		recordTopologyTestPath(slashRel, acc)
		return nil
	}
	recordTopologySourceFile(path, slashRel, d, lang, acc)
	return nil
}

func recordTopologyTestPath(slashRel string, acc *topologyAccumulator) {
	parts := strings.Split(slashRel, "/")
	if len(parts) > 1 && isDedicatedTestDir(parts[0]) {
		acc.testRoots[parts[0]+"/"] = true
		return
	}
	acc.hasColocTests = true
}

func recordTopologySourceFile(absPath, slashRel string, d fs.DirEntry, lang string, acc *topologyAccumulator) {
	acc.langCounts[lang]++
	parts := strings.Split(slashRel, "/")
	if len(parts) == 1 {
		if !isBuildOrConfigScript(d.Name()) {
			acc.hasRootSource = true
		}
	} else if !isNonPackageTopDir(parts[0]) {
		acc.dirCounts[parts[0]+"/"]++
		if len(parts) >= 3 {
			acc.dirCounts[parts[0]+"/"+parts[1]+"/"]++
		}
	}
	if isSharedUtilityFilename(d.Name()) && !isNonPackageTopDir(parts[0]) {
		acc.sharedMods = append(acc.sharedMods, slashRel)
	}
	info, iErr := d.Info()
	if iErr != nil || info.Size() < hotspotMinBytes {
		return
	}
	if lines := countFileLinesFast(absPath); lines >= hotspotMinLines {
		acc.hotspots = append(acc.hotspots, BrownfieldHotspot{Path: slashRel, Lines: lines})
	}
}

func selectSourceRoots(acc *topologyAccumulator) []string {
	var roots []string
	if acc.hasRootSource {
		roots = append(roots, "./")
	}
	ranked := rankCountsDesc(acc.dirCounts, maxSourceRoots)
	for _, dir := range ranked {
		if len(roots) >= maxSourceRoots {
			break
		}
		roots = append(roots, dir)
	}
	return roots
}

func selectTestRoots(acc *topologyAccumulator) []string {
	var out []string
	for k := range acc.testRoots {
		out = append(out, k)
	}
	sort.Strings(out)
	if acc.hasColocTests {
		out = append(out, "co-located *_test")
	}
	return out
}

func finalizeSharedModules(mods []string) []string {
	sort.Slice(mods, func(i, j int) bool {
		di, dj := strings.Count(mods[i], "/"), strings.Count(mods[j], "/")
		if di != dj {
			return di < dj
		}
		return mods[i] < mods[j]
	})
	if len(mods) > maxSharedModules {
		return mods[:maxSharedModules]
	}
	return mods
}

func finalizeHotspots(items []BrownfieldHotspot) []BrownfieldHotspot {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Lines != items[j].Lines {
			return items[i].Lines > items[j].Lines
		}
		return items[i].Path < items[j].Path
	})
	if len(items) > maxHotspotFiles {
		return items[:maxHotspotFiles]
	}
	return items
}

func countFileLinesFast(path string) int {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return 0
	}
	return bytes.Count(data, []byte("\n")) + 1
}

func rankCountsDesc(counts map[string]int, limit int) []string {
	type kv struct {
		key   string
		count int
	}
	items := make([]kv, 0, len(counts))
	for k, v := range counts {
		if v > 0 {
			items = append(items, kv{key: k, count: v})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].key < items[j].key
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	res := make([]string, 0, len(items))
	for _, it := range items {
		res = append(res, it.key)
	}
	return res
}
