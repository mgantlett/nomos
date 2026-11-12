package impact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractProtocolMarkerBlock_AndMarkerParity(t *testing.T) {
	canonicalBlock := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Nomos Protocol\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->"
	withCustomSuffix := canonicalBlock + "\n\n---\n\n# Project Specific Guidelines\n- Use PostgreSQL\n"

	b1, ok1 := ExtractProtocolMarkerBlock([]byte(canonicalBlock))
	b2, ok2 := ExtractProtocolMarkerBlock([]byte(withCustomSuffix))
	if !ok1 || !ok2 {
		t.Fatalf("expected marker extraction to succeed: ok1=%v ok2=%v", ok1, ok2)
	}
	if string(b1) != string(b2) {
		t.Fatalf("expected identical extracted protocol blocks, got %q vs %q", string(b1), string(b2))
	}

	root := t.TempDir()
	tplDir := filepath.Join(root, "substrate", "cli", "templates")
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "AGENTS.md"), []byte(canonicalBlock+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(withCustomSuffix), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("@AGENTS.md\n\n# Custom Claude Notes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	drifts := AuditTemplateByteParity(root)
	if len(drifts) != 0 {
		t.Fatalf("expected zero parity drift when custom project notes follow protocol end marker and CLAUDE.md imports @AGENTS.md, got: %v", drifts)
	}

	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(withCustomSuffix), 0644); err != nil {
		t.Fatal(err)
	}
	if claudeDrifts := AuditTemplateByteParity(root); len(claudeDrifts) == 0 {
		t.Fatalf("expected parity drift when CLAUDE.md duplicates managed protocol block instead of @AGENTS.md import")
	}
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("@AGENTS.md\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tampered := strings.Replace(withCustomSuffix, "# Nomos Protocol", "# Tampered Protocol", 1)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(tampered), 0644); err != nil {
		t.Fatal(err)
	}
	driftsAfterTamper := AuditTemplateByteParity(root)
	if len(driftsAfterTamper) == 0 {
		t.Fatalf("expected parity drift when protocol block inside markers is modified")
	}
}
