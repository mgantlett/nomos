package treesitter

import (
	"regexp"
	"strings"
)

var (
	pyFuncRegex    = regexp.MustCompile(`^[ \t]*(?:async\s+)?def\s+(\w+)\s*\(`)
	pyBranchWordRe = regexp.MustCompile(`\b(?:if|elif|for|while|except|case|and|or)\b`)
)

func scanPython(filePath string, content []byte) ([]FunctionMetric, error) {
	if err := ValidatePythonSyntax(filePath, content); err != nil {
		return nil, err
	}
	origLines := strings.Split(string(content), "\n")
	cleanLines := strings.Split(stripPythonLiterals(string(content)), "\n")
	spans := collectPyFuncSpans(origLines, cleanLines)
	metrics := make([]FunctionMetric, 0, len(spans))

	for _, sp := range spans {
		maskedBody := maskNestedLines(cleanLines, sp.headerEnd+1, sp.endIdx, spans)
		origBody := origLines[sp.headerEnd+1 : sp.endIdx+1]
		cleanBody := cleanLines[sp.headerEnd+1 : sp.endIdx+1]
		metrics = append(metrics, FunctionMetric{
			File:       filePath,
			Line:       sp.startIdx + 1,
			Name:       sp.name,
			Complexity: calculatePyComplexity(maskedBody),
			LOC:        sp.endIdx - sp.startIdx + 1,
			Documented: checkPyDoc(origLines, sp.startIdx, sp.headerEnd),
			EmptyBlock: isPyEmptyCleanBody(origBody, cleanBody),
		})
	}
	return metrics, nil
}

func collectPyFuncSpans(origLines, cleanLines []string) []funcSpan {
	var spans []funcSpan
	for i, cleanLine := range cleanLines {
		match := pyFuncRegex.FindStringSubmatch(cleanLine)
		if len(match) < 2 {
			continue
		}
		baseIndent := countIndent(cleanLine)
		sigEndIdx := advancePySignatureEnd(cleanLines, i)
		endIdx := findPyBodyEnd(origLines, cleanLines, sigEndIdx, baseIndent)
		spans = append(spans, funcSpan{name: match[1], startIdx: i, headerEnd: sigEndIdx, endIdx: endIdx})
	}
	return spans
}

func findPyBodyEnd(origLines, cleanLines []string, sigEndIdx, baseIndent int) int {
	endIdx := sigEndIdx
	for i := sigEndIdx + 1; i < len(cleanLines); i++ {
		trimmedClean := strings.TrimSpace(cleanLines[i])
		if trimmedClean == "" {
			if i < len(origLines) {
				trimmedOrig := strings.TrimSpace(origLines[i])
				if trimmedOrig != "" && (!strings.HasPrefix(trimmedOrig, "#") || countIndent(origLines[i]) > baseIndent) {
					endIdx = i
				}
			}
			continue
		}
		if countIndent(cleanLines[i]) <= baseIndent {
			break
		}
		endIdx = i
	}
	return endIdx
}

func advancePySignatureEnd(lines []string, startIdx int) int {
	depth := 0
	for i := startIdx; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '(' {
				depth++
			} else if ch == ')' {
				depth--
			}
		}
		if depth <= 0 && strings.Contains(lines[i], ":") {
			return i
		}
	}
	return startIdx
}

func countIndent(line string) int {
	indent := 0
	for _, ch := range line {
		if ch == ' ' {
			indent++
		} else if ch == '\t' {
			indent += 4
		} else {
			break
		}
	}
	return indent
}

func checkPyDoc(lines []string, startIdx, sigEndIdx int) bool {
	for idx := startIdx - 1; idx >= 0 && startIdx-idx <= 3; idx-- {
		trimmed := strings.TrimSpace(lines[idx])
		if strings.HasPrefix(trimmed, "#") {
			return true
		}
		if trimmed == "" || !strings.HasPrefix(trimmed, "@") {
			break
		}
	}
	if sigEndIdx+1 < len(lines) {
		next := strings.TrimSpace(lines[sigEndIdx+1])
		if strings.HasPrefix(next, `"""`) || strings.HasPrefix(next, `'''`) || strings.HasPrefix(next, `r"""`) {
			return true
		}
	}
	return false
}

func calculatePyComplexity(cleanBodyLines []string) int {
	if len(cleanBodyLines) == 0 {
		return 1
	}
	return 1 + len(pyBranchWordRe.FindAllStringIndex(strings.Join(cleanBodyLines, "\n"), -1))
}

func isPyEmptyCleanBody(origBody, cleanBody []string) bool {
	if len(cleanBody) == 0 {
		return len(origBody) == 0
	}
	for _, line := range cleanBody {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == "pass" || trimmed == "..." || strings.HasPrefix(trimmed, "raise NotImplementedError") {
			return true
		}
		return false
	}
	return true
}

