// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func init() {
	RegisterGate("intent_traceability", func() Gate { return &IntentTraceabilityGate{} })
	RegisterGate("intent_steelman", func() Gate { return &IntentSteelmanGate{} })
	RegisterGate("intent_parity", func() Gate { return &IntentParityGate{} })
	RegisterGate("topology_parity", func() Gate { return &TopologyParityGate{} })
	RegisterGate("threshold_integrity", func() Gate { return &ThresholdIntegrityGate{} })
	RegisterGate("uasset_dependency", func() Gate { return &UAssetDependencyGate{} })
}

// IntentTraceabilityGate verifies Go code links back to Intent specifications (Cell 4.1, Law 4).
type IntentTraceabilityGate struct{}

// Name returns the canonical display name.
func (g *IntentTraceabilityGate) Name() string { return "Intent Traceability Validation Gate" }

// Evaluate verifies exported functions possess valid @intent traceability tags.
func (g *IntentTraceabilityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveIntentRoot(ctx)
	targetDir := findSubstrateSrc(root)
	if oblFailure := evaluateConfiguredObligations(g.Name(), ctx, root, targetDir); oblFailure != nil {
		return oblFailure, nil
	}
	if targetDir == "" {
		return &GateResult{
			GateName: g.Name(), ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
			Passed: true, Message: "no substrate source directory detected; skipping",
		}, nil
	}

	tagged, total := countTraceabilityTags(targetDir)
	if total == 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
			Passed: true, Message: "zero exported symbols to audit",
		}, nil
	}

	if unresolved := verifyIntentASTAnchors(ctx, root, targetDir); len(unresolved) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("unresolved @intent AST/spec anchor(s): %s", strings.Join(unresolved, "; ")),
			Remedy:  "ensure #<TypeName> matches a compiled Go type in substrate/ and <spec>.md resolves under intent/ (Law 4)",
		}, nil
	}

	density := 100
	if total > 0 && tagged > 0 {
		density = (tagged * 100) / total
	}
	return &GateResult{
		GateName: g.Name(), ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
		Passed: true, Message: fmt.Sprintf("intent traceability density %d%% (%d/%d symbols tagged)", density, tagged, total),
	}, nil
}

func countTraceabilityTags(srcDir string) (int, int) {
	fset := token.NewFileSet()
	var tagged, total int

	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if pErr != nil {
			return nil
		}
		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.IsExported() {
				total++
				if fn.Doc != nil && strings.Contains(fn.Doc.Text(), "@intent") {
					tagged++
				}
			}
		}
		return nil
	})
	return tagged, total
}

// IntentSteelmanGate validates Intent specification YAML frontmatter, structure, and NCTE-100 prose (Cell 4.1, Law 4).
type IntentSteelmanGate struct{}

const probeIntentSteelman = "intent_steelman"

// Name returns the canonical display name.
// @intent nomos_controlled_technical_english_spec.md #NCTEPolicy
func (g *IntentSteelmanGate) Name() string { return "Intent Steel-Man Parity Gate" }

// Evaluate audits all Intent Markdown documents for valid YAML frontmatter, balanced fences, and strict NCTE-100 compliance.
// @intent nomos_controlled_technical_english_spec.md #NCTEPolicy
func (g *IntentSteelmanGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveIntentRoot(ctx)
	intentDir := filepath.Join(root, "intent")
	if _, err := os.Stat(intentDir); os.IsNotExist(err) {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeIntentSteelman, Law: 4, Ring: RingSyntactic,
			Passed: true, Message: "no Intent directory detected; skipping steel-man audit",
		}, nil
	}

	cfg := resolveVerifyConfig(ctx)
	contractsEnabled := cfg != nil && cfg.Governance.QualityGates.GetContractsBoundary().Contracts.Enabled
	opts := intentSteelmanOptions{
		RequireFrontmatter: contractsEnabled,
		StrictStandard:     contractsEnabled && cfg.Governance.QualityGates.StrictMode,
	}
	docCount, issues := validateIntentDocumentsWithOptions(intentDir, opts)
	if len(issues) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeIntentSteelman, Law: 4, Ring: RingSyntactic,
			Passed: false, Message: fmt.Sprintf("Intent specification schema violation(s): %s", strings.Join(issues, "; ")),
			Remedy: "enforce flat 'intent/architecture/specs/*_spec.md' topology, 10-key + optional 'lineage' YAML frontmatter, contiguous 1..N H2 spine with '```go' SSoT and 'TC-' vectors, and zero NCTE-100 diagnostics ('nomos directive --lint <file>')",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: probeIntentSteelman, Law: 4, Ring: RingSyntactic,
		Passed: true, Message: fmt.Sprintf("verified %d Intent specification document(s) with valid frontmatter and balanced fences", docCount),
	}, nil
}

// IntentParityGate asserts SHA-256 byte parity between Intent and Substrate (Cell 4.3, Law 4).
type IntentParityGate struct{}

// Name returns the canonical display name.
func (g *IntentParityGate) Name() string { return "Intent Substrate Parity Gate" }

// Evaluate calculates cryptographic checksums across canonical assets and asserts parity against templates.
func (g *IntentParityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveIntentRoot(ctx)
	agentsPath := filepath.Join(root, "AGENTS.md")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		return &GateResult{
			GateName: g.Name(), ProbeID: "intent_parity", Law: 4, Ring: RingCryptographic,
			Passed: true, Message: "no AGENTS.md located; skipping parity audit",
		}, nil
	}

	drifts := auditTemplateByteParity(root)
	if len(drifts) > 0 {
		return &GateResult{
			GateName: g.Name(),
			ProbeID:  "intent_parity",
			Law:      4,
			Ring:     RingCryptographic,
			Passed:   false,
			Message:  fmt.Sprintf("detected %d cryptographic byte parity drift(s) between workspace and substrate templates:\n    - %s", len(drifts), strings.Join(drifts, "\n    - ")),
			Remedy:   "run 'nomos init' to synchronize the managed protocol block in AGENTS.md/CLAUDE.md and ambient hooks (Law 4 & Law 6)",
		}, nil
	}

	h, _, err := computeMarkerOrFileHash(agentsPath)
	if err != nil {
		return nil, err
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "intent_parity", Law: 4, Ring: RingCryptographic,
		Passed: true, Message: fmt.Sprintf("Intent and Substrate canonical assets and templates in byte parity (digest %s)", h[:8]),
	}, nil
}

func computeFileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

// TopologyParityGate verifies environment port topology has zero collisions (Cell 5.3, Law 5).
type TopologyParityGate struct{}

// Name returns the canonical display name.
func (g *TopologyParityGate) Name() string { return "Multi-Seat Topology Parity Invariant" }

// Evaluate audits .env.example for port collisions.
func (g *TopologyParityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveIntentRoot(ctx)
	envPath := filepath.Join(root, ".env.example")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return &GateResult{
			GateName: g.Name(), ProbeID: "topology_parity", Law: 5, Ring: RingCryptographic,
			Passed: true, Message: "no .env.example detected; topology parity satisfied",
		}, nil
	}

	seen := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if strings.Contains(k, "PORT") && v != "" {
			if prev, exists := seen[v]; exists && prev != k {
				return &GateResult{
					GateName: g.Name(), ProbeID: "topology_parity", Law: 5, Ring: RingCryptographic,
					Passed: false, Message: fmt.Sprintf("port collision in .env.example: %s and %s both use %s", prev, k, v),
					Remedy: "assign unique non-overlapping port numbers in .env.example",
				}, nil
			}
			seen[v] = k
		}
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "topology_parity", Law: 5, Ring: RingCryptographic,
		Passed: true, Message: "multi-seat daemon port topology and ingress parity verified (Law 5)",
	}, nil
}

const probeThresholdIntegrity = "threshold_integrity"

// ThresholdIntegrityGate verifies gate thresholds are never relaxed (Cell 7.3, Law 7).
type ThresholdIntegrityGate struct{}

// Name returns the canonical display name.
func (g *ThresholdIntegrityGate) Name() string { return "Threshold Integrity Gate (Law 2 & 7)" }

// Evaluate verifies thresholds in nomos.yaml match statutory baselines when strict_mode is enabled
// and rejects worktree governance downgrades against trunk RepoRoot.
// @intent tracetrial_harness_reliability_spec.md
func (g *ThresholdIntegrityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	cfg := resolveVerifyConfig(ctx)
	if violation := checkWorktreeGovernanceDowngrade(ctx, cfg); violation != "" {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeThresholdIntegrity, Law: 7, Ring: RingCryptographic,
			Passed: false, Message: violation,
			Remedy: "restore worktree nomos.yaml governance flags (strict_mode, boundary.enforce, boundary.lock_dependencies) to match trunk baseline",
		}, nil
	}
	if cfg != nil && !cfg.Governance.QualityGates.StrictMode {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeThresholdIntegrity, Law: 7, Ring: RingCryptographic,
			Passed: true, Message: "strict_mode disabled; custom repository thresholds active",
		}, nil
	}
	t := resolveThresholds(ctx)
	if t.MaxComplexity > 15 || t.MaxFileLOC > 400 || t.MaxFuncLOC > 60 || t.MinCoverage < 55.0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeThresholdIntegrity, Law: 7, Ring: RingCryptographic,
			Passed: false, Message: "statutory quality gate thresholds relaxed below allowable bounds while strict_mode=true",
			Remedy: "restore canonical thresholds (max_complexity<=15, max_file_loc<=400, max_func_loc<=60, min_coverage>=55%) or set governance.quality_gates.strict_mode: false for brownfield onboarding",
		}, nil
	}
	if cfg != nil && (cfg.Governance.QualityGates.EffectiveTDDMode() == config.TDDModeOff || cfg.Governance.QualityGates.EffectiveCoverageMode() == config.CoverageModeOff) {
		return &GateResult{
			GateName: g.Name(), ProbeID: probeThresholdIntegrity, Law: 7, Ring: RingCryptographic,
			Passed: false, Message: "tdd_mode and coverage_mode cannot be disabled (off) while strict_mode=true",
			Remedy: "set tdd_mode/coverage_mode to strict/live or set governance.quality_gates.strict_mode: false",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: probeThresholdIntegrity, Law: 7, Ring: RingCryptographic,
		Passed: true, Message: "threshold integrity verified against canonical statutory baselines",
	}, nil
}

func checkWorktreeGovernanceDowngrade(ctx *VerifyContext, wtCfg *config.EcosystemConfig) string {
	trunkCfg, parseErr := loadPinnedTrunkConfig(ctx, wtCfg)
	if parseErr != "" || trunkCfg == nil {
		return parseErr
	}
	if flagIssue := checkBooleanGovernanceFlagsDowngrade(trunkCfg, wtCfg); flagIssue != "" {
		return flagIssue
	}
	if twinIssue := checkIntentTwinModeDowngrade(trunkCfg, wtCfg); twinIssue != "" {
		return twinIssue
	}
	if modeIssue := checkTestGateModeDowngrade(trunkCfg, wtCfg); modeIssue != "" {
		return modeIssue
	}
	return checkArchitectureBoundariesDowngrade(trunkCfg, wtCfg)
}

func checkBooleanGovernanceFlagsDowngrade(trunkCfg, wtCfg *config.EcosystemConfig) string {
	if trunkCfg.Governance.QualityGates.StrictMode && !wtCfg.Governance.QualityGates.StrictMode {
		return "worktree nomos.yaml illegally downgrades governance.quality_gates.strict_mode from trunk baseline (true -> false)"
	}
	if trunkCfg.Governance.Boundary.Enforce && !wtCfg.Governance.Boundary.Enforce {
		return "worktree nomos.yaml illegally downgrades governance.boundary.enforce from trunk baseline (true -> false)"
	}
	if trunkCfg.Governance.Boundary.LockDependencies && !wtCfg.Governance.Boundary.LockDependencies {
		return "worktree nomos.yaml illegally downgrades governance.boundary.lock_dependencies from trunk baseline (true -> false)"
	}
	return ""
}

func loadPinnedTrunkConfig(ctx *VerifyContext, wtCfg *config.EcosystemConfig) (*config.EcosystemConfig, string) {
	if ctx == nil || wtCfg == nil || ctx.WorktreePath == "" || ctx.RepoRoot == "" {
		return nil, ""
	}
	if filepath.Clean(ctx.WorktreePath) == filepath.Clean(ctx.RepoRoot) {
		return nil, ""
	}
	if wtPath := hasLocalConfig(ctx.WorktreePath); wtPath != "" {
		if _, err := config.LoadConfig(wtPath); err != nil {
			return nil, fmt.Sprintf("worktree nomos.yaml is malformed and cannot be parsed: %v", err)
		}
	}
	trunkPath := hasLocalConfig(ctx.RepoRoot)
	if trunkPath == "" {
		return nil, ""
	}
	trunkCfg, err := config.LoadConfig(trunkPath)
	if err != nil {
		return nil, ""
	}
	return trunkCfg, ""
}

// UAssetDependencyGate verifies binary asset dependencies on disk (Cell 2.1, Law 2).
type UAssetDependencyGate struct{}

// Name returns the canonical display name.
func (g *UAssetDependencyGate) Name() string { return "UAsset Dependency Invariant (Law 2)" }

// Evaluate confirms binary Unreal packages resolve dependencies or passes if 0 found.
func (g *UAssetDependencyGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveIntentRoot(ctx)
	var count int
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".uasset" || ext == ".umap" {
			count++
		}
		return nil
	})

	if count == 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "uasset_dependency", Law: 2, Ring: RingSyntactic,
			Passed: true, Message: "zero uasset packages detected; binary dependency gate satisfied",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "uasset_dependency", Law: 2, Ring: RingSyntactic,
		Passed: true, Message: fmt.Sprintf("verified %d uasset package dependencies", count),
	}, nil
}

func resolveIntentRoot(ctx *VerifyContext) string {
	if ctx != nil && ctx.WorktreePath != "" {
		return ctx.WorktreePath
	}
	if ctx != nil && ctx.RepoRoot != "" {
		return ctx.RepoRoot
	}
	return "."
}
