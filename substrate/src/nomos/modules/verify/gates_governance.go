// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"bufio"
	"fmt"
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

func init() {
	RegisterGate("forbidden_literals", func() vscope.Gate { return &ForbiddenLiteralsGate{} })
	RegisterGate("dead_code", func() vscope.Gate { return &DeadCodeGate{} })
	RegisterGate("review_receipt", func() vscope.Gate { return &ReviewReceiptGate{} })
}

// ForbiddenLiteralsGate verifies that no hardcoded paths exist in source files.
type ForbiddenLiteralsGate struct{}

// Name returns the canonical display name of the forbidden literals gate.
func (g *ForbiddenLiteralsGate) Name() string { return "Forbidden Literals Invariant" }

// Evaluate scans source files against forbidden path and IP literal patterns.
func (g *ForbiddenLiteralsGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := ctx.WorktreePath
	if root == "" {
		root = ctx.RepoRoot
	}
	cfg := vscope.ResolveVerifyConfig(ctx)
	patterns := cfg.Governance.QualityGates.ForbiddenLiterals
	if len(patterns) == 0 && !cfg.Governance.QualityGates.HasForbiddenLiteralsOverride {
		patterns = defaultForbiddenLiterals()
	}
	if len(patterns) == 0 {
		return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "forbidden literal scanning disabled via empty configuration"}, nil
	}

	cleanPatterns := make([]string, 0, len(patterns))
	for _, p := range patterns {
		cleaned := strings.Trim(p, "\"'")
		if len(cleaned) > 0 {
			cleanPatterns = append(cleanPatterns, cleaned)
		}
	}

	var violations []string
	err := vscope.WalkTargetFiles(ctx, root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !isScannableSourceFile(path) {
			return nil
		}
		cur := scanFileForLiterals(root, path, cleanPatterns)
		cur = filterRatchetedForbiddenLiterals(ctx, root, path, cleanPatterns, cur)
		violations = append(violations, cur...)
		return nil
	})
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "forbidden literals walk failed: %v", err)
	}

	if len(violations) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   false,
			Message:  fmt.Sprintf("detected %d forbidden literal violation(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:   "remove hardcoded filesystem paths; resolve dynamically via config.LoadConfig() (Law 5)",
		}, nil
	}
	return &vscope.GateResult{GateName: g.Name(), Passed: true, Message: "zero forbidden literals detected"}, nil
}

func defaultForbiddenLiterals() []string {
	return []string{
		"/home/", "/Users/", "/var/home/",
	}
}

func isScannableSourceFile(path string) bool {
	if !isPolyglotTargetFile(path) {
		return false
	}
	if strings.HasSuffix(path, "gates_governance.go") {
		return false
	}
	return true
}

func scanFileForLiterals(root, path string, patterns []string) []string {
	rel, rErr := filepath.Rel(root, path)
	if rErr != nil {
		rel = path
	}
	if !strings.HasSuffix(path, ".go") {
		return scanPolyglotFileForLiterals(path, rel, patterns)
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}

	var found []string
	ast.Inspect(node, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val := strings.Trim(lit.Value, "`\"")
		for _, p := range patterns {
			if strings.Contains(val, p) {
				pos := fset.Position(lit.Pos())
				found = append(found, fmt.Sprintf("%s:%d contains forbidden literal %q", rel, pos.Line, p))
			}
		}
		return true
	})
	return found
}

func scanPolyglotFileForLiterals(path, rel string, patterns []string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var found []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, p := range patterns {
			if strings.Contains(line, p) {
				found = append(found, fmt.Sprintf("%s:%d contains forbidden literal %q", rel, lineNum, p))
			}
		}
	}
	return found
}

const probeIDDeadCode = "dead_code"

// DeadCodeGate verifies reachable CLI commands, canonical gate registration, and package imports (Cell 2.1, Law 2).
type DeadCodeGate struct{}

// Name returns the canonical display name.
func (g *DeadCodeGate) Name() string { return "Dead-Code Reachability Gate (Law 2)" }

// Evaluate inspects CLI command registrations, gate registry parity, and internal package imports.
func (g *DeadCodeGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	if unexecuted := findUnexecutedRegisteredGates(); len(unexecuted) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d registered gate(s) excluded from CanonicalGateOrder: %s", len(unexecuted), strings.Join(unexecuted, ", ")),
			Remedy:  "wire gate IDs into CanonicalGateOrder or remove phantom gate registrations",
		}, nil
	}

	root := resolveQualityRoot(ctx)
	if orphanPkgs := findUnimportedInternalPackages(root); len(orphanPkgs) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d unimported internal Go package(s): %s", len(orphanPkgs), strings.Join(orphanPkgs, ", ")),
			Remedy:  "remove orphan Go packages or import them into active substrate modules",
		}, nil
	}
	if orphanMods := findUnimportedPolyglotModules(ctx, root); len(orphanMods) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d unimported internal polyglot module(s): %s", len(orphanMods), strings.Join(orphanMods, ", ")),
			Remedy:  "remove orphan polyglot modules or import them into active modules",
		}, nil
	}

	cliDir := polyrun.ResolveNomosInternalCLIDir(ctx, root)
	if cliDir == "" {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
			Passed: true, Message: "no CLI directory detected; skipping reachability check",
		}, nil
	}

	unreachable := findUnreachableCommands(cliDir)
	if len(unreachable) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d unreachable command constructors: %s", len(unreachable), strings.Join(unreachable, ", ")),
			Remedy:  "wire command constructors into root command tree in root.go",
		}, nil
	}

	return &vscope.GateResult{
		GateName: g.Name(), ProbeID: probeIDDeadCode, Law: 2, Ring: RingSyntactic,
		Passed: true, Message: "all CLI command constructors, registered gates, and internal packages are reachable",
	}, nil
}

func findUnexecutedRegisteredGates() []string {
	canonical := make(map[string]bool, len(CanonicalGateOrder))
	for _, id := range CanonicalGateOrder {
		canonical[id] = true
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	var unexecuted []string
	for id := range gateRegistry {
		if !canonical[id] {
			unexecuted = append(unexecuted, id)
		}
	}
	return unexecuted
}

func findUnimportedInternalPackages(root string) []string {
	subDir := filepath.Join(root, "substrate")
	srcDir := filepath.Join(subDir, "src")
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	imported := collectAllImports(subDir)
	var orphans []string
	if wErr := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.IsDir() {
			return nil
		}
		hasGo, hasTest := inspectPackageFiles(path)
		if !hasGo && !hasTest {
			return nil
		}
		rel, rErr := filepath.Rel(subDir, path)
		if rErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if !imported[relSlash] {
			orphans = append(orphans, relSlash)
		}
		return nil
	}); wErr != nil {
		return orphans
	}
	return orphans
}

func collectAllImports(subDir string) map[string]bool {
	imported := make(map[string]bool)
	fset := token.NewFileSet()
	if wErr := filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if pErr != nil {
			return nil
		}
		for _, imp := range node.Imports {
			val := strings.Trim(imp.Path.Value, `"`)
			if idx := strings.Index(val, "/substrate/"); idx >= 0 {
				imported[val[idx+len("/substrate/"):]] = true
			}
		}
		return nil
	}); wErr != nil {
		return imported
	}
	return imported
}

func findUnreachableCommands(cliDir string) []string {
	fset := token.NewFileSet()
	registered := collectRootRegisteredCommands(filepath.Join(cliDir, "root.go"), fset)
	if registered == nil {
		return nil
	}

	entries, rErr := os.ReadDir(cliDir)
	if rErr != nil {
		return nil
	}

	var unreachable []string
	for _, e := range entries {
		unreachable = append(unreachable, checkEntryUnreachable(cliDir, e, fset, registered)...)
	}
	return unreachable
}

func collectRootRegisteredCommands(rootFile string, fset *token.FileSet) map[string]bool {
	rootNode, err := parser.ParseFile(fset, rootFile, nil, 0)
	if err != nil {
		return nil
	}
	registered := make(map[string]bool)
	ast.Inspect(rootNode, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok {
				registered[ident.Name] = true
			}
		}
		return true
	})
	return registered
}

func checkEntryUnreachable(cliDir string, e os.DirEntry, fset *token.FileSet, registered map[string]bool) []string {
	if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") || e.Name() == "root.go" {
		return nil
	}
	node, pErr := parser.ParseFile(fset, filepath.Join(cliDir, e.Name()), nil, 0)
	if pErr != nil {
		return nil
	}
	var unreachable []string
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && strings.HasPrefix(fn.Name.Name, "New") && strings.HasSuffix(fn.Name.Name, "Cmd") {
			if !registered[fn.Name.Name] {
				unreachable = append(unreachable, fn.Name.Name)
			}
		}
	}
	return unreachable
}

// ReviewReceiptGate verifies cryptographic proof or task review metadata before merge.
type ReviewReceiptGate struct{}

// Name returns the canonical display name of the review receipt gate.
func (g *ReviewReceiptGate) Name() string { return "Review Receipt Gate" }

// Evaluate asserts review readiness across lifecycle phases.
func (g *ReviewReceiptGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	phase := types.Phase(ctx.Phase)
	if phase == types.PhaseEdit || phase == types.PhasePlan || phase == types.PhaseIdle || ctx.Phase == "" {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  fmt.Sprintf("phase %s does not require pre-commit review receipts", ctx.Phase),
		}, nil
	}

	if ctx.TaskID != "" && ctx.WorktreePath != "" && phase == types.PhaseSync {
		walkthrough := filepath.Join(ctx.RepoRoot, ".tmp", "walkthrough.md")
		taskWalkthrough := filepath.Join(ctx.WorktreePath, ".tmp", "walkthrough.md")
		_, errRoot := os.Stat(walkthrough)
		_, errTask := os.Stat(taskWalkthrough)
		if os.IsNotExist(errRoot) && os.IsNotExist(errTask) {
			return &vscope.GateResult{
				GateName: g.Name(),
				Passed:   false,
				Message:  "missing required walkthrough receipt in .tmp/walkthrough.md for 2PC sync release",
				Remedy:   "generate release walkthrough before sync: nomos task sync -F <walkthrough-file>",
			}, nil
		}
	}

	return &vscope.GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  fmt.Sprintf("review receipts satisfied for phase %s", ctx.Phase),
	}, nil
}
