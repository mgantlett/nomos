package diag

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestStatusCmd_Text(t *testing.T) {
	cmd := NewStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute failed: %v", err)
	}
}

func TestStatusCmd_JSON(t *testing.T) {
	cmd := NewStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute with --json failed: %v", err)
	}

	report := CollectStatusReport()
	if report == nil {
		t.Fatalf("collectStatusReport returned nil")
	}
	bytesOut, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("failed marshaling report: %v", err)
	}
	var parsed StatusReport
	if err := json.Unmarshal(bytesOut, &parsed); err != nil {
		t.Fatalf("failed unmarshaling report: %v", err)
	}
	if parsed.Phase == "" {
		t.Errorf("expected non-empty phase in status report")
	}
}
