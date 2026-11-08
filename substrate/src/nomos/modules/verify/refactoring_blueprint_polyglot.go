// Package verify implements 11-language seam-extraction blueprints for long or complex functions.
package verify

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	polyglotBranchTokenRe = regexp.MustCompile(`\b(?:if|elif|elsif|for|while|switch|match|when|case|catch|except|rescue)\b`)
	rbBlockOpenTokenRe    = regexp.MustCompile(`(?:\b(?:def|if|unless|case|while|until|for|begin|class|module)\b|\bdo\b)`)
	rbBlockEndTokenRe     = regexp.MustCompile(`\bend\b`)
)

func prescribePolyglotFunctionSeams(filePath string, src []byte, funcLine int) []string {
	lines := strings.Split(string(src), "\n")
	startIdx := funcLine - 1
	if startIdx < 0 || startIdx >= len(lines) {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	bodyLines, endIdx := sliceLanguageFuncBody(ext, lines, startIdx)
	bodyText := strings.Join(bodyLines, "\n")

	var seams []string
	if params := countSignatureParamsAtLine(ext, string(src), lines, startIdx); params > MaxFunctionParams {
		seams = append(seams, fmt.Sprintf("[SEAM (parameter arity %d > %d): encapsulate parameters into a cohesive options/request type]", params, MaxFunctionParams))
	}
	seams = append(seams, extractPolyglotSubBlockSeams(bodyLines, funcLine)...)

	boolOps := countPolyglotBooleanOps(ext, bodyText)
	if boolOps >= 5 {
		if isJSFamilyExt(ext) {
			seams = append(seams, fmt.Sprintf("[SEAM: %d inline fallback/ternary branches (|| / ?? / ?:) -> replace with typed default object / Object.assign]", boolOps))
		} else {
			seams = append(seams, fmt.Sprintf("[SEAM: %d inline boolean/fallback branches -> extract predicate helper or lookup table]", boolOps))
		}
	}
	span := endIdx - startIdx + 1
	if span >= 20 {
		mid := startIdx + span/2
		seams = append(seams, fmt.Sprintf("[SEAM lines %d-%d (%d LOC body) -> split around line %d into cohesive helper functions]", funcLine, endIdx+1, span, mid+1))
	}
	return seams
}

func sliceLanguageFuncBody(ext string, lines []string, startIdx int) ([]string, int) {
	switch ext {
	case extPy:
		return slicePythonFuncBody(lines, startIdx)
	case extRB, extRake:
		return sliceRubyFuncBody(lines, startIdx)
	default:
		return sliceBraceFuncBody(lines, startIdx)
	}
}

func slicePythonFuncBody(lines []string, startIdx int) ([]string, int) {
	baseIndent := countLeadingIndentCols(lines[startIdx])
	bodyStart := startIdx + 1
	parenDepth := strings.Count(lines[startIdx], "(") - strings.Count(lines[startIdx], ")")
	for bodyStart < len(lines) && parenDepth > 0 {
		parenDepth += strings.Count(lines[bodyStart], "(") - strings.Count(lines[bodyStart], ")")
		bodyStart++
	}
	endIdx := startIdx
	for i := bodyStart; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if countLeadingIndentCols(lines[i]) <= baseIndent {
			break
		}
		endIdx = i
	}
	if endIdx < startIdx {
		endIdx = startIdx
	}
	return lines[startIdx : endIdx+1], endIdx
}

func sliceRubyFuncBody(lines []string, startIdx int) ([]string, int) {
	depth := 0
	maxIdx := minInt(len(lines), startIdx+160)
	for i := startIdx; i < maxIdx; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if idx := strings.Index(trimmed, "#"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		depth += len(rbBlockOpenTokenRe.FindAllStringIndex(trimmed, -1))
		depth -= len(rbBlockEndTokenRe.FindAllStringIndex(trimmed, -1))
		if i >= startIdx && depth <= 0 {
			return lines[startIdx : i+1], i
		}
	}
	return lines[startIdx:maxIdx], maxIdx - 1
}

func sliceBraceFuncBody(lines []string, startIdx int) ([]string, int) {
	depth := 0
	started := false
	maxIdx := minInt(len(lines), startIdx+160)
	for i := startIdx; i < maxIdx; i++ {
		clean := stripQuotedAndLineComments(lines[i])
		for _, ch := range clean {
			if ch == '{' {
				depth++
				started = true
			} else if ch == '}' {
				depth--
			}
		}
		if started && depth <= 0 {
			return lines[startIdx : i+1], i
		}
	}
	return lines[startIdx:maxIdx], maxIdx - 1
}

func extractPolyglotSubBlockSeams(bodyLines []string, funcStartLine int) []string {
	var seams []string
	for i := 1; i < len(bodyLines); i++ {
		trimmed := strings.TrimSpace(bodyLines[i])
		m := polyglotBranchTokenRe.FindString(trimmed)
		if m == "" || !strings.HasPrefix(trimmed, m) {
			continue
		}
		branches, endRel := measureSubBlockExtent(bodyLines, i)
		span := endRel - i + 1
		if branches >= 3 || span >= 12 {
			seams = append(seams, fmt.Sprintf("[SEAM lines %d-%d (%s block, +%d complexity, %d LOC) -> extract pure helper]", funcStartLine+i, funcStartLine+endRel, m, branches, span))
			i = endRel
		}
	}
	return seams
}

func measureSubBlockExtent(bodyLines []string, startRel int) (int, int) {
	branches := len(polyglotBranchTokenRe.FindAllStringIndex(strings.TrimSpace(bodyLines[startRel]), -1))
	endRel := startRel
	baseIndent := countLeadingIndentCols(bodyLines[startRel])
	for j := startRel + 1; j < len(bodyLines); j++ {
		jt := strings.TrimSpace(bodyLines[j])
		if jt == "" {
			continue
		}
		if countLeadingIndentCols(bodyLines[j]) <= baseIndent && jt != "}" && jt != "end" {
			break
		}
		branches += len(polyglotBranchTokenRe.FindAllStringIndex(jt, -1))
		endRel = j
		if countLeadingIndentCols(bodyLines[j]) <= baseIndent && (jt == "}" || jt == "end") {
			break
		}
	}
	return branches, endRel
}

func countPolyglotBooleanOps(ext, bodyText string) int {
	ops := strings.Count(bodyText, "&&") + strings.Count(bodyText, "||") +
		strings.Count(bodyText, "??") + strings.Count(bodyText, " ? ")
	if ext == extPy || ext == extRB || ext == extRake {
		ops += strings.Count(bodyText, " and ") + strings.Count(bodyText, " or ")
	}
	return ops
}

func stripQuotedAndLineComments(line string) string {
	var b strings.Builder
	var inQuote byte
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inQuote != 0 {
			if ch == '\\' {
				i++
				continue
			}
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			break
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inQuote = ch
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
