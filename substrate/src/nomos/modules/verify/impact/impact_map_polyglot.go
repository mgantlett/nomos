// Package impact implements 11-language paired-test resolution for ImpactMap and TDDGate.
package impact

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const PairedTestPrefix = "paired test: "

// ResolvePairedTestStatus executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func ResolvePairedTestStatus(root, rel string) string {
	ext := strings.ToLower(filepath.Ext(rel))
	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	dir := filepath.Dir(rel)

	if ext == astdebt.ExtRS && HasInlineRustTestModule(filepath.Join(root, rel)) {
		return PairedTestPrefix + "inline #[cfg(test)]"
	}

	for _, c := range buildPairedTestCandidates(rel, dir, stem, ext) {
		if _, err := os.Stat(filepath.Join(root, c)); err == nil {
			return PairedTestPrefix + filepath.Base(c)
		}
	}
	if matched := searchNestedLanguageTest(root, stem, ext); matched != "" {
		return PairedTestPrefix + matched
	}
	return "no paired unit test"
}

func buildPairedTestCandidates(rel, dir, stem, ext string) []string {
	if astdebt.IsJSFamilyExt(ext) {
		return jsFamilyPairedTestCandidates(dir, stem, ext)
	}
	switch ext {
	case astdebt.ExtGo:
		return []string{filepath.Join(dir, stem+astdebt.SuffixGoTest)}
	case astdebt.ExtPy:
		return []string{
			filepath.Join(dir, astdebt.PrefixPyTest+stem+astdebt.ExtPy),
			filepath.Join(dir, stem+astdebt.SuffixPyTest),
			filepath.Join(astdebt.DirTests, astdebt.PrefixPyTest+stem+astdebt.ExtPy),
			filepath.Join(astdebt.DirTests, stem+astdebt.SuffixPyTest),
			filepath.Join(astdebt.DirTest, astdebt.PrefixPyTest+stem+astdebt.ExtPy),
		}
	case astdebt.ExtRB, astdebt.ExtRake:
		return rubyPairedTestCandidates(dir, stem)
	case astdebt.ExtRS:
		return []string{
			filepath.Join(astdebt.DirTests, stem+astdebt.ExtRS),
			filepath.Join(astdebt.DirTests, stem+"_test.rs"),
			filepath.Join(dir, stem+"_test.rs"),
		}
	default:
		return nativePairedTestCandidates(rel, dir, stem, ext)
	}
}

func nativePairedTestCandidates(rel, dir, stem, ext string) []string {
	switch ext {
	case astdebt.ExtJava:
		return jvmPairedTestCandidates(rel, dir, stem, "java", astdebt.ExtJava, []string{"Test", "Tests"})
	case astdebt.ExtKT, astdebt.ExtKTS:
		return jvmPairedTestCandidates(rel, dir, stem, "kotlin", astdebt.ExtKT, []string{"Test", "Tests", "Spec"})
	case astdebt.ExtCS:
		return []string{filepath.Join(dir, stem+"Tests.cs"), filepath.Join(dir, stem+"Test.cs")}
	case astdebt.ExtC, astdebt.ExtH, astdebt.ExtCPP, astdebt.ExtCC, astdebt.ExtCXX, astdebt.ExtHPP:
		return cFamilyPairedTestCandidates(dir, stem)
	default:
		return nil
	}
}

func jsFamilyPairedTestCandidates(dir, stem, ext string) []string {
	testExts := []string{ext, astdebt.ExtTS, astdebt.ExtTSX, astdebt.ExtJS, astdebt.ExtJSX, astdebt.ExtMJS}
	var out []string
	seen := make(map[string]bool)
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, te := range testExts {
		if te == astdebt.ExtSvelte || te == "" {
			continue
		}
		add(filepath.Join(dir, stem+".test"+te))
		add(filepath.Join(dir, stem+".spec"+te))
		add(filepath.Join(dir, "__tests__", stem+".test"+te))
		add(filepath.Join(dir, "__tests__", stem+".spec"+te))
		add(filepath.Join(astdebt.DirTests, stem+".test"+te))
		add(filepath.Join(astdebt.DirTest, stem+".test"+te))
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
	testMarker := "src/" + astdebt.DirTest + "/" + langDir + "/"
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
	exts := []string{astdebt.ExtC, astdebt.ExtCPP, astdebt.ExtCC, astdebt.ExtCXX}
	dirs := []string{dir, astdebt.DirTests, astdebt.DirTest}
	var out []string
	for _, d := range dirs {
		for _, e := range exts {
			out = append(out, filepath.Join(d, stem+"_test"+e), filepath.Join(d, astdebt.PrefixPyTest+stem+e))
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
			if IsSkippableSecurityDir(d.Name()) && path != root {
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
	case astdebt.ExtCS:
		return map[string]bool{stem + "Tests.cs": true, stem + "Test.cs": true}
	case astdebt.ExtJava:
		return map[string]bool{stem + "Test.java": true, stem + "Tests.java": true}
	case astdebt.ExtKT, astdebt.ExtKTS:
		return map[string]bool{stem + "Test.kt": true, stem + "Tests.kt": true, stem + "Spec.kt": true}
	default:
		return nil
	}
}
