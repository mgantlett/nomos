package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func canonicalBoundaryPatterns() []string {
	return []string{
		"/" + constants.DirWorktrees + "/",
		"/" + constants.DirTmp + "/",
		"/.vscode/",
		"/" + constants.DirNomosHome + "/",
		"/go.work",
		"/go.work.sum",
	}
}

func scaffoldNomosBoundaries(dir string) error {
	boundaries := canonicalBoundaryPatterns()
	gitIgnorePath := filepath.Join(dir, ".gitignore")
	if err := appendMissingPatterns(gitIgnorePath, "# Nomos Operational Boundaries", boundaries); err != nil {
		return err
	}

	gitDir := filepath.Join(dir, constants.DirGit)
	if fi, err := os.Stat(gitDir); err == nil && fi.IsDir() {
		infoDir := filepath.Join(gitDir, "info")
		if mkErr := os.MkdirAll(infoDir, 0755); mkErr == nil {
			gitExcludePath := filepath.Join(infoDir, "exclude")
			if aErr := appendMissingPatterns(gitExcludePath, "# Nomos Operational Boundaries", boundaries); aErr != nil {
				return aErr
			}
		}
	}
	return EnsureIDEBoundarySettings(dir)
}

func hasIgnorePattern(existingContent, pattern string) bool {
	trimmedAnchor := strings.TrimPrefix(pattern, "/")
	for _, rawLine := range strings.Split(existingContent, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == pattern || (trimmedAnchor != "" && line == trimmedAnchor) {
			return true
		}
	}
	return false
}

func appendMissingPatterns(filePath, header string, patterns []string) error {
	existingBytes, err := os.ReadFile(filePath)
	var existingContent string
	if err == nil {
		existingContent = string(existingBytes)
	}

	var toAdd []string
	for _, p := range patterns {
		if !hasIgnorePattern(existingContent, p) {
			toAdd = append(toAdd, p)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	payload := formatIgnoreBlock(existingContent, header, toAdd)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			// non-fatal close
		}
	}()

	_, err = f.WriteString(payload)
	return err
}

func formatIgnoreBlock(existingContent, header string, toAdd []string) string {
	var sb strings.Builder
	if len(existingContent) > 0 && !strings.HasSuffix(existingContent, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(header)
	sb.WriteString("\n")
	for _, p := range toAdd {
		sb.WriteString(p)
		sb.WriteString("\n")
	}
	return sb.String()
}
