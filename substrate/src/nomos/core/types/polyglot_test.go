package types

import (
	"testing"
)

func TestSupportedLanguageValidation(t *testing.T) {
	for _, lang := range AllSupportedLanguages {
		if !IsValidLanguage(lang) {
			t.Errorf("expected %s to be recognized as supported language", lang)
		}
	}
	if !IsValidLanguage("rust") {
		t.Errorf("expected rust to be supported in expanded schema")
	}
	if IsValidLanguage("ruby") {
		t.Errorf("expected ruby to be unsupported in current schema")
	}
}

func TestFindingRuleValidation(t *testing.T) {
	for _, rule := range AllFindingRules {
		if !IsValidFindingRule(rule) {
			t.Errorf("expected %s to be recognized as finding rule", rule)
		}
	}
	if IsValidFindingRule("unknown_rule") {
		t.Errorf("expected unknown_rule to be unrecognized")
	}
}

func TestSourceRootConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *SourceRootConfig
		expectErr bool
	}{
		{
			name: "valid relative source root",
			cfg: &SourceRootConfig{
				Path:     "substrate/src/nomos",
				Language: LanguageGo,
			},
			expectErr: false,
		},
		{
			name: "valid rust source root",
			cfg: &SourceRootConfig{
				Path:     "crates/core",
				Language: LanguageRust,
			},
			expectErr: false,
		},
		{
			name: "invalid absolute path",
			cfg: &SourceRootConfig{
				Path:     "/opt/nomos/src",
				Language: LanguageGo,
			},
			expectErr: true,
		},
		{
			name: "invalid traversal path",
			cfg: &SourceRootConfig{
				Path:     "../nomos/src",
				Language: LanguageGo,
			},
			expectErr: true,
		},
		{
			name: "unsupported language",
			cfg: &SourceRootConfig{
				Path:     "frontend",
				Language: "ruby",
			},
			expectErr: true,
		},
		{
			name:      "nil source root config",
			cfg:       nil,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected clean validation, got: %v", err)
			}
		})
	}
}

func TestPolyglotASTFindingValidation(t *testing.T) {
	finding := &PolyglotASTFinding{
		File:     "types.go",
		Line:     15,
		FuncName: "Validate",
		Rule:     RuleCognitiveComplexity,
		Message:  "complexity exceeded threshold",
	}

	if err := finding.Validate(); err != nil {
		t.Errorf("expected clean finding validation, got: %v", err)
	}

	invalidFinding := &PolyglotASTFinding{
		File:    "types.go",
		Line:    -1,
		Rule:    RuleMagicPort,
		Message: "magic port found",
	}
	if err := invalidFinding.Validate(); err == nil {
		t.Errorf("expected error on negative line count")
	}
}

func TestPolyglotScanReportValidation(t *testing.T) {
	report := &PolyglotScanReport{
		WorktreePath: "worktrees/nomos-SUB-1",
		ScannedFiles: 3,
		Findings: []PolyglotASTFinding{
			{
				File:    "types.go",
				Line:    1,
				Rule:    RuleDocstringDensity,
				Message: "docstring density ok",
			},
		},
		Passed: true,
	}

	if err := report.Validate(); err != nil {
		t.Errorf("expected clean report validation, got: %v", err)
	}

	manifest := &PolyglotWorkspaceManifest{
		SourceRoots: []SourceRootConfig{
			{Path: "substrate", Language: LanguageGo},
		},
		StrictPolyglot: true,
	}
	if err := manifest.Validate(); err != nil {
		t.Errorf("expected clean manifest validation, got: %v", err)
	}
}

func TestTreeSitterWasmConfigValidation(t *testing.T) {
	cfg := &TreeSitterWasmConfig{
		EngineName:         "treesitter_wasm_wazero",
		Runtime:            "wazero",
		PureGo:             true,
		SupportedLanguages: []SupportedLanguage{LanguageGo, LanguageTypeScript, LanguageSvelte},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected clean validation, got: %v", err)
	}

	invalidRuntime := &TreeSitterWasmConfig{
		EngineName: "test",
		Runtime:    "v8",
	}
	if err := invalidRuntime.Validate(); err == nil {
		t.Errorf("expected error for non-wazero runtime")
	}
}
