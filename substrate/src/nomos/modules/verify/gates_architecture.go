// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func init() {
	RegisterGate("boundary", func() Gate { return &BoundaryGate{} })
	RegisterGate("ecosystem_drift", func() Gate { return &EcosystemDriftGate{} })
	RegisterGate("zero_alias", func() Gate { return &ZeroAliasGate{} })
	RegisterGate("magic_strings", func() Gate { return &MagicStringsGate{} })
	RegisterGate("magic_string_duplication", func() Gate { return &MagicStringDuplicationGate{} })
}

// BoundaryGate enforces open-core edition boundaries (Cell 1.1, Law 1).
type BoundaryGate struct{}

// Name returns the canonical display name.
func (g *BoundaryGate) Name() string { return "Open-Core Boundary Invariant (Charter §4)" }

// Evaluate checks whether boundary policies are enforced and audits prohibited capabilities.
func (g *BoundaryGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveArchRoot(ctx)
	cfg := resolveVerifyConfig(ctx)
	if !cfg.Governance.Boundary.Enforce {
		return &GateResult{
			GateName: g.Name(), ProbeID: "boundary", Law: 1, Ring: RingSyntactic,
			Passed: true, Message: "boundary not enforced for this repository (no policy opt-in)",
		}, nil
	}

	violations := scanBoundaryLeaks(root, cfg.Governance.Boundary.ProhibitedPackages)
	violations = append(violations, scanProhibitedCommands(root, cfg.Governance.Boundary.ProhibitedCommands)...)
	violations = append(violations, scanLockedDependencyMutations(ctx, root, cfg.Governance.Boundary.LockDependencies)...)
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "boundary", Law: 1, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d boundary policy violation(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:  "revert prohibited package, command, or locked dependency manifest mutations",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "boundary", Law: 1, Ring: RingSyntactic,
		Passed: true, Message: "zero prohibited packages or commands detected",
	}, nil
}

func scanBoundaryLeaks(root string, prohibited []string) []string {
	if len(prohibited) == 0 {
		return nil
	}
	fset := token.NewFileSet()
	var violations []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if pErr != nil {
			return nil
		}
		for _, imp := range node.Imports {
			val := strings.Trim(imp.Path.Value, "\"")
			for _, p := range prohibited {
				if val == p || strings.HasPrefix(val, p+"/") {
					violations = append(violations, fmt.Sprintf("%s: imports %s", path, val))
				}
			}
		}
		return nil
	})
	return violations
}

// EcosystemDriftGate asserts zero uncommitted trunk mutations (Cell 3.2, Law 3).
type EcosystemDriftGate struct{}

// Name returns the canonical display name.
func (g *EcosystemDriftGate) Name() string {
	return "Ecosystem Cross-Repo Drift Invariant (Law 3 & Law 7)"
}

// Evaluate verifies there are no uncommitted changes on trunk.
func (g *EcosystemDriftGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveArchRoot(ctx)
	if isWorktreePath(root) {
		return &GateResult{
			GateName: g.Name(), ProbeID: "ecosystem_drift", Law: 3, Ring: RingDynamic,
			Passed: true, Message: "transient worktree detected; skipping trunk drift check",
		}, nil
	}

	cmd := workspace.SafeGitCommand(root, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var dirty []string
	for _, l := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(l)
		if len(trimmed) < 3 {
			continue
		}
		clean := filepath.Clean(strings.TrimSpace(trimmed[2:]))
		if !strings.HasPrefix(clean, "worktrees") && !strings.HasPrefix(clean, ".tmp") {
			dirty = append(dirty, clean)
		}
	}

	if len(dirty) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "ecosystem_drift", Law: 3, Ring: RingDynamic,
			Passed:  false,
			Message: fmt.Sprintf("uncommitted trunk drift detected (%d files): %s", len(dirty), strings.Join(dirty, ", ")),
			Remedy:  "discard unwanted trunk edits via 'git restore <file>' or isolate mutations via 'nomos task chore'",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "ecosystem_drift", Law: 3, Ring: RingDynamic,
		Passed: true, Message: "zero uncommitted trunk drift detected",
	}, nil
}

func isWorktreePath(path string) bool {
	clean := filepath.ToSlash(path)
	if abs, err := filepath.Abs(path); err == nil {
		clean = filepath.ToSlash(abs)
	}
	return strings.Contains(clean, "/worktrees/") || strings.HasPrefix(clean, "worktrees/")
}

// ZeroAliasGate verifies that the CLI command tree contains zero aliases (Cell 6.2, Law 6).
type ZeroAliasGate struct{}

// Name returns the canonical display name.
func (g *ZeroAliasGate) Name() string { return "CLI Zero-Alias & Deprecation Invariant" }

// Evaluate inspects CLI command tree to verify canonical singularity.
func (g *ZeroAliasGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveArchRoot(ctx)
	cliDir := resolveNomosInternalCLIDir(ctx, root)
	if cliDir == "" {
		return &GateResult{
			GateName: g.Name(), ProbeID: "zero_alias", Law: 6, Ring: RingDynamic,
			Passed: true, Message: "no CLI directory detected; zero-alias gate satisfied",
		}, nil
	}

	aliases, err := scanCobraAliasesInDir(cliDir)
	if err != nil {
		return nil, err
	}
	if len(aliases) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "zero_alias", Law: 6, Ring: RingDynamic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d prohibited command alias(es): %s", len(aliases), strings.Join(aliases, ", ")),
			Remedy:  "remove Aliases from cobra.Command declarations under Law 6 (Canonical Singularity)",
		}, nil
	}

	surfaceViolations, sErr := ScanCobraSurfaceCompletenessInDir(cliDir)
	if sErr != nil {
		return nil, sErr
	}
	if len(surfaceViolations) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "zero_alias", Law: 6, Ring: RingDynamic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d incomplete CLI command surface contract(s): %s", len(surfaceViolations), strings.Join(surfaceViolations, "; ")),
			Remedy:  "equip every static cobra.Command with non-empty Use, Short, Long, and Example fields",
		}, nil
	}

	if typeAliases := scanGoTypeAliases(root); len(typeAliases) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "zero_alias", Law: 6, Ring: RingDynamic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d prohibited Go type alias(es) (type X = Y): %s", len(typeAliases), strings.Join(typeAliases, ", ")),
			Remedy:  "replace Go type aliases (type X = Y) with canonical single types under Law 6",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "zero_alias", Law: 6, Ring: RingDynamic,
		Passed: true, Message: "zero aliases and 100% Cobra help/example surface completeness verified (Law 6)",
	}, nil
}

// MagicStringsGate scans polyglot source code for hardcoded ports and URLs (Cell 5.1, Law 5).
type MagicStringsGate struct{}

// Name returns the canonical display name.
func (g *MagicStringsGate) Name() string {
	return "Magic Strings & Topology Hardcoding Gate (Law 5)"
}

// Evaluate checks source files for hardcoded network literals.
func (g *MagicStringsGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveArchRoot(ctx)
	targetDir := findSubstrateSrc(root)
	if targetDir == "" {
		return &GateResult{
			GateName: g.Name(), ProbeID: "magic_strings", Law: 5, Ring: RingSyntactic,
			Passed: true, Message: "no substrate source directory detected; skipping",
		}, nil
	}

	rawPortRegex := regexp.MustCompile(`['"]:(?:[1-9][0-9]{3,4})['"]`)
	localhostRegex := regexp.MustCompile(`https?://(?:localhost|127\.0\.0\.1):[0-9]{4,5}`)

	var findings []string
	WalkTargetFiles(ctx, targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isPolyglotTargetFile(path) && !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rErr := os.ReadFile(path)
		if rErr != nil {
			return nil
		}
		content := string(data)
		if rawPortRegex.MatchString(content) || localhostRegex.MatchString(content) {
			if baseBytes, ok := loadBaselineFileBytes(ctx, targetDir, path); ok {
				baseStr := string(baseBytes)
				if rawPortRegex.MatchString(baseStr) || localhostRegex.MatchString(baseStr) {
					return nil
				}
			}
			rel, rErr := filepath.Rel(targetDir, path)
			if rErr != nil {
				rel = path
			}
			findings = append(findings, rel)
		}
		return nil
	})

	if len(findings) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "magic_strings", Law: 5, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected hardcoded network literals in %d files: %s", len(findings), strings.Join(findings, ", ")),
			Remedy:  "resolve topology mappings and ports dynamically via configuration",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "magic_strings", Law: 5, Ring: RingSyntactic,
		Passed: true, Message: "zero magic strings or hardcoded topology mappings detected",
	}, nil
}

// MagicStringDuplicationGate detects repeated string literals in code (Cell 5.1, Law 5).
type MagicStringDuplicationGate struct{}

// Name returns the canonical display name.
func (g *MagicStringDuplicationGate) Name() string { return "Magic String Duplication Gate" }

// Evaluate scans AST for string literals repeated > 2 times within packages.
func (g *MagicStringDuplicationGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveArchRoot(ctx)
	targetDir := findSubstrateSrc(root)
	if targetDir == "" {
		return &GateResult{
			GateName: g.Name(), ProbeID: "magic_string_duplication", Law: 5, Ring: RingSyntactic,
			Passed: true, Message: "no substrate source directory detected; skipping",
		}, nil
	}

	counts := make(map[string]map[string]int)
	baseCounts := make(map[string]map[string]int)
	fset := token.NewFileSet()
	WalkTargetFiles(ctx, targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !isPolyglotTargetFile(path) {
			return nil
		}
		collectBaselineStringLiterals(ctx, targetDir, path, fset, baseCounts)
		if !strings.HasSuffix(path, ".go") {
			collectPolyglotStringLiterals(path, counts)
			return nil
		}
		collectGoStringLiterals(fset, path, counts)
		return nil
	})

	maxDup := resolveThresholds(ctx).MaxMagicStringDuplication
	var duplicates []string
	for pkg, m := range counts {
		for str, cnt := range m {
			ceil := maxInt(maxDup, baseCounts[pkg][str])
			if cnt > ceil {
				duplicates = append(duplicates, fmt.Sprintf("%s: %q repeated %d times", pkg, str, cnt))
			}
		}
	}

	if len(duplicates) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "magic_string_duplication", Law: 5, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d magic string duplications:\n    - %s", len(duplicates), strings.Join(duplicates, "\n    - ")),
			Remedy:  "extract duplicated strings into domain constants",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "magic_string_duplication", Law: 5, Ring: RingSyntactic,
		Passed: true, Message: "zero magic string duplications detected",
	}, nil
}

func isIgnoredLiteral(val string) bool {
	if len(val) <= 3 {
		return true
	}
	if strings.Contains(val, "%") || strings.HasPrefix(val, "http") || strings.HasPrefix(val, "SELECT") {
		return true
	}
	if strings.Contains(val, "/") || strings.Contains(val, "\n") || strings.Contains(val, `\n`) || strings.Contains(val, "- ") || strings.HasSuffix(val, ".go") || strings.Contains(val, `:"`) {
		return true
	}
	common := map[string]bool{
		"intent": true, "substrate": true, "main": true, "REVIEW": true,
		"EDIT": true, "PLAN": true, "IDLE": true, "SYNC": true, "DONE": true,
		"strings": true, "status": true, "error": true, "nomos": true,
		"message": true, "id": true, "data": true,
		"type": true, "name": true, "json": true, "ok": true, "text": true,
	}
	return common[val]
}

func resolveArchRoot(ctx *VerifyContext) string {
	if ctx != nil && ctx.WorktreePath != "" {
		return ctx.WorktreePath
	}
	if ctx != nil && ctx.RepoRoot != "" {
		return ctx.RepoRoot
	}
	return "."
}
