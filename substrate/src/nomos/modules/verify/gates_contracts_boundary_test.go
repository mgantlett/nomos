package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func TestContractParityGate(t *testing.T) {
	gate := &ContractParityGate{}
	if gate.Name() != "Contract Schema Integrity Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	tempDir := t.TempDir()
	ctx := &vscope.VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir}

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
        schema_files: ["nomos.schema.json"]
        backend_roots: ["substrate/src/nomos"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed writing test nomos.yaml: %v", err)
	}

	// In an empty brownfield directory (no intent directory or schema file), contract boundary skips gracefully
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected brownfield skip on empty dir, got failure: %v", res)
	}

	// If intent/ exists without nomos.schema.json, it must fail closed
	intentDir := filepath.Join(tempDir, "intent")
	_ = os.MkdirAll(intentDir, 0755)
	resMissingSchema, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resMissingSchema.Passed {
		t.Errorf("expected failure when intent scope exists without nomos.schema.json")
	}

	// Drifted nomos.schema.json must fail closed
	_ = os.WriteFile(filepath.Join(tempDir, "nomos.schema.json"), []byte(`{"type":"object"}`), 0644)
	resDrifted, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resDrifted.Passed || !strings.Contains(resDrifted.Message, "drifted") {
		t.Errorf("expected failure on drifted nomos.schema.json, got: %+v", resDrifted)
	}

	// Valid generated nomos.schema.json and Go backend files pass cleanly
	schemaBytes, sErr := config.GenerateConfigJSONSchema()
	if sErr != nil {
		t.Fatalf("failed generating schema: %v", sErr)
	}
	_ = os.WriteFile(filepath.Join(tempDir, "nomos.schema.json"), schemaBytes, 0644)
	goDir := filepath.Join(tempDir, "substrate", "src", "nomos")
	_ = os.MkdirAll(goDir, 0755)
	_ = os.WriteFile(filepath.Join(goDir, "main.go"), []byte("package main"), 0644)

	resGood, errGood := gate.Evaluate(ctx)
	if errGood != nil || !resGood.Passed {
		t.Errorf("expected pass with valid nomos.schema.json and Go files: %v, %v", resGood, errGood)
	}
}

func TestInfraPortIngressGate(t *testing.T) {
	gate := &InfraPortIngressGate{}
	if gate.Name() != "Port Ingress Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	tempDir := t.TempDir()
	ctx := &vscope.VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir}

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

	ctx := &vscope.VerifyContext{}
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

	ctx := &vscope.VerifyContext{
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

// TestContractParityGate_OptionalDefaultSchema verifies omitted schema_files behavior.
// @intent nomos_intent_traceability_spec.md #ContractsConfig
// @verifies TC-TRC-005
func TestContractParityGate_OptionalDefaultSchema(t *testing.T) {
	gate := &ContractParityGate{}
	cases := []struct {
		name         string
		schemaConfig string
		writeBackend bool
		writeDrifted bool
		wantPass     bool
		wantSub      string
	}{
		{
			name:         "omitted_schema_files_without_root_schema_enforces_backend_roots",
			schemaConfig: "",
			writeBackend: true,
			wantPass:     true,
		},
		{
			name:         "omitted_schema_files_fails_when_backend_roots_empty_on_disk",
			schemaConfig: "",
			writeBackend: false,
			wantPass:     false,
			wantSub:      "0 polyglot backend files",
		},
		{
			name:         "omitted_schema_files_still_verifies_root_schema_when_present",
			schemaConfig: "",
			writeBackend: true,
			writeDrifted: true,
			wantPass:     false,
			wantSub:      "drifted",
		},
		{
			name:         "explicit_schema_files_fails_closed_when_missing",
			schemaConfig: "        schema_files: [\"nomos.schema.json\"]\n",
			writeBackend: true,
			wantPass:     false,
			wantSub:      "missing or unreadable schema artifact",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "intent"), 0755); err != nil {
				t.Fatalf("mkdir intent: %v", err)
			}
			cfgYAML := "project: schema-opt-test\ndefault_branch: main\ngovernance:\n  quality_gates:\n    contracts:\n      contracts:\n        enabled: true\n" + tc.schemaConfig + "        backend_roots: [\"substrate/pkg\"]\n"
			if err := os.WriteFile(filepath.Join(root, "nomos.yaml"), []byte(cfgYAML), 0644); err != nil {
				t.Fatalf("write nomos.yaml: %v", err)
			}
			if tc.writeBackend {
				pkgDir := filepath.Join(root, "substrate", "pkg")
				if err := os.MkdirAll(pkgDir, 0755); err != nil {
					t.Fatalf("mkdir pkg: %v", err)
				}
				if err := os.WriteFile(filepath.Join(pkgDir, "svc.go"), []byte("package pkg\n"), 0644); err != nil {
					t.Fatalf("write svc.go: %v", err)
				}
			}
			if tc.writeDrifted {
				if err := os.WriteFile(filepath.Join(root, "nomos.schema.json"), []byte(`{"type":"object"}`), 0644); err != nil {
					t.Fatalf("write drifted schema: %v", err)
				}
			}
			res, err := gate.Evaluate(&vscope.VerifyContext{RepoRoot: root, WorktreePath: root})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Passed != tc.wantPass {
				t.Fatalf("expected Passed=%v, got res=%+v", tc.wantPass, res)
			}
			if tc.wantSub != "" && !strings.Contains(res.Message, tc.wantSub) {
				t.Fatalf("expected message containing %q, got %q", tc.wantSub, res.Message)
			}
		})
	}
}
