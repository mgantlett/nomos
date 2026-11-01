// Package verify implements deterministic 1-hop AST blast-radius and impact mapping.
package verify

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// ListModifiedWorktreeFiles returns sorted workspace-relative paths modified in the active worktree (@intent: #HandshakeEvent).
func ListModifiedWorktreeFiles(worktreeDir, taskKey string) []string {
	if strings.TrimSpace(worktreeDir) == "" {
		return nil
	}
	seen := make(map[string]bool)
	addRel := func(raw string) {
		clean := strings.TrimSpace(filepath.ToSlash(raw))
		if clean == "" || strings.HasPrefix(clean, ".tmp/") || strings.HasPrefix(clean, "worktrees/") {
			return
		}
		seen[clean] = true
	}

	if out, err := workspace.SafeGitCommand(worktreeDir, "status", "--porcelain").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if len(line) > 3 && !strings.HasPrefix(line, "D ") && !strings.HasPrefix(line, " D") {
				rel := strings.TrimSpace(line[3:])
				if idx := strings.LastIndex(rel, sepArrow); idx >= 0 {
					rel = rel[idx+len(sepArrow):]
				}
				addRel(rel)
			}
		}
	}
	collectCommittedBranchFiles(worktreeDir, taskKey, addRel)

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectCommittedBranchFiles(worktreeDir, taskKey string, addRel func(string)) {
	if out, err := workspace.SafeGitCommand(worktreeDir, "diff", "--name-only", "main...HEAD").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			addRel(line)
		}
	}
	if taskKey == "" {
		return
	}
	if out, err := workspace.SafeGitCommand(worktreeDir, "log", "-E", "--grep=\\[(Task )?"+taskKey, "--name-only", "--pretty=format:").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			addRel(line)
		}
	}
}

// ComputeWorkspaceImpact synthesizes a 1-hop blast-radius map for modified files (@intent: #HandshakeEvent).
func ComputeWorkspaceImpact(worktreeDir string, modifiedFiles []string) []string {
	if strings.TrimSpace(worktreeDir) == "" {
		return nil
	}
	if len(modifiedFiles) == 0 {
		modifiedFiles = ListModifiedWorktreeFiles(worktreeDir, "")
	}
	if len(modifiedFiles) == 0 {
		return nil
	}

	goImportIndex := buildGoReverseImportIndex(worktreeDir)
	var summaries []string
	seenDirs := make(map[string]bool)

	for _, rel := range modifiedFiles {
		if len(summaries) >= 6 {
			break
		}
		if !isSupportedPolyglotSourceFile(rel) || isTestSourceFile(rel) {
			continue
		}
		entry := buildSingleFileImpact(worktreeDir, rel, goImportIndex, seenDirs)
		if entry != "" {
			summaries = append(summaries, entry)
		}
	}
	return summaries
}

func isTestSourceFile(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") ||
		isJSTestOrDeclFile(base) ||
		isNativeTestFile(base)
}

func buildSingleFileImpact(root, rel string, goReverse map[string][]string, seenGoDirs map[string]bool) string {
	absPath := filepath.Join(root, rel)
	testNote := resolvePairedTestStatus(root, rel)
	anchors := extractFileIntentAnchors(absPath)

	var revNote string
	if strings.HasSuffix(rel, ".go") {
		pkgDir := filepath.ToSlash(filepath.Dir(rel))
		if !seenGoDirs[pkgDir] {
			seenGoDirs[pkgDir] = true
			if rev := lookupGoReverseImporters(pkgDir, goReverse); len(rev) > 0 {
				revNote = fmt.Sprintf("importers(%d): %s", len(rev), strings.Join(limitSlice(rev, 3), ", "))
			}
		}
	} else {
		if cnt := countPolyglotStemImporters(root, rel); cnt > 0 {
			revNote = fmt.Sprintf("1-hop dependents: %d file(s)", cnt)
		}
	}

	parts := []string{testNote}
	if revNote != "" {
		parts = append(parts, revNote)
	}
	if len(anchors) > 0 {
		parts = append(parts, "intent: "+strings.Join(limitSlice(anchors, 2), ", "))
	}
	return fmt.Sprintf("%s -> %s", rel, strings.Join(parts, " | "))
}

func resolvePairedTestStatus(root, rel string) string {
	ext := filepath.Ext(rel)
	stem := strings.TrimSuffix(filepath.Base(rel), ext)
	dir := filepath.Dir(rel)

	var candidates []string
	switch ext {
	case ".go":
		candidates = []string{filepath.Join(dir, stem+"_test.go")}
	case ".py":
		candidates = []string{
			filepath.Join(dir, "test_"+stem+".py"),
			filepath.Join(dir, stem+"_test.py"),
			filepath.Join("tests", "test_"+stem+".py"),
		}
	case ".ts", ".tsx", ".js", ".jsx":
		candidates = []string{
			filepath.Join(dir, stem+".test"+ext),
			filepath.Join(dir, stem+".spec"+ext),
		}
	default:
		return "source modified"
	}

	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(root, c)); err == nil {
			return "paired test: " + filepath.Base(c)
		}
	}
	return "no paired unit test"
}

func extractFileIntentAnchors(absPath string) []string {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range intentCUEAnchorRe.FindAllStringSubmatch(string(data), -1) {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

func buildGoReverseImportIndex(root string) map[string][]string {
	index := make(map[string]map[string]bool)
	fset := token.NewFileSet()

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if pErr != nil {
			return nil
		}
		relDir := "."
		if r, rErr := filepath.Rel(root, filepath.Dir(path)); rErr == nil {
			relDir = filepath.ToSlash(r)
		}
		for _, imp := range node.Imports {
			if imp.Path == nil {
				continue
			}
			impVal := strings.Trim(imp.Path.Value, "\"")
			if index[impVal] == nil {
				index[impVal] = make(map[string]bool)
			}
			index[impVal][relDir] = true
		}
		return nil
	})

	flat := make(map[string][]string, len(index))
	for imp, callers := range index {
		list := make([]string, 0, len(callers))
		for c := range callers {
			list = append(list, c)
		}
		sort.Strings(list)
		flat[imp] = list
	}
	return flat
}

func lookupGoReverseImporters(pkgDir string, goReverse map[string][]string) []string {
	seen := make(map[string]bool)
	for impPath, callers := range goReverse {
		if impPath == pkgDir || strings.HasSuffix(impPath, "/"+pkgDir) {
			for _, c := range callers {
				if c != pkgDir {
					seen[filepath.Base(c)] = true
				}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

var ignoredPolyglotStems = map[string]bool{
	"index": true, "main": true, "__init__": true,
}

func countPolyglotStemImporters(root, targetRel string) int {
	ext := filepath.Ext(targetRel)
	stem := strings.TrimSuffix(filepath.Base(targetRel), ext)
	if len(stem) < 3 || ignoredPolyglotStems[stem] {
		return 0
	}
	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if fileImportsPolyglotStem(root, path, targetRel, ext, stem) {
			count++
		}
		return nil
	})
	return count
}

func fileImportsPolyglotStem(root, path, targetRel, ext, stem string) bool {
	if filepath.Ext(path) != ext {
		return false
	}
	if rel, rErr := filepath.Rel(root, path); rErr == nil && filepath.ToSlash(rel) == targetRel {
		return false
	}
	data, rErr := os.ReadFile(path)
	if rErr != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, stem) && strings.Contains(content, "import")
}

func limitSlice(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return items[:max]
}
