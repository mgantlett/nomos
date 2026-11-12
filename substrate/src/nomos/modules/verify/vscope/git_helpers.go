package vscope

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const SepArrow = " -> "

var excludedPolyglotSegments = []string{"/build/", "/dist/", "/.svelte-kit/", "/node_modules/", "/.vitepress/", "/cache/"}

// IsExcludedPolyglotDir executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsExcludedPolyglotDir(clean string) bool {
	for _, seg := range excludedPolyglotSegments {
		if strings.Contains(clean, seg) {
			return true
		}
	}
	return false
}

// IsJSTestOrDeclFile executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsJSTestOrDeclFile(path string) bool {
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

// IsRubyTestFile executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsRubyTestFile(path string) bool {
	return strings.HasSuffix(path, "_spec.rb") || strings.HasSuffix(path, "_test.rb")
}

// IsNativeTestFile executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsNativeTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.rs") || strings.HasSuffix(path, "_test.c") || strings.HasSuffix(path, "_test.cpp") ||
		strings.HasSuffix(path, "Test.java") || strings.HasSuffix(path, "Tests.java") ||
		strings.HasSuffix(path, "Test.kt") || strings.HasSuffix(path, "Tests.kt") ||
		strings.HasSuffix(path, "Test.cs") || strings.HasSuffix(path, "Tests.cs")
}

// IsSupportedPolyglotSourceFile executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func IsSupportedPolyglotSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case astdebt.ExtGo, astdebt.ExtTS, astdebt.ExtTSX, astdebt.ExtJS, astdebt.ExtJSX, astdebt.ExtMJS, astdebt.ExtCJS, astdebt.ExtSvelte, astdebt.ExtRS, astdebt.ExtPy, astdebt.ExtRB, astdebt.ExtRake, astdebt.ExtC, astdebt.ExtCPP, astdebt.ExtCC, astdebt.ExtCXX, astdebt.ExtH, astdebt.ExtHPP, astdebt.ExtJava, astdebt.ExtKT, astdebt.ExtKTS, astdebt.ExtCS:
		return true
	default:
		return false
	}
}

// ResolveGitDir executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func ResolveGitDir(ctx *VerifyContext) string {
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
func GetModifiedPolyglotFiles(ctx *VerifyContext) (map[string]bool, error) {
	if ctx == nil || ctx.AllFiles {
		return nil, nil
	}
	return memoizedPolyglotFiles(ctx, func() (map[string]bool, error) {
		return loadModifiedPolyglotFilesUncached(ctx)
	})
}

func loadModifiedPolyglotFilesUncached(ctx *VerifyContext) (map[string]bool, error) {
	if HasCommitRangeScope(ctx) {
		return collectRangePolyglotFiles(ctx), nil
	}
	if ctx.IsBaseline || ResolveEffectiveVerifyScope(ctx) == config.VerifyScopeRepo {
		return nil, nil
	}

	dir := ResolveGitDir(ctx)
	if dir == "" {
		return nil, nil
	}

	modified := make(map[string]bool)
	CollectTaskCommitPolyglotFiles(dir, ctx.TaskID, modified)
	collectUncommittedPolyglotFiles(dir, modified)

	if len(modified) == 0 && ctx.TaskID == "" {
		return nil, nil
	}

	return modified, nil
}

// CollectTaskCommitPolyglotFiles executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func CollectTaskCommitPolyglotFiles(dir, taskID string, modified map[string]bool) {
	if taskID == "" {
		return
	}
	pattern := "\\[(Task )?" + regexp.QuoteMeta(taskID) + "(\\]|:)"
	cmd := workspace.SafeGitCommand(dir, "log", "-E", "--grep="+pattern, "--name-only", "--pretty=format:")
	out, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && IsSupportedPolyglotSourceFile(line) {
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
		if strings.Contains(relPath, SepArrow) {
			parts := strings.Split(relPath, SepArrow)
			relPath = parts[len(parts)-1]
		}
		if IsSupportedPolyglotSourceFile(relPath) {
			modified[filepath.Join(dir, relPath)] = true
		}
	}
}

// ResolveRenamedBaselinePath maps a moved file path back to its baseline path using git diff -M50% (@intent intent.md tree_sitter_ast_gate_spec.md).
func ResolveRenamedBaselinePath(ctx *VerifyContext, gitRoot, baseRef, currentRelSlash string) string {
	if gitRoot == "" || baseRef == "" || currentRelSlash == "" {
		return ""
	}
	key := gitRoot + "|" + baseRef
	renames := MemoizedRenameMap(ctx, key, func() map[string]string {
		out, err := workspace.RunSafeGit(gitRoot, "diff", "-z", "--find-renames=50%", "--name-status", baseRef)
		if err != nil || len(out) == 0 {
			return map[string]string{}
		}
		return parseGitNameStatusRenames(out)
	})
	return renames[filepath.ToSlash(currentRelSlash)]
}

func parseGitNameStatusRenames(rawZ []byte) map[string]string {
	tokens := strings.Split(string(rawZ), "\x00")
	renames := make(map[string]string)
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "" {
			continue
		}
		code := tok[0]
		if code == 'R' || code == 'C' {
			if i+2 >= len(tokens) {
				break
			}
			if code == 'R' && tokens[i+1] != "" && tokens[i+2] != "" {
				renames[filepath.ToSlash(tokens[i+2])] = filepath.ToSlash(tokens[i+1])
			}
			i += 2
			continue
		}
		i++
	}
	return renames
}
