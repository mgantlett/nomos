package config

import "testing"

func TestVerifyScopeAndTDDModeHelpers(t *testing.T) {
	var nilQG *QualityGatesConfig
	if got := nilQG.EffectiveVerifyScope(); got != VerifyScopeDiff {
		t.Fatalf("expected nil QualityGatesConfig verify_scope=%s, got %s", VerifyScopeDiff, got)
	}
	if got := nilQG.EffectiveTDDMode(); got != TDDModeStrict {
		t.Fatalf("expected nil QualityGatesConfig tdd_mode=%s, got %s", TDDModeStrict, got)
	}
	if got := nilQG.EffectiveCoverageMode(); got != CoverageModeLive {
		t.Fatalf("expected nil QualityGatesConfig coverage_mode=%s, got %s", CoverageModeLive, got)
	}

	qg := &QualityGatesConfig{StrictMode: false, VerifyScope: " repo "}
	if got := qg.EffectiveVerifyScope(); got != VerifyScopeRepo {
		t.Fatalf("expected normalized verify_scope=repo, got %s", got)
	}
	if got := qg.EffectiveTDDMode(); got != TDDModeRatchet {
		t.Fatalf("expected brownfield default tdd_mode=ratchet, got %s", got)
	}

	cases := []struct {
		scope string
		rank  int
		valid bool
	}{
		{"", -1, true},
		{"diff", 1, true},
		{"REPO", 2, true},
		{"bogus", -1, false},
	}
	for _, tc := range cases {
		r, ok := VerifyScopeRank(tc.scope)
		if r != tc.rank || ok != tc.valid {
			t.Errorf("VerifyScopeRank(%q) = (%d, %v), want (%d, %v)", tc.scope, r, ok, tc.rank, tc.valid)
		}
	}

	cfg := DefaultNomosConfig()
	cfg.Governance.QualityGates.VerifyScope = "invalid"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatalf("expected ValidateConfig to reject invalid verify_scope")
	}
	cfg.Governance.QualityGates.VerifyScope = VerifyScopeRepo
	cfg.Governance.QualityGates.TDDExclude = []string{"**/*.svelte"}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid VerifyScope and TDDExclude to pass ValidateConfig: %v", err)
	}
}

