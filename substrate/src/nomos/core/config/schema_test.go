package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestWorkspaceNomosSchemaByteParity(t *testing.T) {
	schemaPath := filepath.Clean("../../../../../nomos.schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Skipf("workspace nomos.schema.json not present at %s: %v", schemaPath, err)
	}
	if err := VerifyConfigJSONSchemaParity(data); err != nil {
		t.Fatalf("root nomos.schema.json drifted from compiled EcosystemConfig: %v", err)
	}
}
