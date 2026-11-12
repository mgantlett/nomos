// Package astdebt provides Go AST contract-surface extraction for Twin parity checks.
package astdebt

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// GoSurfaceFingerprint captures CLI command/flag, YAML struct tag, and governance token sets (#GoSurfaceFingerprint).
type GoSurfaceFingerprint struct {
	CLITokens  map[string]bool
	YAMLFields map[string]bool
	GovTokens  map[string]bool
}

var cobraFlagVarMethods = map[string]int{
	"StringVar":      1,
	"StringVarP":     1,
	"BoolVar":        1,
	"BoolVarP":       1,
	"IntVar":         1,
	"IntVarP":        1,
	"Int64Var":       1,
	"Int64VarP":      1,
	"UintVar":        1,
	"UintVarP":       1,
	"Float64Var":     1,
	"Float64VarP":    1,
	"DurationVar":    1,
	"DurationVarP":   1,
	"StringSliceVar": 1,
	"StringArrayVar": 1,
	"CountVar":       1,
	"CountVarP":      1,
}

var cobraDirectFlagMethods = map[string]bool{
	"String":      true,
	"StringP":     true,
	"Bool":        true,
	"BoolP":       true,
	"Int":         true,
	"IntP":        true,
	"Duration":    true,
	"StringSlice": true,
}

// ParseGoSurfaceFingerprint executes deterministic astdebt verification logic (@intent tree_sitter_ast_gate_spec.md).
func ParseGoSurfaceFingerprint(src []byte) GoSurfaceFingerprint {
	fp := GoSurfaceFingerprint{
		CLITokens:  make(map[string]bool),
		YAMLFields: make(map[string]bool),
		GovTokens:  make(map[string]bool),
	}
	if len(src) == 0 {
		return fp
	}
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "surface.go", src, 0)
	if err != nil || node == nil {
		return fp
	}
	for _, decl := range node.Decls {
		if gd, ok := decl.(*ast.GenDecl); ok {
			inspectGenDeclSurface(gd, &fp)
		}
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CompositeLit:
			inspectCobraCommandLit(x, fp.CLITokens)
		case *ast.CallExpr:
			inspectSurfaceCallExpr(x, &fp)
		}
		return true
	})
	return fp
}

func inspectGenDeclSurface(gd *ast.GenDecl, fp *GoSurfaceFingerprint) {
	for _, spec := range gd.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if st, ok := s.Type.(*ast.StructType); ok {
				collectStructYAMLFields(s.Name.Name, st, fp.YAMLFields)
			}
		case *ast.ValueSpec:
			collectGovernanceValueSpec(s, fp.GovTokens)
		}
	}
}

func collectStructYAMLFields(typeName string, st *ast.StructType, out map[string]bool) {
	if st == nil || st.Fields == nil {
		return
	}
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		rawTag, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		yamlVal := reflect.StructTag(rawTag).Get("yaml")
		if yamlVal == "" {
			continue
		}
		yamlName := strings.TrimSpace(strings.Split(yamlVal, ",")[0])
		if yamlName == "" || yamlName == "-" {
			continue
		}
		out[typeName+"."+yamlName] = true
	}
}

func collectGovernanceValueSpec(vs *ast.ValueSpec, out map[string]bool) {
	for i, nameIdent := range vs.Names {
		if nameIdent.Name != "goldenSubjectSchema" || i >= len(vs.Values) {
			continue
		}
		call, ok := vs.Values[i].(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			continue
		}
		if pat, ok := extractStringLiteral(call.Args[0]); ok && pat != "" {
			out["commit_schema:"+pat] = true
		}
	}
}

func inspectCobraCommandLit(lit *ast.CompositeLit, cliOut map[string]bool) {
	if !isCobraCommandType(lit.Type) {
		return
	}
	for _, elt := range lit.Elts {
		if cmdName := extractUseKeyValue(elt); cmdName != "" {
			cliOut["cmd:"+cmdName] = true
		}
	}
}

func extractUseKeyValue(expr ast.Expr) string {
	kv, ok := expr.(*ast.KeyValueExpr)
	if !ok {
		return ""
	}
	keyIdent, ok := kv.Key.(*ast.Ident)
	if !ok || keyIdent.Name != "Use" {
		return ""
	}
	useVal, ok := extractStringLiteral(kv.Value)
	if !ok {
		return ""
	}
	fields := strings.Fields(useVal)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func inspectSurfaceCallExpr(call *ast.CallExpr, fp *GoSurfaceFingerprint) {
	if probeID := extractRegisterGateCall(call); probeID != "" {
		fp.GovTokens["gate:"+probeID] = true
		return
	}
	if flagName := extractCobraFlagCall(call); flagName != "" {
		fp.CLITokens["--"+flagName] = true
	}
}

func extractRegisterGateCall(call *ast.CallExpr) string {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || ident.Name != "RegisterGate" || len(call.Args) == 0 {
		return ""
	}
	if probeID, ok := extractStringLiteral(call.Args[0]); ok {
		return probeID
	}
	return ""
}

func extractCobraFlagCall(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return ""
	}
	if argIdx, isVar := cobraFlagVarMethods[sel.Sel.Name]; isVar && len(call.Args) > argIdx {
		if name, ok := extractStringLiteral(call.Args[argIdx]); ok {
			return name
		}
		return ""
	}
	if cobraDirectFlagMethods[sel.Sel.Name] && isFlagSetChain(sel.X) && len(call.Args) > 0 {
		if name, ok := extractStringLiteral(call.Args[0]); ok {
			return name
		}
	}
	return ""
}

func isFlagSetChain(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil {
		return false
	}
	switch sel.Sel.Name {
	case "Flags", "PersistentFlags", "LocalFlags":
		return true
	default:
		return false
	}
}

func extractStringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return val, true
}

// MergeSymmetricDiff executes deterministic astdebt verification logic (@intent tree_sitter_ast_gate_spec.md).
func MergeSymmetricDiff(dst, base, curr map[string]bool) {
	for k := range curr {
		if !base[k] {
			dst["+"+k] = true
		}
	}
	for k := range base {
		if !curr[k] {
			dst["-"+k] = true
		}
	}
}
