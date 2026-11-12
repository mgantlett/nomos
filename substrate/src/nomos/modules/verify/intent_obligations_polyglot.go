// Package verify implements polyglot specification contract field parity,
// polyglot @intent type anchor discovery, and polyglot environment variable scanning.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	polyglotFencedBlockRe = regexp.MustCompile("(?s)```(ts|typescript|js|javascript|py|python|rs|rust|java|kt|kotlin|cs|csharp)\\s*\\n(.*?)```")
	braceTypeBlockRe      = regexp.MustCompile(`(?ms)\b(?:interface|class|struct|record)\s+([A-Z][A-Za-z0-9_]*)[^{]*\{([^}]*)\}`)
	tsTypeLiteralBlockRe  = regexp.MustCompile(`(?ms)\btype\s+([A-Z][A-Za-z0-9_]*)\s*=\s*\{([^}]*)\}`)
	braceFieldLineRe      = regexp.MustCompile(`^\s*(?:readonly\s+|pub\s+|public\s+|private\s+|protected\s+|final\s+|val\s+|var\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*\??\s*(?:[:=;]|/[/*]|$)`)
	jvmTypedFieldLineRe   = regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|final\s+|readonly\s+)*[A-Za-z0-9_<>\[\]?.,]+\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\{\s*get|[;=])`)
	recordHeaderRe        = regexp.MustCompile(`\b(?:record|data\s+class)\s+([A-Z][A-Za-z0-9_]*)\s*\(([^)]*)\)`)
	pyClassHeaderRe       = regexp.MustCompile(`^class\s+([A-Z][A-Za-z0-9_]*)\b[^:]*:`)
	pyTypedFieldRe        = regexp.MustCompile(`^\s{2,}([A-Za-z][A-Za-z0-9_]*)\s*:`)
	pySelfAssignFieldRe   = regexp.MustCompile(`^\s{4,}self\.([A-Za-z][A-Za-z0-9_]*)\s*=`)
	polyglotTypeDeclRe    = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:public\s+|pub\s+|abstract\s+|final\s+|sealed\s+|data\s+)*(?:class|interface|type|struct|record|enum)\s+([A-Za-z_][A-Za-z0-9_]*)\b`)

	jsProcessEnvRe  = regexp.MustCompile(`\b(?:process\.env|import\.meta\.env)\.([A-Z][A-Z0-9_]+)\b`)
	pyEnvLookupRe   = regexp.MustCompile(`\bos\.(?:getenv|environ\.get)\s*\(\s*['"]([A-Z][A-Z0-9_]+)['"]`)
	pyEnvironIdxRe  = regexp.MustCompile(`\bos\.environ\s*\[\s*['"]([A-Z][A-Z0-9_]+)['"]\s*\]`)
	rustEnvVarRe    = regexp.MustCompile(`\benv::var(?:_os)?\s*\(\s*"([A-Z][A-Z0-9_]+)"\s*\)`)
	jvmCSharpEnvRe  = regexp.MustCompile(`\b(?:System\.getenv|Environment\.GetEnvironmentVariable)\s*\(\s*"([A-Z][A-Z0-9_]+)"\s*\)`)
	rubyEnvLookupRe = regexp.MustCompile(`\bENV(?:\[\s*|\.fetch\s*\(\s*)['"]([A-Z][A-Z0-9_]+)['"]`)
)

func verifyPolyglotSpecSignatures(root string, specFiles []string) []string {
	specContracts := make(map[string]map[string]map[string]bool)
	hasAny := false
	for _, specFile := range specFiles {
		parsed := extractPolyglotSpecContractFields(specFile)
		if len(parsed) > 0 {
			specContracts[specFile] = parsed
			hasAny = true
		}
	}
	if !hasAny {
		return nil
	}
	sourceFields := collectPolyglotSourceTypeFields(root)
	var issues []string
	for _, specFile := range specFiles {
		contracts := specContracts[specFile]
		if len(contracts) == 0 {
			continue
		}
		relSpec := toRelSlash(root, specFile)
		issues = append(issues, diffSpecStructsAgainstAST(relSpec, contracts, sourceFields)...)
	}
	return issues
}

func diffSpecStructsAgainstAST(relSpec string, specStructs, compiledStructs map[string]map[string]bool) []string {
	names := make([]string, 0, len(specStructs))
	for name := range specStructs {
		names = append(names, name)
	}
	sort.Strings(names)
	var issues []string
	for _, typeName := range names {
		actualFields, exists := compiledStructs[typeName]
		if !exists {
			issues = append(issues, fmt.Sprintf("%s: missing compiled Go struct %s", relSpec, typeName))
			continue
		}
		issues = append(issues, findMissingStructFields(relSpec, typeName, specStructs[typeName], actualFields)...)
	}
	return issues
}

func findMissingStructFields(relSpec, typeName string, expected, actual map[string]bool) []string {
	fields := make([]string, 0, len(expected))
	for field := range expected {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	var missing []string
	for _, field := range fields {
		if !actual[field] {
			missing = append(missing, fmt.Sprintf("%s: struct %s missing declared field %s", relSpec, typeName, field))
		}
	}
	return missing
}

func extractPolyglotSpecContractFields(specFile string) map[string]map[string]bool {
	raw, err := os.ReadFile(specFile)
	if err != nil {
		return nil
	}
	out := make(map[string]map[string]bool)
	for _, m := range polyglotFencedBlockRe.FindAllStringSubmatch(string(raw), -1) {
		if len(m) < 3 {
			continue
		}
		lang := strings.ToLower(strings.TrimSpace(m[1]))
		parsePolyglotContractSnippet(lang, m[2], out)
	}
	return out
}

func parsePolyglotContractSnippet(lang, src string, out map[string]map[string]bool) {
	if lang == "py" || lang == "python" {
		extractPythonClassFields(src, out)
		return
	}
	extractRecordHeaderFields(src, out)
	for _, re := range []*regexp.Regexp{braceTypeBlockRe, tsTypeLiteralBlockRe} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if len(m) == 3 {
				recordBraceBlockFields(m[1], m[2], out)
			}
		}
	}
}

func extractRecordHeaderFields(src string, out map[string]map[string]bool) {
	for _, m := range recordHeaderRe.FindAllStringSubmatch(src, -1) {
		if len(m) != 3 {
			continue
		}
		fields := ensureStructFieldSet(out, m[1])
		for _, part := range strings.Split(m[2], ",") {
			clean := strings.TrimSpace(part)
			if clean == "" {
				continue
			}
			recordSingleRecordParam(clean, fields)
		}
	}
}

func recordSingleRecordParam(param string, fields map[string]bool) {
	if idx := strings.Index(param, ":"); idx > 0 {
		toks := strings.Fields(strings.TrimSpace(param[:idx]))
		if len(toks) > 0 {
			fields[toks[len(toks)-1]] = true
		}
		return
	}
	toks := strings.Fields(strings.Split(param, "=")[0])
	if len(toks) >= 2 {
		fields[toks[len(toks)-1]] = true
	}
}

func recordBraceBlockFields(typeName, body string, out map[string]map[string]bool) {
	fields := ensureStructFieldSet(out, typeName)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") || strings.Contains(trimmed, "(") {
			continue
		}
		if m := braceFieldLineRe.FindStringSubmatch(line); len(m) == 2 && !isBraceKeyword(m[1]) {
			fields[m[1]] = true
			continue
		}
		if m := jvmTypedFieldLineRe.FindStringSubmatch(line); len(m) == 2 && !isBraceKeyword(m[1]) {
			fields[m[1]] = true
		}
	}
}

func isBraceKeyword(word string) bool {
	switch word {
	case "return", "const", "let", "var", "fn", "func", "public", "private", "protected", "static":
		return true
	default:
		return false
	}
}

func extractPythonClassFields(src string, out map[string]map[string]bool) {
	var currentClass string
	for _, line := range strings.Split(src, "\n") {
		if m := pyClassHeaderRe.FindStringSubmatch(strings.TrimSpace(line)); len(m) == 2 {
			currentClass = m[1]
			ensureStructFieldSet(out, currentClass)
			continue
		}
		if currentClass == "" {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.TrimSpace(line) != "" {
			currentClass = ""
			continue
		}
		recordPythonFieldLine(line, out[currentClass])
	}
}

func recordPythonFieldLine(line string, fields map[string]bool) {
	if m := pyTypedFieldRe.FindStringSubmatch(line); len(m) == 2 && !strings.HasPrefix(m[1], "_") {
		fields[m[1]] = true
		return
	}
	if m := pySelfAssignFieldRe.FindStringSubmatch(line); len(m) == 2 && !strings.HasPrefix(m[1], "_") {
		fields[m[1]] = true
	}
}

func ensureStructFieldSet(out map[string]map[string]bool, name string) map[string]bool {
	if out[name] == nil {
		out[name] = make(map[string]bool)
	}
	return out[name]
}

func collectPolyglotSourceTypeFields(root string) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if impact.IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") || !vscope.IsSupportedPolyglotSourceFile(p) || impact.IsVerificationSuiteFile(info.Name()) {
			return nil
		}
		scanPolyglotFileTypeFields(p, out)
		return nil
	})
	if walkErr != nil {
		return out
	}
	return out
}

func scanPolyglotFileTypeFields(path string, out map[string]map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	parsePolyglotContractSnippet(ext, string(raw), out)
}

func collectPolyglotDefinedTypeAnchors(root string, defined map[string]bool) {
	walkErr := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if impact.IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") || !vscope.IsSupportedPolyglotSourceFile(p) || impact.IsVerificationSuiteFile(info.Name()) {
			return nil
		}
		raw, rErr := os.ReadFile(p)
		if rErr == nil {
			for _, m := range polyglotTypeDeclRe.FindAllStringSubmatch(string(raw), -1) {
				if len(m) == 2 {
					defined["#"+m[1]] = true
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return
	}
}

func scanPolyglotFileIntentAnchors(root, path string, definedTypes, definedSpecs map[string]bool) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(raw)
	if !strings.Contains(text, "@intent") {
		return nil
	}
	rel := toRelSlash(root, path)
	var unresolved []string
	for lineNo, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "@intent") {
			label := fmt.Sprintf("L%d", lineNo+1)
			unresolved = append(unresolved, checkDocIntentTokens(rel, label, line, definedTypes, definedSpecs)...)
		}
	}
	return unresolved
}

func extractPolyglotEnvVarsFromFile(path string, envVars map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(raw)
	for _, re := range []*regexp.Regexp{
		jsProcessEnvRe, pyEnvLookupRe, pyEnvironIdxRe, rustEnvVarRe, jvmCSharpEnvRe, rubyEnvLookupRe,
	} {
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			if len(m) == 2 {
				envVars[m[1]] = true
			}
		}
	}
}
