// Package config provides deterministic JSON Schema Draft 2020-12 synthesis
// and byte-parity verification derived directly from EcosystemConfig (Law 4).
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const (
	// JSONSchemaDraft202012URI is the canonical JSON Schema Draft 2020-12 meta-schema identifier.
	JSONSchemaDraft202012URI = "https://json-schema.org/draft/2020-12/schema"
	// NomosConfigSchemaID is the canonical public schema identifier for nomos.yaml.
	NomosConfigSchemaID = "https://github.com/mgantlett/nomos/blob/main/nomos.schema.json"
)

// JSONSchemaNode models a deterministic JSON Schema Draft 2020-12 node.
type JSONSchemaNode struct {
	Schema               string                     `json:"$schema,omitempty"`
	ID                   string                     `json:"$id,omitempty"`
	Title                string                     `json:"title,omitempty"`
	Description          string                     `json:"description,omitempty"`
	Type                 string                     `json:"type,omitempty"`
	Enum                 []string                   `json:"enum,omitempty"`
	Default              any                        `json:"default,omitempty"`
	Minimum              *int                       `json:"minimum,omitempty"`
	Maximum              *int                       `json:"maximum,omitempty"`
	Items                *JSONSchemaNode            `json:"items,omitempty"`
	Properties           map[string]*JSONSchemaNode `json:"properties,omitempty"`
	AdditionalProperties *bool                      `json:"additionalProperties,omitempty"`
}

var configFieldDescriptions = map[string]string{
	"project":                      "Canonical repository or project identifier.",
	"default_branch":               "Protected trunk branch name (default: main).",
	"governance":                   "Boundary enforcement and Definition of Done quality gate policies.",
	"boundary":                     "Package import and CLI command boundary protections.",
	"enforce":                      "Enforce declared boundary constraints during verification.",
	"prohibited_packages":          "Import path substrings disallowed inside this repository.",
	"prohibited_commands":          "CLI subcommands disallowed inside this repository.",
	"lock_dependencies":            "Reject unapproved dependency graph mutations in worktrees.",
	"allowed_mcp_servers":          "Explicit allowlist of permitted MCP server identifiers on PreToolUse.",
	"blocked_mcp_servers":          "Explicit denylist of prohibited MCP server identifiers on PreToolUse.",
	"egress_mode":                  "Outbound agent HTTP/HTTPS egress policy on PreToolUse: standard (default), offline, or allowlist.",
	"allowed_egress_domains":       "Explicit allowlist of permitted outbound domains when egress_mode is allowlist.",
	"blocked_egress_domains":       "Explicit denylist of prohibited outbound domains blocked on PreToolUse.",
	"quality_gates":                "Static AST, test hygiene, and cross-plane verification settings.",
	"forbidden_literals":           "Hardcoded string or path literals rejected by Law 5.",
	"forbidden_shims":              "Stub or placeholder patterns rejected by the Anti-Shim Gate.",
	"strict_mode":                  "Enforce statutory baseline thresholds without relaxation.",
	"verify_scope":                 "Default verification scope inside task worktrees: diff (default, evaluate modified files and packages) or repo (scan full repository on every verification).",
	"intent_twin_mode":             "Cross-plane INT twin enforcement for SUB commits: off (default), feat (require INT twin on feat commits), or strict (require INT twin on all SUB commits).",
	"tdd_mode":                     "Paired unit test enforcement policy: strict (default when strict_mode=true), ratchet (default when strict_mode=false), diff (require paired or modified test on every touched file), or off.",
	"tdd_exclude":                  "Workspace-relative glob patterns excluded from paired unit test enforcement in TDDGate.",
	"coverage_mode":                "Statement coverage enforcement policy: live (default zero-dep live probes plus artifacts), artifact (on-disk coverage reports only), or off.",
	"polyglot_toolchains":          "Enable automatic polyglot test/linter toolchain discovery.",
	"exclude":                      "Workspace-relative glob patterns excluded from AST gates.",
	"intent_source_roots":          "Directories classified under the Intent plane (Law 7).",
	"substrate_source_roots":       "Directories classified under the Substrate plane (Law 7).",
	"architecture_boundaries":      "Declarative directional package import rules enforced across Go AST and polyglot source files.",
	"name":                         "Optional human-readable identifier for an architecture boundary rule.",
	"from":                         "Workspace-relative source file glob governed by this boundary rule.",
	"forbid_imports":               "Import path patterns disallowed from matching source files.",
	"allow_only":                   "Exclusive allowlist of permitted import path patterns for matching source files.",
	"contracts":                    "Contract schema parity, UI membrane, and ingress boundaries.",
	"spec_roots":                   "Intent specification directories audited for contract grounding.",
	"schema_files":                 "Generated JSON Schema artifacts verified for Go SSoT byte parity.",
	"backend_roots":                "Backend source directories verified against contract schemas.",
	"strict_types":                 "Require strict structural type alignment across boundaries.",
	"verify_obligations":           "Require executable test bindings (@verifies TC-*) for all specification TC-* vectors.",
	"fail_on_stale_obligations":    "Reject verification when an Intent specification changed after its latest SUB-* receipt.",
	"membrane":                     "Frontend-to-backend DTO boundary verification settings.",
	"frontend_roots":               "Frontend component directories audited for defensive masks.",
	"backend_dto_roots":            "Backend DTO directories paired with frontend roots.",
	"banish_defensive_masks":       "Reject silent fallback masks that conceal contract drift.",
	"fail_on_contract_drift":       "Fail verification when DTO shapes diverge across boundaries.",
	"infra":                        "Environment variable and port ingress parity configuration.",
	"ingress_configs":              "Infrastructure manifest paths audited for port collisions.",
	"env_declarations":             "Environment template files (for example .env.example).",
	"port_scan":                    "Audit declared service ports for multi-seat collisions.",
	"fail_on_orphan_env":           "Fail when code reads environment variables absent from templates.",
	"auto_promote":                 "Allow automatic graduation of recurring verified invariants.",
	"thresholds":                   "Numerical AST complexity, size, clone, and coverage bounds.",
	"max_cognitive_debt_score":     "Maximum allowable per-file Cognitive Debt Score (statutory: 45).",
	"max_cyclomatic_complexity":    "Maximum per-function cyclomatic complexity (statutory: 15).",
	"max_file_loc":                 "Maximum lines of code per source file (statutory: 400).",
	"max_func_loc":                 "Maximum lines of code per function (statutory: 60).",
	"min_comment_density_pct":      "Minimum comment density percentage (0-100).",
	"min_test_coverage_pct":        "Minimum statement test coverage percentage (statutory: 55).",
	"min_clone_lines":              "Minimum AST line span flagged as a structural Type-2 clone.",
	"max_func_params":              "Maximum parameter count per function signature (statutory: 8).",
	"max_magic_string_duplication": "Maximum repeated string literal occurrences per package.",
}

// GenerateConfigJSONSchema synthesizes the deterministic Draft 2020-12 JSON Schema
// for nomos.yaml directly from EcosystemConfig and DefaultNomosConfig (@intent: #EcosystemConfig).
func GenerateConfigJSONSchema() ([]byte, error) {
	defaults := DefaultNomosConfig()
	rootNode := reflectStructSchema(reflect.TypeOf(*defaults), reflect.ValueOf(*defaults))
	rootNode.Schema = JSONSchemaDraft202012URI
	rootNode.ID = NomosConfigSchemaID
	rootNode.Title = "Nomos Ecosystem Configuration (nomos.yaml)"
	rootNode.Description = "Deterministic configuration schema synthesized via Go reflection from config.EcosystemConfig (Law 4 Single Source of Truth)."

	encoded, err := json.MarshalIndent(rootNode, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed marshaling EcosystemConfig JSON schema: %w", err)
	}
	return append(encoded, '\n'), nil
}

// VerifyConfigJSONSchemaParity asserts that the on-disk nomos.schema.json matches
// the compiled EcosystemConfig SSoT byte-for-byte (@intent: #EcosystemConfig).
func VerifyConfigJSONSchemaParity(onDisk []byte) error {
	expected, err := GenerateConfigJSONSchema()
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.TrimSpace(onDisk), bytes.TrimSpace(expected)) {
		return fmt.Errorf("nomos.schema.json has drifted from compiled config.EcosystemConfig SSoT (Law 4)")
	}
	return nil
}

func reflectStructSchema(t reflect.Type, defVal reflect.Value) *JSONSchemaNode {
	closed := false
	props := make(map[string]*JSONSchemaNode)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, skip := parseJSONTagName(field)
		if skip {
			continue
		}
		var fieldDef reflect.Value
		if defVal.IsValid() && defVal.Kind() == reflect.Struct {
			fieldDef = defVal.Field(i)
		}
		propNode := reflectFieldSchema(field.Type, fieldDef, name)
		if desc, ok := configFieldDescriptions[name]; ok && propNode.Description == "" {
			propNode.Description = desc
		}
		applyStringEnumMetadata(name, propNode)
		applyNumericBounds(name, propNode)
		props[name] = propNode
	}

	return &JSONSchemaNode{
		Type:                 "object",
		Properties:           props,
		AdditionalProperties: &closed,
	}
}

func applyStringEnumMetadata(name string, propNode *JSONSchemaNode) {
	switch name {
	case "verify_scope":
		propNode.Enum = []string{VerifyScopeDiff, VerifyScopeRepo}
		propNode.Default = VerifyScopeDiff
	case "intent_twin_mode":
		propNode.Enum = []string{IntentTwinModeOff, IntentTwinModeFeat, IntentTwinModeStrict}
		propNode.Default = IntentTwinModeOff
	case "tdd_mode":
		propNode.Enum = []string{TDDModeOff, TDDModeRatchet, TDDModeDiff, TDDModeStrict}
		propNode.Default = TDDModeStrict
	case "coverage_mode":
		propNode.Enum = []string{CoverageModeOff, CoverageModeArtifact, CoverageModeLive}
		propNode.Default = CoverageModeLive
	case "egress_mode":
		propNode.Enum = []string{EgressModeStandard, EgressModeOffline, EgressModeAllowlist}
		propNode.Default = EgressModeStandard
	}
}

func reflectFieldSchema(ft reflect.Type, defVal reflect.Value, fieldName string) *JSONSchemaNode {
	if ft.Kind() == reflect.Ptr {
		var elemVal reflect.Value
		if defVal.IsValid() && !defVal.IsNil() {
			elemVal = defVal.Elem()
		}
		return reflectFieldSchema(ft.Elem(), elemVal, fieldName)
	}

	switch ft.Kind() {
	case reflect.Struct:
		return reflectStructSchema(ft, defVal)
	case reflect.Slice:
		elemNode := reflectFieldSchema(ft.Elem(), reflect.Value{}, "")
		return &JSONSchemaNode{Type: "array", Items: elemNode}
	default:
		return reflectScalarSchema(ft, defVal, fieldName)
	}
}

func reflectScalarSchema(ft reflect.Type, defVal reflect.Value, fieldName string) *JSONSchemaNode {
	switch ft.Kind() {
	case reflect.Bool:
		node := &JSONSchemaNode{Type: "boolean"}
		if defVal.IsValid() && defVal.Bool() {
			node.Default = true
		}
		return node
	case reflect.Int, reflect.Int64:
		node := &JSONSchemaNode{Type: "integer"}
		if defVal.IsValid() && defVal.Int() != 0 {
			node.Default = defVal.Int()
		}
		return node
	default:
		node := &JSONSchemaNode{Type: "string"}
		if isNonEmptyDefaultString(defVal, fieldName) {
			node.Default = defVal.String()
		}
		return node
	}
}

func isNonEmptyDefaultString(defVal reflect.Value, fieldName string) bool {
	if !defVal.IsValid() || defVal.Kind() != reflect.String {
		return false
	}
	return defVal.String() != "" && fieldName != "project"
}

func parseJSONTagName(field reflect.StructField) (string, bool) {
	raw := field.Tag.Get("json")
	if raw == "-" {
		return "", true
	}
	if raw == "" {
		raw = field.Tag.Get("yaml")
	}
	if raw == "-" || raw == "" {
		return strings.ToLower(field.Name), raw == "-"
	}
	parts := strings.Split(raw, ",")
	name := strings.TrimSpace(parts[0])
	if name == "" || name == "-" {
		return "", true
	}
	return name, false
}

func applyNumericBounds(fieldName string, node *JSONSchemaNode) {
	if node == nil || node.Type != "integer" {
		return
	}
	zero := 0
	one := 1
	hundred := 100
	if strings.HasSuffix(fieldName, "_pct") {
		node.Minimum = &zero
		node.Maximum = &hundred
		return
	}
	if strings.HasPrefix(fieldName, "max_") || strings.HasPrefix(fieldName, "min_") {
		node.Minimum = &one
	}
}
