// Package impact implements deterministic 1-hop AST blast-radius and impact mapping.
package impact

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
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
				if idx := strings.LastIndex(rel, vscope.SepArrow); idx >= 0 {
					rel = rel[idx+len(vscope.SepArrow):]
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
	if out, err := workspace.SafeGitCommand(worktreeDir, "log", "-E", "--grep=\\[(Task )?"+taskKey+"(\\]|:)", "--name-only", "--pretty=format:").Output(); err == nil {
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

	goImportIndex := BuildGoReverseImportIndex(worktreeDir)
	var summaries []string
	seenDirs := make(map[string]bool)

	for _, rel := range modifiedFiles {
		if len(summaries) >= 6 {
			break
		}
		if !vscope.IsSupportedPolyglotSourceFile(rel) || IsTestSourceFile(rel) {
			continue
		}
		entry := buildSingleFileImpact(worktreeDir, rel, goImportIndex, seenDirs)
		if entry != "" {
			summaries = append(summaries, entry)
		}
	}
	return summaries
}

// IsTestSourceFile executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsTestSourceFile(rel string) bool {
	base := filepath.Base(rel)
	return strings.HasSuffix(base, astdebt.SuffixGoTest) ||
		strings.HasPrefix(base, astdebt.PrefixPyTest) ||
		strings.HasSuffix(base, astdebt.SuffixPyTest) ||
		vscope.IsRubyTestFile(base) ||
		vscope.IsJSTestOrDeclFile(base) ||
		vscope.IsNativeTestFile(base)
}

func buildSingleFileImpact(root, rel string, goReverse map[string][]string, seenGoDirs map[string]bool) string {
	absPath := filepath.Join(root, rel)
	testNote := ResolvePairedTestStatus(root, rel)
	anchors := extractFileIntentAnchors(absPath)

	var revNote string
	if strings.HasSuffix(rel, ".go") {
		pkgDir := filepath.ToSlash(filepath.Dir(rel))
		if !seenGoDirs[pkgDir] {
			seenGoDirs[pkgDir] = true
			if dirs := LookupGoReverseImporterDirs(root, pkgDir, goReverse); len(dirs) > 0 {
				revNote = formatGoImportersSummary(dirs)
			}
		}
	} else {
		if deps := listPolyglotStemImporters(root, rel); len(deps) > 0 {
			revNote = fmt.Sprintf("1-hop dependents: %d file(s) [%s]", len(deps), strings.Join(LimitSlice(deps, 4), ", "))
		}
	}

	parts := []string{testNote}
	if revNote != "" {
		parts = append(parts, revNote)
	}
	if len(anchors) > 0 {
		parts = append(parts, "intent: "+strings.Join(LimitSlice(anchors, 2), ", "))
	}
	return fmt.Sprintf("%s -> %s", rel, strings.Join(parts, " | "))
}

func rubyPairedTestCandidates(dir, stem string) []string {
	out := []string{
		filepath.Join(dir, stem+"_test.rb"),
		filepath.Join(dir, stem+"_spec.rb"),
		filepath.Join(astdebt.DirTest, stem+"_test.rb"),
		filepath.Join(astdebt.DirSpec, stem+"_spec.rb"),
	}
	slashDir := filepath.ToSlash(dir)
	if strings.HasPrefix(slashDir, "app/") {
		sub := strings.TrimPrefix(slashDir, "app/")
		out = append(out,
			filepath.Join(astdebt.DirTest, sub, stem+"_test.rb"),
			filepath.Join(astdebt.DirSpec, sub, stem+"_spec.rb"),
		)
	}
	return out
}

func extractFileIntentAnchors(absPath string) []string {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, m := range intentGoTypeAnchorRe.FindAllStringSubmatch(string(data), -1) {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// BuildGoReverseImportIndex executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func BuildGoReverseImportIndex(root string) map[string][]string {
	index := make(map[string]map[string]bool)
	fset := token.NewFileSet()

	if wErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if IsSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		recordGoFileImports(fset, root, path, index)
		return nil
	}); wErr != nil {
		return nil
	}
	return flattenGoReverseIndex(index)
}

func recordGoFileImports(fset *token.FileSet, root, path string, index map[string]map[string]bool) {
	if !strings.HasSuffix(path, astdebt.ExtGo) || strings.HasSuffix(path, astdebt.SuffixGoTest) {
		return
	}
	node, pErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if pErr != nil {
		return
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
}

func flattenGoReverseIndex(index map[string]map[string]bool) map[string][]string {
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
	return LookupGoReverseImporterDirs("", pkgDir, goReverse)
}

// LookupGoReverseImporterDirs executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func LookupGoReverseImporterDirs(root, pkgDir string, goReverse map[string][]string) []string {
	exactImport := resolveExactGoImportPath(root, pkgDir)
	seen := make(map[string]bool)
	for impPath, callers := range goReverse {
		matched := false
		if exactImport != "" {
			matched = impPath == exactImport
		} else {
			matched = impPath == pkgDir || strings.HasSuffix(impPath, "/"+pkgDir)
		}
		if !matched {
			continue
		}
		for _, c := range callers {
			if c != pkgDir {
				seen[c] = true
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

func formatGoImportersSummary(dirs []string) string {
	baseSeen := make(map[string]bool, len(dirs))
	names := make([]string, 0, len(dirs))
	hasNested := false
	for _, d := range dirs {
		base := filepath.Base(d)
		if d != base {
			hasNested = true
		}
		if !baseSeen[base] {
			baseSeen[base] = true
			names = append(names, base)
		}
	}
	summary := fmt.Sprintf("importers(%d): %s", len(dirs), strings.Join(LimitSlice(names, 3), ", "))
	if hasNested {
		summary += fmt.Sprintf(" [%s]", strings.Join(LimitSlice(dirs, 4), ", "))
	}
	return summary
}

func resolveExactGoImportPath(root, pkgDir string) string {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(pkgDir) == "" {
		return ""
	}
	absRoot := filepath.Clean(root)
	cur := filepath.Clean(filepath.Join(absRoot, filepath.FromSlash(pkgDir)))
	for {
		modName := readGoModModulePath(filepath.Join(cur, "go.mod"))
		if modName != "" {
			rel, err := filepath.Rel(cur, filepath.Join(absRoot, filepath.FromSlash(pkgDir)))
			if err != nil {
				return ""
			}
			relSlash := filepath.ToSlash(rel)
			if relSlash == "." || relSlash == "" {
				return modName
			}
			return modName + "/" + relSlash
		}
		if cur == absRoot {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur || !strings.HasPrefix(parent, absRoot) {
			break
		}
		cur = parent
	}
	return ""
}

func readGoModModulePath(goModPath string) string {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				return strings.Trim(fields[1], "\"")
			}
		}
	}
	return ""
}

var ignoredPolyglotStems = map[string]bool{
	"index": true, "main": true, "__init__": true, "mod": true, "lib": true,
}

// LimitSlice executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func LimitSlice(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	return items[:max]
}
