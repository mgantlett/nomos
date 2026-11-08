package treesitter

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// SCMQuerySpec represents a compiled Tree-sitter S-expression (.scm) query specification
// bound from the embedded QueryFS (Law 2 Non-Verbal Proof).
type SCMQuerySpec struct {
	Language    string
	RawSource   string
	Captures    map[string]bool
	BranchNodes []string
}

var (
	scmCache   = make(map[string]*SCMQuerySpec)
	scmCacheMu sync.RWMutex
	scmCapRe   = regexp.MustCompile(`@([a-zA-Z0-9_.]+)`)
	scmNodeRe  = regexp.MustCompile(`\(([a-zA-Z_][a-zA-Z0-9_]*)`)
)

// LoadSCMQuery reads and parses the embedded .scm S-expression query for a target language.
func LoadSCMQuery(lang string) (*SCMQuerySpec, error) {
	scmCacheMu.RLock()
	if spec, ok := scmCache[lang]; ok {
		scmCacheMu.RUnlock()
		return spec, nil
	}
	scmCacheMu.RUnlock()

	queryPath := fmt.Sprintf("queries/%s.scm", lang)
	raw, err := QueryFS.ReadFile(queryPath)
	if err != nil {
		return nil, fmt.Errorf("missing embedded .scm query for language %s: %w", lang, err)
	}
	src := string(raw)
	if err := validateSCMSyntax(queryPath, src); err != nil {
		return nil, err
	}

	caps := make(map[string]bool)
	for _, m := range scmCapRe.FindAllStringSubmatch(src, -1) {
		if len(m) > 1 {
			caps[m[1]] = true
		}
	}

	var nodes []string
	seen := make(map[string]bool)
	for _, m := range scmNodeRe.FindAllStringSubmatch(src, -1) {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			nodes = append(nodes, m[1])
		}
	}

	spec := &SCMQuerySpec{
		Language:    lang,
		RawSource:   src,
		Captures:    caps,
		BranchNodes: nodes,
	}

	scmCacheMu.Lock()
	scmCache[lang] = spec
	scmCacheMu.Unlock()
	return spec, nil
}

// ValidateFunctionSCMContract verifies that a function-bearing language's .scm query binds @func.name, @func.doc, and @branch.
// @intent full_dod_gate_suite_and_polyglot_spec.md #SCMQuerySpec
func ValidateFunctionSCMContract(lang string) (*SCMQuerySpec, error) {
	spec, err := LoadSCMQuery(lang)
	if err != nil {
		return nil, err
	}
	for _, capName := range []string{"func.name", "func.doc", "branch"} {
		if !spec.Captures[capName] {
			return nil, fmt.Errorf("incomplete .scm capture contract for %s: missing @%s", lang, capName)
		}
	}
	if len(spec.BranchNodes) == 0 {
		return nil, fmt.Errorf("incomplete .scm branch contract for %s: zero AST branch nodes", lang)
	}
	return spec, nil
}

func validateSCMSyntax(queryPath, src string) error {
	cleaned := stripSCMCommentsAndStrings(src)
	if cleaned == "" && len(strings.TrimSpace(src)) > 0 {
		return fmt.Errorf("unclosed string literal in %s", queryPath)
	}
	parens, brackets := 0, 0
	for i := 0; i < len(cleaned); i++ {
		parens, brackets = stepSCMDelim(cleaned[i], parens, brackets)
		if parens < 0 || brackets < 0 {
			return fmt.Errorf("unbalanced closing delimiter in %s", queryPath)
		}
	}
	if parens != 0 || brackets != 0 {
		return fmt.Errorf("unclosed S-expression in %s", queryPath)
	}
	return nil
}

func stripSCMCommentsAndStrings(src string) string {
	var out strings.Builder
	inStr := false
	for i := 0; i < len(src); i++ {
		if src[i] == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if src[i] == ';' {
			nl := strings.IndexByte(src[i:], '\n')
			if nl < 0 {
				break
			}
			i += nl
		}
		out.WriteByte(src[i])
	}
	if inStr {
		return ""
	}
	return out.String()
}

func stepSCMDelim(ch byte, parens, brackets int) (int, int) {
	switch ch {
	case '(':
		parens++
	case ')':
		parens--
	case '[':
		brackets++
	case ']':
		brackets--
	}
	return parens, brackets
}

var (
	tsSkipRe     = regexp.MustCompile(`\b(?:it|test|describe)\.skip\s*\(`)
	pySkipRe     = regexp.MustCompile(`@(?:pytest\.mark\.skip(?:if)?|unittest\.skip(?:If|Unless)?)\b`)
	rbSkipRe     = regexp.MustCompile(`(?:^\s*(?:xit|xdescribe|xcontext)\b|^\s*skip\b)`)
	rsSkipRe     = regexp.MustCompile(`#\s*\[\s*ignore\b`)
	jvmSkipRe    = regexp.MustCompile(`@(?:Disabled|Ignore)\b`)
	csharpSkipRe = regexp.MustCompile(`\[\s*(?:Fact|Theory)\s*\([^]]*\bSkip\s*=|\[\s*Ignore\b`)
)

// DetectSkippedTests scans stripped source code for skipped or disabled test markers (@test.skip)
// to prevent test-skip reward hacking across polyglot test suites (Law 2).
func DetectSkippedTests(lang string, content []byte) []string {
	var cleaned string
	switch lang {
	case "python":
		cleaned = stripPythonLiterals(string(content))
	case "ruby":
		cleaned = stripRubyLiterals(string(content))
	default:
		cleaned = stripCLikeLiterals(string(content), lang != "rust")
	}

	re := skipRegexForLang(lang)
	if re == nil {
		return nil
	}

	var findings []string
	lines := strings.Split(cleaned, "\n")
	for idx, line := range lines {
		if m := re.FindString(line); m != "" {
			findings = append(findings, fmt.Sprintf("line %d: %s", idx+1, strings.TrimSpace(m)))
		}
	}
	return findings
}

func skipRegexForLang(lang string) *regexp.Regexp {
	switch lang {
	case "typescript", "svelte":
		return tsSkipRe
	case "python":
		return pySkipRe
	case "ruby":
		return rbSkipRe
	case "rust":
		return rsSkipRe
	case "java", "kotlin":
		return jvmSkipRe
	case "csharp":
		return csharpSkipRe
	default:
		return nil
	}
}
