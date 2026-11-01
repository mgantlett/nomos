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
	return strings.HasSuffix(path, ".d.ts") || strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".spec.ts")
}

func isNativeTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.rs") || strings.HasSuffix(path, "_test.c") || strings.HasSuffix(path, "_test.cpp")
}

func isSupportedPolyglotSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".svelte", ".rs", ".py", ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp":
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
