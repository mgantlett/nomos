// Package verify implements sub-10ms polyglot CWE sink detection, intra-package
// Type-2 clone early-warning, and first-edit AST co-change hints for PostToolUse.
package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	dbExecSinkRe       = regexp.MustCompile(`(?i)\.(?:execute(?:many)?|Query(?:Row|Context)?|Exec(?:Context)?)\s*\(`)
	pyFStringSQLWhere  = regexp.MustCompile(`(?i)f["']\s*(?:SELECT|INSERT|UPDATE|DELETE)\b[^\n]*?\bWHERE\b[^\n]*?(?:=|\bLIKE\b|\bIN\b)\s*['"]?\{`)
	pyUnboundExecFStr  = regexp.MustCompile(`(?i)\.execute\s*\(\s*f["']\s*(?:INSERT|UPDATE|DELETE)\b[^\n"']*\{`)
	pyFormatConcatSQL  = regexp.MustCompile(`(?is)(?:SELECT|INSERT|UPDATE|DELETE)\b[^\n;]{0,160}(?:\.format\s*\(|%\s*\(|\+\s*[A-Za-z_])`)
	pyFStringOrderBy   = regexp.MustCompile(`(?i)f["']\s*SELECT\b[^\n]*?\bORDER\s+BY\s+\{`)
	sqlAllowlistHintRe = regexp.MustCompile(`(?i)(?:allow|valid|whitelist|permitted|\.get\s*\(|\bin\s*[\(\[\{])`)
	goSprintfSQLRe     = regexp.MustCompile(`(?is)fmt\.Sprintf\s*\(\s*["` + "`" + `]\s*(?:SELECT|INSERT|UPDATE|DELETE)\b[^"` + "`" + `]*?%[svq]`)

	baseDirParamRe    = regexp.MustCompile(`\b(?:base_dir|baseDir|root_dir|rootDir)\b`)
	fileReadSinkRe    = regexp.MustCompile(`(?:\.read_text\s*\(|\bopen\s*\(|\bos\.ReadFile\s*\(|\bfs\.readFileSync\s*\()`)
	safePathConfineRe = regexp.MustCompile(`(?:relative_to\s*\(|is_relative_to\s*\(|commonpath\s*\(|filepath\.Rel\s*\(|os\.sep|\+\s*["']/["'])`)
)

// DetectPolyglotCWESinks scans a non-test source file for CWE-89 dynamic SQL injection
// and CWE-22 unconfined base-directory path traversal sinks.
func DetectPolyglotCWESinks(relFile string, data []byte) []string {
	slashRel := filepath.ToSlash(relFile)
	if isCWESkippedFile(slashRel) {
		return nil
	}
	src := string(data)
	var hits []string
	if msg := detectCWE89SQLSink(src); msg != "" {
		hits = append(hits, msg)
	}
	if msg := detectCWE22PathSink(src); msg != "" {
		hits = append(hits, msg)
	}
	return hits
}

func isCWESkippedFile(slashRel string) bool {
	if impact.IsTestSourceFile(slashRel) || strings.Contains(slashRel, "tests_local/") || strings.Contains(slashRel, "/tests/") || strings.HasPrefix(slashRel, "tests/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(slashRel))
	return ext != ".py" && ext != ".go" && ext != ".js" && ext != ".ts"
}

func detectCWE89SQLSink(src string) string {
	if !dbExecSinkRe.MatchString(src) {
		return ""
	}
	if pyFStringSQLWhere.MatchString(src) || pyUnboundExecFStr.MatchString(src) || pyFormatConcatSQL.MatchString(src) || goSprintfSQLRe.MatchString(src) {
		return "CWE-89 dynamic SQL interpolation at execution sink; use '?' parameter binding and allowlist identifiers (SecurityGate)"
	}
	if pyFStringOrderBy.MatchString(src) && !sqlAllowlistHintRe.MatchString(src) {
		return "CWE-89 unvalidated dynamic ORDER BY identifier interpolation; map sort columns through a strict allowlist (SecurityGate)"
	}
	return ""
}

func detectCWE22PathSink(src string) string {
	if !baseDirParamRe.MatchString(src) || !fileReadSinkRe.MatchString(src) {
		return ""
	}
	if safePathConfineRe.MatchString(src) {
		return ""
	}
	return "CWE-22 unconfined path join/read from base_dir without canonical resolve + relative_to/filepath.Rel confinement (SecurityGate)"
}

func scanRatchetedCWELeaks(ctx *vscope.VerifyContext, wtRoot, absPath, relFile string, data []byte) []string {
	curHits := DetectPolyglotCWESinks(relFile, data)
	if len(curHits) == 0 || !vscope.IsBrownfieldRatchetActive(ctx) {
		return curHits
	}
	baseBytes, ok := loadBaselineFileBytes(ctx, wtRoot, absPath)
	if !ok {
		return curHits
	}
	baseSet := make(map[string]bool)
	for _, h := range DetectPolyglotCWESinks(relFile, baseBytes) {
		baseSet[h] = true
	}
	var novel []string
	for _, h := range curHits {
		if !baseSet[h] {
			novel = append(novel, h)
		}
	}
	return novel
}

func evaluateFastCWESinks(ctx *vscope.VerifyContext, wtRoot, absPath, relFile string, data []byte, res *FastFileRadarResult) {
	res.Breaches = append(res.Breaches, scanRatchetedCWELeaks(ctx, wtRoot, absPath, relFile, data)...)
}

func evaluateFastIntraPackageClones(ctx *vscope.VerifyContext, wtRoot, absPath string, data []byte, res *FastFileRadarResult) {
	if impact.IsTestSourceFile(res.File) {
		return
	}
	pkgDir := filepath.Dir(absPath)
	curMap, baseMap := loadSiblingPackageSources(ctx, wtRoot, pkgDir, absPath, res.File, data)
	if len(curMap) == 0 {
		return
	}
	curGroups := filterCloneGroupsForFile(astdebt.DetectStructuralClones(curMap, astdebt.DefaultMinCloneLines), res.File)
	if len(curGroups) == 0 {
		return
	}
	if len(baseMap) > 0 {
		baseGroups := filterCloneGroupsForFile(astdebt.DetectStructuralClones(baseMap, astdebt.DefaultMinCloneLines), res.File)
		if len(curGroups) <= len(baseGroups) {
			return
		}
	}
	formatted := astdebt.FormatCloneViolations(curGroups[:1])
	if len(formatted) > 0 {
		res.Breaches = append(res.Breaches, formatted[0]+" - extract shared helper before finish (CloneGate)")
	}
}

func loadSiblingPackageSources(ctx *vscope.VerifyContext, wtRoot, pkgDir, targetAbs, targetRel string, targetData []byte) (map[string][]byte, map[string][]byte) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil || len(entries) > 32 {
		return nil, nil
	}
	ratchet := vscope.IsBrownfieldRatchetActive(ctx)
	curMap := map[string][]byte{targetRel: targetData}
	var baseMap map[string][]byte
	if ratchet {
		baseMap = make(map[string][]byte)
		if bData, ok := loadBaselineFileBytes(ctx, wtRoot, targetAbs); ok {
			baseMap[targetRel] = bData
		}
	}
	for _, ent := range entries {
		appendSiblingFileBytes(ctx, wtRoot, targetAbs, ent, curMap, baseMap)
	}
	return curMap, baseMap
}

func appendSiblingFileBytes(ctx *vscope.VerifyContext, wtRoot, targetAbs string, ent os.DirEntry, curMap, baseMap map[string][]byte) {
	if ent.IsDir() {
		return
	}
	sibAbs := filepath.Join(filepath.Dir(targetAbs), ent.Name())
	if sibAbs == targetAbs || !vscope.IsSupportedPolyglotSourceFile(sibAbs) || impact.IsTestSourceFile(ent.Name()) {
		return
	}
	info, err := ent.Info()
	if err != nil || info.Size() > 512*1024 {
		return
	}
	sibRel := relativeOrFullPath(wtRoot, sibAbs)
	if sibBytes, rErr := os.ReadFile(sibAbs); rErr == nil {
		curMap[sibRel] = sibBytes
	}
	if baseMap != nil {
		if bBytes, ok := loadBaselineFileBytes(ctx, wtRoot, sibAbs); ok {
			baseMap[sibRel] = bBytes
		}
	}
}

func filterCloneGroupsForFile(groups []astdebt.CloneGroup, relFile string) []astdebt.CloneGroup {
	slashTarget := filepath.ToSlash(relFile)
	var matched []astdebt.CloneGroup
	for _, g := range groups {
		for _, occ := range g.Occurrences {
			if filepath.ToSlash(occ.File) == slashTarget {
				matched = append(matched, g)
				break
			}
		}
	}
	return matched
}

// ComputeFirstEditCoChangeHint builds a compact 1-line 1-hop impact and sibling interface hint
// strictly when multi-file coupling exists (remaining 100% silent on isolated standalone files).
func ComputeFirstEditCoChangeHint(wtRoot, relFile string) string {
	slashRel := filepath.ToSlash(relFile)
	if wtRoot == "" || slashRel == "" || impact.IsTestSourceFile(slashRel) || !vscope.IsSupportedPolyglotSourceFile(slashRel) {
		return ""
	}
	var parts []string
	for _, imp := range impact.ComputeWorkspaceImpact(wtRoot, []string{slashRel}) {
		if strings.Contains(imp, "dependent") || strings.Contains(imp, "importer") {
			parts = append(parts, imp)
		}
	}
	if strings.HasSuffix(slashRel, ".go") {
		if sibs := findGoPackageInterfacePeers(filepath.Join(wtRoot, filepath.Dir(slashRel)), filepath.Base(slashRel)); len(sibs) > 0 {
			parts = append(parts, slashRel+" -> package interface/type peers: "+strings.Join(sibs, ", "))
		}
	}
	return strings.Join(parts, " | ")
}

func findGoPackageInterfacePeers(pkgDir, editedBase string) []string {
	entries, err := os.ReadDir(pkgDir)
	if err != nil || len(entries) > 32 {
		return nil
	}
	fset := token.NewFileSet()
	var peers []string
	for _, ent := range entries {
		name := ent.Name()
		if ent.IsDir() || name == editedBase || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		node, pErr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if pErr != nil {
			continue
		}
		if hasGoInterfaceOrMethodReceiver(node) {
			peers = append(peers, name)
		}
	}
	sort.Strings(peers)
	return impact.LimitSlice(peers, 4)
}

func hasGoInterfaceOrMethodReceiver(node *ast.File) bool {
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil {
			return true
		}
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, isType := spec.(*ast.TypeSpec); isType {
				if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
					return true
				}
			}
		}
	}
	return false
}
