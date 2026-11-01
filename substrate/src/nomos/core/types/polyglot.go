// Package types provides foundational domain types, schema bindings, and
// invariant contracts synthesized directly from authoritative Intent CUE schemas.
package types

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// SupportedLanguage defines languages supported by polyglot AST verification, bound to #SupportedLanguage in polyglot.cue.
type SupportedLanguage string

const (
	LanguageGo         SupportedLanguage = "go"
	LanguageTypeScript SupportedLanguage = "typescript"
	LanguageJavaScript SupportedLanguage = "javascript"
	LanguageSvelte     SupportedLanguage = "svelte"
	LanguageMarkdown   SupportedLanguage = "markdown"
	LanguageYAML       SupportedLanguage = "yaml"
	LanguagePython     SupportedLanguage = "python"
	LanguageRust       SupportedLanguage = "rust"
	LanguageC          SupportedLanguage = "c"
	LanguageCPP        SupportedLanguage = "cpp"
	LanguageHTML       SupportedLanguage = "html"
	LanguageCSS        SupportedLanguage = "css"
)

// AllSupportedLanguages enumerates all active polyglot languages.
var AllSupportedLanguages = []SupportedLanguage{
	LanguageGo,
	LanguageTypeScript,
	LanguageJavaScript,
	LanguageSvelte,
	LanguageMarkdown,
	LanguageYAML,
	LanguagePython,
	LanguageRust,
	LanguageC,
	LanguageCPP,
	LanguageHTML,
	LanguageCSS,
}

// FindingRule defines AST rule violation codes, bound to #FindingRule in polyglot.cue.
type FindingRule string

const (
	RuleMagicPort           FindingRule = "magic_port"
	RuleLocalhostURL        FindingRule = "localhost_url"
	RuleEcosystemMap        FindingRule = "ecosystem_map"
	RuleIPLeak              FindingRule = "ip_leak"
	RuleCognitiveComplexity FindingRule = "cognitive_complexity"
	RuleDocstringDensity    FindingRule = "docstring_density"
)

// AllFindingRules enumerates all active AST finding rules.
var AllFindingRules = []FindingRule{
	RuleMagicPort,
	RuleLocalhostURL,
	RuleEcosystemMap,
	RuleIPLeak,
	RuleCognitiveComplexity,
	RuleDocstringDensity,
}

// IsValidLanguage checks whether a language string matches a supported language.
func IsValidLanguage(l SupportedLanguage) bool {
	for _, supported := range AllSupportedLanguages {
		if l == supported {
			return true
		}
	}
	return false
}

// IsValidFindingRule checks whether a rule matches an authoritative rule enum.
func IsValidFindingRule(r FindingRule) bool {
	for _, rule := range AllFindingRules {
		if r == rule {
			return true
		}
	}
	return false
}

// SourceRootConfig binds a directory path to a polyglot language scanner, bound to #SourceRootConfig in polyglot.cue.
type SourceRootConfig struct {
	Path            string            `json:"path"`
	Language        SupportedLanguage `json:"language"`
	ExcludePatterns []string          `json:"exclude_patterns,omitempty"`
}

// Validate checks path syntax and language support (Law 5 Zero Hardcoded Path).
func (src *SourceRootConfig) Validate() error {
	if src == nil {
		return fmt.Errorf("source root config cannot be nil")
	}
	if strings.TrimSpace(src.Path) == "" {
		return fmt.Errorf("source root path cannot be empty")
	}
	if strings.HasPrefix(src.Path, "/") {
		return fmt.Errorf("source root path %q cannot be absolute (must not start with /)", src.Path)
	}
	if strings.Contains(src.Path, "..") {
		return fmt.Errorf("source root path %q cannot contain parent traversal (..)", src.Path)
	}
	if !IsValidLanguage(src.Language) {
		return fmt.Errorf("unsupported language %q", src.Language)
	}
	return nil
}

// PolyglotWorkspaceManifest declares workspace source roots and polyglot enforcement rules,
// bound to #PolyglotWorkspaceManifest in polyglot.cue.
type PolyglotWorkspaceManifest struct {
	SourceRoots    []SourceRootConfig `json:"source_roots"`
	StrictPolyglot bool               `json:"strict_polyglot"`
}

// Validate checks each configured source root.
func (pwm *PolyglotWorkspaceManifest) Validate() error {
	if pwm == nil {
		return fmt.Errorf("workspace manifest cannot be nil")
	}
	for i, root := range pwm.SourceRoots {
		if err := root.Validate(); err != nil {
			return fmt.Errorf("invalid source root at index %d: %w", i, err)
		}
	}
	return nil
}

// ResolvedFileContext binds a discovered file to its AST parser strategy (#ResolvedFileContext).
type ResolvedFileContext struct {
	RelativePath string
	MatchedRoot  SourceRootConfig
	Language     SupportedLanguage
	IsExcluded   bool
}

// SourceRootResolver defines the contract for dynamic root resolution.
type SourceRootResolver interface {
	Resolve(ctx context.Context, relPath string) (*ResolvedFileContext, error)
	ValidateManifest(ctx context.Context, roots []SourceRootConfig) error
}

// DefaultSourceRootResolver implements SourceRootResolver based on PolyglotWorkspaceManifest.
type DefaultSourceRootResolver struct {
	Manifest *PolyglotWorkspaceManifest
}

// NewSourceRootResolver creates a new DefaultSourceRootResolver.
func NewSourceRootResolver(manifest *PolyglotWorkspaceManifest) *DefaultSourceRootResolver {
	return &DefaultSourceRootResolver{Manifest: manifest}
}

// ValidateManifest verifies all configured source roots.
func (r *DefaultSourceRootResolver) ValidateManifest(ctx context.Context, roots []SourceRootConfig) error {
	m := &PolyglotWorkspaceManifest{SourceRoots: roots}
	return m.Validate()
}

// Resolve resolves the target relative path to its nearest-enclosing source root.
func (r *DefaultSourceRootResolver) Resolve(ctx context.Context, relPath string) (*ResolvedFileContext, error) {
	if strings.HasPrefix(relPath, "/") {
		return nil, fmt.Errorf("Law 5 violation: absolute path %q is prohibited in source root resolution", relPath)
	}
	if strings.Contains(relPath, "..") {
		return nil, fmt.Errorf("Law 5 violation: parent traversal in path %q is prohibited", relPath)
	}

	cleanPath := filepath.Clean(relPath)
	if r.Manifest == nil || len(r.Manifest.SourceRoots) == 0 {
		return nil, fmt.Errorf("no source roots configured in manifest")
	}

	bestRoot := findBestRoot(r.Manifest.SourceRoots, cleanPath)
	if bestRoot == nil {
		if r.Manifest.StrictPolyglot {
			return nil, fmt.Errorf("Law 7 fail-closed: path %q matches no configured source root under strict mode", cleanPath)
		}
		return nil, fmt.Errorf("unmapped source root for path %q", cleanPath)
	}

	isExcluded := isPathExcluded(bestRoot.ExcludePatterns, cleanPath)
	return &ResolvedFileContext{
		RelativePath: cleanPath,
		MatchedRoot:  *bestRoot,
		Language:     bestRoot.Language,
		IsExcluded:   isExcluded,
	}, nil
}

func findBestRoot(roots []SourceRootConfig, cleanPath string) *SourceRootConfig {
	var best *SourceRootConfig
	bestLen := -1
	for i := range roots {
		root := &roots[i]
		cleanRoot := filepath.Clean(root.Path)
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+"/") {
			if len(cleanRoot) > bestLen {
				bestLen = len(cleanRoot)
				best = root
			}
		}
	}
	return best
}

func isPathExcluded(patterns []string, cleanPath string) bool {
	for _, pat := range patterns {
		matched, err := filepath.Match(pat, cleanPath)
		if err == nil && matched {
			return true
		}
		if strings.HasPrefix(pat, "**/") {
			sub := strings.TrimPrefix(pat, "**/")
			sub = strings.TrimSuffix(sub, "/**")
			if strings.Contains(cleanPath, sub) {
				return true
			}
		}
	}
	return false
}

// TreeSitterQueryConfig binds an AST query pattern to a finding rule, bound to #TreeSitterQueryConfig in polyglot.cue.
type TreeSitterQueryConfig struct {
	Language      SupportedLanguage `json:"language"`
	QueryPattern  string            `json:"query_pattern"`
	TargetRule    FindingRule       `json:"target_rule"`
	CaptureSymbol string            `json:"capture_symbol,omitempty"`
}

// Validate verifies query configuration syntax.
func (tsq *TreeSitterQueryConfig) Validate() error {
	if tsq == nil {
		return fmt.Errorf("tree sitter query config cannot be nil")
	}
	if !IsValidLanguage(tsq.Language) {
		return fmt.Errorf("unsupported language %q", tsq.Language)
	}
	if strings.TrimSpace(tsq.QueryPattern) == "" {
		return fmt.Errorf("query pattern cannot be empty")
	}
	if !IsValidFindingRule(tsq.TargetRule) {
		return fmt.Errorf("unsupported finding rule %q", tsq.TargetRule)
	}
	return nil
}

// PolyglotASTFinding represents a concrete AST violation discovered during static analysis,
// bound to #PolyglotASTFinding in polyglot.cue.
type PolyglotASTFinding struct {
	File     string      `json:"file"`
	Line     int         `json:"line"`
	FuncName string      `json:"func_name,omitempty"`
	Rule     FindingRule `json:"rule"`
	Message  string      `json:"message"`
}

// Validate ensures finding coordinates and messages are well-formed.
func (paf *PolyglotASTFinding) Validate() error {
	if paf == nil {
		return fmt.Errorf("polyglot AST finding cannot be nil")
	}
	if strings.TrimSpace(paf.File) == "" {
		return fmt.Errorf("finding file path cannot be empty")
	}
	if paf.Line < 0 {
		return fmt.Errorf("finding line cannot be negative: %d", paf.Line)
	}
	if !IsValidFindingRule(paf.Rule) {
		return fmt.Errorf("invalid finding rule %q", paf.Rule)
	}
	if strings.TrimSpace(paf.Message) == "" {
		return fmt.Errorf("finding message cannot be empty")
	}
	return nil
}

// PolyglotScanReport summarizes the results of an AST verification pass,
// bound to #PolyglotScanReport in polyglot.cue.
type PolyglotScanReport struct {
	WorktreePath string               `json:"worktree_path"`
	ScannedFiles int                  `json:"scanned_files"`
	Findings     []PolyglotASTFinding `json:"findings"`
	Passed       bool                 `json:"passed"`
}

// Validate checks scan report data consistency.
func (psr *PolyglotScanReport) Validate() error {
	if psr == nil {
		return fmt.Errorf("scan report cannot be nil")
	}
	if psr.ScannedFiles < 0 {
		return fmt.Errorf("scanned files count cannot be negative: %d", psr.ScannedFiles)
	}
	for i, finding := range psr.Findings {
		if err := finding.Validate(); err != nil {
			return fmt.Errorf("invalid finding at index %d: %w", i, err)
		}
	}
	return nil
}

// TreeSitterWasmConfig defines WebAssembly runtime and language bindings for Tree-sitter,
// bound to #TreeSitterWasmConfig in polyglot.cue.
type TreeSitterWasmConfig struct {
	EngineName         string              `json:"engine_name"`
	Runtime            string              `json:"runtime"`
	PureGo             bool                `json:"pure_go"`
	SupportedLanguages []SupportedLanguage `json:"supported_languages"`
	QueryRoots         []string            `json:"query_roots,omitempty"`
}

// Validate checks TreeSitterWasmConfig schema invariants.
func (c *TreeSitterWasmConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("treesitter wasm config cannot be nil")
	}
	if strings.TrimSpace(c.EngineName) == "" {
		return fmt.Errorf("engine_name cannot be empty")
	}
	if c.Runtime != "wazero" {
		return fmt.Errorf("unsupported runtime %q (must be wazero)", c.Runtime)
	}
	for i, lang := range c.SupportedLanguages {
		if !IsValidLanguage(lang) {
			return fmt.Errorf("invalid supported language at index %d: %q", i, lang)
		}
	}
	return nil
}
