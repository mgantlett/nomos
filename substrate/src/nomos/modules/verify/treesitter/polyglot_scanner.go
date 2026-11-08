package treesitter

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var langScannerMap = map[string]func(string, []byte) ([]FunctionMetric, error){
	"go":         scanGo,
	"typescript": func(fp string, c []byte) ([]FunctionMetric, error) { return scanTypeScript(fp, c, 0) },
	"svelte":     scanSvelte,
	"python":     scanPython,
	"ruby":       scanRuby,
	"rust":       scanRust,
	"c":          scanC,
	"cpp":        scanC,
	"java":       scanJava,
	"kotlin":     scanKotlin,
	"csharp":     scanCSharp,
}

// ExtractFunctions parses source code according to language semantics and extracts function metrics.
func ExtractFunctions(lang, filePath string, content []byte) ([]FunctionMetric, error) {
	if scanner, ok := langScannerMap[lang]; ok {
		if _, err := ValidateFunctionSCMContract(lang); err != nil {
			return nil, err
		}
		return scanner(filePath, content)
	}
	switch lang {
	case "markdown", "yaml":
		return []FunctionMetric{}, nil
	case "html", "css":
		if _, err := LoadSCMQuery(lang); err != nil {
			return nil, err
		}
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
	tsFuncRegex  = regexp.MustCompile(`(?m)(?:export\s+)?(?:default\s+)?(?:(?:public|private|protected|static|readonly|override|async|get|set)\s+)*(?:function\s*\*?\s*(\w+)?|(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(?:async\s*)?(?:\([^)]*\)|[a-zA-Z0-9_$]+)\s*(?::[^=]+)?=>|(\w+)\s*(?:<[^>]*>)?\s*\([^)]*\)\s*:\s*[^={]+=>|(\w+)\s*(?:<[^>]*>)?\s*\([^)]*\)\s*(?::\s*[^={]+\s*)?\{)`)
	svelteScript = regexp.MustCompile(`(?s)<script(?:\s+lang=["'](ts|js)["'])?\s*>(.*?)</script>`)
)

func scanTypeScript(filePath string, content []byte, lineOffset int) ([]FunctionMetric, error) {
	if err := ValidateBraceSyntax("typescript/javascript", filePath, content); err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	cleanLines := strings.Split(stripCLikeLiterals(string(content), true), "\n")
	spans, err := collectTSFuncSpans(cleanLines)
	if err != nil {
		return nil, err
	}
	metrics := make([]FunctionMetric, 0, len(spans))
	for _, sp := range spans {
		masked := maskNestedLines(cleanLines, sp.startIdx, sp.endIdx, spans)
		origBody := lines[sp.startIdx : sp.endIdx+1]
		metrics = append(metrics, FunctionMetric{
			File:       filePath,
			Line:       sp.startIdx + 1 + lineOffset,
			Name:       sp.name,
			Complexity: calculateTSComplexity(masked),
			LOC:        sp.endIdx - sp.startIdx + 1,
			Documented: checkTSDoc(lines, sp.startIdx),
			EmptyBlock: isTSEmptyBody(origBody),
		})
	}
	return metrics, nil
}

func collectTSFuncSpans(cleanLines []string) ([]funcSpan, error) {
	var spans []funcSpan
	skipUntilHeader := -1
	for i := 0; i < len(cleanLines); i++ {
		if i <= skipUntilHeader || strings.TrimSpace(cleanLines[i]) == "" {
			continue
		}
		header, headerEnd := collapseHeaderLines(cleanLines, i)
		match := tsFuncRegex.FindStringSubmatch(header)
		if len(match) == 0 {
			continue
		}
		name := resolveTSFuncName(match)
		if name == "" {
			continue
		}
		_, endLine, bErr := extractTSBody(cleanLines, i)
		if bErr != nil {
			return nil, bErr
		}
		skipUntilHeader = headerEnd
		spans = append(spans, funcSpan{name: name, startIdx: i, headerEnd: headerEnd, endIdx: endLine - 1})
	}
	return spans, nil
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

func extractTSBody(cleanLines []string, startIdx int) ([]string, int, error) {
	depth := 0
	started := false
	var body []string
	endIdx := startIdx

	for i := startIdx; i < len(cleanLines); i++ {
		body = append(body, cleanLines[i])
		endIdx = i
		for _, ch := range cleanLines[i] {
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

func isTSEmptyBody(bodyLines []string) bool {
	joined := strings.Join(bodyLines, " ")
	firstBrace := strings.Index(joined, "{")
	lastBrace := strings.LastIndex(joined, "}")
	if firstBrace >= 0 && lastBrace > firstBrace {
		inner := strings.TrimSpace(joined[firstBrace+1 : lastBrace])
		lowerInner := strings.ToLower(inner)
		if strings.HasPrefix(lowerInner, "throw new error") && strings.Contains(lowerInner, "not implemented") {
			return true
		}
		return inner == "" || inner == "return;" || inner == "return"
	}
	body := strings.TrimSpace(joined)
	return body == "{}" || body == ""
}

