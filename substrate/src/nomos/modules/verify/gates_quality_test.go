package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityGatesRegistration(t *testing.T) {
	for _, id := range []string{"tdd", "coverage", "docstrings", "dead_code", "visual_regression"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestTDDGateClean(t *testing.T) {
	g := &TDDGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected tdd gate to pass: %s", res.Message)
	}
}

func TestDocstringsGateClean(t *testing.T) {
	g := &DocstringDensityGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected docstrings gate to pass: %s", res.Message)
	}
}

func TestVisualRegressionGateClean(t *testing.T) {
	g := &VisualRegressionGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected visual regression gate to pass: %s", res.Message)
	}
}

func TestTDDGate_BrownfieldSubstrateRootsAndTestdataSkip(t *testing.T) {
	root := t.TempDir()
	mustWriteGoFixture(t, filepath.Join(root, "internal", "legacy", "legacy.go"), "package legacy\nfunc Old() int { return 1 }\n")
	mustWriteGoFixture(t, filepath.Join(root, "cmd", "scanner", "mcp", "testdata", "proj", "fixture.go"), "package proj\nfunc Fix() {}\n")
	mustWriteGoFixture(t, filepath.Join(root, "cmd", "scanner", "mcp", "handler.go"), "package mcp\nfunc Handle() int { return 2 }\n")
	mustWriteGoFixture(t, filepath.Join(root, "nomos.yaml"), "project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    substrate_source_roots:\n      - cmd/scanner/mcp\n")

	g := &TDDGate{}
	ctx := &VerifyContext{RepoRoot: root}

	resFail, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resFail.Passed {
		t.Fatalf("expected TDDGate to fail when cmd/scanner/mcp lacks *_test.go")
	}
	if strings.Contains(resFail.Message, "legacy") || strings.Contains(resFail.Message, "testdata") {
		t.Errorf("expected legacy and testdata packages to be excluded, got: %s", resFail.Message)
	}

	mustWriteGoFixture(t, filepath.Join(root, "cmd", "scanner", "mcp", "handler_test.go"), "package mcp\nimport \"testing\"\nfunc TestHandle(t *testing.T) {}\n")
	resPass, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resPass.Passed {
		t.Errorf("expected TDDGate to pass once governed brownfield root has tests, got: %s", resPass.Message)
	}
}

func TestResolveGoTestPackagesAndEnv_BrownfieldIsolation(t *testing.T) {
	root := t.TempDir()
	mcpDir := filepath.Join(root, "cmd", "scanner", "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	mustWriteGoFixture(t, filepath.Join(root, "nomos.yaml"), "project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    substrate_source_roots:\n      - cmd/scanner/mcp\n")

	ctx := &VerifyContext{RepoRoot: root}
	pkgs := resolveGoTestPackages(ctx, root, root)
	if len(pkgs) != 1 || pkgs[0] != "./cmd/scanner/mcp/..." {
		t.Errorf("expected [./cmd/scanner/mcp/...], got %v", pkgs)
	}

	mustWriteGoFixture(t, filepath.Join(root, "nomos.yaml"), "project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: true\n    substrate_source_roots:\n      - cmd/scanner/mcp\n")
	strictPkgs := resolveGoTestPackages(ctx, root, root)
	if len(strictPkgs) != 1 || strictPkgs[0] != "./..." {
		t.Errorf("expected [./...] in strict mode, got %v", strictPkgs)
	}

	t.Setenv("GOWORK", "")
	t.Setenv("GOTOOLCHAIN", "")
	env := strings.Join(buildGoToolchainEnv(root, root), "\n")
	if !strings.Contains(env, "GOWORK=off") || !strings.Contains(env, "GOTOOLCHAIN=auto") {
		t.Errorf("expected GOWORK=off and GOTOOLCHAIN=auto in toolchain env, got:\n%s", env)
	}
}

func mustWriteGoFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}
