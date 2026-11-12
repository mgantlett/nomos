// Package astdebt implements the deterministic Definition of Done (DoD) quality gate pipeline.
package astdebt

import (
	"os"
	"path/filepath"
	"strings"
)

// hasLocalGoWork reports whether targetDir or root uses an explicit Go workspace,
// either via a direct go.work file or via the nearest ancestor go.work whose use
// directives explicitly enroll targetDir (or root).
func HasLocalGoWork(root, targetDir string) bool {
	if hasDirectGoWork(root, targetDir) {
		return true
	}
	return hasAncestorGoWorkUse(root, targetDir)
}

func hasDirectGoWork(root, targetDir string) bool {
	for _, d := range []string{targetDir, root} {
		if d != "" && isRegularGoWorkFile(filepath.Join(d, "go.work")) {
			return true
		}
	}
	return false
}

func isRegularGoWorkFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func hasAncestorGoWorkUse(root, targetDir string) bool {
	startDir := strings.TrimSpace(targetDir)
	if startDir == "" {
		startDir = strings.TrimSpace(root)
	}
	if startDir == "" {
		return false
	}
	goWorkPath := findNearestAncestorGoWork(startDir)
	if goWorkPath == "" {
		return false
	}
	return goWorkUsesTarget(goWorkPath, startDir, root)
}

func findNearestAncestorGoWork(startDir string) string {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	for {
		candidate := filepath.Join(dir, "go.work")
		if isRegularGoWorkFile(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func goWorkUsesTarget(goWorkPath, targetDir, root string) bool {
	workDir := filepath.Dir(goWorkPath)
	wantTarget := canonicalGoWorkDir(targetDir)
	wantRoot := canonicalGoWorkDir(root)
	for _, relUse := range ParseGoWorkUseDirs(goWorkPath) {
		resolved := canonicalGoWorkDir(resolveGoWorkUsePath(workDir, relUse))
		if matchesGoWorkTarget(resolved, wantTarget, wantRoot) {
			return true
		}
	}
	return false
}

func matchesGoWorkTarget(resolved, wantTarget, wantRoot string) bool {
	if resolved == "" {
		return false
	}
	if resolved == wantTarget {
		return true
	}
	return wantRoot != "" && resolved == wantRoot
}

func resolveGoWorkUsePath(workDir, useEntry string) string {
	if filepath.IsAbs(useEntry) {
		return filepath.Clean(useEntry)
	}
	return filepath.Join(workDir, filepath.FromSlash(useEntry))
}

func canonicalGoWorkDir(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}
	if eval, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
		return filepath.Clean(eval)
	}
	return filepath.Clean(abs)
}

// ParseGoWorkUseDirs executes deterministic astdebt verification logic (@intent tree_sitter_ast_gate_spec.md).
func ParseGoWorkUseDirs(goWorkPath string) []string {
	raw, err := os.ReadFile(goWorkPath)
	if err != nil {
		return nil
	}
	var dirs []string
	inBlock := false
	for _, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(stripGoWorkComment(rawLine))
		if line == "" {
			continue
		}
		var entry string
		entry, inBlock = parseGoWorkLine(line, inBlock)
		if entry != "" {
			dirs = append(dirs, entry)
		}
	}
	return dirs
}

func parseGoWorkLine(line string, inBlock bool) (string, bool) {
	if inBlock {
		return parseGoWorkBlockLine(line)
	}
	return parseGoWorkTopLine(line)
}

func parseGoWorkBlockLine(line string) (string, bool) {
	if idx := strings.IndexByte(line, ')'); idx >= 0 {
		return cleanGoWorkToken(line[:idx]), false
	}
	return cleanGoWorkToken(line), true
}

func parseGoWorkTopLine(line string) (string, bool) {
	rest, ok := cutGoWorkUseDirective(line)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(rest, "(") {
		return parseGoWorkBlockLine(strings.TrimSpace(rest[1:]))
	}
	return cleanGoWorkToken(rest), false
}

func cutGoWorkUseDirective(line string) (string, bool) {
	if !strings.HasPrefix(line, "use") {
		return "", false
	}
	tail := line[len("use"):]
	if tail == "" || !isGoWorkDirectiveSep(tail[0]) {
		return "", false
	}
	return strings.TrimSpace(tail), true
}

func isGoWorkDirectiveSep(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '('
}

func stripGoWorkComment(line string) string {
	inQuote := byte(0)
	for i := 0; i < len(line); i++ {
		inQuote = nextGoWorkQuoteState(line[i], inQuote)
		if inQuote == 0 && isGoWorkLineCommentAt(line, i) {
			return line[:i]
		}
	}
	return line
}

func nextGoWorkQuoteState(ch, current byte) byte {
	if current != 0 {
		if ch == current {
			return 0
		}
		return current
	}
	if ch == '"' || ch == '`' {
		return ch
	}
	return 0
}

func isGoWorkLineCommentAt(line string, idx int) bool {
	return line[idx] == '/' && idx+1 < len(line) && line[idx+1] == '/'
}

func cleanGoWorkToken(tok string) string {
	trimmed := strings.TrimSpace(tok)
	if unquoted, ok := unquoteGoWorkToken(trimmed); ok {
		return unquoted
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func unquoteGoWorkToken(s string) (string, bool) {
	if len(s) < 2 {
		return "", false
	}
	first, last := s[0], s[len(s)-1]
	if (first == '"' && last == '"') || (first == '`' && last == '`') {
		return strings.TrimSpace(s[1 : len(s)-1]), true
	}
	return "", false
}
