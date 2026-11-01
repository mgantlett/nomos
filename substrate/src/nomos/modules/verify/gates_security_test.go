package verify

import (
	"testing"
)

func TestSecurityGatesRegistration(t *testing.T) {
	for _, id := range []string{"secret", "ip_leak", "broken_wire", "config_immutability"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestSecretGateClean(t *testing.T) {
	g := &SecretGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected secret gate to pass on clean workspace: %s", res.Message)
	}
}

func TestBrokenWireGateClean(t *testing.T) {
	g := &BrokenWireGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected broken wire gate to pass on clean workspace: %s", res.Message)
	}
}

func TestConfigImmutabilityGate(t *testing.T) {
	g := &ConfigImmutabilityGate{}
	ctx := &VerifyContext{TaskID: "INT-737"}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected intent task to pass config immutability: %s", res.Message)
	}
}

func TestIsIntentConfigPath(t *testing.T) {
	tests := []struct {
		path           string
		intentRoots    []string
		substrateRoots []string
		want           bool
	}{
		{"nomos.yaml", nil, nil, true},
		{".nomos.yaml", nil, nil, true},
		{"intent/config/nomos.yaml", nil, nil, true},
		{"substrate/cli/templates/nomos.yaml", nil, nil, false},
		{"src/templates/nomos.yaml", []string{"docs"}, []string{"src"}, false},
		{"docs/config/nomos.yaml", []string{"docs"}, []string{"src"}, true},
		{"substrate/main.go", nil, nil, false},
	}

	for _, tt := range tests {
		got := isIntentConfigPath(tt.path, tt.intentRoots, tt.substrateRoots)
		if got != tt.want {
			t.Errorf("isIntentConfigPath(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}
