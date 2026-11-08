// Package verify implements 11-language paired-test resolution for ImpactMap and TDDGate.
package verify

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const pairedTestPrefix = "paired test: "

func resolvePairedTestStatus(root, rel string) string {
	ext := strings.ToLower(filepath.Ext(rel))
	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	dir := filepath.Dir(rel)

	if ext == extRS && hasInlineRustTestModule(filepath.Join(root, rel)) {
		return pairedTestPrefix + "inline #[cfg(test)]"
	}

	for _, c := range buildPairedTestCandidates(rel, dir, stem, ext) {
		if _, err := os.Stat(filepath.Join(root, c)); err == nil {
			return pairedTestPrefix + filepath.Base(c)
		}
	}
	if matched := searchNestedLanguageTest(root, stem, ext); matched != "" {
		return pairedTestPrefix + matched
	}
	return "no paired unit test"
}

func buildPairedTestCandidates(rel, dir, stem, ext string) []string {
	if isJSFamilyExt(ext) {
		return jsFamilyPairedTestCandidates(dir, stem, ext)
	}
	switch ext {
	case extGo:
		return []string{filepath.Join(dir, stem+suffixGoTest)}
	case extPy:
		return []string{
			filepath.Join(dir, prefixPyTest+stem+extPy),
			filepath.Join(dir, stem+suffixPyTest),
			filepath.Join(dirTests, prefixPyTest+stem+extPy),
			filepath.Join(dirTests, stem+suffixPyTest),
			filepath.Join(dirTest, prefixPyTest+stem+extPy),
		}
	case extRB, extRake:
		return rubyPairedTestCandidates(dir, stem)
	case extRS:
		return []string{
			filepath.Join(dirTests, stem+extRS),
			filepath.Join(dirTests, stem+"_test.rs"),
			filepath.Join(dir, stem+"_test.rs"),
		}
	default:
		return nativePairedTestCandidates(rel, dir, stem, ext)
	}
}

func nativePairedTestCandidates(rel, dir, stem, ext string) []string {
	switch ext {
	case extJava:
		return jvmPairedTestCandidates(rel, dir, stem, "java", extJava, []string{"Test", "Tests"})
	case extKT, extKTS:
		return jvmPairedTestCandidates(rel, dir, stem, "kotlin", extKT, []string{"Test", "Tests", "Spec"})
	case extCS:
		return []string{filepath.Join(dir, stem+"Tests.cs"), filepath.Join(dir, stem+"Test.cs")}
	case extC, extH, extCPP, extCC, extCXX, extHPP:
		return cFamilyPairedTestCandidates(dir, stem)
	default:
		return nil
	}
}

func jsFamilyPairedTestCandidates(dir, stem, ext string) []string {
	testExts := []string{ext, extTS, extTSX, extJS, extJSX, extMJS}
	var out []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, te := range testExts {
		if te == extSvelte || te == "" {
			continue
		}
		add(filepath.Join(dir, stem+".test"+te))
		add(filepath.Join(dir, stem+".spec"+te))
		add(filepath.Join(dir, "__tests__", stem+".test"+te))
		add(filepath.Join(dir, "__tests__", stem+".spec"+te))
		add(filepath.Join(dirTests, stem+".test"+te))
		add(filepath.Join(dirTest, stem+".test"+te))
	}
	return out
}

func jvmPairedTestCandidates(rel, dir, stem, langDir, ext string, suffixes []string) []string {
	var out []string
	for _, sfx := range suffixes {
		out = append(out, filepath.Join(dir, stem+sfx+ext))
	}
	slashRel := filepath.ToSlash(rel)
	mainMarker := "src/main/" + langDir + "/"
	testMarker := "src/" + dirTest + "/" + langDir + "/"
	if idx := strings.Index(slashRel, mainMarker); idx >= 0 {
		prefix := slashRel[:idx]
		pkgSub := filepath.Dir(slashRel[idx+len(mainMarker):])
		for _, sfx := range suffixes {
			out = append(out, filepath.FromSlash(prefix+testMarker+filepath.ToSlash(filepath.Join(pkgSub, stem+sfx+ext))))
		}
	}
	return out
}

func cFamilyPairedTestCandidates(dir, stem string) []string {
	exts := []string{extC, extCPP, extCC, extCXX}
	dirs := []string{dir, dirTests, dirTest}
	var out []string
	for _, d := range dirs {
		for _, e := range exts {
			out = append(out, filepath.Join(d, stem+"_test"+e), filepath.Join(d, prefixPyTest+stem+e))
		}
	}
	return out
}

func searchNestedLanguageTest(root, stem, ext string) string {
	targetNames := nestedLanguageTestNames(stem, ext)
	if len(targetNames) == 0 {
		return ""
	}
	found := ""
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
		if targetNames[d.Name()] {
			found = d.Name()
			return fs.SkipAll
		}
		return nil
	}); err != nil {
		return found
	}
	return found
}

func nestedLanguageTestNames(stem, ext string) map[string]bool {
	switch ext {
	case extCS:
		return map[string]bool{stem + "Tests.cs": true, stem + "Test.cs": true}
	case extJava:
		return map[string]bool{stem + "Test.java": true, stem + "Tests.java": true}
	case extKT, extKTS:
		return map[string]bool{stem + "Test.kt": true, stem + "Tests.kt": true, stem + "Spec.kt": true}
	default:
		return nil
	}
}
