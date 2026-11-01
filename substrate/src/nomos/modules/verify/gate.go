// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
// It enforces Law 2 (The Non-Verbal Proof Law), proving code correctness through compilation, AST checks,
// and mathematical invariant proofs with zero trust in conversational claims.
package verify

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// GateResult represents the outcome of an individual quality gate evaluation.
type GateResult struct {
	GateName       string `json:"gate_name"`
	ProbeID        string `json:"probe_id,omitempty"`
	Law            int    `json:"law,omitempty"`
	Ring           string `json:"ring,omitempty"`
	Passed         bool   `json:"passed"`
	Skipped        bool   `json:"skipped,omitempty"`
	Message        string `json:"message"`
	Remedy         string `json:"remedy,omitempty"`
	DurationMicros int64  `json:"duration_micros,omitempty"`
	DebtFlagged    bool   `json:"debt_flagged,omitempty"`
}

// VerifyContext provides contextual metadata for quality gate evaluation.
type VerifyContext struct {
	RepoRoot         string `json:"repo_root"`
	WorktreePath     string `json:"worktree_path"`
	TaskID           string `json:"task_id"`
	CommitRange      string `json:"commit_range,omitempty"`
	Phase            string `json:"phase"`
	IsBaseline       bool   `json:"is_baseline"`
	AllFiles         bool   `json:"all_files,omitempty"`
	Strict           bool   `json:"strict,omitempty"`
	AdvisoryDebtMode bool   `json:"advisory_debt_mode,omitempty"`
	LayerScope       any    `json:"layer_scope,omitempty"`
	CoverageRep      any    `json:"-"`
}

// Gate defines the interface for an invariant evaluation step.
type Gate interface {
	Name() string
	Evaluate(ctx *VerifyContext) (*GateResult, error)
}

// TimedEvaluate executes a gate while measuring its elapsed duration in microseconds
// and recovering runtime panics into fail-closed ERR_CORE errors.
// @intent tracetrial_harness_reliability_spec.md
func TimedEvaluate(g Gate, ctx *VerifyContext) (res *GateResult, err error) {
	if g == nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "cannot evaluate nil gate (fail-closed abort)")
	}
	defer func() {
		if r := recover(); r != nil {
			res = nil
			err = types.NewNomosError(types.ErrCodeCore, "gate %s panicked during evaluation (fail-closed abort): %v", g.Name(), r)
		}
	}()
	start := time.Now()
	res, err = g.Evaluate(ctx)
	elapsed := time.Since(start).Microseconds()
	if res != nil {
		res.DurationMicros = elapsed
	}
	return res, err
}

func hasLocalConfig(root string) string {
	if root == "" {
		return ""
	}
	candidates := []string{
		"nomos.yaml", "nomos.yml", ".nomos.yaml", ".nomos.yml",
		filepath.Join(".agents", "nomos.yaml"),
		filepath.Join(".agents", "nomos_sov.yaml"),
		filepath.Join(".nomos", "nomos.yaml"),
		filepath.Join("intent", "config", "nomos.yaml"),
		filepath.Join("intent", "runtime", "config", "nomos.yaml"),
	}
	for _, sub := range candidates {
		p := filepath.Join(root, sub)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// resolveVerifyConfig resolves ecosystem configuration, falling back to ctx.RepoRoot when inside a worktree.
func resolveVerifyConfig(ctx *VerifyContext) *config.EcosystemConfig {
	if ctx == nil {
		return config.DefaultNomosConfig()
	}
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	if p := hasLocalConfig(root); p != "" {
		if cfg, err := config.LoadConfig(p); err == nil && cfg != nil {
			return cfg
		}
	}
	if ctx.RepoRoot != "" && ctx.RepoRoot != root {
		if p := hasLocalConfig(ctx.RepoRoot); p != "" {
			if cfg, err := config.LoadConfig(p); err == nil && cfg != nil {
				return cfg
			}
		}
	}
	return config.DefaultNomosConfig()
}

func isWorktreeContext(ctx *VerifyContext, workDir string) bool {
	if ctx != nil && ctx.AllFiles {
		return false
	}
	if hasCommitRangeScope(ctx) {
		return true
	}
	if ctx != nil && ctx.IsBaseline {
		return false
	}
	gitPath := filepath.Join(workDir, ".git")
	if fi, err := os.Stat(gitPath); err == nil && !fi.IsDir() {
		return true
	}
	if strings.Contains(workDir, "/worktrees/") {
		return true
	}
	if ctx != nil && ctx.TaskID != "" {
		return true
	}
	return false
}

func collectScopePolyglotFiles(ctx *VerifyContext, workDir string, fileSet map[string]struct{}) {
	if ctx == nil {
		return
	}
	poly, err := getModifiedPolyglotFiles(ctx)
	if err != nil || len(poly) == 0 {
		return
	}
	for f := range poly {
		if rel, rErr := filepath.Rel(workDir, f); rErr == nil {
			fileSet[rel] = struct{}{}
		} else {
			fileSet[f] = struct{}{}
		}
	}
}

func collectScopeStatusFiles(workDir string, fileSet map[string]struct{}) {
	out, err := workspace.RunSafeGit(workDir, "status", "--porcelain")
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(out), "\n") {
		if len(rawLine) < 4 {
			continue
		}
		path := strings.TrimSpace(rawLine[3:])
		path = strings.Trim(path, "\"")
		if idx := strings.Index(path, " -> "); idx != -1 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if path != "" {
			fileSet[path] = struct{}{}
		}
	}
}

func collectScopeDiffFiles(workDir string, fileSet map[string]struct{}) {
	branches := []string{"main...HEAD", "origin/main...HEAD", "master...HEAD", "develop...HEAD", "trunk...HEAD"}
	for _, b := range branches {
		out, err := workspace.RunSafeGit(workDir, "diff", "--name-only", b)
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				line = strings.TrimSpace(strings.Trim(line, "\""))
				if line != "" {
					fileSet[line] = struct{}{}
				}
			}
			return
		}
	}
}

func getModifiedFilesInScope(ctx *VerifyContext, workDir string) []string {
	fileSet := make(map[string]struct{})
	collectScopePolyglotFiles(ctx, workDir, fileSet)
	collectScopeStatusFiles(workDir, fileSet)
	collectScopeDiffFiles(workDir, fileSet)

	var files []string
	for f := range fileSet {
		files = append(files, f)
	}
	return files
}

func resolveScopeExcludes(ctx *VerifyContext, workDir, rootDir string) []string {
	excludes := workspace.GetExcludes(workDir)
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil {
		return excludes
	}
	for _, ex := range cfg.Governance.QualityGates.Exclude {
		cleanEx := filepath.Clean(ex)
		if filepath.IsAbs(cleanEx) {
			excludes = append(excludes, cleanEx)
		} else {
			excludes = append(excludes, filepath.Join(workDir, cleanEx))
			if rootDir != "" && rootDir != workDir {
				excludes = append(excludes, filepath.Join(rootDir, cleanEx))
			}
		}
	}
	return excludes
}

func resolveWalkDirs(ctx *VerifyContext, root string) (string, string, string) {
	workDir := root
	if ctx != nil && ctx.WorktreePath != "" {
		workDir = ctx.WorktreePath
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}
	return workDir, absWorkDir, absRoot
}

// WalkTargetFiles traverses files in scope for verification.
// If ctx.AllFiles or ctx.IsBaseline is true (without CommitRange), or if workDir is not a task worktree, it evaluates using workspace.WalkWithExcludes.
// In a task worktree or commit-range context, it inspects only the modified or unattested files in scope.
func WalkTargetFiles(ctx *VerifyContext, root string, fn filepath.WalkFunc) error {
	workDir, absWorkDir, absRoot := resolveWalkDirs(ctx, root)
	excludes := resolveScopeExcludes(ctx, absWorkDir, absRoot)

	if hasCommitRangeScope(ctx) {
		return walkExplicitRelativeFiles(resolveUnattestedRangeFiles(ctx, workDir), absWorkDir, absRoot, excludes, fn)
	}
	if !isWorktreeContext(ctx, workDir) {
		return workspace.WalkWithExcludes(root, excludes, fn)
	}

	modified := getModifiedFilesInScope(ctx, workDir)
	if len(modified) == 0 {
		return workspace.WalkWithExcludes(root, excludes, fn)
	}
	return walkExplicitRelativeFiles(modified, absWorkDir, absRoot, excludes, fn)
}

func walkExplicitRelativeFiles(relFiles []string, absWorkDir, absRoot string, excludes []string, fn filepath.WalkFunc) error {
	for _, rel := range relFiles {
		fullPath := filepath.Join(absWorkDir, rel)
		if !strings.HasPrefix(fullPath, absRoot) {
			continue
		}
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if workspace.IsExcluded(fullPath, info.IsDir(), info.Name(), excludes) {
			continue
		}
		if walkErr := fn(fullPath, info, nil); walkErr != nil {
			if walkErr == filepath.SkipDir {
				continue
			}
			return walkErr
		}
	}
	return nil
}
