// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

var regexTaskKey = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)

func init() {
	RegisterGate("anti_shim", func() Gate { return &AntiShimGate{} })
	RegisterGate("cognitive_debt", func() Gate { return &CognitiveDebtGate{} })
	RegisterGate("max_file_loc", func() Gate { return &MaxFileLOCGate{} })
	RegisterGate("complexity", func() Gate { return &ComplexityGate{} })
	RegisterGate("hermetic_worktree", func() Gate { return &HermeticWorktreeGate{} })
	RegisterGate("phase_discipline", func() Gate { return &PhaseDisciplineGate{} })
	RegisterGate("canonical_singularity", func() Gate { return &CanonicalSingularityGate{} })
	RegisterGate("unchecked_errors", func() Gate { return &UncheckedErrorsGate{} })
	RegisterGate("zero_filesystem_alias", func() Gate { return &ZeroFilesystemAliasGate{} })
	RegisterGate("plane_separation", func() Gate { return &PlaneSeparationGate{} })
	RegisterGate("go_toolchain", func() Gate { return &GoToolchainGate{} })
	RegisterGate("staged_boundaries", func() Gate { return &StagedBoundariesGate{} })
	RegisterGate("git_hygiene", func() Gate { return &GitSubprocessHygieneGate{} })
	RegisterGate("commit_schema", func() Gate { return &CommitSchemaGate{} })
}

func isPolyglotTargetFile(path string) bool {
	if isExcludedPolyglotDir(filepath.ToSlash(path)) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return !strings.HasSuffix(path, "_test.go")
	case ".ts", ".tsx", ".js", ".jsx":
		return !isJSTestOrDeclFile(path)
	case ".svelte":
		return true
	case ".py":
		return !strings.HasPrefix(filepath.Base(path), "test_") && !strings.HasSuffix(path, "_test.py")
	case ".rs", ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".java", ".kt", ".kts", ".cs":
		return !isNativeTestFile(path)
	default:
		return false
	}
}

// CognitiveDebtGate verifies workspace cognitive debt scores.
type CognitiveDebtGate struct{}

func (g *CognitiveDebtGate) Name() string { return "Cognitive Debt Score Invariant" }

func (g *CognitiveDebtGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	thresh := resolveThresholds(ctx)
	var metrics []FileMetric
	sources := make(map[string][]byte)

	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || !isPolyglotTargetFile(path) {
			return nil
		}
		content, rErr := os.ReadFile(path)
		if rErr != nil {
			return nil
		}
		rep, dErr := AnalyzeFileDebt(path, content, thresh.MaxComplexity, thresh.MaxFuncLOC)
		if dErr == nil && rep != nil {
			rel, rErr := filepath.Rel(root, path)
			if rErr != nil {
				rel = path
			}
			metrics = append(metrics, FileMetric{Name: rel, LOC: rep.TotalLOC, CDS: rep.DebtScore})
			sources[rel] = content
		}
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "cognitive debt walk failed: %v", err)
	}

	violations := EnrichCDSViolations(root, metrics, sources, thresh, isWorktreeContext(ctx, root))
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: fmt.Sprintf("detected %d CDS/clone violation(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:  "apply the prescribed AST extraction seams and consolidate structural clones",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "all files within cognitive debt bounds"}, nil
}

// MaxFileLOCGate verifies that no file exceeds the line length limit.
type MaxFileLOCGate struct{}

func (g *MaxFileLOCGate) Name() string { return "Max File LOC Invariant" }

func (g *MaxFileLOCGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	thresh := resolveThresholds(ctx)
	var metrics []FileMetric

	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || !isPolyglotTargetFile(path) {
			return nil
		}
		lines := countFileLines(path)
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		metrics = append(metrics, FileMetric{Name: rel, LOC: lines})
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "file loc walk failed: %v", err)
	}

	violations := EnrichFileLOCViolations(root, metrics, thresh)
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: strings.Join(violations, "; "),
			Remedy:  "extract the top candidate functions listed above into modular sibling units",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "all files within LOC bounds"}, nil
}

func countFileLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lines := 0
	for scanner.Scan() {
		lines++
	}
	if err := scanner.Err(); err != nil {
		return lines
	}
	return lines
}

// ComplexityGate verifies cyclomatic complexity and function LOC bounds.
type ComplexityGate struct{}

func (g *ComplexityGate) Name() string { return "Cognitive Complexity Invariant" }

func (g *ComplexityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	thresh := resolveThresholds(ctx)
	var funcMetrics []FunctionMetric
	fset := token.NewFileSet()

	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || !isPolyglotTargetFile(path) {
			return nil
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		if !strings.HasSuffix(path, ".go") {
			collectPolyglotComplexity(path, rel, &funcMetrics)
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, 0)
		if pErr != nil {
			return nil
		}
		collectFunctionMetrics(node, fset, rel, &funcMetrics)
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "complexity walk failed: %v", err)
	}

	violations := EnrichComplexityViolations(root, funcMetrics, thresh, isWorktreeContext(ctx, root))
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: fmt.Sprintf("detected %d complexity/arity violation(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:  "decompose complex functions along the prescribed AST seams and encapsulate >8 parameters into structs",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "all functions within complexity bounds"}, nil
}

func collectPolyglotComplexity(path, rel string, out *[]FunctionMetric) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	engine, err := treesitter.GetDefaultEngine()
	if err != nil {
		return
	}
	rep, err := engine.ParseAndEvaluate(context.Background(), path, content)
	if err != nil || rep == nil {
		return
	}
	for _, fn := range rep.Functions {
		*out = append(*out, FunctionMetric{
			File:       rel,
			Line:       fn.Line,
			Name:       fn.Name,
			Complexity: fn.Complexity,
			LOC:        fn.LOC,
		})
	}
}

func collectFunctionMetrics(node *ast.File, fset *token.FileSet, relPath string, out *[]FunctionMetric) {
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Line
		end := fset.Position(fn.End()).Line
		comp := CalculateCyclomaticComplexity(fn)
		*out = append(*out, FunctionMetric{
			File:       relPath,
			Line:       start,
			Name:       fn.Name.Name,
			Complexity: comp,
			LOC:        end - start + 1,
		})
	}
}

// HermeticWorktreeGate verifies worktree boundaries.
type HermeticWorktreeGate struct{}

func (g *HermeticWorktreeGate) Name() string { return "Hermetic Worktree Invariant" }

func (g *HermeticWorktreeGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx.WorktreePath == "" {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no worktree path specified"}, nil
	}
	repo := ctx.RepoRoot
	if repo == "" {
		repo = "."
	}
	branch := ctx.Phase
	if branch == "" {
		branch = "main"
	}
	err := workspace.ValidateWorktreeTaskBoundary(repo, branch, ctx.WorktreePath, ctx.TaskID)
	if err != nil {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: err.Error(), Remedy: "execute mutations strictly within transient worktrees",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "worktree boundaries intact"}, nil
}

// PhaseDisciplineGate verifies lifecycle phase state.
type PhaseDisciplineGate struct{}

func (g *PhaseDisciplineGate) Name() string { return "Phase Discipline Invariant" }

func (g *PhaseDisciplineGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx == nil || ctx.Phase == "" {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no active phase enforced"}, nil
	}
	if !types.IsValidPhase(types.Phase(ctx.Phase)) {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: fmt.Sprintf("invalid workspace phase %q: must be IDLE, PLAN, EDIT, REVIEW, SYNC, or LEARN", ctx.Phase),
			Remedy:  "restore valid phase state via 'nomos handshake' or 'nomos task abort'",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: fmt.Sprintf("phase %s valid", ctx.Phase)}, nil
}

// CanonicalSingularityGate verifies task key formatting.
type CanonicalSingularityGate struct{}

func (g *CanonicalSingularityGate) Name() string { return "Canonical Singularity Invariant" }

func (g *CanonicalSingularityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx.TaskID == "" {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no task id specified"}, nil
	}
	if !regexTaskKey.MatchString(ctx.TaskID) {
		return &GateResult{
			GateName: g.Name(), Passed: false,
			Message: fmt.Sprintf("invalid task key %q: must adhere to INT-<N> or SUB-<N>", ctx.TaskID),
			Remedy:  "use canonical task key prefixes INT- or SUB-",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "canonical task key valid"}, nil
}

// ZeroFilesystemAliasGate strictly enforces Law 6 by rejecting filesystem symlinks.
type ZeroFilesystemAliasGate struct{}

func (g *ZeroFilesystemAliasGate) Name() string { return "Zero Filesystem Alias Invariant" }

func (g *ZeroFilesystemAliasGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	var symlinks []string
	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			rel, rErr := filepath.Rel(root, path)
			if rErr != nil {
				rel = path
			}
			symlinks = append(symlinks, rel)
		}
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "zero filesystem alias walk failed: %v", err)
	}

	violations := EvaluateZeroFilesystemAliasViolations(symlinks)
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  violations[0],
			Remedy:   "replace all filesystem symlinks with canonical physical files (Law 6)",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "zero filesystem aliases (symlinks) detected"}, nil
}


// PlaneSeparationGate enforces Law 7 by rejecting cross-plane mutations (SUB mutating intent, INT mutating substrate).
type PlaneSeparationGate struct{}

func (g *PlaneSeparationGate) Name() string { return "Plane Separation Invariant" }

func (g *PlaneSeparationGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx.TaskID == "" || ctx.IsBaseline {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "plane separation verified (baseline or unassigned task)"}, nil
	}
	workDir := ctx.WorktreePath
	if workDir == "" {
		workDir = ctx.RepoRoot
	}
	modified := getModifiedFilesInScope(ctx, workDir)

	cfg := resolveVerifyConfig(ctx)
	var intentRoots, substrateRoots []string
	if cfg != nil {
		intentRoots = cfg.Governance.QualityGates.IntentSourceRoots
		substrateRoots = cfg.Governance.QualityGates.SubstrateSourceRoots
	}

	violations := EvaluatePlaneViolationsWithRoots(ctx.TaskID, modified, intentRoots, substrateRoots)
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  violations[0],
			Remedy:   PlaneSeparationRemedy(ctx.TaskID),
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "plane separation intact: zero cross-plane mutations"}, nil
}

