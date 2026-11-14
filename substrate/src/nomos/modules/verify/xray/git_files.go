// Package xray implements Git-index-aware candidate file enumeration for brownfield scanning.
package xray

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

func tryScanViaGitIndex(absRoot string, acc *scanAccumulator) bool {
	wtRoot, err := workspace.FindWorkingTreeRoot(absRoot)
	if err != nil || wtRoot == "" {
		return false
	}
	if isIgnoredSubdirOfRepo(wtRoot, absRoot) {
		return false
	}
	out, cmdErr := workspace.SafeGitCommand(
		absRoot, "ls-files", "-z", "--cached", "--others", "--exclude-standard",
	).Output()
	if cmdErr != nil {
		return false
	}
	for _, relSlash := range parseUniqueGitPaths(out) {
		inspectGitIndexedFile(absRoot, relSlash, acc)
	}
	return true
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

func inspectGitIndexedFile(absRoot, relSlash string, acc *scanAccumulator) {
	base := filepath.Base(relSlash)
	if hasSkippedDirSegment(relSlash) || isBundledArtifactFileName(base) || isBundledAssetRelPath(relSlash) {
		return
	}
	lang := treesitter.DetectLanguage(base)
	if !codeLanguages[lang] {
		return
	}
	absPath := filepath.Join(absRoot, filepath.FromSlash(relSlash))
	if !isScannableRegularPath(absPath) {
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return
	}
	ingestSourceFile(relSlash, lang, data, acc)
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
