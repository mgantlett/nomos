package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDirectiveCmd_ListText(t *testing.T) {
	cmd := NewDirectiveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("directive list failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Nomos Just-In-Time (JIT) Directives") {
		t.Errorf("output missing title banner: %s", out)
	}
	if !strings.Contains(out, "00_index_and_taxonomy.md") {
		t.Errorf("output missing 00_index_and_taxonomy.md: %s", out)
	}
	if !strings.Contains(out, "edit_01_deprecated_terms.md") {
		t.Errorf("output missing edit_01_deprecated_terms.md: %s", out)
	}
}

func TestDirectiveCmd_ListJSON(t *testing.T) {
	cmd := NewDirectiveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("directive list --json failed: %v", err)
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed unmarshaling json output: %v, raw: %s", err, buf.String())
	}
	if len(parsed) < 17 {
		t.Errorf("expected at least 17 directives in JSON list, got %d", len(parsed))
	}
}

func TestDirectiveCmd_ShowDirective(t *testing.T) {
	cmd := NewDirectiveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"edit_01"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("directive show edit_01 failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Deprecated Terms & Taxonomy Directives") {
		t.Errorf("output missing expected heading: %s", out)
	}
}

func TestDirectiveCmd_ShowNonexistent(t *testing.T) {
	cmd := NewDirectiveCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"nonexistent_directive_foo"})

	if err := cmd.Execute(); err == nil {
		t.Errorf("expected error for nonexistent directive, got nil")
	}
}
