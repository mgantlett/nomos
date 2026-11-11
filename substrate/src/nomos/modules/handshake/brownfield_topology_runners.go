// Package handshake implements language, shared-module, and test-runner classifiers for brownfield maps.
package handshake

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var bundledAssetHashRe = regexp.MustCompile(`-[A-Za-z0-9_-]{8}\.(?:js|mjs|cjs)$`)

var skippedTopologyDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, ".tmp": true,
	"worktrees": true, "node_modules": true, "vendor": true,
	".venv": true, "venv": true, "env": true, "__pycache__": true,
	"dist": true, "build": true, "out": true, "target": true,
	"static": true, "public": true, "assets": true, "generated": true, "bundles": true,
	"coverage": true, "testdata": true, ".agents": true, ".opencode": true,
}

var nonPackageTopDirs = map[string]bool{
	"docs": true, "doc": true, "examples": true, "example": true,
	"benchmarks": true, "benchmark": true, "bench": true, "scripts": true,
	"hack": true, "contrib": true, "packaging": true, "deploy": true,
	"assets": true, "intent": true, "typings": true, "types": true,
	"test": true, "tests": true, "__tests__": true, "spec": true, "specs": true,
}

var dedicatedTestDirs = map[string]bool{
	"test": true, "tests": true, "__tests__": true, "spec": true, "specs": true,
}

func isSkippedTopologyDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "_") && lower != "__tests__" {
		return true
	}
	return skippedTopologyDirs[lower]
}

func isNonPackageTopDir(name string) bool {
	clean := strings.TrimLeft(strings.ToLower(name), "_")
	return nonPackageTopDirs[clean]
}

func isDedicatedTestDir(name string) bool {
	return dedicatedTestDirs[strings.ToLower(name)]
}

// isBundledOrMinifiedAsset returns true for minified or content-hashed JS artifacts.
func isBundledOrMinifiedAsset(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".min.js") || strings.HasSuffix(lower, ".min.mjs") || strings.HasSuffix(lower, ".bundle.js") {
		return true
	}
	return bundledAssetHashRe.MatchString(name)
}

// topologyLanguageByExt maps canonical source file extensions to brownfield language keys.
var topologyLanguageByExt = map[string]string{
	".go":     "go",
	".py":     "python",
	".ts":     "typescript",
	".tsx":    "typescript",
	".js":     "javascript",
	".jsx":    "javascript",
	".mjs":    "javascript",
	".cjs":    "javascript",
	".svelte": "svelte",
	".rs":     "rust",
	".rb":     "ruby",
	".c":      "cpp",
	".cc":     "cpp",
	".cpp":    "cpp",
	".cxx":    "cpp",
	".h":      "cpp",
	".hpp":    "cpp",
	".java":   "java",
	".kt":     "kotlin",
	".kts":    "kotlin",
	".cs":     "csharp",
}

// classifyTopologyLanguage resolves the source language for a file name, excluding generated bundles and type declarations.
func classifyTopologyLanguage(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".d.ts") || strings.HasSuffix(lower, ".test-d.ts") || isBundledOrMinifiedAsset(name) {
		return ""
	}
	return topologyLanguageByExt[filepath.Ext(lower)]
}

func isTopologyTestFile(slashRel, name string) bool {
	lower := strings.ToLower(name)
	segs := strings.Split(strings.ToLower(slashRel), "/")
	for i := 0; i < len(segs)-1; i++ {
		seg := segs[i]
		if dedicatedTestDirs[seg] || strings.HasSuffix(seg, "-test") || strings.HasSuffix(seg, "_test") {
			return true
		}
	}
	if strings.HasPrefix(lower, "test_") {
		return true
	}
	for _, suf := range []string{
		"_test.go", "_test.py", "_spec.rb", "_test.rb", "_test.rs",
		".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx",
		".test.js", ".spec.js", ".test.mjs", ".test.cjs",
		"test.java", "tests.java", "test.kt", "tests.cs", "test.cs",
	} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

func isBuildOrConfigScript(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "setup.py", "conftest.py", "noxfile.py", "tasks.py", "fabfile.py":
		return true
	}
	for _, suf := range []string{".config.js", ".config.ts", ".config.mjs", ".config.cjs"} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

func isSharedUtilityFilename(name string) bool {
	stem := strings.TrimSuffix(strings.ToLower(name), filepath.Ext(strings.ToLower(name)))
	for _, token := range []string{"helper", "helpers", "util", "utils", "common", "shared", "error", "errors", "token", "tokens", "types"} {
		if stem == token || strings.HasSuffix(stem, "_"+token) {
			return true
		}
	}
	return false
}

func detectBrownfieldTestRunners(root string, langs []string) []string {
	var runners []string
	seen := make(map[string]bool)
	add := func(cmd string) {
		if cmd != "" && !seen[cmd] {
			seen[cmd] = true
			runners = append(runners, cmd)
		}
	}
	if fileExistsAt(root, "go.mod") {
		add("go test ./...")
	}
	if pyCmd := inferPythonTestRunner(root, langs); pyCmd != "" {
		add(pyCmd)
	}
	if jsCmd := inferNodeTestRunner(root); jsCmd != "" {
		add(jsCmd)
	}
	if fileExistsAt(root, "Cargo.toml") {
		add("cargo test")
	}
	if fileExistsAt(root, "Gemfile") || fileExistsAt(root, "Rakefile") {
		add("bundle exec rspec / rake test")
	}
	if fileExistsAt(root, "CMakeLists.txt") {
		add("ctest --test-dir build")
	}
	if fileExistsAt(root, "pom.xml") {
		add("mvn test")
	}
	if fileExistsAt(root, "build.gradle") || fileExistsAt(root, "build.gradle.kts") {
		add("gradle test")
	}
	return runners
}

func inferPythonTestRunner(root string, langs []string) string {
	hasPy := false
	for _, l := range langs {
		if l == "python" {
			hasPy = true
			break
		}
	}
	if !hasPy && !fileExistsAt(root, "pyproject.toml") && !fileExistsAt(root, "setup.py") {
		return ""
	}
	if fileExistsAt(root, "pytest.ini") {
		return "pytest"
	}
	if data, err := os.ReadFile(filepath.Join(root, "pyproject.toml")); err == nil {
		if strings.Contains(string(data), "[tool.pytest") {
			return "pytest"
		}
	}
	return "python3 -m unittest"
}

func inferNodeTestRunner(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if jErr := json.Unmarshal(data, &pkg); jErr == nil {
		if tScript := strings.TrimSpace(pkg.Scripts["test"]); tScript != "" && !strings.Contains(tScript, "no test specified") {
			return "npm test (" + tScript + ")"
		}
	}
	return "node --test"
}

func fileExistsAt(root, rel string) bool {
	fi, err := os.Stat(filepath.Join(root, rel))
	return err == nil && !fi.IsDir()
}
