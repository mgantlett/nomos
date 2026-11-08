package treesitter

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlRawTagRe  = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)>`)
	htmlTagToken  = regexp.MustCompile(`(?is)</?([a-zA-Z][a-zA-Z0-9-]*)\b[^>]*>`)
	pyDefCheckRe  = regexp.MustCompile(`(?m)^[ \t]*(?:async\s+)?def\s+\w+\s*\(`)
)

// ValidateBraceSyntax verifies balanced braces and parentheses outside comments and string literals.
func ValidateBraceSyntax(lang, filePath string, content []byte) error {
	cleaned := stripCLikeLiterals(string(content), lang != "rust")
	if eng := peekDefaultEngine(); eng != nil {
		if ok, handled := eng.scanDelimiterBalanceWasm(cleaned); handled {
			if !ok {
				return fmt.Errorf("%s syntax error: unbalanced or unclosed delimiter in %s", lang, filePath)
			}
			return nil
		}
	}
	braces, parens := 0, 0
	for _, ch := range cleaned {
		braces, parens = stepBraceParen(ch, braces, parens)
		if braces < 0 || parens < 0 {
			return fmt.Errorf("%s syntax error: unexpected closing delimiter in %s", lang, filePath)
		}
	}
	if braces != 0 || parens != 0 {
		return fmt.Errorf("%s syntax error: unclosed block or delimiter in %s", lang, filePath)
	}
	return nil
}

func stepBraceParen(ch rune, braces, parens int) (int, int) {
	switch ch {
	case '{':
		braces++
	case '}':
		braces--
	case '(':
		parens++
	case ')':
		parens--
	}
	return braces, parens
}

func stripCLikeLiterals(src string, singleQuoteStrings bool) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		if next, ok := skipCLikeComment(src, i, &out); ok {
			i = next
			continue
		}
		if next, ok := skipMultilineStringSpan(src, i, &out); ok {
			i = next
			continue
		}
		if next, ok := skipQuotedSpan(src, i, singleQuoteStrings); ok {
			span := src[i+1:]
			if next < len(src) {
				span = src[i+1 : next]
			}
			out.WriteByte(' ')
			out.WriteString(strings.Repeat("\n", strings.Count(span, "\n")))
			i = next
			continue
		}
		out.WriteByte(src[i])
	}
	return out.String()
}

func skipMultilineStringSpan(src string, i int, out *strings.Builder) (int, bool) {
	if i+2 < len(src) && src[i:i+3] == `"""` {
		end := strings.Index(src[i+3:], `"""`)
		if end < 0 {
			return len(src), true
		}
		out.WriteString(strings.Repeat("\n", strings.Count(src[i+3:i+3+end], "\n")))
		return i + 5 + end, true
	}
	if src[i] == '`' {
		end := advanceUntilQuoteClose(src, i, '`')
		span := src[i+1:]
		if end < len(src) {
			span = src[i+1 : end]
		}
		out.WriteString(strings.Repeat("\n", strings.Count(span, "\n")))
		return end, true
	}
	return i, false
}

func skipCLikeComment(src string, i int, out *strings.Builder) (int, bool) {
	if i+1 >= len(src) || src[i] != '/' {
		return i, false
	}
	if src[i+1] == '/' {
		end := strings.IndexByte(src[i+2:], '\n')
		if end < 0 {
			return len(src), true
		}
		out.WriteByte('\n')
		return i + 2 + end, true
	}
	if src[i+1] == '*' {
		end := strings.Index(src[i+2:], "*/")
		if end < 0 {
			return len(src), true
		}
		out.WriteString(strings.Repeat("\n", strings.Count(src[i+2:i+2+end], "\n")))
		return i + 3 + end, true
	}
	return i, false
}

func skipQuotedSpan(src string, i int, singleQuoteStrings bool) (int, bool) {
	ch := src[i]
	if ch == '\'' && !singleQuoteStrings {
		return skipRustCharLiteral(src, i)
	}
	if ch != '"' && ch != '\'' {
		return i, false
	}
	return advanceUntilQuoteClose(src, i, ch), true
}

func skipRustCharLiteral(src string, i int) (int, bool) {
	if i+2 < len(src) && src[i+2] == '\'' && src[i+1] != '\\' {
		return i + 2, true
	}
	if i+3 < len(src) && src[i+1] == '\\' && src[i+3] == '\'' {
		return i + 3, true
	}
	return i, false
}

func advanceUntilQuoteClose(src string, i int, quote byte) int {
	for j := i + 1; j < len(src); j++ {
		if src[j] == '\\' {
			j++
			continue
		}
		if src[j] == quote {
			return j
		}
	}
	return len(src)
}

// ValidatePythonSyntax checks balanced delimiters and complete function headers in Python source.
func ValidatePythonSyntax(filePath string, content []byte) error {
	cleaned := stripPythonLiterals(string(content))
	if err := checkBalancedDelimiters("python", cleaned, filePath); err != nil {
		return err
	}
	for _, loc := range pyDefCheckRe.FindAllStringIndex(cleaned, -1) {
		if !hasValidPySignatureEnd(cleaned[loc[1]-1:]) {
			return fmt.Errorf("python syntax error: malformed function declaration in %s", filePath)
		}
	}
	return nil
}

var pyDelimiterWeights = map[rune][3]int{
	'(': {1, 0, 0}, ')': {-1, 0, 0},
	'[': {0, 1, 0}, ']': {0, -1, 0},
	'{': {0, 0, 1}, '}': {0, 0, -1},
}

func checkBalancedDelimiters(lang, cleaned, filePath string) error {
	var counts [3]int
	for _, ch := range cleaned {
		w, ok := pyDelimiterWeights[ch]
		if !ok {
			continue
		}
		for idx := 0; idx < 3; idx++ {
			counts[idx] += w[idx]
			if counts[idx] < 0 {
				return fmt.Errorf("%s syntax error: unbalanced closing delimiter in %s", lang, filePath)
			}
		}
	}
	if counts != [3]int{0, 0, 0} {
		return fmt.Errorf("%s syntax error: unclosed delimiter in %s", lang, filePath)
	}
	return nil
}

func hasValidPySignatureEnd(tail string) bool {
	depth := 0
	for i := 0; i < len(tail); i++ {
		if tail[i] == '(' {
			depth++
		} else if tail[i] == ')' {
			depth--
			if depth == 0 {
				rest := tail[i+1:]
				colonIdx := strings.IndexByte(rest, ':')
				nlIdx := strings.IndexByte(rest, '\n')
				return colonIdx >= 0 && (nlIdx < 0 || colonIdx < nlIdx)
			}
		}
	}
	return false
}

func stripPythonLiterals(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	for i := 0; i < len(src); i++ {
		if src[i] == '#' {
			end := strings.IndexByte(src[i:], '\n')
			if end < 0 {
				break
			}
			out.WriteByte('\n')
			i += end
			continue
		}
		if next, ok := skipPyQuote(src, i, &out); ok {
			i = next
			continue
		}
		out.WriteByte(src[i])
	}
	return out.String()
}

func skipPyQuote(src string, i int, out *strings.Builder) (int, bool) {
	if i+2 < len(src) && (src[i:i+3] == `"""` || src[i:i+3] == `'''`) {
		delim := src[i : i+3]
		end := strings.Index(src[i+3:], delim)
		if end < 0 {
			return len(src), true
		}
		out.WriteString(strings.Repeat("\n", strings.Count(src[i+3:i+3+end], "\n")))
		return i + 5 + end, true
	}
	if next, ok := skipQuotedSpan(src, i, true); ok {
		span := src[i+1:]
		if next < len(src) {
			span = src[i+1 : next]
		}
		out.WriteByte(' ')
		out.WriteString(strings.Repeat("\n", strings.Count(span, "\n")))
		return next, true
	}
	return i, false
}

// ValidateHTMLSyntax checks balanced angle brackets and paired structural container tags.
func ValidateHTMLSyntax(filePath string, content []byte) error {
	cleaned := htmlCommentRe.ReplaceAllString(string(content), "")
	cleaned = htmlRawTagRe.ReplaceAllString(cleaned, "")
	if err := checkAngleBalance(cleaned, filePath); err != nil {
		return err
	}
	counts := make(map[string]int)
	for _, m := range htmlTagToken.FindAllStringSubmatch(cleaned, -1) {
		full, name := strings.TrimSpace(m[0]), strings.ToLower(m[1])
		if !isPairedHTMLContainer(name) || strings.HasSuffix(full, "/>") {
			continue
		}
		if strings.HasPrefix(full, "</") {
			counts[name]--
		} else {
			counts[name]++
		}
	}
	for tag, delta := range counts {
		if delta != 0 {
			return fmt.Errorf("html syntax error: unclosed <%s> element in %s", tag, filePath)
		}
	}
	return nil
}

func checkAngleBalance(cleaned, filePath string) error {
	inTag := false
	for i := 0; i < len(cleaned); i++ {
		if cleaned[i] == '<' {
			if inTag {
				return fmt.Errorf("html syntax error: nested unclosed '<' in %s", filePath)
			}
			inTag = true
		} else if cleaned[i] == '>' && inTag {
			inTag = false
		}
	}
	if inTag {
		return fmt.Errorf("html syntax error: unclosed tag bracket in %s", filePath)
	}
	return nil
}

func isPairedHTMLContainer(name string) bool {
	switch name {
	case "html", "head", "body", "div", "main", "section":
		return true
	default:
		return false
	}
}
