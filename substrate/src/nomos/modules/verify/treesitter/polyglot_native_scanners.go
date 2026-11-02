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
	var metrics []FunctionMetric
	for i, cleanLine := range cleanLines {
		if m := spec.funcRegex.FindStringSubmatch(cleanLine); len(m) > 1 {
			name := m[1]
			if tsControlKeywords[name] {
				continue
			}
			cleanBody, endIdx := extractBraceBody(cleanLines, i)
			origBody := origLines[i : endIdx+1]
			hasDoc := checkPrecedingDoc(origLines, i, spec.docPrefixes...)
			comp := calculateBranchCount(cleanBody, spec.branchToks)
			empty := isBraceEmpty(origBody, spec.shimMarkers...)
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
