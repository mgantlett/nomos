// Package config provides declarative configuration management, dynamic path discovery,
// and compiled Go Single Source of Truth (SSoT) structures reflected to nomos.schema.json.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"gopkg.in/yaml.v3"
)

const (
	// IntentTwinModeOff disables mandatory INT twin trailers on SUB commits (default).
	IntentTwinModeOff = "off"
	// IntentTwinModeFeat requires a Twin: INT-<N> trailer on SUB feat(...) commits.
	IntentTwinModeFeat = "feat"
	// IntentTwinModeStrict requires a Twin: INT-<N> trailer on all SUB commits.
	IntentTwinModeStrict = "strict"
)

// BoundaryConfig declares architectural package and CLI command boundary protections.
type BoundaryConfig struct {
	Enforce            bool     `yaml:"enforce" json:"enforce"`
	ProhibitedPackages []string `yaml:"prohibited_packages,omitempty" json:"prohibited_packages,omitempty"`
	ProhibitedCommands []string `yaml:"prohibited_commands,omitempty" json:"prohibited_commands,omitempty"`
	LockDependencies   bool     `yaml:"lock_dependencies,omitempty" json:"lock_dependencies,omitempty"`
	AllowedMCPServers  []string `yaml:"allowed_mcp_servers,omitempty" json:"allowed_mcp_servers,omitempty"`
	BlockedMCPServers  []string `yaml:"blocked_mcp_servers,omitempty" json:"blocked_mcp_servers,omitempty"`
}

// ContractsConfig defines contract schema and backend verification roots.
type ContractsConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	SpecRoots    []string `yaml:"spec_roots,omitempty" json:"spec_roots,omitempty"`
	SchemaFiles  []string `yaml:"schema_files,omitempty" json:"schema_files,omitempty"`
	BackendRoots []string `yaml:"backend_roots,omitempty" json:"backend_roots,omitempty"`
	StrictTypes  bool     `yaml:"strict_types" json:"strict_types"`
}

// MembraneConfig defines membrane boundary verification.
type MembraneConfig struct {
	Enabled              bool     `yaml:"enabled" json:"enabled"`
	FrontendRoots        []string `yaml:"frontend_roots,omitempty" json:"frontend_roots,omitempty"`
	BackendDTORoots      []string `yaml:"backend_dto_roots,omitempty" json:"backend_dto_roots,omitempty"`
	BanishDefensiveMasks bool     `yaml:"banish_defensive_masks,omitempty" json:"banish_defensive_masks,omitempty"`
	FailOnContractDrift  bool     `yaml:"fail_on_contract_drift,omitempty" json:"fail_on_contract_drift,omitempty"`
}

// InfraConfig defines infrastructure and environment ingress verification.
type InfraConfig struct {
	Enabled         bool     `yaml:"enabled" json:"enabled"`
	IngressConfigs  []string `yaml:"ingress_configs,omitempty" json:"ingress_configs,omitempty"`
	EnvDeclarations []string `yaml:"env_declarations,omitempty" json:"env_declarations,omitempty"`
	PortScan        bool     `yaml:"port_scan" json:"port_scan"`
	FailOnOrphanEnv bool     `yaml:"fail_on_orphan_env" json:"fail_on_orphan_env"`
}

// ContractsBoundaryConfig defines cross-plane contract and infrastructure verification boundaries.
type ContractsBoundaryConfig struct {
	Contracts   ContractsConfig `yaml:"contracts" json:"contracts"`
	Membrane    *MembraneConfig `yaml:"membrane,omitempty" json:"membrane,omitempty"`
	Infra       InfraConfig     `yaml:"infra" json:"infra"`
	AutoPromote bool            `yaml:"auto_promote,omitempty" json:"auto_promote,omitempty"`
}

// ThresholdsConfig captures numerical AST and test hygiene quality bounds.
type ThresholdsConfig struct {
	MaxCognitiveDebtScore     int `yaml:"max_cognitive_debt_score" json:"max_cognitive_debt_score"`
	MaxCyclomaticComplexity   int `yaml:"max_cyclomatic_complexity" json:"max_cyclomatic_complexity"`
	MaxFileLOC                int `yaml:"max_file_loc" json:"max_file_loc"`
	MaxFuncLOC                int `yaml:"max_func_loc" json:"max_func_loc"`
	MinCommentDensityPct      int `yaml:"min_comment_density_pct" json:"min_comment_density_pct"`
	MinTestCoveragePct        int `yaml:"min_test_coverage_pct" json:"min_test_coverage_pct"`
	MinCloneLines             int `yaml:"min_clone_lines,omitempty" json:"min_clone_lines,omitempty"`
	MaxFuncParams             int `yaml:"max_func_params,omitempty" json:"max_func_params,omitempty"`
	MaxMagicStringDuplication int `yaml:"max_magic_string_duplication,omitempty" json:"max_magic_string_duplication,omitempty"`
}

// QualityGatesConfig defines verification policies.
type QualityGatesConfig struct {
	ForbiddenLiterals            []string                 `yaml:"forbidden_literals,omitempty" json:"forbidden_literals,omitempty"`
	HasForbiddenLiteralsOverride bool                     `yaml:"-" json:"-"`
	ForbiddenShims               []string                 `yaml:"forbidden_shims,omitempty" json:"forbidden_shims,omitempty"`
	StrictMode                   bool                     `yaml:"strict_mode" json:"strict_mode"`
	IntentTwinMode               string                   `yaml:"intent_twin_mode,omitempty" json:"intent_twin_mode,omitempty"`
	PolyglotToolchains           *bool                    `yaml:"polyglot_toolchains,omitempty" json:"polyglot_toolchains,omitempty"`
	Exclude                      []string                 `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	IntentSourceRoots            []string                 `yaml:"intent_source_roots,omitempty" json:"intent_source_roots,omitempty"`
	SubstrateSourceRoots         []string                 `yaml:"substrate_source_roots,omitempty" json:"substrate_source_roots,omitempty"`
	Contracts                    *ContractsBoundaryConfig `yaml:"contracts,omitempty" json:"contracts,omitempty"`
	Thresholds                   ThresholdsConfig         `yaml:"thresholds" json:"thresholds"`
}

// IsPolyglotToolchainsEnabled reports whether polyglot toolchain auto-discovery is active (@intent: #QualityGatesConfig).
func (q *QualityGatesConfig) IsPolyglotToolchainsEnabled() bool {
	if q == nil || q.PolyglotToolchains == nil {
		return true
	}
	return *q.PolyglotToolchains
}

// EffectiveIntentTwinMode returns normalized intent_twin_mode, defaulting to "off" (@intent: #QualityGatesConfig).
func (q *QualityGatesConfig) EffectiveIntentTwinMode() string {
	if q == nil {
		return IntentTwinModeOff
	}
	norm := strings.ToLower(strings.TrimSpace(q.IntentTwinMode))
	if norm == "" {
		return IntentTwinModeOff
	}
	return norm
}

// IntentTwinModeRank returns the strictness rank (0=off, 1=feat, 2=strict) and validity (@intent: #QualityGatesConfig).
func IntentTwinModeRank(mode string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", IntentTwinModeOff:
		return 0, true
	case IntentTwinModeFeat:
		return 1, true
	case IntentTwinModeStrict:
		return 2, true
	default:
		return -1, false
	}
}

// GetContractsBoundary returns active contract boundary config.
func (q *QualityGatesConfig) GetContractsBoundary() ContractsBoundaryConfig {
	if q.Contracts != nil {
		return *q.Contracts
	}
	return ContractsBoundaryConfig{}
}

// GovernanceConfig aggregates governance boundaries and quality gates.
type GovernanceConfig struct {
	Boundary     BoundaryConfig     `yaml:"boundary" json:"boundary"`
	QualityGates QualityGatesConfig `yaml:"quality_gates" json:"quality_gates"`
}

// EcosystemConfig is the authoritative SSoT Go structure matching nomos.yaml.
type EcosystemConfig struct {
	Project       string           `yaml:"project" json:"project"`
	DefaultBranch string           `yaml:"default_branch" json:"default_branch"`
	Governance    GovernanceConfig `yaml:"governance" json:"governance"`
}

// DefaultNomosConfig returns a clean, downstream-safe baseline configuration.
func DefaultNomosConfig() *EcosystemConfig {
	polyglot := true
	return &EcosystemConfig{
		Project:       constants.AppName,
		DefaultBranch: "main",
		Governance: GovernanceConfig{
			Boundary: BoundaryConfig{
				Enforce: true,
			},
			QualityGates: QualityGatesConfig{
				StrictMode:         true,
				PolyglotToolchains: &polyglot,
				Thresholds: ThresholdsConfig{
					MaxCognitiveDebtScore:     45,
					MaxCyclomaticComplexity:   15,
					MaxFileLOC:                400,
					MaxFuncLOC:                60,
					MinCommentDensityPct:      10,
					MinTestCoveragePct:        55,
					MinCloneLines:             18,
					MaxFuncParams:             8,
					MaxMagicStringDuplication: 10,
				},
			},
		},
	}
}

// LoadConfig loads and parses the ecosystem configuration from the specified path or dynamic SSoT.
func LoadConfig(path string) (*EcosystemConfig, error) {
	target, found, err := resolveLoadTarget(path)
	if err != nil {
		return nil, err
	}
	if !found {
		return DefaultNomosConfig(), nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("failed reading config from %s: %w", target, err)
	}

	cfg := DefaultNomosConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed parsing config yaml: %w", err)
	}
	cfg.Governance.QualityGates.HasForbiddenLiteralsOverride = hasExplicitForbiddenLiteralsKey(data)
	return cfg, nil
}

func resolveLoadTarget(path string) (string, bool, error) {
	if path == "" {
		resolved, err := LocateConfigPath()
		if err != nil {
			return "", false, nil
		}
		return resolved, true, nil
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if found := checkConfigPaths(path); found != "" {
			return found, true, nil
		}
		return "", false, nil
	}
	return path, true, nil
}

func hasExplicitForbiddenLiteralsKey(data []byte) bool {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}
	gov, ok := raw["governance"].(map[string]any)
	if !ok {
		return false
	}
	qg, ok := gov["quality_gates"].(map[string]any)
	if !ok {
		return false
	}
	_, exists := qg["forbidden_literals"]
	return exists
}

// ValidateConfig validates an EcosystemConfig struct against architectural invariants.
func ValidateConfig(cfg *EcosystemConfig) error {
	if cfg == nil {
		return fmt.Errorf("ecosystem config cannot be nil")
	}
	if cfg.Project == "" {
		return fmt.Errorf("project identifier cannot be empty")
	}
	if cfg.DefaultBranch == "" {
		return fmt.Errorf("default_branch cannot be empty")
	}
	thresholds := cfg.Governance.QualityGates.Thresholds
	if thresholds.MinTestCoveragePct < 0 || thresholds.MinTestCoveragePct > 100 {
		return fmt.Errorf("min_test_coverage_pct must be between 0 and 100, got %d", thresholds.MinTestCoveragePct)
	}
	if thresholds.MinCommentDensityPct < 0 || thresholds.MinCommentDensityPct > 100 {
		return fmt.Errorf("min_comment_density_pct must be between 0 and 100, got %d", thresholds.MinCommentDensityPct)
	}
	if thresholds.MaxFileLOC <= 0 {
		return fmt.Errorf("max_file_loc must be greater than zero, got %d", thresholds.MaxFileLOC)
	}
	if thresholds.MaxFuncLOC <= 0 {
		return fmt.Errorf("max_func_loc must be greater than zero, got %d", thresholds.MaxFuncLOC)
	}
	if _, ok := IntentTwinModeRank(cfg.Governance.QualityGates.IntentTwinMode); !ok {
		return fmt.Errorf("intent_twin_mode must be off, feat, or strict, got %q", cfg.Governance.QualityGates.IntentTwinMode)
	}
	return nil
}
