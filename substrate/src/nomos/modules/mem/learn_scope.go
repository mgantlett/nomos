// Package mem implements episodic gate-trap distillation (Nomos Learn) in Nomos Mneme.
package mem

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	trapRelPathPattern  = regexp.MustCompile(`([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+\.[A-Za-z0-9]+)`)
	trapBaseLinePattern = regexp.MustCompile(`([A-Za-z0-9_.-]+\.[A-Za-z0-9]+):\d+`)
)

// DeriveDirectoryScope computes a concise directory scope label from modified paths (@intent: #Memory).
func DeriveDirectoryScope(repoRoot string, touchedFiles []string) string {
	fallbackHidden := ""
	for _, f := range touchedFiles {
		dir := normalizeTouchedDir(repoRoot, f)
		if dir == "" {
			continue
		}
		if strings.HasPrefix(dir, ".") {
			if fallbackHidden == "" {
				fallbackHidden = dir
			}
			continue
		}
		return dir
	}
	if fallbackHidden != "" {
		return fallbackHidden
	}
	return workspaceScopeLabel
}

// DeriveTrapScope attributes a gate failure to the exact directory of the failing file in message when available (@intent: #Memory).
func DeriveTrapScope(repoRoot, message string, touchedFiles []string) string {
	if scope := extractMessageRelPathScope(repoRoot, message, touchedFiles); scope != "" {
		return scope
	}
	if scope := matchMessageBasenameScope(repoRoot, message, touchedFiles); scope != "" {
		return scope
	}
	return DeriveDirectoryScope(repoRoot, touchedFiles)
}

func extractMessageRelPathScope(repoRoot, message string, touchedFiles []string) string {
	matches := trapRelPathPattern.FindAllStringSubmatch(message, 16)
	var fallback string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		cand := resolveCandidateRelPath(repoRoot, match[1])
		if isTouchedRepoFile(repoRoot, cand, touchedFiles) {
			return dirToScopeLabel(filepath.ToSlash(filepath.Dir(cand)))
		}
		if fallback == "" && isRecognizedRepoFile(repoRoot, cand, touchedFiles) {
			fallback = dirToScopeLabel(filepath.ToSlash(filepath.Dir(cand)))
		}
	}
	return fallback
}

func resolveCandidateRelPath(repoRoot, raw string) string {
	cand := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(raw)), "./")
	if repoRoot != "" && filepath.IsAbs(raw) {
		if rel, err := filepath.Rel(repoRoot, raw); err == nil && !strings.HasPrefix(rel, "..") {
			cand = filepath.ToSlash(rel)
		}
	}
	return cand
}

func isTouchedRepoFile(repoRoot, cand string, touchedFiles []string) bool {
	if cand == "" || strings.HasPrefix(cand, "github.com/") || strings.HasPrefix(cand, "golang.org/") {
		return false
	}
	for _, tf := range touchedFiles {
		norm := normalizePatternPath(repoRoot, tf)
		if norm == cand || strings.HasSuffix(norm, "/"+cand) {
			return true
		}
	}
	return false
}

func isRecognizedRepoFile(repoRoot, cand string, touchedFiles []string) bool {
	if strings.HasPrefix(cand, "github.com/") || strings.HasPrefix(cand, "golang.org/") {
		return false
	}
	if isTouchedRepoFile(repoRoot, cand, touchedFiles) {
		return true
	}
	if repoRoot != "" {
		if fi, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(cand))); err == nil && !fi.IsDir() {
			return true
		}
	}
	return strings.HasPrefix(cand, "substrate/") || strings.HasPrefix(cand, "intent/") || strings.HasPrefix(cand, "src/")
}

func matchMessageBasenameScope(repoRoot, message string, touchedFiles []string) string {
	for _, match := range trapBaseLinePattern.FindAllStringSubmatch(message, 8) {
		if len(match) < 2 {
			continue
		}
		base := match[1]
		for _, tf := range touchedFiles {
			norm := normalizePatternPath(repoRoot, tf)
			if filepath.Base(norm) == base {
				return dirToScopeLabel(filepath.ToSlash(filepath.Dir(norm)))
			}
		}
	}
	return ""
}

func normalizeTouchedDir(repoRoot, raw string) string {
	clean := filepath.ToSlash(strings.TrimSpace(raw))
	if clean == "" {
		return ""
	}
	if repoRoot != "" && filepath.IsAbs(raw) {
		if rel, err := filepath.Rel(repoRoot, raw); err == nil && !strings.HasPrefix(rel, "..") {
			clean = filepath.ToSlash(rel)
		}
	}
	return dirToScopeLabel(filepath.ToSlash(filepath.Dir(clean)))
}

func dirToScopeLabel(dir string) string {
	if dir == "." || dir == "/" || dir == "" {
		return "root"
	}
	return dir
}

func selectTouchedDiagnosticLine(lines []string, repoRoot string, touchedFiles []string) string {
	if len(touchedFiles) == 0 || len(lines) == 0 {
		return ""
	}
	for i, line := range lines {
		if i == 0 && len(lines) > 1 && strings.HasSuffix(line, ":") {
			continue
		}
		if lineMentionsTouchedFile(repoRoot, line, touchedFiles) {
			return combineColonHeaderWithDetail(lines[0], line)
		}
	}
	return ""
}

func lineMentionsTouchedFile(repoRoot, line string, touchedFiles []string) bool {
	for _, m := range trapRelPathPattern.FindAllStringSubmatch(line, 4) {
		if len(m) > 1 && isTouchedRepoFile(repoRoot, resolveCandidateRelPath(repoRoot, m[1]), touchedFiles) {
			return true
		}
	}
	for _, tf := range touchedFiles {
		base := filepath.Base(normalizePatternPath(repoRoot, tf))
		if base != "" && strings.Contains(line, base+":") {
			return true
		}
	}
	return false
}

