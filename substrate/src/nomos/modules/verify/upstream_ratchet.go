// Package verify implements upstream surgical ratchet checks, Go switch default order AST analysis, and corpus probes.
package verify

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

const corpusOracleTimeout = 120 * time.Second

var copyrightEmailRe = regexp.MustCompile(`<([^<>\s]+@[^<>\s]+)>`)

type upstreamBaseSnapshot struct {
	rel       string
	absPath   string
	baseBytes []byte
	curBytes  []byte
}

func isUpstreamSurgicalRatchet(ctx *vscope.VerifyContext) bool {
	if !vscope.IsBrownfieldRatchetActive(ctx) {
		return false
	}
	cfg := vscope.ResolveVerifyConfig(ctx)
	return cfg != nil && cfg.Governance.QualityGates.EffectiveRatchetMode() == config.RatchetModeUpstream
}

// EvaluateUpstreamSurgicalViolations blocks collateral baseline function extractions and mismatched copyright emails (@intent tree_sitter_ast_gate_spec.md #QualityGatesConfig).
func EvaluateUpstreamSurgicalViolations(ctx *vscope.VerifyContext, root string, metrics []astdebt.FileMetric) []string {
	if !isUpstreamSurgicalRatchet(ctx) {
		return nil
	}
	newFiles, baseSnaps := partitionUpstreamFiles(ctx, root, metrics)
	if len(newFiles) == 0 {
		return nil
	}
	var violations []string
	gitEmail := resolveGitAuthorEmail(ctx, root)
	for _, rel := range newFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		if v := checkNetNewCopyrightEmail(rel, raw, gitEmail); v != "" {
			violations = append(violations, v)
		}
	}
	violations = append(violations, detectCollateralFunctionMoves(root, newFiles, baseSnaps)...)
	return violations
}

func partitionUpstreamFiles(ctx *vscope.VerifyContext, root string, metrics []astdebt.FileMetric) ([]string, []upstreamBaseSnapshot) {
	var newFiles []string
	var baseSnaps []upstreamBaseSnapshot
	for _, m := range metrics {
		absPath := filepath.Join(root, m.Name)
		baseBytes, ok := loadBaselineFileBytes(ctx, root, absPath)
		if !ok {
			newFiles = append(newFiles, filepath.ToSlash(m.Name))
			continue
		}
		curBytes, err := os.ReadFile(absPath)
		if err != nil || bytes.Equal(curBytes, baseBytes) {
			continue
		}
		baseSnaps = append(baseSnaps, upstreamBaseSnapshot{
			rel:       filepath.ToSlash(m.Name),
			absPath:   absPath,
			baseBytes: baseBytes,
			curBytes:  curBytes,
		})
	}
	sort.Strings(newFiles)
	return newFiles, baseSnaps
}

func detectCollateralFunctionMoves(root string, newFiles []string, baseSnaps []upstreamBaseSnapshot) []string {
	newFnOwners := indexNewFileFunctions(root, newFiles)
	if len(newFnOwners) == 0 {
		return nil
	}
	var violations []string
	for _, snap := range baseSnaps {
		baseFns := parseBaselineFunctions(snap.absPath, snap.rel, snap.baseBytes)
		curFns := parseBaselineFunctions(snap.absPath, snap.rel, snap.curBytes)
		for _, fnName := range sortedMetricMapKeys(baseFns) {
			if _, retained := curFns[fnName]; retained {
				continue
			}
			if newOwner, moved := newFnOwners[fnName]; moved {
				violations = append(violations, fmt.Sprintf("%s: collateral extraction of baseline function %s into net-new file %s prohibited in ratchet_mode=upstream", snap.rel, fnName, newOwner))
			}
		}
	}
	return violations
}

func indexNewFileFunctions(root string, newFiles []string) map[string]string {
	owners := make(map[string]string)
	for _, rel := range newFiles {
		absPath := filepath.Join(root, filepath.FromSlash(rel))
		raw, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		for fnName := range parseBaselineFunctions(absPath, rel, raw) {
			if _, exists := owners[fnName]; !exists {
				owners[fnName] = rel
			}
		}
	}
	return owners
}

func sortedMetricMapKeys(m map[string]astdebt.FunctionMetric) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func resolveGitAuthorEmail(ctx *vscope.VerifyContext, root string) string {
	gitRoot := resolveGitTreeRootForPath(ctx, root, root)
	if gitRoot == "" {
		gitRoot = root
	}
	out, err := workspace.RunSafeGit(gitRoot, "config", "user.email")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func checkNetNewCopyrightEmail(rel string, content []byte, gitEmail string) string {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for lineIdx := 0; lineIdx < 15 && scanner.Scan(); lineIdx++ {
		line := strings.TrimSpace(scanner.Text())
		if !isCommentHeaderLine(line) || !strings.Contains(strings.ToLower(line), "copyright") {
			continue
		}
		m := copyrightEmailRe.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		headerEmail := strings.TrimSpace(m[1])
		if gitEmail == "" || !strings.EqualFold(headerEmail, gitEmail) {
			return fmt.Sprintf("%s: net-new file carries third-party copyright email <%s> (git user.email is <%s>) in ratchet_mode=upstream", rel, headerEmail, gitEmail)
		}
	}
	return ""
}

func isCommentHeaderLine(line string) bool {
	return strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*")
}

// CheckGoSwitchDefaultOrder flags Go switch or type-switch blocks whose default clause precedes trailing case clauses (@intent tree_sitter_ast_gate_spec.md #GateEvaluationReport).
func CheckGoSwitchDefaultOrder(node *ast.File, fset *token.FileSet, relPath string) []string {
	if node == nil || fset == nil {
		return nil
	}
	var violations []string
	ast.Inspect(node, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.SwitchStmt:
			if s.Body != nil {
				violations = append(violations, checkSwitchBodyClauseOrder(s.Body.List, fset, relPath)...)
			}
		case *ast.TypeSwitchStmt:
			if s.Body != nil {
				violations = append(violations, checkSwitchBodyClauseOrder(s.Body.List, fset, relPath)...)
			}
		}
		return true
	})
	return violations
}

func checkSwitchBodyClauseOrder(stmts []ast.Stmt, fset *token.FileSet, relPath string) []string {
	var out []string
	for i, stmt := range stmts {
		cc, ok := stmt.(*ast.CaseClause)
		if ok && cc.List == nil && i < len(stmts)-1 {
			pos := fset.Position(cc.Case)
			out = append(out, fmt.Sprintf("%s:%d: non-terminal default clause in switch block (move default: to final branch)", filepath.ToSlash(relPath), pos.Line))
		}
	}
	return out
}

func evaluateGoSwitchIdiomViolations(ctx *vscope.VerifyContext, root string) []string {
	var violations []string
	walkErr := vscope.WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, wErr error) error {
		if wErr != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset, node, pErr := vscope.MemoizedParseGoFile(ctx, path, 0)
		if pErr != nil || node == nil {
			return nil
		}
		rel := relativeOrFullPath(root, path)
		cur := CheckGoSwitchDefaultOrder(node, fset, rel)
		violations = append(violations, filterNetNewLineViolations(ctx, root, path, cur, func(baseBytes []byte) []string {
			bFset := token.NewFileSet()
			bNode, bErr := parser.ParseFile(bFset, path, baseBytes, 0)
			if bErr != nil {
				return nil
			}
			return CheckGoSwitchDefaultOrder(bNode, bFset, rel)
		})...)
		return nil
	})
	if walkErr != nil {
		return violations
	}
	return violations
}

func evaluateCorpusOracle(gateName string, ctx *vscope.VerifyContext, root string) (string, *vscope.GateResult) {
	cfg := vscope.ResolveVerifyConfig(ctx)
	if cfg == nil || strings.TrimSpace(cfg.Governance.QualityGates.CorpusCommand) == "" {
		return "", nil
	}
	cmdStr := strings.TrimSpace(cfg.Governance.QualityGates.CorpusCommand)
	baseRef := resolveBaselineCommitRef(ctx, root)
	tCtx, cancel := context.WithTimeout(context.Background(), corpusOracleTimeout)
	defer cancel()

	cmd := exec.CommandContext(tCtx, "sh", "-c", cmdStr)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"NOMOS_WORKTREE_ROOT="+root,
		"NOMOS_BASELINE_REF="+baseRef,
		polyrun.EnvInToolchainGate+"=1",
		"GOWORK=off",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &vscope.GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  fmt.Sprintf("reference corpus regression check failed: %s", polyrun.TruncateToolchainOutput(string(out))),
			Remedy:   "resolve regressions reported by governance.quality_gates.corpus_command",
		}
	}
	return "Corpus(0 regressions)", nil
}
