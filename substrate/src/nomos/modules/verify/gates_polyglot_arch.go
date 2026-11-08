// Package verify implements polyglot boundary-import and orphan-module reachability checks.
package verify

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	pyFromImportSpecRe  = regexp.MustCompile(`^\s*from\s+([A-Za-z0-9_.]+)\s+import\b`)
	pyImportSpecRe      = regexp.MustCompile(`^\s*import\s+([A-Za-z0-9_.,\s]+)`)
	jsImportSpecRe      = regexp.MustCompile(`(?:from\s+|import\s*\(\s*|require\s*\(\s*|^\s*import\s+)['"]([^'"]+)['"]`)
	rbRequireSpecRe     = regexp.MustCompile(`\brequire(?:_relative)?\s*\(?\s*['"]([^'"]+)['"]`)
	rustUseSpecRe       = regexp.MustCompile(`^\s*(?:pub\s+)?(?:use|extern\s+crate)\s+([A-Za-z0-9_:]+)`)
	jvmImportSpecRe     = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([A-Za-z0-9_.*]+)`)
	csUsingSpecRe       = regexp.MustCompile(`^\s*using\s+(?:static\s+)?([A-Za-z0-9_.]+)\s*;`)
	cIncludeSpecRe      = regexp.MustCompile(`^\s*#\s*include\s*[<"]([^>"]+)[>"]`)
	orphanEntrypointSet = map[string]bool{
		"main": true, "index": true, "mod": true, "lib": true, "build": true,
		"__init__": true, "__main__": true, "conftest": true, "manage": true, "setup": true,
		"app": true, "server": true, "cli": true, "root": true, "config": true, "types": true,
	}
	entrypointRootStems = map[string]bool{
		"main": true, "index": true, "app": true, "server": true, "cli": true, "lib": true, "__main__": true,
	}
	orphanFrameworkDirs = []string{
		"/cmd/", "/bin/", "/routes/", "/pages/", "/controllers/", "/views/",
		"/migrations/", "/plugins/", "/scripts/", "/public/", "/static/",
	}
)

func scanPolyglotBoundaryLeaks(root string, prohibited []string) []string {
	if len(prohibited) == 0 {
		return nil
	}
	var violations []string
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
		if strings.HasSuffix(path, extGo) || !isPolyglotTargetFile(path) {
			return nil
		}
		raw, rErr := os.ReadFile(path)
		if rErr != nil {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, spec := range extractPolyglotImportSpecs(ext, string(raw)) {
			if matchesProhibitedBoundarySpec(spec, prohibited) {
				violations = append(violations, fmt.Sprintf("%s: imports %s", path, spec))
			}
		}
		return nil
	}); err != nil {
		return violations
	}
	return violations
}

func extractPolyglotImportSpecs(ext, content string) []string {
	var specs []string
	for _, line := range strings.Split(content, "\n") {
		specs = append(specs, extractLineImportSpecs(ext, line)...)
	}
	return specs
}

func extractLineImportSpecs(ext, line string) []string {
	if ext == extPy {
		return extractPyLineImports(line)
	}
	if isJSFamilyExt(ext) {
		return extractJSLineImports(line)
	}
	re := selectSingleGroupImportRegex(ext)
	if re == nil {
		return nil
	}
	if m := re.FindStringSubmatch(line); len(m) > 1 {
		return []string{strings.TrimSuffix(m[1], ".*")}
	}
	return nil
}

func extractPyLineImports(line string) []string {
	if m := pyFromImportSpecRe.FindStringSubmatch(line); len(m) > 1 {
		return []string{strings.TrimSpace(m[1])}
	}
	m := pyImportSpecRe.FindStringSubmatch(line)
	if len(m) <= 1 {
		return nil
	}
	var out []string
	for _, part := range strings.Split(m[1], ",") {
		if fields := strings.Fields(strings.TrimSpace(part)); len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}

func extractJSLineImports(line string) []string {
	var out []string
	for _, m := range jsImportSpecRe.FindAllStringSubmatch(line, -1) {
		if len(m) > 1 {
			out = append(out, m[1])
		}
	}
	return out
}

func selectSingleGroupImportRegex(ext string) *regexp.Regexp {
	switch ext {
	case extRB, extRake:
		return rbRequireSpecRe
	case extRS:
		return rustUseSpecRe
	case extJava, extKT, extKTS:
		return jvmImportSpecRe
	case extCS:
		return csUsingSpecRe
	case extC, extH, extCPP, extCC, extCXX, extHPP:
		return cIncludeSpecRe
	default:
		return nil
	}
}

func matchesProhibitedBoundarySpec(spec string, prohibited []string) bool {
	clean := strings.TrimPrefix(strings.TrimSpace(spec), "./")
	for _, p := range prohibited {
		if spec == p || clean == p ||
			strings.HasPrefix(spec, p+"/") || strings.HasPrefix(clean, p+"/") ||
			strings.HasPrefix(spec, p+".") || strings.HasPrefix(spec, p+"::") {
			return true
		}
	}
	return false
}

func findUnimportedPolyglotModules(ctx *VerifyContext, root string) []string {
	scanRoot := resolvePolyglotDeadCodeRoot(ctx, root)
	if scanRoot == "" || !hasPolyglotEntrypointRoot(scanRoot) {
		return nil
	}
	candidates := collectOrphanPolyglotCandidates(ctx, root, scanRoot)
	if len(candidates) == 0 {
		return nil
	}
	referencedRels := markReferencedPolyglotCandidates(root, candidates)
	var orphans []string
	for rel := range candidates {
		if !referencedRels[rel] {
			orphans = append(orphans, rel)
		}
	}
	sort.Strings(orphans)
	return orphans
}

func resolvePolyglotDeadCodeRoot(ctx *VerifyContext, root string) string {
	subSrc := filepath.Join(root, "substrate", "src")
	if fi, err := os.Stat(subSrc); err == nil && fi.IsDir() {
		return subSrc
	}
	if !isBrownfieldRatchetActive(ctx) {
		plainSrc := filepath.Join(root, "src")
		if fi, err := os.Stat(plainSrc); err == nil && fi.IsDir() {
			return plainSrc
		}
	}
	return ""
}

func hasPolyglotEntrypointRoot(scanRoot string) bool {
	found := false
	if err := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != scanRoot {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == extGo || !isPolyglotTargetFile(path) {
			return nil
		}
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if entrypointRootStems[stem] {
			found = true
			return fs.SkipAll
		}
		return nil
	}); err != nil {
		return found
	}
	return found
}

func collectOrphanPolyglotCandidates(ctx *VerifyContext, root, scanRoot string) map[string]string {
	candidates := make(map[string]string)
	eligibleExts := map[string]bool{extPy: true, extTS: true, extJS: true, extMJS: true, extRB: true, extRS: true}
	if err := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != scanRoot {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !eligibleExts[ext] || !isPolyglotTargetFile(path) || isPolyglotEntrypointOrFrameworkFile(path, ext) {
			return nil
		}
		if isBrownfieldRatchetActive(ctx) {
			if _, existed := loadBaselineFileBytes(ctx, root, path); existed {
				return nil
			}
		}
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if len(stem) < 3 || orphanEntrypointSet[stem] {
			return nil
		}
		if rel, rErr := filepath.Rel(root, path); rErr == nil {
			candidates[filepath.ToSlash(rel)] = stem
		}
		return nil
	}); err != nil {
		return candidates
	}
	return candidates
}

func isPolyglotEntrypointOrFrameworkFile(path, ext string) bool {
	slash := "/" + filepath.ToSlash(path)
	base := filepath.Base(path)
	if strings.HasPrefix(base, "+") || strings.Contains(base, ".config.") {
		return true
	}
	for _, dirToken := range orphanFrameworkDirs {
		if strings.Contains(slash, dirToken) {
			return true
		}
	}
	if ext == extPy || ext == extRB {
		if raw, err := os.ReadFile(path); err == nil && strings.Contains(string(raw), "__main__") {
			return true
		}
	}
	return false
}

func markReferencedPolyglotCandidates(root string, candidates map[string]string) map[string]bool {
	referenced := make(map[string]bool)
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
		if !isSupportedPolyglotSourceFile(path) {
			return nil
		}
		relSlash := ""
		if rel, rErr := filepath.Rel(root, path); rErr == nil {
			relSlash = filepath.ToSlash(rel)
		}
		raw, rErr := os.ReadFile(path)
		if rErr != nil {
			return nil
		}
		text := string(raw)
		for ownerRel, stem := range candidates {
			if relSlash != ownerRel && strings.Contains(text, stem) {
				referenced[ownerRel] = true
			}
		}
		return nil
	}); err != nil {
		return referenced
	}
	return referenced
}
