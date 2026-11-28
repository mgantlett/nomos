// Package xray implements read-only zero-init brownfield structural diagnostics.
package xray

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/wiredrift"
)

const maxScannableFileBytes = 1024 * 1024

var (
	codeLanguages = map[string]bool{
		"go": true, "typescript": true, "svelte": true, "python": true,
		"ruby": true, "rust": true, "c": true, "cpp": true,
		"java": true, "kotlin": true, "csharp": true,
	}
	ignoredScanDirs = map[string]bool{
		".git": true, ".hg": true, ".svn": true, ".tmp": true,
		"worktrees": true, "node_modules": true, "vendor": true,
		"vendors": true, "third_party": true, "third-party": true, "3rdparty": true,
		".venv": true, "venv": true, "env": true, "site-packages": true, "dist": true,
		"webdist": true, "build": true, "out": true, "target": true,
		"__pycache__": true, ".tox": true, ".mypy_cache": true,
		".pytest_cache": true, ".ruff_cache": true, ".next": true,
		".svelte-kit": true, "coverage": true, "testdata": true,
	}
	explicitPyStubRe    = regexp.MustCompile(`(?m)^\s*raise\s+NotImplementedError\b`)
	explicitRustStubRe  = regexp.MustCompile(`(?m)^\s*(?:todo|unimplemented)!\s*\(`)
	explicitTSStubRe    = regexp.MustCompile(`(?im)^\s*throw\s+new\s+Error\s*\(\s*["']not\s+implemented`)
	hashedBundleChunkRe = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.(?:js|mjs|cjs)$`)
)

type scanAccumulator struct {
	thresholds     treesitter.EngineThresholds
	nonTestSources map[string][]byte
	testSources    map[string][]byte
	dirFileCounts  map[string]int
	hotspotsByFile map[string]*XrayHotspot
	report         *XrayReport
}

type fileEvalMetrics struct {
	relSlash   string
	lang       string
	loc        int
	cds        int
	maxComp    int
	maxFnLOC   int
	maxParams  int
	maxNesting int
	stubs        int
	sinks        int
	unchecked    int
	secretLeak   int
	secretTokens int
	pathLeaks    int
	highArity    int
	deepNest     int
}

// ScanRepository executes a read-only 11-language structural scan of targetDir without writing files or touching SQLite (@intent tree_sitter_ast_gate_spec.md #XrayReport).
func ScanRepository(targetDir string, opts XrayScanOptions) (*XrayReport, error) {
	start := time.Now()
	absRoot, err := resolveValidScanDir(targetDir)
	if err != nil {
		return nil, err
	}
	topLimit := opts.TopHotspots
	if topLimit <= 0 {
		topLimit = DefaultTopHotspots
	}
	acc := newScanAccumulator(absRoot, opts)
	var candidates []scanCandidate
	diffRef := strings.TrimSpace(opts.DiffBaseRef)
	if diffRef != "" {
		var diffCount int
		candidates, diffCount, err = collectDiffCandidates(absRoot, diffRef)
		if err != nil {
			return nil, err
		}
		acc.report.DiffBaseRef = diffRef
		acc.report.DiffFiles = diffCount
	} else {
		var cErr error
		candidates, cErr = collectRepositoryCandidates(absRoot)
		if cErr != nil {
			return nil, cErr
		}
	}
	outcomes := evaluateCandidatesParallel(candidates, acc.thresholds)
	reduceScannedFiles(outcomes, acc)
	finalizeScanReport(absRoot, topLimit, start, acc)
	return acc.report, nil
}

func resolveValidScanDir(targetDir string) (string, error) {
	target := strings.TrimSpace(targetDir)
	if target == "" {
		target = "."
	}
	absRoot, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("failed resolving target path: %w", err)
	}
	fi, sErr := os.Stat(absRoot)
	if sErr != nil || !fi.IsDir() {
		return "", fmt.Errorf("scan target is not an existing directory: %s", filepath.Base(absRoot))
	}
	return absRoot, nil
}

func newScanAccumulator(absRoot string, opts XrayScanOptions) *scanAccumulator {
	_, statErr := os.Stat(filepath.Join(absRoot, constants.FileNomosYAML))
	initialized := statErr == nil
	nextCmd := "nomos init --baseline"
	if initialized {
		nextCmd = "nomos verify --all"
	}
	thresholds := resolveScanThresholds(absRoot, opts)
	return &scanAccumulator{
		thresholds:     thresholds,
		nonTestSources: make(map[string][]byte),
		testSources:    make(map[string][]byte),
		dirFileCounts:  make(map[string]int),
		hotspotsByFile: make(map[string]*XrayHotspot),
		report: &XrayReport{
			ProjectName:            sanitizeProjectName(absRoot),
			Initialized:            initialized,
			Languages:              make(map[string]int),
			MaxFileLOCThreshold:    thresholds.MaxFileLOC,
			MaxFuncLOCThreshold:    thresholds.MaxFuncLOC,
			MaxComplexityThreshold: thresholds.MaxComplexity,
			NextCommand:            nextCmd,
		},
	}
}

func resolveScanThresholds(absRoot string, opts XrayScanOptions) treesitter.EngineThresholds {
	th := treesitter.DefaultEngineThresholds()
	if cfg, err := config.LoadConfig(filepath.Join(absRoot, constants.FileNomosYAML)); err == nil && cfg != nil {
		ct := cfg.Governance.QualityGates.Thresholds
		if ct.MaxFileLOC > 0 {
			th.MaxFileLOC = ct.MaxFileLOC
		}
		if ct.MaxFuncLOC > 0 {
			th.MaxFuncLOC = ct.MaxFuncLOC
		}
		if ct.MaxCyclomaticComplexity > 0 {
			th.MaxComplexity = ct.MaxCyclomaticComplexity
		}
	}
	if opts.MaxFileLOC > 0 {
		th.MaxFileLOC = opts.MaxFileLOC
	}
	if opts.MaxFuncLOC > 0 {
		th.MaxFuncLOC = opts.MaxFuncLOC
	}
	if opts.MaxComplexity > 0 {
		th.MaxComplexity = opts.MaxComplexity
	}
	return th
}

func sanitizeProjectName(absRoot string) string {
	base := filepath.Base(filepath.Clean(absRoot))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "workspace"
	}
	return base
}

func isBundledAssetRelPath(relSlash string) bool {
	if strings.Contains(relSlash, "/sovereign-sandbox/assets/") || strings.Contains(relSlash, "/_app/immutable/") {
		return true
	}
	return strings.Contains(relSlash, "/assets/") && hashedBundleChunkRe.MatchString(relSlash)
}

func isBundledArtifactFileName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".min.js") || strings.HasSuffix(lower, ".bundle.js") || strings.HasSuffix(lower, ".d.ts")
}

func isMinifiedSourceChunk(lang string, loc, byteLen int) bool {
	if lang != "typescript" || byteLen < 2048 {
		return false
	}
	lines := loc
	if lines < 1 {
		lines = 1
	}
	return byteLen/lines > 300
}

func isReadableRegularFile(d fs.DirEntry) bool {
	info, err := d.Info()
	if err != nil {
		return false
	}
	return info.Size() <= maxScannableFileBytes && (info.Mode()&os.ModeSymlink == 0)
}

func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "build.") || strings.HasPrefix(lower, "dist.") || strings.HasPrefix(lower, "out.") || lower == "_app" {
		return true
	}
	return ignoredScanDirs[lower]
}

func computeImplementationMetrics(relSlash, lang string, loc int, data []byte, thresholds treesitter.EngineThresholds) (fileEvalMetrics, int) {
	funcs, err := treesitter.ExtractFunctions(lang, relSlash, data)
	if err != nil {
		funcs = nil
	}
	eval := treesitter.BuildEvaluationReport(relSlash, loc, funcs, thresholds)
	cds := 0
	if len(eval.Files) > 0 {
		cds = eval.Files[0].CDS
	}
	stubs := len(eval.EmptyBlocks) + countExplicitPolyglotStubs(lang, data)
	sinks := len(verify.DetectPolyglotCWESinks(relSlash, data))
	maxParams, maxNesting, highArity, deepNest := astdebt.SummarizeFileArityAndNesting(
		relSlash, data, astdebt.MaxFunctionParams, astdebt.MaxFunctionNesting,
	)
	unchecked := verify.CountFileUncheckedErrors(relSlash, data)
	secretTokens, pathLeaks := verify.SplitFileSecretAndPathLeaks(relSlash, data)
	maxComp, maxFnLOC, oversized := summarizeFunctionBounds(funcs, thresholds)
	fm := fileEvalMetrics{
		relSlash: relSlash, lang: lang, loc: loc, cds: cds,
		maxComp: maxComp, maxFnLOC: maxFnLOC, maxParams: maxParams, maxNesting: maxNesting,
		stubs: stubs, sinks: sinks, unchecked: unchecked,
		secretLeak: secretTokens + pathLeaks, secretTokens: secretTokens, pathLeaks: pathLeaks,
		highArity: highArity, deepNest: deepNest,
	}
	return fm, oversized
}

func applyImplementationMetrics(fm fileEvalMetrics, oversized int, hotspot *XrayHotspot, acc *scanAccumulator) {
	acc.report.EmptyStubs += fm.stubs
	acc.report.SecuritySinks += fm.sinks
	acc.report.HighArityFuncs += fm.highArity
	acc.report.DeepNestingFuncs += fm.deepNest
	acc.report.UncheckedErrors += fm.unchecked
	acc.report.SecretPathLeaks += fm.secretLeak
	if fm.loc > acc.thresholds.MaxFileLOC {
		acc.report.MonolithFiles++
	}
	acc.report.OversizedFuncs += oversized
	updateBaselineEnvelope(&acc.report.BaselineEnvelope, fm)
	acc.hotspotsByFile[fm.relSlash] = hotspot
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

func finalizeScanReport(absRoot string, topLimit int, start time.Time, acc *scanAccumulator) {
	clones := astdebt.DetectStructuralClones(acc.nonTestSources, astdebt.DefaultMinCloneLines)
	acc.report.CloneGroups = len(clones)
	acc.report.BaselineEnvelope.CloneGroupCount = len(clones)
	applyCloneCountsToHotspots(clones, acc.hotspotsByFile)

	acc.report.MagicStringGroups = verify.CountCorpusMagicStringGroups(
		acc.nonTestSources, verify.DefaultXrayMagicStringThreshold,
	)
	applyBoundaryDriftFindings(wiredrift.ScanCorpusBoundaryWireDrift(acc.nonTestSources), acc.report, acc.hotspotsByFile)
	acc.report.InferredBoundaries = len(impact.InferCleanArchitectureBoundariesForFiles(absRoot, nonTestSourcePaths(acc.nonTestSources)))
	applyTopologyMetrics(absRoot, acc)
	if acc.report.Topology != nil {
		for i := range acc.report.Topology.Spine {
			if hs := acc.hotspotsByFile[acc.report.Topology.Spine[i].File]; hs != nil {
				acc.report.Topology.Spine[i].Language = hs.Language
				acc.report.Topology.Spine[i].LOC = hs.LOC
				acc.report.Topology.Spine[i].CDS = hs.CDS
			}
		}
	}
	acc.report.HangoverScore = computeHangoverScore(acc.report)
	acc.report.Grade = computeHangoverGrade(acc.report.HangoverScore)
	acc.report.Hotspots = selectTopHotspots(acc.hotspotsByFile, topLimit)
	acc.report.DurationMillis = time.Since(start).Milliseconds()
}

func nonTestSourcePaths(sources map[string][]byte) []string {
	paths := make([]string, 0, len(sources))
	for rel := range sources {
		paths = append(paths, rel)
	}
	return paths
}
