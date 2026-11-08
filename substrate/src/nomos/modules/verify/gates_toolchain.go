package verify

import (
	"fmt"
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

func (g *UncheckedErrorsGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	var violations []string
	fset := token.NewFileSet()

	err := WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
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
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("detected %d unchecked error discard(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:   "handle errors explicitly instead of discarding with blank identifier `_` (Law 7)",
		}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: "zero unchecked error suppressions detected"}, nil
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

// GoToolchainGate asserts clean compilation under the standard Go toolchain.
type GoToolchainGate struct{}

func (g *GoToolchainGate) Name() string { return "Go Toolchain Gate" }

func (g *GoToolchainGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if os.Getenv(envInToolchainGate) == "1" {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "re-entrant toolchain execution bypassed"}, nil
	}
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	targetDir := resolveGoTargetDir(root)
	var stacks []string
	var rec *ToolchainSuiteReceipt
	if targetDir != "" {
		rec = evaluateSharedGoSuite(ctx, root, targetDir)
		if !rec.GoPassed {
			return &GateResult{
				GateName: g.Name(),
				Passed:   false,
				Message:  fmt.Sprintf("Go compilation/test failed: %s", truncateToolchainOutput(rec.GoOutput)),
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
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no Go or polyglot toolchain module detected in scope"}, nil
	}
	return &GateResult{GateName: g.Name(), Passed: true, Message: fmt.Sprintf("verified toolchains: %s", strings.Join(stacks, ", "))}, nil
}

func evaluateOrReusePolyglotToolchains(ctx *VerifyContext, root string, rec *ToolchainSuiteReceipt) ([]string, *GateResult) {
	if rec != nil && rec.CacheHit && rec.PolyglotVerified && canUseToolchainReceiptCache(ctx) {
		return rec.PolyglotStacks, nil
	}
	polyStacks, polyFail := evaluatePolyglotToolchains(ctx, root)
	if polyFail != nil {
		return nil, polyFail
	}
	if rec != nil && rec.GoPassed && !rec.GoSkippedNonGo && rec.SnapshotSHA256 != "" {
		rec.PolyglotVerified = true
		rec.PolyglotStacks = polyStacks
		trySaveDiskReceipt(ctx, rec.SnapshotSHA256, rec)
	}
	return polyStacks, nil
}

func formatGoToolchainStackLabel(rec *ToolchainSuiteReceipt) string {
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
