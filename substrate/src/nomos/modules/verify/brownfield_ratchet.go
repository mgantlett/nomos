// Package verify implements baseline AST delta ratcheting for brownfield repositories.
package verify

import (
	"bufio"
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func isBrownfieldRatchetActive(ctx *VerifyContext) bool {
	if ctx != nil && ctx.Strict {
		return false
	}
	cfg := resolveVerifyConfig(ctx)
	return cfg != nil && !cfg.Governance.QualityGates.StrictMode
}

func hasGitMetadata(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func resolveGitTreeRootForPath(ctx *VerifyContext, fallbackRoot, absFile string) string {
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
		if hasGitMetadata(absC) && (absFile == absC || strings.HasPrefix(absFile, absC+string(filepath.Separator))) {
			return absC
		}
	}
	if wtRoot, err := workspace.FindWorkingTreeRoot(filepath.Dir(absFile)); err == nil && hasGitMetadata(wtRoot) {
		return wtRoot
	}
	return ""
}

func resolveBaselineCommitRef(ctx *VerifyContext, gitRoot string) string {
	cfg := resolveVerifyConfig(ctx)
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
	if !isWorktreeContext(ctx, gitRoot) {
		if out, err := workspace.RunSafeGit(gitRoot, "rev-parse", "--verify", "HEAD"); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func loadBaselineFileBytes(ctx *VerifyContext, fallbackRoot, targetPath string) ([]byte, bool) {
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
	out, sErr := workspace.RunSafeGit(gitRoot, "show", baseRef+":"+filepath.ToSlash(rel))
	if sErr != nil {
		return nil, false
	}
	return out, true
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

func filterRatchetedFileLOC(ctx *VerifyContext, root string, metrics []FileMetric, thresh Thresholds) []FileMetric {
	if !isBrownfieldRatchetActive(ctx) {
		return metrics
	}
	filtered := make([]FileMetric, 0, len(metrics))
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

func parseBaselineFunctions(absPath, rel string, content []byte) map[string]FunctionMetric {
	var list []FunctionMetric
	if strings.HasSuffix(absPath, ".go") {
		fset := token.NewFileSet()
		if node, err := parser.ParseFile(fset, absPath, content, 0); err == nil {
			collectFunctionMetrics(node, fset, rel, &list)
		}
	} else {
		collectPolyglotComplexityFromBytes(absPath, rel, content, &list)
	}
	byName := make(map[string]FunctionMetric, len(list))
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

func filterRatchetedComplexity(ctx *VerifyContext, root string, funcs []FunctionMetric, thresh Thresholds) ([]FunctionMetric, bool) {
	if !isBrownfieldRatchetActive(ctx) {
		return funcs, isWorktreeContext(ctx, root)
	}
	baseCache := make(map[string]map[string]FunctionMetric)
	var out []FunctionMetric
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
