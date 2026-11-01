package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContractParityGate(t *testing.T) {
	gate := &ContractParityGate{}
	if gate.Name() != "Contract Schema Integrity Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	tempDir := t.TempDir()
	ctx := &VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir}

	// Without contracts enabled in config, gate skips gracefully
	resDisabled, err := gate.Evaluate(ctx)
	if err != nil || !resDisabled.Passed {
		t.Fatalf("expected skip when contracts unconfigured: %v", resDisabled)
	}

	// Enable contracts in local nomos.yaml
	cfgContent := `
governance:
  quality_gates:
    contracts:
      contracts:
        enabled: true
        cue_roots: ["intent/contracts/cue"]
        backend_roots: ["substrate/src/nomos"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed writing test nomos.yaml: %v", err)
	}

	// In an empty brownfield directory (no intent directory), contract boundary skips gracefully
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected brownfield skip on empty dir, got failure: %v", res)
	}

	// If intent/contracts/cue exists but has 0 CUE schemas, it must fail closed
	cueDir := filepath.Join(tempDir, "intent", "contracts", "cue")
	_ = os.MkdirAll(cueDir, 0755)
	resEmptyCue, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resEmptyCue.Passed {
		t.Errorf("expected failure when intent CUE root exists but has 0 schemas")
	}

	// Scaffold CUE and Go roots matching default config
	goDir := filepath.Join(tempDir, "substrate", "src", "nomos")
	_ = os.MkdirAll(goDir, 0755)
	_ = os.WriteFile(filepath.Join(cueDir, "task.cue"), []byte("package schemas"), 0644)
	_ = os.WriteFile(filepath.Join(goDir, "main.go"), []byte("package main"), 0644)

	resGood, errGood := gate.Evaluate(ctx)
	if errGood != nil || !resGood.Passed {
		t.Errorf("expected pass with discovered CUE and Go files: %v, %v", resGood, errGood)
	}
}

func TestInfraPortIngressGate(t *testing.T) {
	gate := &InfraPortIngressGate{}
	if gate.Name() != "Port Ingress Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	tempDir := t.TempDir()
	ctx := &VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir}

	// Without infra enabled in config, gate skips gracefully
	resDisabled, err := gate.Evaluate(ctx)
	if err != nil || !resDisabled.Passed {
		t.Fatalf("expected skip when infra unconfigured: %v", resDisabled)
	}

	// Enable infra in local nomos.yaml
	cfgContent := `
governance:
  quality_gates:
    contracts:
      infra:
        enabled: true
        env_declarations: [".env.example"]
        fail_on_orphan_env: true
`
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed writing test nomos.yaml: %v", err)
	}

	res, err := gate.Evaluate(ctx)
	if err != nil || !res.Passed {
		t.Fatalf("expected clean pass on empty dir, got: %v", res)
	}

	// Create a code file using os.Getenv("PORT")
	codeDir := filepath.Join(tempDir, "src")
	_ = os.MkdirAll(codeDir, 0755)
	_ = os.WriteFile(filepath.Join(codeDir, "server.go"), []byte("package main\nimport \"os\"\nfunc run() { _ = os.Getenv(\"PORT\") }\n"), 0644)

	// Create .env.example missing PORT
	_ = os.WriteFile(filepath.Join(tempDir, ".env.example"), []byte("DATABASE_URL=postgres://...\n"), 0644)

	resMissing, _ := gate.Evaluate(ctx)
	if resMissing.Passed {
		t.Errorf("expected failure when environment variable not in .env.example")
	}

	// Add PORT to .env.example
	_ = os.WriteFile(filepath.Join(tempDir, ".env.example"), []byte("PORT=8080\nDATABASE_URL=postgres://...\n"), 0644)
	resGood, _ := gate.Evaluate(ctx)
	if !resGood.Passed {
		t.Errorf("expected pass when PORT is declared in .env.example: %v", resGood)
	}
}

func TestUIAntiMaskSentinelGate(t *testing.T) {
	gate := &UIAntiMaskSentinelGate{}
	if gate.Name() != "UI Anti-Mask Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	ctx := &VerifyContext{}
	res, err := gate.Evaluate(ctx)
	if err != nil || !res.Passed {
		t.Errorf("expected pass when membrane boundary disabled: %v", res)
	}
}

func TestContractParityGate_WorktreeInheritance(t *testing.T) {
	gate := &ContractParityGate{}
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "sub-1")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	nomosYAML := "project: test-downstream\ndefault_branch: main\ngovernance:\n  quality_gates:\n    contracts:\n      contracts:\n        enabled: false\n"
	if err := os.WriteFile(filepath.Join(repoDir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("failed writing nomos.yaml: %v", err)
	}

	ctx := &VerifyContext{
		RepoRoot:     repoDir,
		WorktreePath: wtDir,
	}

	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected pass when contracts disabled in parent repo config, got: %s", res.Message)
	}
}
