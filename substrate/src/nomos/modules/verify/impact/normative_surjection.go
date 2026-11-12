// Package impact provides orphaned test obligation and normative registry surjection AST checks.
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
	"strconv"
	"strings"
)

var (
	verifiesCommentRe = regexp.MustCompile(`@verifies\s+(TC-[A-Z0-9]+-[0-9]+)\b`)
	testFuncTCNameRe  = regexp.MustCompile(`^Test.*TC_([A-Z0-9]+)_([0-9]+)\b`)
	tableCaseTCNameRe = regexp.MustCompile(`^(TC-[A-Z0-9]+-[0-9]+)\b`)
	errConstCodeRe    = regexp.MustCompile(`^ERR_[A-Z0-9_]+$`)
)

// ScanOrphanedGoTestObligations parses *_test.go AST comments, TestTC_* functions, and table case names
// and returns TC-* obligations absent from declaredTCs.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func ScanOrphanedGoTestObligations(root string, declaredTCs map[string]bool) []string {
	fset := token.NewFileSet()
	var orphans []string
	seen := make(map[string]bool)
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, tc := range extractRealGoTestTCs(fset, path) {
			if !declaredTCs[tc] {
				item := fmt.Sprintf("%s -> %s", toRelSlashPath(root, path), tc)
				if !seen[item] {
					seen[item] = true
					orphans = append(orphans, item)
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		sort.Strings(orphans)
		return orphans
	}
	sort.Strings(orphans)
	return orphans
}

func extractRealGoTestTCs(fset *token.FileSet, path string) []string {
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	for _, cg := range node.Comments {
		for _, c := range cg.List {
			for _, m := range verifiesCommentRe.FindAllStringSubmatch(c.Text, -1) {
				if len(m) == 2 {
					seen[m[1]] = true
				}
			}
		}
	}
	ast.Inspect(node, func(n ast.Node) bool {
		collectASTTestNodeTCs(n, seen)
		return true
	})
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectASTTestNodeTCs(n ast.Node, seen map[string]bool) {
	switch x := n.(type) {
	case *ast.FuncDecl:
		if x.Name != nil {
			if m := testFuncTCNameRe.FindStringSubmatch(x.Name.Name); len(m) == 3 {
				seen[fmt.Sprintf("TC-%s-%s", m[1], m[2])] = true
			}
		}
	case *ast.KeyValueExpr:
		extractTableCaseKeyValueTC(x, seen)
	}
}

func extractTableCaseKeyValueTC(kv *ast.KeyValueExpr, seen map[string]bool) {
	ident, ok := kv.Key.(*ast.Ident)
	if !ok || (ident.Name != "name" && ident.Name != "Name") {
		return
	}
	if val, okStr := unquoteStringArg(kv.Value); okStr {
		if m := tableCaseTCNameRe.FindStringSubmatch(strings.TrimSpace(val)); len(m) == 2 {
			seen[m[1]] = true
		}
	}
}

// VerifyRepoNormativeSurjection checks that 100% of registered Gate ProbeIDs, core/config structs,
// Cobra CLI --flags, and ERR_* error codes in root/substrate appear across specText.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
func VerifyRepoNormativeSurjection(root, specText string) []string {
	var issues []string
	for _, probeID := range scanGateProbeIDs(filepath.Join(root, "substrate", "src", "nomos", "modules", "verify")) {
		if !strings.Contains(specText, probeID) {
			issues = append(issues, fmt.Sprintf("undocumented gate ProbeID %q", probeID))
		}
	}
	for _, cfgStruct := range scanExportedStructNames(filepath.Join(root, "substrate", "src", "nomos", "core", "config")) {
		if !strings.Contains(specText, "type "+cfgStruct+" struct") {
			issues = append(issues, fmt.Sprintf("undocumented core/config struct %s", cfgStruct))
		}
	}
	for _, flagName := range scanCobraCLIFlags(filepath.Join(root, "substrate", "cli")) {
		if !containsCLIFlagToken(specText, flagName) {
			issues = append(issues, fmt.Sprintf("undocumented CLI flag %s", flagName))
		}
	}
	for _, errCode := range scanLiveErrCodes(filepath.Join(root, "substrate")) {
		if !strings.Contains(specText, errCode) {
			issues = append(issues, fmt.Sprintf("undocumented canonical error code %s", errCode))
		}
	}
	sort.Strings(issues)
	return issues
}

func containsCLIFlagToken(specText, flag string) bool {
	idx := 0
	for {
		pos := strings.Index(specText[idx:], flag)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		end := abs + len(flag)
		prevOK := abs == 0 || !isFlagIdentChar(specText[abs-1])
		nextOK := end == len(specText) || !isFlagIdentChar(specText[end])
		if prevOK && nextOK {
			return true
		}
		idx = abs + 1
	}
}

func isFlagIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-'
}

func scanGateProbeIDs(verifyDir string) []string {
	set := make(map[string]bool)
	walkNonTestAST(verifyDir, func(node *ast.File) {
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			if ident, isID := call.Fun.(*ast.Ident); isID && ident.Name == "RegisterGate" {
				if s, okStr := unquoteStringArg(call.Args[0]); okStr && s != "" {
					set[s] = true
				}
			}
			return true
		})
	})
	return sortedBoolKeys(set)
}

func scanExportedStructNames(configDir string) []string {
	set := make(map[string]bool)
	walkNonTestAST(configDir, func(node *ast.File) {
		for _, decl := range node.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, sp := range gd.Specs {
				if ts, isTS := sp.(*ast.TypeSpec); isTS && ts.Name != nil && ts.Name.IsExported() {
					if _, isSt := ts.Type.(*ast.StructType); isSt {
						set[ts.Name.Name] = true
					}
				}
			}
		}
	})
	return sortedBoolKeys(set)
}

func scanCobraCLIFlags(cliDir string) []string {
	set := make(map[string]bool)
	walkNonTestAST(cliDir, func(node *ast.File) {
		ast.Inspect(node, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name := extractCobraFlagCallName(call); name != "" {
				set["--"+name] = true
			}
			return true
		})
	})
	return sortedBoolKeys(set)
}

func extractCobraFlagCallName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return ""
	}
	recvCall, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return ""
	}
	recvSel, ok := recvCall.Fun.(*ast.SelectorExpr)
	if !ok || (recvSel.Sel.Name != "Flags" && recvSel.Sel.Name != "PersistentFlags") {
		return ""
	}
	argIdx := 0
	if strings.Contains(sel.Sel.Name, "Var") {
		argIdx = 1
	}
	if argIdx >= len(call.Args) {
		return ""
	}
	if s, okStr := unquoteStringArg(call.Args[argIdx]); okStr && s != "" {
		return s
	}
	return ""
}

func scanLiveErrCodes(subDir string) []string {
	set := make(map[string]bool)
	walkNonTestAST(subDir, func(node *ast.File) {
		ast.Inspect(node, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, okStr := unquoteStringArg(lit); okStr && errConstCodeRe.MatchString(s) {
				set[s] = true
			}
			return true
		})
	})
	return sortedBoolKeys(set)
}

func walkNonTestAST(dir string, visit func(*ast.File)) {
	cleanDir := filepath.Clean(dir)
	fi, err := os.Stat(cleanDir)
	if err != nil || !fi.IsDir() {
		return
	}
	fset := token.NewFileSet()
	walkErr := filepath.Walk(cleanDir, func(path string, info os.FileInfo, wErr error) error {
		if wErr != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if filepath.Clean(path) != cleanDir && IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isScannableGoSourceFile(path) {
			if node, pErr := parser.ParseFile(fset, path, nil, 0); pErr == nil {
				visit(node)
			}
		}
		return nil
	})
	if walkErr != nil {
		return
	}
}

func unquoteStringArg(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func sortedBoolKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func toRelSlashPath(root, fullPath string) string {
	if rel, err := filepath.Rel(root, fullPath); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(fullPath)
}
