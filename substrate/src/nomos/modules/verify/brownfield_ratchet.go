// Package verify implements baseline AST delta ratcheting for brownfield repositories.
package verify

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func resolveGitTreeRootForPath(ctx *vscope.VerifyContext, fallbackRoot, absFile string) string {
	candidates := []string{}
	if ctx != nil && ctx.WorktreePath != "" {
		candidates = append(candidates, ctx.WorktreePath)
	}
	if fallbackRoot != "" {
		candidates = append(candidates, fallbackRoot)
	}
	if ctx != nil && ctx.RepoRoot != "" {
		candidates = append(candidates, ctx.RepoRoot)
	}
	for _, c := range candidates {
		absC, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if vscope.HasGitMetadata(absC) && (absFile == absC || strings.HasPrefix(absFile, absC+string(filepath.Separator))) {
			return absC
		}
	}
	if wtRoot, err := workspace.FindWorkingTreeRoot(filepath.Dir(absFile)); err == nil && vscope.HasGitMetadata(wtRoot) {
		return wtRoot
	}
	return ""
}

func resolveBaselineCommitRef(ctx *vscope.VerifyContext, gitRoot string) string {
	cfg := vscope.ResolveVerifyConfig(ctx)
	refs := []string{}
	if cfg != nil && strings.TrimSpace(cfg.DefaultBranch) != "" {
		refs = append(refs, strings.TrimSpace(cfg.DefaultBranch))
	}
	refs = append(refs, "main", "origin/main", "master", "origin/master")
	for _, ref := range refs {
		out, err := workspace.RunSafeGit(gitRoot, "merge-base", "HEAD", ref)
		if err == nil {
			if sha := strings.TrimSpace(string(out)); sha != "" {
				return sha
			}
		}
	}
	if !vscope.IsWorktreeContext(ctx, gitRoot) {
		if out, err := workspace.RunSafeGit(gitRoot, "rev-parse", "--verify", "HEAD"); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func loadBaselineFileBytes(ctx *vscope.VerifyContext, fallbackRoot, targetPath string) ([]byte, bool) {
	absFile, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, false
	}
	gitRoot := resolveGitTreeRootForPath(ctx, fallbackRoot, absFile)
	if gitRoot == "" {
		return nil, false
	}
	rel, rErr := filepath.Rel(gitRoot, absFile)
	if rErr != nil || strings.HasPrefix(rel, "..") {
		return nil, false
	}
	baseRef := resolveBaselineCommitRef(ctx, gitRoot)
	if baseRef == "" {
		return nil, false
	}
	relSlash := filepath.ToSlash(rel)
	out, sErr := workspace.RunSafeGit(gitRoot, "show", baseRef+":"+relSlash)
	if sErr == nil {
		return out, true
	}
	oldRel := vscope.ResolveRenamedBaselinePath(ctx, gitRoot, baseRef, relSlash)
	if oldRel == "" || oldRel == relSlash {
		return nil, false
	}
	renamedOut, rShowErr := workspace.RunSafeGit(gitRoot, "show", baseRef+":"+oldRel)
	if rShowErr != nil {
		return nil, false
	}
	return renamedOut, true
}

func countBytesLines(data []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lines := 0
	for scanner.Scan() {
		lines++
	}
	return lines
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}
	return a
}

func filterRatchetedFileLOC(ctx *vscope.VerifyContext, root string, metrics []astdebt.FileMetric, thresh astdebt.Thresholds) []astdebt.FileMetric {
	if !vscope.IsBrownfieldRatchetActive(ctx) {
		return metrics
	}
	filtered := make([]astdebt.FileMetric, 0, len(metrics))
	for _, m := range metrics {
		if m.LOC <= thresh.MaxFileLOC {
			continue
		}
		baseBytes, ok := loadBaselineFileBytes(ctx, root, filepath.Join(root, m.Name))
		if !ok {
			filtered = append(filtered, m)
			continue
		}
		baseLOC := countBytesLines(baseBytes)
		allowed := thresh.MaxFileLOC
		if baseLOC > thresh.MaxFileLOC {
			allowed = baseLOC + thresh.MaxFuncLOC
		}
		if m.LOC > allowed {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func parseBaselineFunctions(absPath, rel string, content []byte) map[string]astdebt.FunctionMetric {
	var list []astdebt.FunctionMetric
	if strings.HasSuffix(absPath, ".go") {
		fset := token.NewFileSet()
		if node, err := parser.ParseFile(fset, absPath, content, 0); err == nil {
			collectFunctionMetrics(node, fset, rel, &list)
		}
	} else {
		collectPolyglotComplexityFromBytes(absPath, rel, content, &list)
	}
	byName := make(map[string]astdebt.FunctionMetric, len(list))
	for _, fn := range list {
		prev, exists := byName[fn.Name]
		if !exists {
			byName[fn.Name] = fn
			continue
		}
		prev.Complexity = maxInt(prev.Complexity, fn.Complexity)
		prev.LOC = maxInt(prev.LOC, fn.LOC)
		byName[fn.Name] = prev
	}
	return byName
}

func filterRatchetedComplexity(ctx *vscope.VerifyContext, root string, funcs []astdebt.FunctionMetric, thresh astdebt.Thresholds) ([]astdebt.FunctionMetric, bool) {
	if !vscope.IsBrownfieldRatchetActive(ctx) {
		return funcs, vscope.IsWorktreeContext(ctx, root)
	}
	baseCache := make(map[string]map[string]astdebt.FunctionMetric)
	var out []astdebt.FunctionMetric
	for _, fn := range funcs {
		if fn.Complexity <= thresh.MaxComplexity && fn.LOC <= thresh.MaxFuncLOC {
			continue
		}
		baseMap, loaded := baseCache[fn.File]
		if !loaded {
			absPath := filepath.Join(root, fn.File)
			if baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath); ok {
				baseMap = parseBaselineFunctions(absPath, fn.File, baseBytes)
			}
			baseCache[fn.File] = baseMap
		}
		if baseMap == nil {
			out = append(out, fn)
			continue
		}
		baseFn := baseMap[fn.Name]
		adj := fn
		if fn.Complexity <= maxInt(thresh.MaxComplexity, baseFn.Complexity) {
			adj.Complexity = thresh.MaxComplexity
		}
		if fn.LOC <= maxInt(thresh.MaxFuncLOC, baseFn.LOC) {
			adj.LOC = thresh.MaxFuncLOC
		}
		if adj.Complexity > thresh.MaxComplexity || adj.LOC > thresh.MaxFuncLOC {
			out = append(out, adj)
		}
	}
	return out, false
}

// EvaluatePackageFanOutViolations enforces MaxPackageFiles per directory with brownfield ratcheting (@intent tree_sitter_ast_gate_spec.md #Thresholds).
func EvaluatePackageFanOutViolations(ctx *vscope.VerifyContext, root string, metrics []astdebt.FileMetric, thresh astdebt.Thresholds) []string {
	maxFiles := thresh.MaxPackageFiles
	if maxFiles <= 0 {
		maxFiles = astdebt.DefaultThresholds().MaxPackageFiles
	}
	dirs := collectUniqueMetricDirs(metrics)
	var violations []string
	for _, relDir := range dirs {
		if v := evaluateSingleDirFanOut(ctx, root, relDir, maxFiles); v != "" {
			violations = append(violations, v)
		}
	}
	return violations
}

func collectUniqueMetricDirs(metrics []astdebt.FileMetric) []string {
	seen := make(map[string]bool, len(metrics))
	var dirs []string
	for _, m := range metrics {
		relDir := filepath.ToSlash(filepath.Dir(m.Name))
		if !seen[relDir] {
			seen[relDir] = true
			dirs = append(dirs, relDir)
		}
	}
	sort.Strings(dirs)
	return dirs
}

func evaluateSingleDirFanOut(ctx *vscope.VerifyContext, root, relDir string, maxFiles int) string {
	absDir := filepath.Join(root, filepath.FromSlash(relDir))
	curCount := countDirNonTestSourceFiles(absDir)
	if curCount <= maxFiles {
		return ""
	}
	if shouldRatchetLegacyDirFanOut(ctx, root) {
		if baseCount, ok := countBaselineDirSourceFiles(ctx, root, absDir); ok {
			ceil := maxInt(maxFiles, baseCount)
			if curCount <= ceil {
				return ""
			}
			return fmt.Sprintf("%s has %d non-test source files (exceeds brownfield ceiling %d, max %d) -> [1-TURN RECIPE: extract >=%d cohesive file(s) from %s into a sub-package]",
				relDir, curCount, ceil, maxFiles, curCount-ceil, relDir)
		}
	}
	return fmt.Sprintf("%s has %d non-test source files (exceeds max %d) -> [1-TURN RECIPE: extract >=%d cohesive file(s) from %s into focused sub-packages]",
		relDir, curCount, maxFiles, curCount-maxFiles, relDir)
}

func shouldRatchetLegacyDirFanOut(ctx *vscope.VerifyContext, root string) bool {
	if vscope.IsBrownfieldRatchetActive(ctx) {
		return true
	}
	return ctx != nil && !ctx.Strict && !ctx.AllFiles && vscope.IsWorktreeContext(ctx, root)
}

func countDirNonTestSourceFiles(absDir string) int {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && isPolyglotTargetFile(filepath.Join(absDir, e.Name())) {
			count++
		}
	}
	return count
}

func countBaselineDirSourceFiles(ctx *vscope.VerifyContext, fallbackRoot, absDir string) (int, bool) {
	cleanAbs, err := filepath.Abs(absDir)
	if err != nil {
		return 0, false
	}
	gitRoot := resolveGitTreeRootForPath(ctx, fallbackRoot, cleanAbs)
	baseRef := resolveBaselineCommitRef(ctx, gitRoot)
	if gitRoot == "" || baseRef == "" {
		return 0, false
	}
	rel, rErr := filepath.Rel(gitRoot, cleanAbs)
	if rErr != nil || strings.HasPrefix(rel, "..") {
		return 0, false
	}
	treeSpec := baseRef
	if rel != "." {
		treeSpec = baseRef + ":" + filepath.ToSlash(rel)
	}
	out, lErr := workspace.RunSafeGit(gitRoot, "ls-tree", "--name-only", treeSpec)
	if lErr != nil {
		return 0, false
	}
	count := 0
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name != "" && isPolyglotTargetFile(filepath.Join(cleanAbs, name)) {
			count++
		}
	}
	return count, true
}
