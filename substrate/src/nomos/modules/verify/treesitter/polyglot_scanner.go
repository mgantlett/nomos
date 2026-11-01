package treesitter

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// ExtractFunctions parses source code according to language semantics and extracts function metrics.
func ExtractFunctions(lang, filePath string, content []byte) ([]FunctionMetric, error) {
	switch lang {
	case "go":
		return scanGo(filePath, content)
	case "typescript":
		return scanTypeScript(filePath, content, 0)
	case "svelte":
		return scanSvelte(filePath, content)
	case "python":
		return scanPython(filePath, content)
	case "rust":
		return scanRust(filePath, content)
	case "c", "cpp":
		return scanC(filePath, content)
	case "markdown", "yaml", "html", "css":
		return []FunctionMetric{}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

func scanGo(filePath string, content []byte) ([]FunctionMetric, error) {
	src := content
	lineOffset := 0
	if !strings.Contains(string(content), "package ") {
		src = append([]byte("package p\n"), content...)
		lineOffset = 1
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("go parse error (fail-closed): %w", err)
	}

	var metrics []FunctionMetric
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		m := evaluateGoFunc(fn, fset, filePath)
		m.Line -= lineOffset
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func evaluateGoFunc(fn *ast.FuncDecl, fset *token.FileSet, filePath string) FunctionMetric {
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	comp := 1

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			comp++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				comp++
			}
		}
		return true
	})

	hasDoc := false
	if fn.Doc != nil && len(fn.Doc.List) > 0 {
		docEnd := fset.Position(fn.Doc.End()).Line
		if start-docEnd <= 1 {
			hasDoc = true
		}
	}

	isEmpty := false
	if fn.Body == nil || len(fn.Body.List) == 0 {
		isEmpty = true
	}

	return FunctionMetric{
		File:       filePath,
		Line:       start,
		Name:       fn.Name.Name,
		Complexity: comp,
		LOC:        end - start + 1,
		Documented: hasDoc,
		EmptyBlock: isEmpty,
	}
}

var (
	tsFuncRegex  = regexp.MustCompile(`(?m)(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\s*(\w+)?|const\s+(\w+)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[a-zA-Z0-9_$]+)\s*=>|(\w+)\s*\([^)]*\)\s*:\s*[^={]+=>|(\w+)\s*\([^)]*\)\s*(?::\s*[^={]+\s*)?\{)`)
	pyFuncRegex  = regexp.MustCompile(`^[ \t]*(?:async\s+)?def\s+(\w+)\s*\(`)
	svelteScript = regexp.MustCompile(`(?s)<script(?:\s+lang=["'](ts|js)["'])?\s*>(.*?)</script>`)
)

func scanTypeScript(filePath string, content []byte, lineOffset int) ([]FunctionMetric, error) {
	if err := ValidateBraceSyntax("typescript/javascript", filePath, content); err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	var metrics []FunctionMetric

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		if match := tsFuncRegex.FindStringSubmatch(line); len(match) > 0 {
			name := resolveTSFuncName(match)
			if name == "" {
				continue
			}
			startLine := i + 1 + lineOffset
			bodyLines, endLine, bErr := extractTSBody(lines, i)
			if bErr != nil {
				return nil, bErr
			}
			hasDoc := checkTSDoc(lines, i)
			comp := calculateTSComplexity(bodyLines)

			metrics = append(metrics, FunctionMetric{
				File:       filePath,
				Line:       startLine,
				Name:       name,
				Complexity: comp,
				LOC:        endLine - i,
				Documented: hasDoc,
				EmptyBlock: isTSEmptyBody(bodyLines),
			})
		}
	}
	return metrics, nil
}

var tsControlKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "with": true, "else": true,
}

func resolveTSFuncName(match []string) string {
	for j := 1; j < len(match); j++ {
		if match[j] != "" {
			name := match[j]
			if tsControlKeywords[name] {
				return ""
			}
			return name
		}
	}
	return ""
}

func extractTSBody(lines []string, startIdx int) ([]string, int, error) {
	depth := 0
	started := false
	var body []string
	endIdx := startIdx

	for i := startIdx; i < len(lines); i++ {
		body = append(body, lines[i])
		endIdx = i
		for _, ch := range stripCLikeLiterals(lines[i], true) {
			if ch == '{' {
				depth++
				started = true
			} else if ch == '}' {
				depth--
			}
		}
		if started && depth <= 0 {
			break
		}
	}
	if started && depth > 0 {
		return nil, 0, fmt.Errorf("unclosed block in typescript/javascript (syntax error)")
	}
	return body, endIdx + 1, nil
}

func checkTSDoc(lines []string, startIdx int) bool {
	if startIdx > 0 {
		prev := strings.TrimSpace(lines[startIdx-1])
		if strings.HasPrefix(prev, "/**") || strings.HasPrefix(prev, "//") || strings.HasSuffix(prev, "*/") {
			return true
		}
	}
	return false
}

func calculateTSComplexity(bodyLines []string) int {
	comp := 1
	body := strings.Join(bodyLines, "\n")
	tokens := []string{"if (", "if(", "for (", "for(", "while (", "while(", "case ", "catch (", "catch(", "&&", "||", "??", " ? "}
	for _, tok := range tokens {
		comp += strings.Count(body, tok)
	}
	return comp
}

func scanSvelte(filePath string, content []byte) ([]FunctionMetric, error) {
	text := string(content)
	matches := svelteScript.FindAllStringSubmatchIndex(text, -1)
	var metrics []FunctionMetric

	for _, loc := range matches {
		scriptStartByte := loc[4]
		if scriptStartByte < 0 {
			continue
		}
		scriptEndByte := loc[5]
		scriptContent := text[scriptStartByte:scriptEndByte]

		// Calculate starting line number in original svelte file
		lineOffset := strings.Count(text[:scriptStartByte], "\n")
		funcs, err := scanTypeScript(filePath, []byte(scriptContent), lineOffset)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, funcs...)
	}
	return metrics, nil
}

func scanPython(filePath string, content []byte) ([]FunctionMetric, error) {
	if err := ValidatePythonSyntax(filePath, content); err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	var metrics []FunctionMetric

	for i, line := range lines {
		if match := pyFuncRegex.FindStringSubmatch(line); len(match) > 1 {
			name := match[1]
			baseIndent := countIndent(line)
			sigEndIdx := advancePySignatureEnd(lines, i)
			bodyLines, endIdx := extractPyBody(lines, sigEndIdx, baseIndent)
			hasDoc := checkPyDoc(lines, i, sigEndIdx)
			comp := calculatePyComplexity(bodyLines)

			metrics = append(metrics, FunctionMetric{
				File:       filePath,
				Line:       i + 1,
				Name:       name,
				Complexity: comp,
				LOC:        endIdx - i + 1,
				Documented: hasDoc,
				EmptyBlock: isPyEmptyBody(bodyLines),
			})
		}
	}
	return metrics, nil
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

func extractPyBody(lines []string, startIdx, baseIndent int) ([]string, int) {
	var body []string
	endIdx := startIdx

	for i := startIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			endIdx = i
			body = append(body, lines[i])
			continue
		}
		if countIndent(lines[i]) <= baseIndent {
			break
		}
		endIdx = i
		body = append(body, lines[i])
	}
	return body, endIdx
}

func checkPyDoc(lines []string, startIdx, sigEndIdx int) bool {
	if startIdx > 0 && strings.HasPrefix(strings.TrimSpace(lines[startIdx-1]), "#") {
		return true
	}
	if sigEndIdx+1 < len(lines) {
		next := strings.TrimSpace(lines[sigEndIdx+1])
		if strings.HasPrefix(next, `"""`) || strings.HasPrefix(next, `'''`) {
			return true
		}
	}
	return false
}

func calculatePyComplexity(bodyLines []string) int {
	comp := 1
	body := strings.Join(bodyLines, "\n")
	branchWords := []string{" if ", "\tif ", " elif ", "\telif ", " for ", "\tfor ", " while ", "\twhile ", " except:", " except ", " and ", " or "}
	for _, word := range branchWords {
		comp += strings.Count(body, word)
	}
	return comp
}

func isTSEmptyBody(bodyLines []string) bool {
	joined := strings.Join(bodyLines, " ")
	firstBrace := strings.Index(joined, "{")
	lastBrace := strings.LastIndex(joined, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		inner := strings.TrimSpace(joined[firstBrace+1 : lastBrace])
		return inner == "" || inner == "return;" || inner == "return"
	}
	body := strings.TrimSpace(joined)
	return body == "{}" || body == ""
}

func isPyEmptyBody(bodyLines []string) bool {
	if len(bodyLines) == 0 {
		return true
	}
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "pass" || trimmed == "..." {
			return true
		}
		return false
	}
	return true
}

