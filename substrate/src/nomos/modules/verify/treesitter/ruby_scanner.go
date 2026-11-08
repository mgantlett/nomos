package treesitter

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	rubyDefRe         = regexp.MustCompile(`^\s*def\s+(?:[a-zA-Z_]\w*\.)?([a-zA-Z_]\w*[?!=]?)`)
	rubyEndlessRe     = regexp.MustCompile(`^\s*def\s+(?:[a-zA-Z_]\w*\.)?[a-zA-Z_]\w*[?!]?(?:\([^)]*\))?\s+=\s*[^=>]`)
	rubyEndlessParen  = regexp.MustCompile(`^\s*def\s+(?:[a-zA-Z_]\w*\.)?[a-zA-Z_]\w*[?!]?\([^)]*\)\s*=\s*[^=>]`)
	rubyHeredocOpenRe = regexp.MustCompile(`<<[-~]?['"]?([A-Z_][A-Z0-9_]*)['"]?`)
	rubyDotMemberRe   = regexp.MustCompile(`\.[a-zA-Z_]\w*[?!=]?`)
	rubySymbolRe      = regexp.MustCompile(`:[a-zA-Z_]\w*[?!=]?`)
	rubyKwargKeyRe    = regexp.MustCompile(`\b[a-zA-Z_]\w*:`)
	rubyAlwaysOpenRe  = regexp.MustCompile(`\b(?:class|module|begin|case|do)\b`)
	rubyDefTokenRe    = regexp.MustCompile(`\bdef\b`)
	rubyLeadCondRe    = regexp.MustCompile(`^(?:if|unless|while|until|for)\b`)
	rubyExprCondRe    = regexp.MustCompile(`(?:[=(\[,]|<<|\|\|=|&&=|[+\-*/]=)\s*(?:if|unless|while|until|for)\b`)
	rubyEndTokenRe    = regexp.MustCompile(`\bend\b`)
	rubyVisibilityRe  = regexp.MustCompile(`^(?:private|protected|public|module_function|ruby2_keywords)\b`)
)

// ValidateRubySyntax checks balanced delimiters and block/end structure in Ruby source files.
func ValidateRubySyntax(filePath string, content []byte) error {
	cleaned := stripRubyLiterals(string(content))
	if err := checkBalancedDelimiters("ruby", cleaned, filePath); err != nil {
		return err
	}
	depth := 0
	for _, line := range strings.Split(cleaned, "\n") {
		depth += countRubyBlockDelta(line)
		if depth < 0 {
			return fmt.Errorf("ruby syntax error: unexpected closing 'end' in %s", filePath)
		}
	}
	if depth != 0 {
		return fmt.Errorf("ruby syntax error: unclosed block (missing 'end') in %s", filePath)
	}
	return nil
}

func scanRuby(filePath string, content []byte) ([]FunctionMetric, error) {
	if err := ValidateRubySyntax(filePath, content); err != nil {
		return nil, err
	}
	origLines := strings.Split(string(content), "\n")
	cleanLines := strings.Split(stripRubyLiterals(string(content)), "\n")
	var spans []funcSpan
	for i, cleanLine := range cleanLines {
		m := rubyDefRe.FindStringSubmatch(cleanLine)
		if len(m) < 2 {
			continue
		}
		endIdx := i
		if !isRubyEndlessDef(cleanLine) {
			endIdx = findRubyBlockEnd(cleanLines, i)
		}
		spans = append(spans, funcSpan{name: m[1], startIdx: i, headerEnd: i, endIdx: endIdx})
	}
	metrics := make([]FunctionMetric, 0, len(spans))
	for _, sp := range spans {
		metrics = append(metrics, buildRubyMethodMetric(filePath, origLines, cleanLines, sp, spans))
	}
	return metrics, nil
}

func buildRubyMethodMetric(filePath string, origLines, cleanLines []string, sp funcSpan, spans []funcSpan) FunctionMetric {
	cleanHeader := cleanLines[sp.startIdx]
	hasDoc := checkRubyDoc(origLines, sp.startIdx)
	if isRubyEndlessDef(cleanHeader) {
		return FunctionMetric{
			File:       filePath,
			Line:       sp.startIdx + 1,
			Name:       sp.name,
			Complexity: calculateRubyComplexity([]string{cleanHeader}),
			LOC:        1,
			Documented: hasDoc,
			EmptyBlock: isRubyShimExpr(extractRubyEndlessBody(origLines[sp.startIdx])),
		}
	}
	maskedBody := maskNestedLines(cleanLines, sp.startIdx, sp.endIdx, spans)
	origBody := origLines[sp.startIdx : sp.endIdx+1]
	return FunctionMetric{
		File:       filePath,
		Line:       sp.startIdx + 1,
		Name:       sp.name,
		Complexity: calculateRubyComplexity(maskedBody),
		LOC:        sp.endIdx - sp.startIdx + 1,
		Documented: hasDoc,
		EmptyBlock: isRubyEmptyBody(origBody),
	}
}

func isRubyEndlessDef(cleanLine string) bool {
	return rubyEndlessRe.MatchString(cleanLine) || rubyEndlessParen.MatchString(cleanLine)
}

func extractRubyEndlessBody(origLine string) string {
	searchFrom := 0
	if closeParen := strings.IndexByte(origLine, ')'); closeParen >= 0 {
		searchFrom = closeParen + 1
	}
	tail := origLine[searchFrom:]
	eqIdx := strings.IndexByte(tail, '=')
	if eqIdx < 0 || eqIdx+1 >= len(tail) {
		return ""
	}
	return strings.TrimSpace(tail[eqIdx+1:])
}

func findRubyBlockEnd(cleanLines []string, startIdx int) int {
	depth := 0
	for j := startIdx; j < len(cleanLines); j++ {
		depth += countRubyBlockDelta(cleanLines[j])
		if depth <= 0 {
			return j
		}
	}
	return len(cleanLines) - 1
}

func countRubyBlockDelta(cleanLine string) int {
	sanitized := rubyDotMemberRe.ReplaceAllString(cleanLine, "")
	sanitized = rubySymbolRe.ReplaceAllString(sanitized, "")
	sanitized = rubyKwargKeyRe.ReplaceAllString(sanitized, "")

	delta := 0
	for _, rawClause := range strings.Split(sanitized, ";") {
		clause := strings.TrimSpace(rawClause)
		if clause == "" {
			continue
		}
		delta += countRubyClauseOpeners(clause)
		delta -= len(rubyEndTokenRe.FindAllStringIndex(clause, -1))
	}
	return delta
}

func countRubyClauseOpeners(clause string) int {
	openers := len(rubyAlwaysOpenRe.FindAllStringIndex(clause, -1))
	if rubyDefTokenRe.MatchString(clause) && !isRubyEndlessDef(clause) {
		openers++
	}
	if rubyLeadCondRe.MatchString(clause) {
		openers++
	}
	openers += len(rubyExprCondRe.FindAllStringIndex(clause, -1))
	return openers
}

func checkRubyDoc(lines []string, startIdx int) bool {
	for idx := startIdx - 1; idx >= 0 && startIdx-idx <= 2; idx-- {
		trimmed := strings.TrimSpace(lines[idx])
		if strings.HasPrefix(trimmed, "#") {
			return true
		}
		if trimmed == "" || !rubyVisibilityRe.MatchString(trimmed) {
			return false
		}
	}
	return false
}

var rubyBranchTokens = []string{
	" if ", "\tif ", ";if ",
	" unless ", "\tunless ",
	" elsif ", "\telsif ",
	" when ", "\twhen ",
	" while ", "\twhile ",
	" until ", "\tuntil ",
	" for ", "\tfor ",
	" rescue", "\trescue",
	"&&", "||", " and ", " or ", " ? ",
}

func calculateRubyComplexity(cleanBodyLines []string) int {
	comp := 1
	padded := " " + strings.Join(cleanBodyLines, "\n ") + " "
	for _, tok := range rubyBranchTokens {
		comp += strings.Count(padded, tok)
	}
	return comp
}

func isRubyEmptyBody(origBody []string) bool {
	if len(origBody) == 1 {
		return isSingleLineRubyShim(origBody[0])
	}
	var stmts []string
	for i := 1; i < len(origBody)-1; i++ {
		trimmed := strings.TrimSpace(origBody[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		stmts = append(stmts, trimmed)
	}
	if len(stmts) == 0 {
		return true
	}
	return len(stmts) == 1 && isRubyShimExpr(stmts[0])
}

func isSingleLineRubyShim(line string) bool {
	semi := strings.IndexByte(line, ';')
	endIdx := strings.LastIndex(line, "end")
	if semi < 0 || endIdx <= semi {
		return true
	}
	inner := strings.TrimSpace(line[semi+1 : endIdx])
	return inner == "" || isRubyShimExpr(inner)
}

func isRubyShimExpr(expr string) bool {
	return strings.HasPrefix(expr, "raise NotImplementedError") || strings.HasPrefix(expr, "fail NotImplementedError")
}

func stripRubyLiterals(src string) string {
	pre := stripRubyBlockConstructs(src)
	var out strings.Builder
	out.Grow(len(pre))
	for i := 0; i < len(pre); i++ {
		if pre[i] == '#' {
			nl := strings.IndexByte(pre[i:], '\n')
			if nl < 0 {
				break
			}
			out.WriteByte('\n')
			i += nl
			continue
		}
		if pre[i] == '"' || pre[i] == '\'' {
			end := advanceUntilQuoteClose(pre, i, pre[i])
			span := pre[i+1:]
			if end < len(pre) {
				span = pre[i+1 : end]
			}
			out.WriteByte(' ')
			out.WriteString(strings.Repeat("\n", strings.Count(span, "\n")))
			i = end
			continue
		}
		out.WriteByte(pre[i])
	}
	return out.String()
}

func stripRubyBlockConstructs(src string) string {
	lines := strings.Split(src, "\n")
	inBlockComment := false
	heredocEnd := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inBlockComment {
			lines[i] = ""
			if strings.HasPrefix(trimmed, "=end") {
				inBlockComment = false
			}
			continue
		}
		if heredocEnd != "" {
			lines[i] = ""
			if trimmed == heredocEnd {
				heredocEnd = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "=begin") {
			lines[i] = ""
			inBlockComment = true
			continue
		}
		if loc := rubyHeredocOpenRe.FindStringSubmatchIndex(line); len(loc) >= 4 {
			if hashIdx := strings.IndexByte(line, '#'); hashIdx < 0 || loc[0] < hashIdx {
				heredocEnd = line[loc[2]:loc[3]]
				lines[i] = line[:loc[0]]
			}
		}
	}
	return strings.Join(lines, "\n")
}
