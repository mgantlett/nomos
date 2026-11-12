package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// UncheckedErrorsGate verifies that error return values are checked.
type UncheckedErrorsGate struct{}

func (g *UncheckedErrorsGate) Name() string { return "Unchecked Error Hygiene Invariant" }

func (g *UncheckedErrorsGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	var violations []string
	fset := token.NewFileSet()

	err := vscope.WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() || !isPolyglotTargetFile(path) {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			violations = append(violations, evaluatePolyglotUncheckedFile(ctx, root, path)...)
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, 0)
		if pErr != nil {
			return nil
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		var fileViolations []string
		checkUncheckedErrorsInFile(node, fset, rel, &fileViolations)
		violations = append(violations, filterNetNewLineViolations(ctx, root, path, fileViolations, func(baseBytes []byte) []string {
			baseFset := token.NewFileSet()
			baseNode, bErr := parser.ParseFile(baseFset, path, baseBytes, 0)
			if bErr != nil {
				return nil
			}
			var baseV []string
			checkUncheckedErrorsInFile(baseNode, baseFset, rel, &baseV)
			return baseV
		})...)
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "unchecked error walk failed: %v", err)
	}

	if len(violations) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("detected %d unchecked error discard(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:   "handle errors explicitly instead of discarding with blank identifier `_` (Law 7)",
		}, nil
	}
	return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "zero unchecked error suppressions detected"}, nil
}

func isIgnoredCallExpr(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		switch fun.Sel.Name {
		case "Close", "Write", "Flush", "Scan", "Sscanf", "Encode", "Decode",
			"Print", "Printf", "Println", "Sprint", "Sprintf", "Fprint", "Fprintf", "Fprintln",
			"Rollback", "Add", "Done", "Wait", "Seek", "Read", "WriteString":
			return true
		}
		if xIdent, ok := fun.X.(*ast.Ident); ok {
			switch xIdent.Name {
			case "fmt", "json", "yaml", "os", "io", "bufio", "strings", "bytes", "math", "rand", "log", "filepath", "strconv", "time", "regexp", "template", "http":
				return true
			}
		}
	case *ast.Ident:
		switch fun.Name {
		case "print", "println", "close", "recover", "append", "copy":
			return true
		}
	}
	return false
}

func checkUncheckedErrorsInFile(file *ast.File, fset *token.FileSet, relPath string, violations *[]string) {
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 {
			return true
		}
		for _, expr := range assign.Rhs {
			if call, isCall := expr.(*ast.CallExpr); isCall {
				if isIgnoredCallExpr(call) {
					continue
				}
				lastIdx := len(assign.Lhs) - 1
				if ident, isIdent := assign.Lhs[lastIdx].(*ast.Ident); isIdent && ident.Name == "_" {
					pos := fset.Position(assign.Pos())
					*violations = append(*violations, fmt.Sprintf("%s:%d: discarded call result via blank identifier", relPath, pos.Line))
					break
				}
			}
		}
		return true
	})
}

func evaluatePolyglotUncheckedFile(ctx *vscope.VerifyContext, root, path string) []string {
	ext := strings.ToLower(filepath.Ext(path))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	cur := astdebt.CheckPolyglotUncheckedErrors(ext, rel, raw)
	return filterNetNewLineViolations(ctx, root, path, cur, func(baseBytes []byte) []string {
		return astdebt.CheckPolyglotUncheckedErrors(ext, rel, baseBytes)
	})
}

// GoToolchainGate asserts clean compilation under the standard Go toolchain.
type GoToolchainGate struct{}

func (g *GoToolchainGate) Name() string { return "Go Toolchain Gate" }

func (g *GoToolchainGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	if os.Getenv(polyrun.EnvInToolchainGate) == "1" {
		return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "re-entrant toolchain execution bypassed"}, nil
	}
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	targetDir := resolveGoTargetDir(root)
	var stacks []string
	var rec *polyrun.ToolchainSuiteReceipt
	if targetDir != "" {
		rec = polyrun.EvaluateSharedGoSuite(ctx, root, targetDir)
		if !rec.GoPassed {
			return &vscope.GateResult{
				GateName: g.Name(),
				Passed:   false,
				Message:  fmt.Sprintf("Go compilation/test failed: %s", polyrun.TruncateToolchainOutput(rec.GoOutput)),
				Remedy:   "resolve Go compiler and test errors (GOWORK=off go test ./...)",
			}, nil
		}
		stacks = append(stacks, formatGoToolchainStackLabel(rec))
	}

	polyStacks, polyFail := evaluateOrReusePolyglotToolchains(ctx, root, rec)
	if polyFail != nil {
		return polyFail, nil
	}
	stacks = append(stacks, polyStacks...)
	if len(stacks) == 0 {
		return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "no Go or polyglot toolchain module detected in scope"}, nil
	}
	return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: fmt.Sprintf("verified toolchains: %s", strings.Join(stacks, ", "))}, nil
}

func evaluateOrReusePolyglotToolchains(ctx *vscope.VerifyContext, root string, rec *polyrun.ToolchainSuiteReceipt) ([]string, *vscope.GateResult) {
	if rec != nil && rec.CacheHit && rec.PolyglotVerified && polyrun.CanUseToolchainReceiptCache(ctx) {
		return rec.PolyglotStacks, nil
	}
	polyStacks, polyFail := polyrun.EvaluatePolyglotToolchains(ctx, root)
	if polyFail != nil {
		return nil, polyFail
	}
	if rec != nil && rec.GoPassed && !rec.GoSkippedNonGo && rec.SnapshotSHA256 != "" {
		rec.PolyglotVerified = true
		rec.PolyglotStacks = polyStacks
		polyrun.TrySaveDiskReceipt(ctx, rec.SnapshotSHA256, rec)
	}
	return polyStacks, nil
}

func formatGoToolchainStackLabel(rec *polyrun.ToolchainSuiteReceipt) string {
	if rec != nil && rec.GoSkippedNonGo {
		return "Go(diff-scoped skip)"
	}
	if rec != nil && rec.CacheHit {
		return "Go(receipt cache)"
	}
	return "Go(go test -cover)"
}

func resolveGoTargetDir(root string) string {
	subDir := filepath.Join(root, "substrate")
	if fi, err := os.Stat(filepath.Join(subDir, "go.mod")); err == nil && !fi.IsDir() {
		return subDir
	}
	if fi, err := os.Stat(filepath.Join(root, "go.mod")); err == nil && !fi.IsDir() {
		return root
	}
	return ""
}
