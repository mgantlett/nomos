// Package impact provides AST impact, boundary, and bijective specification contract analyzers.
package impact

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var goFenceBlockRe = regexp.MustCompile("(?s)```go\\s*\\n(.*?)```")

// SpecStructKey identifies a parsed Go struct inside a specification block (#ContractsConfig).
type SpecStructKey struct {
	Pkg  string
	Name string
}

// CompiledStructIndex indexes exported Go struct fields and canonical types by package and type name (#ContractsConfig).
type CompiledStructIndex struct {
	ByPkgType    map[string]map[string]string
	TypePackages map[string][]string
}

// CollectCompiledStructIndex walks scanDir and indexes exported Go struct fields and canonical types.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func CollectCompiledStructIndex(scanDir string) *CompiledStructIndex {
	idx := &CompiledStructIndex{
		ByPkgType:    make(map[string]map[string]string),
		TypePackages: make(map[string][]string),
	}
	cleanRoot := filepath.Clean(scanDir)
	fset := token.NewFileSet()
	walkErr := filepath.Walk(cleanRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if filepath.Clean(path) != cleanRoot && IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isScannableGoSourceFile(path) {
			if node, pErr := parser.ParseFile(fset, path, nil, 0); pErr == nil {
				recordIndexedASTStructs(node, idx)
			}
		}
		return nil
	})
	if walkErr != nil {
		return idx
	}
	return idx
}

func isScannableGoSourceFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func recordIndexedASTStructs(node *ast.File, idx *CompiledStructIndex) {
	pkgName := ""
	if node.Name != nil {
		pkgName = node.Name.Name
	}
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || !ts.Name.IsExported() {
				continue
			}
			if st, isStruct := ts.Type.(*ast.StructType); isStruct && st.Fields != nil {
				addStructToIndex(idx, pkgName, ts.Name.Name, st)
			}
		}
	}
}

func addStructToIndex(idx *CompiledStructIndex, pkgName, typeName string, st *ast.StructType) {
	qual := pkgName + "." + typeName
	fields := idx.ByPkgType[qual]
	if fields == nil {
		fields = make(map[string]string)
		idx.ByPkgType[qual] = fields
		appendUniquePkg(idx, typeName, pkgName)
	}
	extractExportedStructFields(st, fields)
}

func appendUniquePkg(idx *CompiledStructIndex, typeName, pkgName string) {
	for _, existing := range idx.TypePackages[typeName] {
		if existing == pkgName {
			return
		}
	}
	idx.TypePackages[typeName] = append(idx.TypePackages[typeName], pkgName)
	sort.Strings(idx.TypePackages[typeName])
}

// ExtractSpecStructIndex extracts package-qualified Go struct fields and canonical types from fenced go blocks.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func ExtractSpecStructIndex(markdown string) map[SpecStructKey]map[string]string {
	out := make(map[SpecStructKey]map[string]string)
	fset := token.NewFileSet()
	for _, match := range goFenceBlockRe.FindAllStringSubmatch(markdown, -1) {
		if len(match) < 2 {
			continue
		}
		parseSingleGoSpecFence(fset, match[1], out)
	}
	return out
}

func parseSingleGoSpecFence(fset *token.FileSet, rawBlock string, out map[SpecStructKey]map[string]string) {
	src := strings.TrimSpace(rawBlock)
	explicitPkg := strings.HasPrefix(src, "package ")
	if !explicitPkg {
		src = "package specstub\n" + src
	}
	node, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return
	}
	pkgName := ""
	if explicitPkg && node.Name != nil {
		pkgName = node.Name.Name
	}
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		recordSpecGenDeclStructs(pkgName, gd, out)
	}
}

func recordSpecGenDeclStructs(pkgName string, gd *ast.GenDecl, out map[SpecStructKey]map[string]string) {
	for _, spec := range gd.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name == nil || !ts.Name.IsExported() {
			continue
		}
		st, isStruct := ts.Type.(*ast.StructType)
		if !isStruct || st.Fields == nil {
			continue
		}
		key := SpecStructKey{Pkg: pkgName, Name: ts.Name.Name}
		if out[key] == nil {
			out[key] = make(map[string]string)
		}
		extractExportedStructFields(st, out[key])
	}
}

func extractExportedStructFields(st *ast.StructType, out map[string]string) {
	for _, f := range st.Fields.List {
		canonType := FormatCanonicalASTType(f.Type)
		for _, ident := range f.Names {
			if ident != nil && ident.IsExported() {
				out[ident.Name] = canonType
			}
		}
	}
}

// FormatCanonicalASTType renders a package-qualifier-normalized Go type signature for parity checks.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func FormatCanonicalASTType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		if t.Name == "interface{}" {
			return "any"
		}
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return "*" + FormatCanonicalASTType(t.X)
	case *ast.ArrayType:
		return "[]" + FormatCanonicalASTType(t.Elt)
	case *ast.MapType:
		return "map[" + FormatCanonicalASTType(t.Key) + "]" + FormatCanonicalASTType(t.Value)
	case *ast.InterfaceType:
		return "any"
	case *ast.ChanType:
		return "chan " + FormatCanonicalASTType(t.Value)
	case *ast.FuncType:
		return "func"
	default:
		return "any"
	}
}

// LookupFields resolves compiled fields for (pkgName, typeName) with unique-type fallback across subpackages.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func (idx *CompiledStructIndex) LookupFields(pkgName, typeName string) (map[string]string, bool) {
	if idx == nil {
		return nil, false
	}
	if pkgName != "" {
		if exact, ok := idx.ByPkgType[pkgName+"."+typeName]; ok {
			return exact, true
		}
	}
	pkgs := idx.TypePackages[typeName]
	if len(pkgs) == 0 {
		return nil, false
	}
	if len(pkgs) == 1 {
		return idx.ByPkgType[pkgs[0]+"."+typeName], true
	}
	merged := make(map[string]string)
	for _, p := range pkgs {
		for k, v := range idx.ByPkgType[p+"."+typeName] {
			merged[k] = v
		}
	}
	return merged, true
}

// DiffSpecStructsBijective checks missing structs, missing fields, extra compiled fields, and field type drift.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func DiffSpecStructsBijective(relSpec string, specStructs map[SpecStructKey]map[string]string, compiled *CompiledStructIndex, delta *SiblingDelta) []string {
	keys := make([]SpecStructKey, 0, len(specStructs))
	for k := range specStructs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Name != keys[j].Name {
			return keys[i].Name < keys[j].Name
		}
		return keys[i].Pkg < keys[j].Pkg
	})
	var issues []string
	for _, key := range keys {
		issues = append(issues, diffOneSpecStruct(relSpec, key, specStructs[key], compiled, delta)...)
	}
	return issues
}

func diffOneSpecStruct(relSpec string, key SpecStructKey, expected map[string]string, compiled *CompiledStructIndex, delta *SiblingDelta) []string {
	actual, exists := compiled.LookupFields(key.Pkg, key.Name)
	if !exists {
		if delta.HasAddedStruct(key.Name) {
			return nil
		}
		return []string{fmt.Sprintf("%s: missing compiled Go struct %s", relSpec, key.Name)}
	}
	var issues []string
	for _, field := range sortedMapKeys(expected) {
		actType, has := actual[field]
		if !has {
			if !delta.HasAddedField(key.Name, field) {
				issues = append(issues, fmt.Sprintf("%s: struct %s missing declared field %s", relSpec, key.Name, field))
			}
			continue
		}
		if expected[field] != actType && !delta.HasAddedField(key.Name, field) {
			issues = append(issues, fmt.Sprintf("%s: struct %s field %s type drift (spec=%s, ast=%s)", relSpec, key.Name, field, expected[field], actType))
		}
	}
	for _, actField := range sortedMapKeys(actual) {
		if _, ok := expected[actField]; !ok {
			issues = append(issues, fmt.Sprintf("%s: struct %s has undocumented compiled field %s omitted from specification", relSpec, key.Name, actField))
		}
	}
	return issues
}

func sortedMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
