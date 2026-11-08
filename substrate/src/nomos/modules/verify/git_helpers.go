package verify

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const sepArrow = " -> "

var excludedPolyglotSegments = []string{"/build/", "/dist/", "/.svelte-kit/", "/node_modules/", "/.vitepress/", "/cache/"}

func isExcludedPolyglotDir(clean string) bool {
	for _, seg := range excludedPolyglotSegments {
		if strings.Contains(clean, seg) {
			return true
		}
	}
	return false
}

func isJSTestOrDeclFile(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".d.ts") {
		return true
	}
	for _, suf := range []string{
		".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx",
		".test.js", ".spec.js", ".test.jsx", ".spec.jsx",
		".test.mjs", ".spec.mjs", ".test.cjs", ".spec.cjs",
	} {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

func isRubyTestFile(path string) bool {
	return strings.HasSuffix(path, "_spec.rb") || strings.HasSuffix(path, "_test.rb")
}

func isNativeTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.rs") || strings.HasSuffix(path, "_test.c") || strings.HasSuffix(path, "_test.cpp") ||
		strings.HasSuffix(path, "Test.java") || strings.HasSuffix(path, "Tests.java") ||
		strings.HasSuffix(path, "Test.kt") || strings.HasSuffix(path, "Tests.kt") ||
		strings.HasSuffix(path, "Test.cs") || strings.HasSuffix(path, "Tests.cs")
}

func isSupportedPolyglotSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case extGo, extTS, extTSX, extJS, extJSX, extMJS, extCJS, extSvelte, extRS, extPy, extRB, extRake, extC, extCPP, extCC, extCXX, extH, extHPP, extJava, extKT, extKTS, extCS:
		return true
	default:
		return false
	}
}

func resolveGitDir(ctx *VerifyContext) string {
	dir := ctx.WorktreePath
	if dir == "" {
		dir = ctx.RepoRoot
	}
	if dir == "" {
		return ""
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return dir
	}
	if ctx.RepoRoot != "" {
		if _, err := os.Stat(filepath.Join(ctx.RepoRoot, ".git")); err == nil {
			return ctx.RepoRoot
		}
	}
	return ""
}

// getModifiedPolyglotFiles returns a set of absolute paths for supported polyglot files modified in the current worktree or commit range.
func getModifiedPolyglotFiles(ctx *VerifyContext) (map[string]bool, error) {
	if ctx == nil || ctx.AllFiles {
		return nil, nil
	}
	if hasCommitRangeScope(ctx) {
		return collectRangePolyglotFiles(ctx), nil
	}
	if ctx.IsBaseline {
		return nil, nil
	}

	dir := resolveGitDir(ctx)
	if dir == "" {
		return nil, nil
	}

	modified := make(map[string]bool)
	collectTaskCommitPolyglotFiles(dir, ctx.TaskID, modified)
	collectUncommittedPolyglotFiles(dir, modified)

	if len(modified) == 0 && ctx.TaskID == "" {
		return nil, nil
	}

	return modified, nil
}

func collectTaskCommitPolyglotFiles(dir, taskID string, modified map[string]bool) {
	if taskID == "" {
		return
	}
	cmd := workspace.SafeGitCommand(dir, "log", "-E", "--grep=\\[(Task )?"+taskID, "--name-only", "--pretty=format:")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && isSupportedPolyglotSourceFile(line) {
			modified[filepath.Join(dir, line)] = true
		}
	}
}

func collectUncommittedPolyglotFiles(dir string, modified map[string]bool) {
	cmd := workspace.SafeGitCommand(dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) <= 3 {
			continue
		}
		relPath := strings.TrimSpace(line[3:])
		if strings.Contains(relPath, sepArrow) {
			parts := strings.Split(relPath, sepArrow)
			relPath = parts[len(parts)-1]
		}
		if isSupportedPolyglotSourceFile(relPath) {
			modified[filepath.Join(dir, relPath)] = true
		}
	}
}
