// Package verify implements 11-language 1-hop reverse-dependent scanning for ImpactMap.
package verify

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	csNamespaceDeclRe = regexp.MustCompile(`(?m)^\s*namespace\s+([A-Za-z0-9_.]+)`)
	jvmPackageDeclRe  = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)`)
)

type polyglotTargetSpec struct {
	rel    string
	ext    string
	stem   string
	meta   string
	wordRe *regexp.Regexp
}

func countPolyglotStemImporters(root, targetRel string) int {
	targetExt := strings.ToLower(filepath.Ext(targetRel))
	stem := strings.TrimSuffix(filepath.Base(targetRel), filepath.Ext(targetRel))
	if len(stem) < 2 || ignoredPolyglotStems[stem] {
		return 0
	}
	spec := polyglotTargetSpec{
		rel:    targetRel,
		ext:    targetExt,
		stem:   stem,
		meta:   extractTargetModuleMeta(filepath.Join(root, targetRel), targetExt),
		wordRe: regexp.MustCompile(`\b` + regexp.QuoteMeta(stem) + `\b`),
	}

	count := 0
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if fileDependsOnPolyglotTarget(root, path, spec) {
			count++
		}
		return nil
	}); err != nil {
		return count
	}
	return count
}

func extractTargetModuleMeta(absPath, ext string) string {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	switch ext {
	case extCS:
		if m := csNamespaceDeclRe.FindSubmatch(raw); len(m) > 1 {
			return string(m[1])
		}
	case extJava, extKT, extKTS:
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
	if isJSFamilyExt(callerExt) && isJSFamilyExt(targetExt) {
		return true
	}
	if isCFamilyExt(callerExt) && isCFamilyExt(targetExt) {
		return true
	}
	if isJVMFamilyExt(callerExt) && isJVMFamilyExt(targetExt) {
		return true
	}
	return (callerExt == extRB || callerExt == extRake) && (targetExt == extRB || targetExt == extRake)
}

func isCFamilyExt(ext string) bool {
	switch ext {
	case extC, extH, extCPP, extCC, extCXX, extHPP:
		return true
	default:
		return false
	}
}

func isJVMFamilyExt(ext string) bool {
	return ext == extJava || ext == extKT || ext == extKTS
}

func matchesLanguageImportSyntax(callerExt, content string, spec polyglotTargetSpec) bool {
	if callerExt == extCS || isJVMFamilyExt(callerExt) {
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
	if isJSFamilyExt(callerExt) {
		return matchesJSImportLine(trimmed, spec.stem)
	}
	if isCFamilyExt(callerExt) {
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
	case extPy:
		return hasAnyLinePrefix(trimmed, "import ", "from ") && spec.wordRe.MatchString(trimmed)
	case extRB, extRake:
		return strings.Contains(trimmed, spec.stem) && hasAnyLinePrefix(trimmed, "require ", "require_relative ", "require(")
	case extRS:
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
	if callerExt == extCS {
		return spec.meta == "" || strings.Contains(content, "using "+spec.meta) || strings.Contains(content, "namespace "+spec.meta)
	}
	if strings.Contains(content, "import ") && strings.Contains(content, "."+spec.stem) {
		return true
	}
	return spec.meta != "" && (strings.Contains(content, "package "+spec.meta) || strings.Contains(content, "import "+spec.meta+".*"))
}

