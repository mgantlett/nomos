// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// requiredCobraFields lists the mandatory self-documenting contract fields for every static CLI command.
var requiredCobraFields = []string{"Use", "Short", "Long", "Example"}

func scanCobraAliasesInDir(cliDir string) ([]string, error) {
	fset := token.NewFileSet()
	var aliases []string
	err := filepath.Walk(cliDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, 0)
		if pErr != nil {
			return nil
		}
		aliases = append(aliases, extractAliasesFromNode(node)...)
		return nil
	})
	return aliases, err
}

func extractAliasesFromNode(node *ast.File) []string {
	var aliases []string
	ast.Inspect(node, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, isIdent := kv.Key.(*ast.Ident)
		if isIdent && key.Name == "Aliases" {
			aliases = append(aliases, fmt.Sprintf("%v", kv.Value))
		}
		return true
	})
	return aliases
}

// ScanCobraSurfaceCompletenessInDir inspects CLI Go ASTs to ensure every static cobra.Command defines Use, Short, Long, and Example.
func ScanCobraSurfaceCompletenessInDir(cliDir string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.Walk(cliDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, 0)
		if pErr != nil {
			return nil
		}
		rel, rErr := filepath.Rel(cliDir, path)
		if rErr != nil {
			rel = filepath.Base(path)
		}
		ast.Inspect(node, func(n ast.Node) bool {
			comp, ok := n.(*ast.CompositeLit)
			if !ok || !isCobraCommandType(comp.Type) {
				return true
			}
			for _, missing := range ValidateCobraCommandCompositeLit(comp) {
				pos := fset.Position(comp.Pos())
				violations = append(violations, fmt.Sprintf("%s:%d (%s)", rel, pos.Line, missing))
			}
			return true
		})
		return nil
	})
	return violations, err
}

func isCobraCommandType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	return ok && pkgIdent.Name == "cobra" && sel.Sel.Name == "Command"
}

// ValidateCobraCommandCompositeLit returns missing or empty contract fields on a cobra.Command literal.
// Dynamic passthrough shims with DisableFlagParsing: true delegate help directly to external binaries.
func ValidateCobraCommandCompositeLit(comp *ast.CompositeLit) []string {
	if comp == nil {
		return nil
	}
	present := make(map[string]bool, len(comp.Elts))
	cmdUse := "<unnamed>"
	passthrough := false

	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if key.Name == "DisableFlagParsing" && isTrueIdent(kv.Value) {
			passthrough = true
		}
		if !isEmptyStringLiteral(kv.Value) {
			present[key.Name] = true
			if key.Name == "Use" {
				if lit, ok := kv.Value.(*ast.BasicLit); ok {
					cmdUse = strings.Trim(lit.Value, "\"`")
				}
			}
		}
	}

	if passthrough {
		return nil
	}

	var missing []string
	for _, req := range requiredCobraFields {
		if !present[req] {
			missing = append(missing, fmt.Sprintf("command %q missing non-empty %s", cmdUse, req))
		}
	}
	return missing
}

func isTrueIdent(expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == "true"
}

func isEmptyStringLiteral(expr ast.Expr) bool {
	if expr == nil {
		return true
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	return strings.TrimSpace(strings.Trim(lit.Value, "\"`")) == ""
}
