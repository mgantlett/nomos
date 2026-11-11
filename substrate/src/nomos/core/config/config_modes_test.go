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

	var nilBoundary *BoundaryConfig
	if got := nilBoundary.EffectiveCredentialReadMode(); got != CredentialReadModeHostDeny {
		t.Fatalf("expected nil BoundaryConfig credential_read_mode=%s, got %s", CredentialReadModeHostDeny, got)
	}
	cfg.Governance.Boundary.CredentialReadMode = " STRICT "
	if got := cfg.Governance.Boundary.EffectiveCredentialReadMode(); got != CredentialReadModeStrict {
		t.Fatalf("expected normalized credential_read_mode=strict, got %s", got)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected strict credential_read_mode to pass ValidateConfig: %v", err)
	}
	cfg.Governance.Boundary.CredentialReadMode = "invalid_mode"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatalf("expected ValidateConfig to reject invalid credential_read_mode")
	}
}

