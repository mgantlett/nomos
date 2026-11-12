// Package vscope provides Definition of Done (DoD) quality gate pipeline execution.
package vscope

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// GitSubprocessHygieneGate enforces that all git executions use workspace.SafeGitCommand
// to ensure GIT_* environment variables are stripped and cannot corrupt host repository state.
type GitSubprocessHygieneGate struct{}

// Name returns the canonical name of the git subprocess hygiene quality gate.
func (g *GitSubprocessHygieneGate) Name() string {
	return "Git Subprocess Hygiene Invariant (Law 1/3)"
}

// Evaluate scans Go source files for direct exec.Command("git") or exec.CommandContext(..., "git") calls.
func (g *GitSubprocessHygieneGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	if root == "" {
		root = "."
	}

	violations, err := collectGitSubprocessViolations(root)
	if err != nil {
		return nil, err
	}

	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("detected %d un-sanitized git subprocess invocation(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:   "use workspace.SafeGitCommand() instead of raw os/exec",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  "zero un-sanitized git subprocess calls detected; all git executions routed through workspace.SafeGitCommand",
	}, nil
}

func collectGitSubprocessViolations(root string) ([]string, error) {
	var violations []string
	fset := token.NewFileSet()

	err := workspace.WalkScope(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		if isGitRunnerExempt(path) {
			return nil
		}
		relPath, rErr := filepath.Rel(root, path)
		if rErr != nil {
			relPath = path
		}
		node, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}
		inspectFileForRawGitExec(node, fset, relPath, &violations)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func isGitRunnerExempt(path string) bool {
	clean := filepath.ToSlash(path)
	return strings.HasSuffix(clean, "core/workspace/git.go") || strings.HasSuffix(clean, "core/workspace/git_test.go")
}

func inspectFileForRawGitExec(node *ast.File, fset *token.FileSet, relPath string, out *[]string) {
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if violation := checkRawGitCall(call, fset, relPath); violation != "" {
			*out = append(*out, violation)
		}
		return true
	})
}

func checkRawGitCall(call *ast.CallExpr, fset *token.FileSet, relPath string) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "exec" {
		return ""
	}

	pos := fset.Position(call.Pos())
	fnName := sel.Sel.Name
	if fnName == "Command" && len(call.Args) > 0 && isGitLit(call.Args[0]) {
		return fmt.Sprintf("%s:%d (raw exec.Command(\"git\"))", relPath, pos.Line)
	}
	if fnName == "CommandContext" && len(call.Args) > 1 && isGitLit(call.Args[1]) {
		return fmt.Sprintf("%s:%d (raw exec.CommandContext(..., \"git\"))", relPath, pos.Line)
	}
	return ""
}

func isGitLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	return strings.Trim(lit.Value, "`\"") == "git"
}
