// Package polyrun implements Go coverprofile persistence and incremental package merging.
package polyrun

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
)

const (
	coverProfileDirName     = ".tmp"
	coverProfileFileName    = "coverage.out"
	coverProfileRunFileName = "coverage-run.out"
	coverProfileModePrefix  = "mode:"
)

func prepareGoCoverProfileRunPath(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	tmpDir := filepath.Join(root, coverProfileDirName)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return ""
	}
	absRun, err := filepath.Abs(filepath.Join(tmpDir, coverProfileRunFileName))
	if err != nil {
		return ""
	}
	return absRun
}

func cleanupRunCoverProfile(runProfilePath string) {
	if runProfilePath == "" {
		return
	}
	if err := os.Remove(runProfilePath); err != nil && !os.IsNotExist(err) {
		return
	}
}

func isFullGoPackageSet(pkgs []string) bool {
	return len(pkgs) == 1 && pkgs[0] == "./..."
}

func persistGoCoverProfile(ctx *vscope.VerifyContext, root, runProfilePath string, pkgs []string) {
	if runProfilePath == "" || strings.TrimSpace(root) == "" {
		return
	}
	defer cleanupRunCoverProfile(runProfilePath)

	runBytes, err := os.ReadFile(runProfilePath)
	if err != nil || len(runBytes) == 0 {
		return
	}
	destPath := filepath.Join(root, coverProfileDirName, coverProfileFileName)
	if isFullGoPackageSet(pkgs) {
		writeCoverProfileBytes(destPath, runBytes)
		return
	}
	baseBytes := loadBaselineCoverProfileBytes(ctx, root, destPath)
	if len(baseBytes) == 0 {
		writeCoverProfileBytes(destPath, runBytes)
		return
	}
	writeCoverProfileBytes(destPath, MergeGoCoverProfiles(baseBytes, runBytes))
}

func loadBaselineCoverProfileBytes(ctx *vscope.VerifyContext, root, destPath string) []byte {
	if data, err := os.ReadFile(destPath); err == nil && len(data) > 0 {
		return data
	}
	if ctx != nil && ctx.RepoRoot != "" && filepath.Clean(ctx.RepoRoot) != filepath.Clean(root) {
		repoProfile := filepath.Join(ctx.RepoRoot, coverProfileDirName, coverProfileFileName)
		if data, err := os.ReadFile(repoProfile); err == nil && len(data) > 0 {
			return data
		}
	}
	return nil
}

func writeCoverProfileBytes(destPath string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return
	}
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return
	}
}

// mergeGoCoverProfiles overlays updated file block entries onto an existing Go coverprofile.
func MergeGoCoverProfiles(base, overlay []byte) []byte {
	modeLine, overlayLines := parseCoverProfileLines(string(overlay))
	baseMode, baseLines := parseCoverProfileLines(string(base))
	updatedFiles := indexCoverProfileFiles(overlayLines)
	if modeLine == "" {
		modeLine = baseMode
	}
	if modeLine == "" {
		modeLine = "mode: set"
	}

	var b strings.Builder
	b.WriteString(modeLine)
	b.WriteByte('\n')
	for _, line := range baseLines {
		if fileKey := extractCoverLineFile(line); fileKey != "" && !updatedFiles[fileKey] {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	for _, line := range overlayLines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func parseCoverProfileLines(raw string) (string, []string) {
	var modeLine string
	var blocks []string
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, coverProfileModePrefix) {
			modeLine = line
			continue
		}
		if extractCoverLineFile(line) != "" {
			blocks = append(blocks, line)
		}
	}
	return modeLine, blocks
}

func indexCoverProfileFiles(lines []string) map[string]bool {
	files := make(map[string]bool, len(lines))
	for _, line := range lines {
		if fk := extractCoverLineFile(line); fk != "" {
			files[fk] = true
		}
	}
	return files
}

func extractCoverLineFile(line string) string {
	idx := strings.LastIndex(line, ":")
	if idx <= 0 {
		return ""
	}
	return line[:idx]
}
