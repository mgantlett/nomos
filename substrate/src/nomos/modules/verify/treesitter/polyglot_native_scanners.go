package treesitter

import (
	"regexp"
	"strings"
)

var (
	rustFuncRegex = regexp.MustCompile(`(?m)^\s*(?:pub(?:\([^)]+\))?\s+)?(?:async\s+)?(?:unsafe\s+)?fn\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:<[^>]*>)?\s*\(`)
	cFuncRegex    = regexp.MustCompile(`(?m)^\s*(?:[a-zA-Z_][a-zA-Z0-9_*\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*\{`)
)

func scanRust(filePath string, content []byte) ([]FunctionMetric, error) {
	if err := ValidateBraceSyntax("rust", filePath, content); err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	var metrics []FunctionMetric
	for i, line := range lines {
		if m := rustFuncRegex.FindStringSubmatch(line); len(m) > 1 {
			name := m[1]
			bodyLines, endIdx := extractBraceBody(lines, i)
			hasDoc := checkPrecedingDoc(lines, i, "///", "//", "/**")
			tokens := []string{"if ", "match ", "for ", "while ", "loop ", "?", "&&", "||"}
			comp := calculateBranchCount(bodyLines, tokens)
			empty := isBraceEmpty(bodyLines, "todo!()", "unimplemented!()")
			metrics = append(metrics, FunctionMetric{
				File:       filePath,
				Line:       i + 1,
				Name:       name,
				Complexity: comp,
				LOC:        endIdx - i + 1,
				Documented: hasDoc,
				EmptyBlock: empty,
			})
		}
	}
	return metrics, nil
}

func scanC(filePath string, content []byte) ([]FunctionMetric, error) {
	if err := ValidateBraceSyntax("c/c++", filePath, content); err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	var metrics []FunctionMetric
	for i, line := range lines {
		if m := cFuncRegex.FindStringSubmatch(line); len(m) > 1 {
			name := m[1]
			bodyLines, endIdx := extractBraceBody(lines, i)
			hasDoc := checkPrecedingDoc(lines, i, "/**", "/*", "//")
			tokens := []string{"if (", "if(", "for (", "for(", "while (", "while(", "case ", "switch (", "&&", "||"}
			comp := calculateBranchCount(bodyLines, tokens)
			empty := isBraceEmpty(bodyLines)
			metrics = append(metrics, FunctionMetric{
				File:       filePath,
				Line:       i + 1,
				Name:       name,
				Complexity: comp,
				LOC:        endIdx - i + 1,
				Documented: hasDoc,
				EmptyBlock: empty,
			})
		}
	}
	return metrics, nil
}

func extractBraceBody(lines []string, startLine int) ([]string, int) {
	braceDepth := 0
	foundFirst := false
	var body []string
	endLine := startLine

	for j := startLine; j < len(lines); j++ {
		l := lines[j]
		body = append(body, l)
		cleanLine := stripCLikeLiterals(l, false)
		for _, ch := range cleanLine {
			if ch == '{' {
				braceDepth++
				foundFirst = true
			} else if ch == '}' {
				braceDepth--
			}
		}
		if foundFirst && braceDepth <= 0 {
			endLine = j
			break
		}
	}
	return body, endLine
}

func calculateBranchCount(bodyLines []string, tokens []string) int {
	comp := 1
	joined := strings.Join(bodyLines, "\n")
	for _, tok := range tokens {
		comp += strings.Count(joined, tok)
	}
	return comp
}

func checkPrecedingDoc(lines []string, startIdx int, docPrefixes ...string) bool {
	if startIdx <= 0 {
		return false
	}
	prev := strings.TrimSpace(lines[startIdx-1])
	for _, p := range docPrefixes {
		if strings.HasPrefix(prev, p) || strings.HasSuffix(prev, "*/") {
			return true
		}
	}
	return false
}

func isBraceEmpty(bodyLines []string, shims ...string) bool {
	joined := strings.Join(bodyLines, " ")
	firstBrace := strings.Index(joined, "{")
	lastBrace := strings.LastIndex(joined, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		inner := strings.TrimSpace(joined[firstBrace+1 : lastBrace])
		if inner == "" || inner == "return;" || inner == "return" {
			return true
		}
		for _, s := range shims {
			if strings.TrimSpace(inner) == s {
				return true
			}
		}
	}
	return false
}

func scanMarkdown(_ string, content []byte) ([]int, bool) {
	lines := strings.Split(string(content), "\n")
	var headingLevels []int
	for _, line := range lines {
		if lvl := markdownHeadingLevel(strings.TrimSpace(line)); lvl > 0 {
			headingLevels = append(headingLevels, lvl)
		}
	}
	return headingLevels, len(headingLevels) > 0
}

func markdownHeadingLevel(trimmed string) int {
	if !strings.HasPrefix(trimmed, "#") {
		return 0
	}
	level := 0
	for _, r := range trimmed {
		if r != '#' {
			break
		}
		level++
	}
	if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
		return level
	}
	return 0
}
