package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-SSOT-01 TC-SSOT-02
func TestGenerateAndVerifyConfigJSONSchema(t *testing.T) {
	raw, err := GenerateConfigJSONSchema()
	if err != nil {
		t.Fatalf("GenerateConfigJSONSchema failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("expected non-empty JSON schema output")
	}

	// Determinism check: consecutive invocations must produce identical bytes.
	second, err := GenerateConfigJSONSchema()
	if err != nil {
		t.Fatalf("second GenerateConfigJSONSchema failed: %v", err)
	}
	if string(raw) != string(second) {
		t.Fatalf("expected byte-identical deterministic JSON schema output")
	}

	var parsed JSONSchemaNode
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("generated schema is not valid JSON: %v", err)
	}
	if parsed.Schema != JSONSchemaDraft202012URI {
		t.Errorf("expected $schema %s, got %s", JSONSchemaDraft202012URI, parsed.Schema)
	}
	if parsed.AdditionalProperties == nil || *parsed.AdditionalProperties {
		t.Errorf("expected top-level additionalProperties=false for strict typo rejection")
	}
	gov, ok := parsed.Properties["governance"]
	if !ok || gov == nil {
		t.Fatalf("expected governance property in root schema")
	}
	qg, ok := gov.Properties["quality_gates"]
	if !ok || qg == nil {
		t.Fatalf("expected quality_gates property under governance")
	}
	if _, exists := qg.Properties["hasforbiddenliteralsoverride"]; exists {
		t.Errorf("internal json:\"-\" field HasForbiddenLiteralsOverride must not leak into JSON schema")
	}
	thresholds, ok := qg.Properties["thresholds"]
	if !ok || thresholds == nil {
		t.Fatalf("expected thresholds object under quality_gates")
	}
	maxLOC, ok := thresholds.Properties["max_file_loc"]
	if !ok || maxLOC == nil || maxLOC.Type != "integer" {
		t.Fatalf("expected integer max_file_loc schema node, got %+v", maxLOC)
	}

	if err := VerifyConfigJSONSchemaParity(raw); err != nil {
		t.Fatalf("expected generated schema to pass parity check: %v", err)
	}

	drifted := strings.Replace(string(raw), `"max_file_loc"`, `"max_file_lines_drifted"`, 1)
	if err := VerifyConfigJSONSchemaParity([]byte(drifted)); err == nil {
		t.Fatalf("expected drifted schema payload to fail closed in VerifyConfigJSONSchemaParity")
	}
}

// @verifies TC-SSOT-06
func TestMaxPackageFilesDefaultAndSchema(t *testing.T) {
	def := DefaultNomosConfig()
	if def.Governance.QualityGates.Thresholds.MaxPackageFiles != 40 {
		t.Fatalf("expected DefaultNomosConfig MaxPackageFiles=40, got %d", def.Governance.QualityGates.Thresholds.MaxPackageFiles)
	}
	raw, err := GenerateConfigJSONSchema()
	if err != nil {
		t.Fatalf("GenerateConfigJSONSchema failed: %v", err)
	}
	var parsed JSONSchemaNode
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	maxPkg := parsed.Properties["governance"].Properties["quality_gates"].Properties["thresholds"].Properties["max_package_files"]
	if maxPkg == nil || maxPkg.Type != "integer" || maxPkg.Minimum == nil || *maxPkg.Minimum != 1 || maxPkg.Default != float64(40) {
		t.Fatalf("expected integer max_package_files schema node with min=1 and default=40, got %+v", maxPkg)
	}
}

// @verifies TC-SSOT-07
func TestRatchetModeAndCorpusCommandSchema(t *testing.T) {
	raw, err := GenerateConfigJSONSchema()
	if err != nil {
		t.Fatalf("GenerateConfigJSONSchema failed: %v", err)
	}
	var parsed JSONSchemaNode
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	qg := parsed.Properties["governance"].Properties["quality_gates"]
	rm := qg.Properties["ratchet_mode"]
	if rm == nil || rm.Type != "string" || rm.Default != RatchetModeStandard || len(rm.Enum) != 2 {
		t.Fatalf("expected ratchet_mode enum [standard, upstream] with default standard, got %+v", rm)
	}
	cc := qg.Properties["corpus_command"]
	if cc == nil || cc.Type != "string" || cc.Description == "" {
		t.Fatalf("expected corpus_command string schema node, got %+v", cc)
	}
	cfg := DefaultNomosConfig()
	cfg.Governance.QualityGates.RatchetMode = "invalid_mode"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatalf("expected invalid ratchet_mode to fail ValidateConfig")
	}
}

func TestWorkspaceNomosSchemaByteParity(t *testing.T) {
	schemaPath := filepath.Clean("../../../../../nomos.schema.json")
	if os.Getenv("NOMOS_WRITE_SCHEMA") == "1" {
		raw, gErr := GenerateConfigJSONSchema()
		if gErr != nil {
			t.Fatalf("GenerateConfigJSONSchema failed: %v", gErr)
		}
		if wErr := os.WriteFile(schemaPath, raw, 0644); wErr != nil {
			t.Fatalf("write nomos.schema.json: %v", wErr)
		}
	}
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Skipf("workspace nomos.schema.json not present at %s: %v", schemaPath, err)
	}
	if err := VerifyConfigJSONSchemaParity(data); err != nil {
		t.Fatalf("root nomos.schema.json drifted from compiled EcosystemConfig: %v", err)
	}
}

// @verifies TC-SSOT-05
func TestLoadConfigRejectsUnknownKeysAndDisambiguatesContractsDescriptions(t *testing.T) {
	tmpDir := t.TempDir()
	misnested := filepath.Join(tmpDir, "nomos.yaml")
	badYAML := "project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    contracts:\n      enabled: true\n"
	if err := os.WriteFile(misnested, []byte(badYAML), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadConfig(misnested); err == nil {
		t.Fatalf("expected KnownFields(true) to reject mis-nested quality_gates.contracts.enabled")
	}

	misplacedRoot := filepath.Join(tmpDir, "misplaced-root.yaml")
	if err := os.WriteFile(misplacedRoot, []byte("project: demo\nquality_gates:\n  strict_mode: true\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadConfig(misplacedRoot); err == nil {
		t.Fatalf("expected root-level quality_gates mis-indentation to fail closed")
	}

	composedYAML := filepath.Join(tmpDir, "composed.yaml")
	extBody := "project: demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\nplugins:\n  analytics:\n    enabled: true\n"
	if err := os.WriteFile(composedYAML, []byte(extBody), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	loadedComposed, err := LoadConfig(composedYAML)
	if err != nil || loadedComposed.Project != "demo" || !loadedComposed.Governance.QualityGates.StrictMode {
		t.Fatalf("expected inline plugin extension block to coexist with strict governance defaults, got cfg=%+v err=%v", loadedComposed, err)
	}

	commentOnly := filepath.Join(tmpDir, "comments.yaml")
	if err := os.WriteFile(commentOnly, []byte("# comment only\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadConfig(commentOnly); err != nil {
		t.Fatalf("expected comment-only config to fall back to defaults cleanly: %v", err)
	}

	rawSchema, err := GenerateConfigJSONSchema()
	if err != nil {
		t.Fatalf("GenerateConfigJSONSchema: %v", err)
	}
	var root JSONSchemaNode
	if err := json.Unmarshal(rawSchema, &root); err != nil {
		t.Fatalf("Unmarshal schema: %v", err)
	}
	outer := root.Properties["governance"].Properties["quality_gates"].Properties["contracts"]
	inner := outer.Properties["contracts"]
	if outer == nil || inner == nil || outer.Description == "" || inner.Description == "" || outer.Description == inner.Description {
		t.Fatalf("expected distinct outer vs inner contracts descriptions, got outer=%q inner=%q", outer.Description, inner.Description)
	}
}
