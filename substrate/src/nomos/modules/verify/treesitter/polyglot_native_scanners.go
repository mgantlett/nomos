package treesitter

import (
	"regexp"
	"strings"
)

var (
	rustFuncRegex   = regexp.MustCompile(`(?m)^\s*(?:pub(?:\([^)]+\))?\s+)?(?:async\s+)?(?:unsafe\s+)?fn\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:<[^>]*>)?\s*\(`)
	cFuncRegex      = regexp.MustCompile(`(?m)^\s*(?:[a-zA-Z_][a-zA-Z0-9_*\s]+)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*\{`)
	kotlinFuncRegex = regexp.MustCompile(`(?m)^\s*(?:(?:public|private|protected|internal|override|suspend|inline|open)\s+)*fun\s+(?:<[^>]+>\s*)?([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	javaMethodRegex = regexp.MustCompile(`(?m)^\s*(?:(?:public|private|protected|static|final|synchronized|abstract|default)\s+)+(?:<[^>]+>\s*)?[a-zA-Z_][a-zA-Z0-9_<>,\[\]\s?]*\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)\s*(?:throws\s+[a-zA-Z0-9_,\s]+)?\{`)
	csharpMethodRe  = regexp.MustCompile(`(?m)^\s*(?:(?:public|private|protected|internal|static|virtual|override|async|sealed|new)\s+)+(?:[a-zA-Z_][a-zA-Z0-9_<>,\[\]?]*\s+)+([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:<[^>]+>)?\s*\([^)]*\)\s*\{`)
)

type braceScanSpec struct {
	langName    string
	funcRegex   *regexp.Regexp
	docPrefixes []string
	branchToks  []string
	shimMarkers []string
}

func scanBraceLanguage(filePath string, content []byte, spec braceScanSpec) ([]FunctionMetric, error) {
	if err := ValidateBraceSyntax(spec.langName, filePath, content); err != nil {
		return nil, err
	}
	origLines := strings.Split(string(content), "\n")
	cleanLines := strings.Split(stripCLikeLiterals(string(content), spec.langName != "rust"), "\n")
	spans := collectBraceFuncSpans(cleanLines, spec.funcRegex)
	metrics := make([]FunctionMetric, 0, len(spans))
	for _, sp := range spans {
		masked := maskNestedLines(cleanLines, sp.startIdx, sp.endIdx, spans)
		origBody := origLines[sp.startIdx : sp.endIdx+1]
		metrics = append(metrics, FunctionMetric{
			File:       filePath,
			Line:       sp.startIdx + 1,
			Name:       sp.name,
			Complexity: calculateBranchCount(masked, spec.branchToks),
			LOC:        sp.endIdx - sp.startIdx + 1,
			Documented: checkPrecedingDoc(origLines, sp.startIdx, spec.docPrefixes...),
			EmptyBlock: isBraceEmpty(origBody, spec.shimMarkers...),
		})
	}
	return metrics, nil
}

func collectBraceFuncSpans(cleanLines []string, funcRegex *regexp.Regexp) []funcSpan {
	var spans []funcSpan
	skipUntilHeader := -1
	for i := 0; i < len(cleanLines); i++ {
		if i <= skipUntilHeader || strings.TrimSpace(cleanLines[i]) == "" {
			continue
		}
		header, headerEnd := collapseHeaderLines(cleanLines, i)
		m := funcRegex.FindStringSubmatch(header)
		if len(m) < 2 || tsControlKeywords[m[1]] {
			continue
		}
		_, endIdx := extractBraceBody(cleanLines, i)
		skipUntilHeader = headerEnd
		spans = append(spans, funcSpan{name: m[1], startIdx: i, headerEnd: headerEnd, endIdx: endIdx})
	}
	return spans
}

func scanRust(filePath string, content []byte) ([]FunctionMetric, error) {
	return scanBraceLanguage(filePath, content, braceScanSpec{
		langName:    "rust",
		funcRegex:   rustFuncRegex,
		docPrefixes: []string{"///", "//", "/**"},
		branchToks:  []string{"if ", "match ", "for ", "while ", "loop ", "?", "&&", "||"},
		shimMarkers: []string{"todo!()", "unimplemented!()"},
	})
}

func scanC(filePath string, content []byte) ([]FunctionMetric, error) {
	return scanBraceLanguage(filePath, content, braceScanSpec{
		langName:    "c/c++",
		funcRegex:   cFuncRegex,
		docPrefixes: []string{"/**", "/*", "//"},
		branchToks:  []string{"if (", "if(", "for (", "for(", "while (", "while(", "case ", "switch (", "&&", "||"},
	})
}

func scanJava(filePath string, content []byte) ([]FunctionMetric, error) {
	return scanBraceLanguage(filePath, content, braceScanSpec{
		langName:    "java",
		funcRegex:   javaMethodRegex,
		docPrefixes: []string{"/**", "/*", "//"},
		branchToks:  []string{"if (", "if(", "for (", "for(", "while (", "while(", "case ", "catch (", "catch(", "&&", "||", " ? "},
		shimMarkers: []string{"throw new UnsupportedOperationException"},
	})
}

func scanKotlin(filePath string, content []byte) ([]FunctionMetric, error) {
	return scanBraceLanguage(filePath, content, braceScanSpec{
		langName:    "kotlin",
		funcRegex:   kotlinFuncRegex,
		docPrefixes: []string{"/**", "/*", "//"},
		branchToks:  []string{"if (", "if(", "when (", "when(", "for (", "for(", "while (", "while(", "catch (", "&&", "||", "?:"},
		shimMarkers: []string{"TODO(", "TODO()", "throw UnsupportedOperationException"},
	})
}

func scanCSharp(filePath string, content []byte) ([]FunctionMetric, error) {
	return scanBraceLanguage(filePath, content, braceScanSpec{
		langName:    "csharp",
		funcRegex:   csharpMethodRe,
		docPrefixes: []string{"///", "/**", "/*", "//"},
		branchToks:  []string{"if (", "if(", "for (", "for(", "foreach (", "foreach(", "while (", "while(", "case ", "catch (", "&&", "||", "??"},
		shimMarkers: []string{"throw new NotImplementedException"},
	})
}

func extractBraceBody(cleanLines []string, startLine int) ([]string, int) {
	braceDepth := 0
	foundFirst := false
	var body []string
	endLine := startLine

	for j := startLine; j < len(cleanLines); j++ {
		l := cleanLines[j]
		body = append(body, l)
		for _, ch := range l {
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
			trimmedInner := strings.TrimSpace(inner)
			if trimmedInner == s || strings.HasPrefix(trimmedInner, s) {
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
