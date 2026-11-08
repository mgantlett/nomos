// Package verify enforces polyglot unchecked-error and silent-catch hygiene invariants.
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	emptyCatchBlockRe   = regexp.MustCompile(`\bcatch\s*(?:\([^)]*\))?\s*\{\s*\}`)
	emptyPromiseCatchRe = regexp.MustCompile(`\.catch\s*\(\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z0-9_$]+)?\s*(?:=>\s*\{\s*\}|function\s*\([^)]*\)\s*\{\s*\})\s*\)`)
	pyExceptHeaderRe    = regexp.MustCompile(`^(\s*)except(?:\s+[^:]+)?\s*:(.*)$`)
	rustLetBlankCallRe  = regexp.MustCompile(`^\s*let\s+_\s*=\s*([A-Za-z0-9_.:!]+)\s*\(`)
	rustEmptyErrArmRe   = regexp.MustCompile(`\bErr\s*\([^)]*\)\s*=>\s*(?:\{\s*\}|\(\s*\))`)
	rubyInlineRescueRe  = regexp.MustCompile(`\S+\s+rescue\s+nil\b`)
	rubyRescueHeaderRe  = regexp.MustCompile(`^(\s*)rescue(?:\s+[^#]+)?$`)
)

func evaluatePolyglotUncheckedFile(ctx *VerifyContext, root, path string) []string {
	ext := strings.ToLower(filepath.Ext(path))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	cur := checkPolyglotUncheckedErrors(ext, rel, raw)
	return filterNetNewLineViolations(ctx, root, path, cur, func(baseBytes []byte) []string {
		return checkPolyglotUncheckedErrors(ext, rel, baseBytes)
	})
}

func checkPolyglotUncheckedErrors(ext, relPath string, content []byte) []string {
	text := string(content)
	if isJSFamilyExt(ext) {
		return scanBraceSilentCatches(relPath, text, true)
	}
	switch ext {
	case extPy:
		return scanPythonSilentExcepts(relPath, text)
	case extJava, extKT, extKTS, extCS, extCPP, extCC, extCXX, extHPP:
		return scanBraceSilentCatches(relPath, text, false)
	case extRS:
		return scanRustErrorDiscards(relPath, text)
	case extRB, extRake:
		return scanRubySilentRescues(relPath, text)
	default:
		return nil
	}
}

func scanPythonSilentExcepts(relPath, text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		m := pyExceptHeaderRe.FindStringSubmatch(lines[i])
		if len(m) == 0 {
			continue
		}
		indent := countLeadingIndentCols(m[1])
		tail := strings.TrimSpace(m[2])
		if strings.Contains(tail, "#") {
			continue
		}
		if tail == "pass" || tail == "..." {
			out = append(out, fmt.Sprintf("%s:%d: silent exception suppression in except block", relPath, i+1))
			continue
		}
		if tail == "" && isPythonPassOnlyBlock(lines, i+1, indent) {
			out = append(out, fmt.Sprintf("%s:%d: silent exception suppression in except block", relPath, i+1))
		}
	}
	return out
}

func isPythonPassOnlyBlock(lines []string, startIdx, headerIndent int) bool {
	sawPass := false
	for j := startIdx; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			return false
		}
		curIndent := countLeadingIndentCols(lines[j])
		if curIndent <= headerIndent {
			break
		}
		if strings.Contains(trimmed, "#") {
			return false
		}
		if trimmed != "pass" && trimmed != "..." {
			return false
		}
		sawPass = true
	}
	return sawPass
}

func scanBraceSilentCatches(relPath, text string, checkPromises bool) []string {
	var out []string
	for _, m := range emptyCatchBlockRe.FindAllStringIndex(text, -1) {
		line := strings.Count(text[:m[0]], "\n") + 1
		out = append(out, fmt.Sprintf("%s:%d: empty catch block silently swallows exception", relPath, line))
	}
	if checkPromises {
		for _, m := range emptyPromiseCatchRe.FindAllStringIndex(text, -1) {
			line := strings.Count(text[:m[0]], "\n") + 1
			out = append(out, fmt.Sprintf("%s:%d: empty promise .catch() handler silently swallows rejection", relPath, line))
		}
	}
	return out
}

func scanRustErrorDiscards(relPath, text string) []string {
	if idx := strings.Index(text, "#[cfg(test)]"); idx >= 0 {
		text = text[:idx]
	}
	var out []string
	for idx, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.Contains(trimmed, "//") {
			continue
		}
		if m := rustLetBlankCallRe.FindStringSubmatch(line); len(m) > 1 && !isIgnoredRustBlankCall(m[1]) {
			out = append(out, fmt.Sprintf("%s:%d: discarded call result via let _ =", relPath, idx+1))
			continue
		}
		if rustEmptyErrArmRe.MatchString(line) {
			out = append(out, fmt.Sprintf("%s:%d: silent Err(_) match arm discard", relPath, idx+1))
		}
	}
	return out
}

func isIgnoredRustBlankCall(callee string) bool {
	ignoredSuffixes := []string{"write!", "writeln!", "flush", "close", "print!", "println!", "eprint!", "eprintln!"}
	for _, sfx := range ignoredSuffixes {
		if strings.HasSuffix(callee, sfx) {
			return true
		}
	}
	return false
}

func scanRubySilentRescues(relPath, text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "#") || strings.Contains(trimmed, "#") {
			continue
		}
		if rubyInlineRescueRe.MatchString(trimmed) {
			out = append(out, fmt.Sprintf("%s:%d: silent inline rescue nil suppression", relPath, i+1))
			continue
		}
		if m := rubyRescueHeaderRe.FindStringSubmatch(lines[i]); len(m) > 0 {
			if isRubyEmptyRescueBlock(lines, i+1, countLeadingIndentCols(m[1])) {
				out = append(out, fmt.Sprintf("%s:%d: empty rescue block silently swallows exception", relPath, i+1))
			}
		}
	}
	return out
}

func isRubyEmptyRescueBlock(lines []string, startIdx, rescueIndent int) bool {
	for j := startIdx; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "#") {
			return false
		}
		curIndent := countLeadingIndentCols(lines[j])
		if curIndent <= rescueIndent && (trimmed == "end" || strings.HasPrefix(trimmed, "ensure") || strings.HasPrefix(trimmed, "else") || strings.HasPrefix(trimmed, "rescue")) {
			return true
		}
		if trimmed != "nil" {
			return false
		}
	}
	return false
}

func countLeadingIndentCols(s string) int {
	cols := 0
	for _, ch := range s {
		if ch == ' ' {
			cols++
		} else if ch == '\t' {
			cols += 4
		} else {
			break
		}
	}
	return cols
}
