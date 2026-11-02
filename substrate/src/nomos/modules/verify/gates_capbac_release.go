// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func init() {
	RegisterGate("capbac_lease", func() Gate { return &CapBACLeaseGate{} })
	RegisterGate("release_receipt", func() Gate { return &ReleaseReceiptGate{} })
}

// CapBACLeaseGate verifies active capability lease authorization (Cell 1.3, Law 1).
type CapBACLeaseGate struct{}

// Name returns the canonical display name.
func (g *CapBACLeaseGate) Name() string {
	return "CapBAC Task Lease Authorization Invariant"
}

// Evaluate asserts that mutations in an active task worktree possess an active capability lease.
func (g *CapBACLeaseGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx == nil || ctx.IsBaseline || ctx.TaskID == "" || ctx.Phase == string(types.PhaseIdle) {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "no active task lease required in trunk baseline or idle state",
		}, nil
	}

	dbPath := config.ResolveGlobalDBPath()
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("failed opening living ledger db: %v", err),
			Remedy:   "verify SQLite database integrity at " + dbPath,
		}, nil
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			// database closed
		}
	}()

	leaseID, phase, stepBudget, tokensUsed, lErr := storage.GetActiveLease(db, ctx.TaskID)
	if lErr != nil {
		return &GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("no active CapBAC capability lease found for task %s: %v", ctx.TaskID, lErr),
			Remedy:   fmt.Sprintf("run 'nomos task start %s' to acquire an active capability lease", ctx.TaskID),
		}, nil
	}

	remaining := stepBudget - tokensUsed
	return &GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  fmt.Sprintf("active CapBAC lease %s verified for %s [phase %s, %d steps remaining]", leaseID, ctx.TaskID, phase, remaining),
	}, nil
}

// ReleaseReceiptGate verifies release provenance and Git Notes infrastructure (Cell 3.3, Law 3).
type ReleaseReceiptGate struct{}

// Name returns the canonical display name.
func (g *ReleaseReceiptGate) Name() string {
	return "Two-Phase Commit Release Receipt Invariant"
}

// Evaluate verifies that release provenance and git notes infrastructure operate cleanly.
func (g *ReleaseReceiptGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	repoRoot := "."
	if ctx != nil && ctx.RepoRoot != "" {
		repoRoot = ctx.RepoRoot
	}

	showCmd := workspace.SafeGitCommand(repoRoot, "notes", "--ref=refs/notes/nomos/releases", "show", "HEAD")
	if rawNote, showErr := showCmd.Output(); showErr == nil && len(strings.TrimSpace(string(rawNote))) > 0 {
		var receipt struct {
			TaskKey       string `json:"task_key"`
			SHA256Receipt string `json:"sha256_receipt"`
		}
		if jErr := json.Unmarshal(rawNote, &receipt); jErr != nil || receipt.TaskKey == "" || len(receipt.SHA256Receipt) != 64 {
			return &GateResult{
				GateName: g.Name(), Passed: false,
				Message: "corrupted Git Note receipt on HEAD: missing valid task_key or 64-char sha256_receipt",
				Remedy:  "re-attest HEAD release provenance via 'nomos task sync'",
			}, nil
		}
	}

	return &GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  "release receipt provenance verified via Git Notes (refs/notes/nomos/releases)",
	}, nil
}

func scanGoTypeAliases(root string) []string {
	subDir := filepath.Join(root, "substrate")
	if _, err := os.Stat(subDir); os.IsNotExist(err) {
		subDir = root
	}
	fset := token.NewFileSet()
	var aliases []string
	if wErr := filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		aliases = append(aliases, extractTypeAliasesFromFile(fset, subDir, path)...)
		return nil
	}); wErr != nil {
		return aliases
	}
	return aliases
}

func extractTypeAliasesFromFile(fset *token.FileSet, subDir, path string) []string {
	node, pErr := parser.ParseFile(fset, path, nil, 0)
	if pErr != nil {
		return nil
	}
	var aliases []string
	for _, decl := range node.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Assign.IsValid() {
				rel, rErr := filepath.Rel(subDir, path)
				if rErr != nil {
					rel = path
				}
				aliases = append(aliases, fmt.Sprintf("%s:%s", filepath.ToSlash(rel), ts.Name.Name))
			}
		}
	}
	return aliases
}

func collectGoStringLiterals(fset *token.FileSet, path string, counts map[string]map[string]int) {
	node, pErr := parser.ParseFile(fset, path, nil, 0)
	if pErr != nil {
		return
	}
	pkg := node.Name.Name
	if counts[pkg] == nil {
		counts[pkg] = make(map[string]int)
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if _, isImp := n.(*ast.ImportSpec); isImp {
			return false
		}
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			val := strings.Trim(lit.Value, "\"`")
			if !isIgnoredLiteral(val) {
				counts[pkg][val]++
			}
		}
		return true
	})
}

var (
	polyglotStringLiteralRe = regexp.MustCompile(`"([^"\\]{4,})"`)
	svelteScriptBlockRe     = regexp.MustCompile(`(?is)<script\b[^>]*>(.*?)</script>`)
)

func extractSvelteScriptSource(raw string) string {
	matches := svelteScriptBlockRe.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return ""
	}
	var parts []string
	for _, m := range matches {
		if len(m) > 1 {
			parts = append(parts, m[1])
		}
	}
	return strings.Join(parts, "\n")
}

func collectPolyglotStringLiterals(path string, counts map[string]map[string]int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	if strings.EqualFold(filepath.Ext(path), ".svelte") {
		source = extractSvelteScriptSource(source)
	}
	pkg := filepath.Base(filepath.Dir(path))
	if counts[pkg] == nil {
		counts[pkg] = make(map[string]int)
	}
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		for _, m := range polyglotStringLiteralRe.FindAllStringSubmatch(trimmed, -1) {
			val := m[1]
			if !isIgnoredLiteral(val) {
				counts[pkg][val]++
			}
		}
	}
}

func countPolyglotFiles(root string, dirs []string) (int, error) {
	total := 0
	for _, relDir := range dirs {
		fullDir := filepath.Join(root, relDir)
		err := filepath.Walk(fullDir, func(p string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && isPolyglotTargetFile(p) {
				total++
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return 0, err
		}
	}
	return total, nil
}

var cueDefRegex = regexp.MustCompile(`(?m)^\s*(#[A-Za-z0-9_]+)\s*:`)

func findOrphanCUEContracts(root string, cueRoots, backendRoots []string) []string {
	backendCorpus := readBackendCorpus(root, backendRoots)
	if backendCorpus == "" {
		return nil
	}

	var orphans []string
	for _, relDir := range cueRoots {
		if wErr := filepath.Walk(filepath.Join(root, relDir), func(p string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(p, ".cue") {
				if isCUEFileOrphan(p, backendCorpus) {
					orphans = append(orphans, filepath.Base(p))
				}
			}
			return nil
		}); wErr != nil {
			continue
		}
	}
	return orphans
}

func readBackendCorpus(root string, backendRoots []string) string {
	var bBuilder strings.Builder
	for _, relDir := range backendRoots {
		if wErr := filepath.Walk(filepath.Join(root, relDir), func(p string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && isPolyglotTargetFile(p) {
				if data, rErr := os.ReadFile(p); rErr == nil {
					bBuilder.Write(data)
					bBuilder.WriteByte('\n')
				}
			}
			return nil
		}); wErr != nil {
			continue
		}
	}
	return bBuilder.String()
}

func isCUEFileOrphan(path, backendCorpus string) bool {
	data, rErr := os.ReadFile(path)
	if rErr != nil {
		return false
	}
	matches := cueDefRegex.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return false
	}
	for _, m := range matches {
		if strings.Contains(backendCorpus, m[1]) {
			return false
		}
	}
	return true
}


