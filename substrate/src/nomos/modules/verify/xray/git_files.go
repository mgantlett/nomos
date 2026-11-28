// Package xray implements Git-index-aware candidate enumeration and parallel AST scanning.
package xray

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

type scanCandidate struct {
	relSlash     string
	lang         string
	absPath      string
	prevalidated bool
}

type scannedFileOutcome struct {
	valid        bool
	isTest       bool
	relSlash     string
	lang         string
	loc          int
	skippedTests int
	oversized    int
	data         []byte
	fm           fileEvalMetrics
	hotspot      *XrayHotspot
}

func collectRepositoryCandidates(absRoot string) ([]scanCandidate, error) {
	if cands, ok := tryCollectGitCandidates(absRoot); ok {
		return cands, nil
	}
	var out []scanCandidate
	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, wErr error) error {
		if wErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if path != absRoot && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if cand, ok := buildWalkCandidate(absRoot, path, d); ok {
			out = append(out, cand)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("failed walking repository %s: %w", filepath.Base(absRoot), walkErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].relSlash < out[j].relSlash })
	return out, nil
}

func buildWalkCandidate(absRoot, path string, d fs.DirEntry) (scanCandidate, bool) {
	if isBundledArtifactFileName(d.Name()) {
		return scanCandidate{}, false
	}
	lang := treesitter.DetectLanguage(d.Name())
	if !codeLanguages[lang] || !isReadableRegularFile(d) {
		return scanCandidate{}, false
	}
	rel, err := filepath.Rel(absRoot, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return scanCandidate{}, false
	}
	relSlash := filepath.ToSlash(rel)
	if isBundledAssetRelPath(relSlash) {
		return scanCandidate{}, false
	}
	return scanCandidate{relSlash: relSlash, lang: lang, absPath: path, prevalidated: true}, true
}

func tryCollectGitCandidates(absRoot string) ([]scanCandidate, bool) {
	wtRoot, err := workspace.FindWorkingTreeRoot(absRoot)
	if err != nil || wtRoot == "" || isIgnoredSubdirOfRepo(wtRoot, absRoot) {
		return nil, false
	}
	out, cmdErr := workspace.SafeGitCommand(
		absRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ".",
	).Output()
	if cmdErr != nil {
		return nil, false
	}
	paths := parseUniqueGitPaths(out)
	return buildFilteredCandidates(absRoot, paths), true
}

func evaluateCandidatesParallel(candidates []scanCandidate, thresholds treesitter.EngineThresholds) []scannedFileOutcome {
	if len(candidates) == 0 {
		return nil
	}
	workers := min(max(runtime.GOMAXPROCS(0), 1), len(candidates))
	results := make([]scannedFileOutcome, len(candidates))
	var cursor atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i := int(cursor.Add(1) - 1)
				if i >= len(candidates) {
					return
				}
				results[i] = evaluateSingleCandidate(candidates[i], thresholds)
			}
		}()
	}
	wg.Wait()
	return results
}

func evaluateSingleCandidate(cand scanCandidate, thresholds treesitter.EngineThresholds) scannedFileOutcome {
	if !cand.prevalidated && !isScannableRegularPath(cand.absPath) {
		return scannedFileOutcome{}
	}
	data, err := os.ReadFile(cand.absPath)
	if err != nil {
		return scannedFileOutcome{}
	}
	loc := countLines(data)
	if isMinifiedSourceChunk(cand.lang, loc, len(data)) {
		return scannedFileOutcome{}
	}
	skipped := countSkippedTests(cand.lang, data)
	if impact.IsTestSourceFile(cand.relSlash) {
		return scannedFileOutcome{
			valid: true, isTest: true, relSlash: cand.relSlash,
			lang: cand.lang, loc: loc, skippedTests: skipped, data: data,
		}
	}
	fm, oversized := computeImplementationMetrics(cand.relSlash, cand.lang, loc, data, thresholds)
	return scannedFileOutcome{
		valid: true, relSlash: cand.relSlash, lang: cand.lang,
		loc: loc, skippedTests: skipped, oversized: oversized,
		data: data, fm: fm, hotspot: buildHotspotCandidate(fm, thresholds),
	}
}

func reduceScannedFiles(outcomes []scannedFileOutcome, acc *scanAccumulator) {
	for i := range outcomes {
		r := &outcomes[i]
		if !r.valid {
			continue
		}
		acc.report.FilesScanned++
		acc.report.TotalLOC += r.loc
		acc.report.Languages[r.lang]++
		acc.report.SkippedTests += r.skippedTests
		if r.isTest {
			acc.testSources[r.relSlash] = r.data
			continue
		}
		acc.nonTestSources[r.relSlash] = r.data
		relDir := filepath.ToSlash(filepath.Dir(r.relSlash))
		acc.dirFileCounts[relDir]++
		if acc.dirFileCounts[relDir] > acc.report.BaselineEnvelope.ObservedMaxPackageFiles {
			acc.report.BaselineEnvelope.ObservedMaxPackageFiles = acc.dirFileCounts[relDir]
		}
		applyImplementationMetrics(r.fm, r.oversized, r.hotspot, acc)
	}
}

func isIgnoredSubdirOfRepo(wtRoot, absRoot string) bool {
	if filepath.Clean(wtRoot) == filepath.Clean(absRoot) {
		return false
	}
	return workspace.SafeGitCommand(wtRoot, "check-ignore", "-q", absRoot).Run() == nil
}

func parseUniqueGitPaths(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	parts := bytes.Split(raw, []byte{0})
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		slash := filepath.ToSlash(string(p))
		if !strings.HasPrefix(slash, "..") && !seen[slash] {
			seen[slash] = true
			out = append(out, slash)
		}
	}
	sort.Strings(out)
	return out
}

func hasSkippedDirSegment(relSlash string) bool {
	dir := filepath.ToSlash(filepath.Dir(relSlash))
	if dir == "." || dir == "" {
		return false
	}
	for _, seg := range strings.Split(dir, "/") {
		if seg != "" && shouldSkipDir(seg) {
			return true
		}
	}
	return false
}

func isScannableRegularPath(absPath string) bool {
	fi, err := os.Lstat(absPath)
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular() && fi.Size() <= maxScannableFileBytes
}
