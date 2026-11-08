// Package cli implements workspace initialization and brownfield topology discovery.
package cli

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var brownfieldSourceExts = map[string]bool{
	".go": true, ".py": true, ".ts": true, ".tsx": true,
	".js": true, ".jsx": true, ".svelte": true, ".rs": true,
	".rb": true, ".java": true, ".kt": true, ".cs": true,
	".c": true, ".cpp": true, ".cc": true, ".h": true, ".hpp": true,
}

var brownfieldIgnoredDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	".tmp": true, "worktrees": true, "node_modules": true,
	"vendor": true, ".venv": true, "venv": true, "env": true,
	"dist": true, "build": true, "out": true, "target": true,
	"__pycache__": true, ".tox": true, ".mypy_cache": true,
	".pytest_cache": true, ".ruff_cache": true, ".next": true,
	".svelte-kit": true, "coverage": true, "testdata": true,
}

var nonSubstrateTopDirs = map[string]bool{
	"docs": true, "doc": true, "intent": true, "examples": true,
	"example": true, "benchmarks": true, "benchmark": true,
	"bench": true, "test": true, "tests": true, "__tests__": true,
	"spec": true, "specs": true, "scripts": true, "hack": true,
	"contrib": true, "packaging": true, "deploy": true, "assets": true,
}

type brownfieldScanState struct {
	hasAnySource  bool
	hasRootSource bool
	hasLargeFile  bool
	topDirs       map[string]bool
}

func detectBrownfieldWorkspace(dir string) (bool, []string) {
	st := &brownfieldScanState{topDirs: make(map[string]bool)}
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		return visitBrownfieldEntry(dir, path, d, err, st)
	})
	if walkErr != nil || !st.hasAnySource {
		return false, nil
	}
	if isCanonicalCleanDualPlane(dir, st) {
		return false, nil
	}
	if st.hasRootSource || len(st.topDirs) == 0 {
		return true, nil
	}
	return true, sortedMapKeys(st.topDirs)
}

func visitBrownfieldEntry(root, path string, d fs.DirEntry, err error, st *brownfieldScanState) error {
	if err != nil || d == nil {
		return nil
	}
	if d.IsDir() {
		if path != root && shouldSkipBrownfieldScanDir(d.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if !isImplementationSourceFile(d.Name()) {
		return nil
	}
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		return nil
	}
	recordDiscoveredSourceFile(path, filepath.ToSlash(rel), d.Name(), st)
	return nil
}

func recordDiscoveredSourceFile(absPath, slashRel, name string, st *brownfieldScanState) {
	st.hasAnySource = true
	if !st.hasLargeFile && fileExceedsLineLimit(absPath, 400) {
		st.hasLargeFile = true
	}
	if !strings.Contains(slashRel, "/") {
		if !isRootToolingConfigFile(name) {
			st.hasRootSource = true
		}
		return
	}
	top := strings.SplitN(slashRel, "/", 2)[0]
	if !strings.HasPrefix(top, ".") && !nonSubstrateTopDirs[strings.ToLower(top)] {
		st.topDirs[top] = true
	}
}

func isCanonicalCleanDualPlane(dir string, st *brownfieldScanState) bool {
	if st.hasRootSource || st.hasLargeFile {
		return false
	}
	if len(st.topDirs) != 1 || !st.topDirs["substrate"] {
		return false
	}
	fi, err := os.Stat(filepath.Join(dir, "intent"))
	return err == nil && fi.IsDir()
}

func fileExceedsLineLimit(path string, limit int) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	lines := 0
	for sc.Scan() {
		lines++
		if lines > limit {
			return true
		}
	}
	return false
}

func shouldSkipBrownfieldScanDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	return brownfieldIgnoredDirs[strings.ToLower(name)]
}

func isImplementationSourceFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, "_test.py") || strings.HasPrefix(lower, "test_") {
		return false
	}
	for _, suf := range []string{".test.ts", ".spec.ts", ".test.js", ".spec.js", ".d.ts"} {
		if strings.HasSuffix(lower, suf) {
			return false
		}
	}
	return brownfieldSourceExts[filepath.Ext(lower)]
}

func isRootToolingConfigFile(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "setup.py", "conftest.py", "noxfile.py", "fabfile.py", "tasks.py", "manage.py":
		return true
	}
	for _, suffix := range []string{".config.ts", ".config.js", ".config.mjs", ".config.cjs"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func renderScaffoldedNomosYAML(dir, rawTemplate, projectName, branch string) string {
	content := strings.ReplaceAll(rawTemplate, "{{.ProjectName}}", projectName)
	content = strings.ReplaceAll(content, "{{.DefaultBranch}}", branch)

	isBrownfield, substrateRoots := detectBrownfieldWorkspace(dir)
	if !isBrownfield {
		return content
	}
	content = strings.Replace(content, "        strict_mode: true", "        strict_mode: false", 1)
	if len(substrateRoots) > 0 {
		var sb strings.Builder
		sb.WriteString("        substrate_source_roots:\n")
		for _, r := range substrateRoots {
			sb.WriteString("            - \"")
			sb.WriteString(r)
			sb.WriteString("\"\n")
		}
		commentedBlock := "        # substrate_source_roots:\n        #     - \"src\"\n        #     - \"cmd\"\n"
		if strings.Contains(content, commentedBlock) {
			content = strings.Replace(content, commentedBlock, sb.String(), 1)
		} else {
			content = strings.Replace(content, "        strict_mode: false\n", "        strict_mode: false\n"+sb.String(), 1)
		}
	}
	return content
}
