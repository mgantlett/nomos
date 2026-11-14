// Package xray implements read-only zero-init brownfield structural diagnostics.
package xray

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
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
		".venv": true, "venv": true, "env": true, "dist": true,
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
	stubs      int
	sinks      int
	unchecked  int
	secretLeak int
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
	acc := newScanAccumulator(absRoot)
	if !tryScanViaGitIndex(absRoot, acc) {
		walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, wErr error) error {
			return visitRepositoryEntry(absRoot, path, d, wErr, acc)
		})
		if walkErr != nil {
			return nil, fmt.Errorf("failed walking repository %s: %w", filepath.Base(absRoot), walkErr)
		}
	}
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

func newScanAccumulator(absRoot string) *scanAccumulator {
	_, statErr := os.Stat(filepath.Join(absRoot, constants.FileNomosYAML))
	initialized := statErr == nil
	nextCmd := "nomos init --baseline"
	if initialized {
		nextCmd = "nomos verify --all"
	}
	return &scanAccumulator{
		thresholds:     treesitter.DefaultEngineThresholds(),
		nonTestSources: make(map[string][]byte),
		dirFileCounts:  make(map[string]int),
		hotspotsByFile: make(map[string]*XrayHotspot),
		report: &XrayReport{
			ProjectName: sanitizeProjectName(absRoot),
			Initialized: initialized,
			Languages:   make(map[string]int),
			NextCommand: nextCmd,
		},
	}
}

func sanitizeProjectName(absRoot string) string {
	base := filepath.Base(filepath.Clean(absRoot))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "workspace"
	}
	return base
}

func visitRepositoryEntry(absRoot, path string, d fs.DirEntry, err error, acc *scanAccumulator) error {
	if err != nil || d == nil {
		return nil
	}
	if d.IsDir() {
		if path != absRoot && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	return inspectCandidateFile(absRoot, path, d, acc)
}

func inspectCandidateFile(absRoot, path string, d fs.DirEntry, acc *scanAccumulator) error {
	if isBundledArtifactFileName(d.Name()) {
		return nil
	}
	lang := treesitter.DetectLanguage(d.Name())
	if !codeLanguages[lang] || !isReadableRegularFile(d) {
		return nil
	}
	rel, rErr := filepath.Rel(absRoot, path)
	if rErr != nil || strings.HasPrefix(rel, "..") {
		return nil
	}
	relSlash := filepath.ToSlash(rel)
	if isBundledAssetRelPath(relSlash) {
		return nil
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil
	}
	ingestSourceFile(relSlash, lang, data, acc)
	return nil
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

func ingestSourceFile(relSlash, lang string, data []byte, acc *scanAccumulator) {
	loc := countLines(data)
	if isMinifiedSourceChunk(lang, loc, len(data)) {
		return
	}
	acc.report.FilesScanned++
	acc.report.TotalLOC += loc
	acc.report.Languages[lang]++

	acc.report.SkippedTests += countSkippedTests(lang, data)
	if impact.IsTestSourceFile(relSlash) {
		return
	}

	acc.nonTestSources[relSlash] = data
	relDir := filepath.ToSlash(filepath.Dir(relSlash))
	acc.dirFileCounts[relDir]++
	if acc.dirFileCounts[relDir] > acc.report.BaselineEnvelope.ObservedMaxPackageFiles {
		acc.report.BaselineEnvelope.ObservedMaxPackageFiles = acc.dirFileCounts[relDir]
	}
	evaluateImplementationMetrics(relSlash, lang, loc, data, acc)
}

func evaluateImplementationMetrics(relSlash, lang string, loc int, data []byte, acc *scanAccumulator) {
	funcs, err := treesitter.ExtractFunctions(lang, relSlash, data)
	if err != nil {
		funcs = nil
	}
	eval := treesitter.BuildEvaluationReport(relSlash, loc, funcs, acc.thresholds)
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
	secretLeaks := verify.CountFileSecretAndPathLeaks(relSlash, data)

	acc.report.EmptyStubs += stubs
	acc.report.SecuritySinks += sinks
	acc.report.HighArityFuncs += highArity
	acc.report.DeepNestingFuncs += deepNest
	acc.report.UncheckedErrors += unchecked
	acc.report.SecretPathLeaks += secretLeaks
	if loc > acc.thresholds.MaxFileLOC {
		acc.report.MonolithFiles++
	}
	maxComp, maxFnLOC, oversized := summarizeFunctionBounds(funcs, acc.thresholds)
	acc.report.OversizedFuncs += oversized
	fm := fileEvalMetrics{
		relSlash: relSlash, lang: lang, loc: loc, cds: cds,
		maxComp: maxComp, maxFnLOC: maxFnLOC, maxParams: maxParams, maxNesting: maxNesting,
		stubs: stubs, sinks: sinks, unchecked: unchecked, secretLeak: secretLeaks,
	}
	updateBaselineEnvelope(&acc.report.BaselineEnvelope, fm)
	acc.hotspotsByFile[relSlash] = buildHotspotCandidate(fm, acc.thresholds)
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxScannableFileBytes)
	lines := 0
	for sc.Scan() {
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
	acc.report.InferredBoundaries = len(impact.InferCleanArchitectureBoundaries(absRoot))
	acc.report.HangoverScore = computeHangoverScore(acc.report)
	acc.report.Grade = computeHangoverGrade(acc.report.HangoverScore)
	acc.report.Hotspots = selectTopHotspots(acc.hotspotsByFile, topLimit)
	acc.report.DurationMillis = time.Since(start).Milliseconds()
}
