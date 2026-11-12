// Package impact implements 11-language 1-hop reverse-dependent scanning for ImpactMap.
package impact

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	csNamespaceDeclRe    = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z0-9_.]+)`)
	jvmPackageDeclRe     = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)`)
	intentGoTypeAnchorRe = regexp.MustCompile(`@intent[^\n]*?(#[A-Za-z0-9_]+)`)
)

type polyglotTargetSpec struct {
	rel    string
	ext    string
	stem   string
	meta   string
	wordRe *regexp.Regexp
}

func countPolyglotStemImporters(root, targetRel string) int {
	return len(listPolyglotStemImporters(root, targetRel))
}

func listPolyglotStemImporters(root, targetRel string) []string {
	targetExt := strings.ToLower(filepath.Ext(targetRel))
	stem := strings.TrimSuffix(filepath.Base(targetRel), filepath.Ext(targetRel))
	if len(stem) < 2 || ignoredPolyglotStems[stem] {
		return nil
	}
	spec := polyglotTargetSpec{
		rel:    targetRel,
		ext:    targetExt,
		stem:   stem,
		meta:   extractTargetModuleMeta(filepath.Join(root, targetRel), targetExt),
		wordRe: regexp.MustCompile(`\b` + regexp.QuoteMeta(stem) + `\b`),
	}

	var dependents []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if IsSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if fileDependsOnPolyglotTarget(root, path, spec) {
			if rel, rErr := filepath.Rel(root, path); rErr == nil {
				dependents = append(dependents, filepath.ToSlash(rel))
			}
		}
		return nil
	}); err != nil {
		sort.Strings(dependents)
		return dependents
	}
	sort.Strings(dependents)
	return dependents
}

func extractTargetModuleMeta(absPath, ext string) string {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	switch ext {
	case astdebt.ExtCS:
		if m := csNamespaceDeclRe.FindSubmatch(raw); len(m) > 1 {
			return string(m[1])
		}
	case astdebt.ExtJava, astdebt.ExtKT, astdebt.ExtKTS:
		if m := jvmPackageDeclRe.FindSubmatch(raw); len(m) > 1 {
			return string(m[1])
		}
	}
	return ""
}

func fileDependsOnPolyglotTarget(root, path string, spec polyglotTargetSpec) bool {
	callerExt := strings.ToLower(filepath.Ext(path))
	if !canCallerImportTargetExt(callerExt, spec.ext) {
		return false
	}
	if rel, rErr := filepath.Rel(root, path); rErr == nil && filepath.ToSlash(rel) == spec.rel {
		return false
	}
	data, rErr := os.ReadFile(path)
	if rErr != nil {
		return false
	}
	content := string(data)
	if !strings.Contains(content, spec.stem) {
		return false
	}
	return matchesLanguageImportSyntax(callerExt, content, spec)
}

func canCallerImportTargetExt(callerExt, targetExt string) bool {
	if callerExt == targetExt {
		return true
	}
	if astdebt.IsJSFamilyExt(callerExt) && astdebt.IsJSFamilyExt(targetExt) {
		return true
	}
	if IsCFamilyExt(callerExt) && IsCFamilyExt(targetExt) {
		return true
	}
	if IsJVMFamilyExt(callerExt) && IsJVMFamilyExt(targetExt) {
		return true
	}
	return (callerExt == astdebt.ExtRB || callerExt == astdebt.ExtRake) && (targetExt == astdebt.ExtRB || targetExt == astdebt.ExtRake)
}

// IsCFamilyExt executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsCFamilyExt(ext string) bool {
	switch ext {
	case astdebt.ExtC, astdebt.ExtH, astdebt.ExtCPP, astdebt.ExtCC, astdebt.ExtCXX, astdebt.ExtHPP:
		return true
	default:
		return false
	}
}

// IsJVMFamilyExt executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsJVMFamilyExt(ext string) bool {
	return ext == astdebt.ExtJava || ext == astdebt.ExtKT || ext == astdebt.ExtKTS
}

func matchesLanguageImportSyntax(callerExt, content string, spec polyglotTargetSpec) bool {
	if callerExt == astdebt.ExtCS || IsJVMFamilyExt(callerExt) {
		return matchesManagedNamespaceReference(callerExt, content, spec)
	}
	for _, line := range strings.Split(content, "\n") {
		if lineMatchesDirectImport(callerExt, strings.TrimSpace(line), spec) {
			return true
		}
	}
	return false
}

func lineMatchesDirectImport(callerExt, trimmed string, spec polyglotTargetSpec) bool {
	if astdebt.IsJSFamilyExt(callerExt) {
		return matchesJSImportLine(trimmed, spec.stem)
	}
	if IsCFamilyExt(callerExt) {
		return strings.HasPrefix(trimmed, "#include") && strings.Contains(trimmed, spec.stem+".")
	}
	return matchesScriptOrRustImportLine(callerExt, trimmed, spec)
}

func matchesJSImportLine(trimmed, stem string) bool {
	if !strings.Contains(trimmed, stem) {
		return false
	}
	return strings.Contains(trimmed, "import") || strings.Contains(trimmed, "from ") || strings.Contains(trimmed, "require(")
}

func matchesScriptOrRustImportLine(callerExt, trimmed string, spec polyglotTargetSpec) bool {
	switch callerExt {
	case astdebt.ExtPy:
		return hasAnyLinePrefix(trimmed, "import ", "from ") && spec.wordRe.MatchString(trimmed)
	case astdebt.ExtRB, astdebt.ExtRake:
		return strings.Contains(trimmed, spec.stem) && hasAnyLinePrefix(trimmed, "require ", "require_relative ", "require(")
	case astdebt.ExtRS:
		return spec.wordRe.MatchString(trimmed) && hasAnyLinePrefix(trimmed, "use ", "pub use ", "mod ", "pub mod ")
	default:
		return false
	}
}

func hasAnyLinePrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func matchesManagedNamespaceReference(callerExt, content string, spec polyglotTargetSpec) bool {
	if !spec.wordRe.MatchString(content) {
		return false
	}
	if callerExt == astdebt.ExtCS {
		return spec.meta == "" || strings.Contains(content, "using "+spec.meta) || strings.Contains(content, "namespace "+spec.meta)
	}
	if strings.Contains(content, "import ") && strings.Contains(content, "."+spec.stem) {
		return true
	}
	return spec.meta != "" && (strings.Contains(content, "package "+spec.meta) || strings.Contains(content, "import "+spec.meta+".*"))
}

// HasInlineRustTestModule executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func HasInlineRustTestModule(absPath string) bool {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "#[cfg(test)]") || strings.Contains(string(data), "#[test]")
}

// IsSkippableSecurityDir executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsSkippableSecurityDir(name string) bool {
	skip := map[string]bool{
		".git": true, "node_modules": true, ".tmp": true, "worktrees": true,
		"vendor": true, ".venv": true, "dist": true, "__pycache__": true,
		".svelte-kit": true, ".next": true, ".nuxt": true, ".output": true,
		".turbo": true, ".parcel-cache": true, ".vitepress": true,
	}
	return skip[name]
}

// IsSkippedObligationDir executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsSkippedObligationDir(name string) bool {
	switch name {
	case ".git", "node_modules", "worktrees", "vendor", ".svelte-kit", "dist", "build", "intent":
		return true
	default:
		return false
	}
}

// IsVerificationSuiteFile executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsVerificationSuiteFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		"_test.go", "_test.py", "_test.rs", "_spec.rb", "_test.rb",
		"test.java", "tests.java", "test.kt", "tests.kt", "spec.kt",
		"test.cs", "tests.cs", "_test.c", "_test.cc", "_test.cpp", "_test.cxx",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	for _, prefix := range []string{"test_", "spec_"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.")
}
