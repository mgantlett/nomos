// Package verify implements 11-language parameter-arity analysis with balanced generic delimiters.
package verify

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pyFuncDeclStartRe  = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	rbFuncDeclStartRe  = regexp.MustCompile(`(?m)^\s*def\s+(?:self\.)?([A-Za-z_]\w*[?!]?)\s*\(`)
	rsFuncDeclStartRe  = regexp.MustCompile(`(?m)^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:const\s+)?fn\s+([A-Za-z_]\w*)\s*(?:<[^>]*>)?\s*\(`)
	jsFuncDeclStartRe  = regexp.MustCompile(`(?m)(?:\bfunction\s+([A-Za-z_$][\w$]*)\s*(?:<[^>]*>)?\s*\(|\b(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:<[^>]*>)?\s*\()`)
	cLikeMethodStartRe = regexp.MustCompile(`(?m)^\s*(?:(?:public|private|protected|internal|static|final|virtual|override|async|inline|constexpr|suspend|fun)\s+)*(?:[A-Za-z_][\w:<>,.?\[\]\s*&]*\s+)?([A-Za-z_]\w*)\s*\(`)
)

var nonFuncControlKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"return": true, "sizeof": true, "alignof": true, "decltype": true, "synchronized": true,
	"lock": true, "using": true, "when": true, "match": true,
}

func countSignatureParamsAtLine(ext, fullText string, lines []string, startIdx int) int {
	offset := 0
	for i := 0; i < startIdx && i < len(lines); i++ {
		offset += len(lines[i]) + 1
	}
	if offset >= len(fullText) {
		return 0
	}
	openIdx := strings.IndexByte(fullText[offset:], '(')
	if openIdx < 0 || openIdx > 240 {
		return 0
	}
	paramRaw, _, ok := extractBalancedParenSpanWithEnd(fullText, offset+openIdx)
	if !ok {
		return 0
	}
	return countTopLevelParams(ext, paramRaw)
}

func scanPolyglotParamViolations(relPath string, src []byte, maxParams int) []string {
	ext := strings.ToLower(filepath.Ext(relPath))
	re := selectParamHeaderRegex(ext)
	if re == nil {
		return nil
	}
	text := string(src)
	matches := re.FindAllStringSubmatchIndex(text, -1)
	var violations []string
	for _, m := range matches {
		name := extractFirstCapturedGroup(text, m)
		openParen := m[1] - 1
		if name == "" || nonFuncControlKeywords[name] || openParen < 0 || text[openParen] != '(' {
			continue
		}
		if isJSFamilyExt(ext) && !isFollowedByFuncBodyOrArrow(text, openParen) {
			continue
		}
		paramRaw, closeIdx, ok := extractBalancedParenSpanWithEnd(text, openParen)
		if !ok {
			continue
		}
		if count := countTopLevelParams(ext, paramRaw); count > maxParams {
			startLine := strings.Count(text[:m[0]], "\n") + 1
			endLine := strings.Count(text[:closeIdx], "\n") + 1
			if endLine <= startLine {
				endLine = startLine + 1
			}
			groupMin := (count - maxParams) + 1
			span := formatFileSpan(relPath, startLine, endLine)
			violations = append(violations, fmt.Sprintf("%s (%s) parameter count %d exceeds max %d -> [1-TURN RECIPE: group >=%d parameters into a %sOptions object/dataclass (%s)]",
				span, name, count, maxParams, groupMin, name, span))
		}
	}
	return violations
}

func isJSFamilyExt(ext string) bool {
	switch ext {
	case extTS, extTSX, extJS, extJSX, extMJS, extCJS, extSvelte:
		return true
	default:
		return false
	}
}

func selectParamHeaderRegex(ext string) *regexp.Regexp {
	if isJSFamilyExt(ext) {
		return jsFuncDeclStartRe
	}
	switch ext {
	case extPy:
		return pyFuncDeclStartRe
	case extRB, extRake:
		return rbFuncDeclStartRe
	case extRS:
		return rsFuncDeclStartRe
	case extC, extH, extCPP, extCC, extCXX, extHPP, extJava, extKT, extKTS, extCS:
		return cLikeMethodStartRe
	default:
		return nil
	}
}

func extractFirstCapturedGroup(text string, m []int) string {
	for i := 2; i+1 < len(m); i += 2 {
		if m[i] >= 0 && m[i+1] >= 0 {
			return text[m[i]:m[i+1]]
		}
	}
	return ""
}

func isFollowedByFuncBodyOrArrow(text string, openParen int) bool {
	_, closeIdx, ok := extractBalancedParenSpanWithEnd(text, openParen)
	if !ok || closeIdx+1 >= len(text) {
		return false
	}
	rest := strings.TrimSpace(text[closeIdx+1:])
	if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "=>") {
		return true
	}
	if strings.HasPrefix(rest, ":") {
		if brace := strings.Index(rest, "{"); brace >= 0 && brace < 120 {
			return true
		}
		if arrow := strings.Index(rest, "=>"); arrow >= 0 && arrow < 120 {
			return true
		}
	}
	return false
}

func extractBalancedParenSpanWithEnd(text string, openParen int) (string, int, bool) {
	if openParen < 0 || openParen >= len(text) || text[openParen] != '(' {
		return "", -1, false
	}
	depth := 0
	var inQuote byte
	for i := openParen; i < len(text); i++ {
		ch := text[i]
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
		if ch == '"' || ch == '\'' || ch == '`' {
			inQuote = ch
			continue
		}
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth == 0 {
				return text[openParen+1 : i], i, true
			}
		}
	}
	return "", -1, false
}

func countTopLevelParams(ext, paramRaw string) int {
	trimmed := strings.TrimSpace(paramRaw)
	if trimmed == "" || ((ext == extC || ext == extH || ext == extCPP) && trimmed == "void") {
		return 0
	}
	count := 0
	for _, p := range splitTopLevelCommaParams(trimmed) {
		tok := cleanParamToken(p)
		if tok != "" && !isImplicitReceiverOrMarkerParam(ext, tok) {
			count++
		}
	}
	return count
}

func splitTopLevelCommaParams(s string) []string {
	var parts []string
	start, depth := 0, 0
	var inQuote byte
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote != 0 {
			if ch == '\\' {
				i++
			} else if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inQuote = ch
			continue
		}
		prev := byte(0)
		if i > 0 {
			prev = s[i-1]
		}
		depth = updateNestedParamDepth(depth, ch, prev)
		if ch == ',' && depth == 0 {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func updateNestedParamDepth(depth int, ch, prev byte) int {
	switch ch {
	case '(', '<', '[', '{':
		return depth + 1
	case ')', ']', '}':
		if depth > 0 {
			return depth - 1
		}
	case '>':
		if depth > 0 && prev != '=' && prev != '-' {
			return depth - 1
		}
	}
	return depth
}

func cleanParamToken(raw string) string {
	var kept []string
	for _, l := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(l)
		if idx := strings.Index(t, "//"); idx >= 0 {
			t = strings.TrimSpace(t[:idx])
		}
		if idx := strings.Index(t, "#"); idx >= 0 {
			t = strings.TrimSpace(t[:idx])
		}
		if t != "" {
			kept = append(kept, t)
		}
	}
	return strings.TrimSpace(strings.Join(kept, " "))
}

func isImplicitReceiverOrMarkerParam(ext, tok string) bool {
	if ext == ".py" {
		name := strings.TrimSpace(strings.SplitN(strings.SplitN(tok, ":", 2)[0], "=", 2)[0])
		return name == "self" || name == "cls" || name == "*" || name == "/"
	}
	if ext == ".rs" {
		return tok == "self" || tok == "mut self" || tok == "&self" || (strings.HasPrefix(tok, "&") && strings.HasSuffix(tok, " self"))
	}
	return false
}
