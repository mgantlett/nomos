package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeClaudeImportContent(t *testing.T) {
	t.Run("empty_becomes_import_stub", func(t *testing.T) {
		got := string(mergeClaudeImportContent(nil))
		if got != "@AGENTS.md\n" {
			t.Fatalf("expected '@AGENTS.md\\n', got %q", got)
		}
	})

	t.Run("idempotent_when_import_present", func(t *testing.T) {
		in := "@AGENTS.md\n\n# Claude Specific\n- Prefer tables\n"
		got := string(mergeClaudeImportContent([]byte(in)))
		if got != in {
			t.Fatalf("expected idempotent output %q, got %q", in, got)
		}
	})

	t.Run("brownfield_prepends_import_and_preserves_custom", func(t *testing.T) {
		in := "# Custom Claude Notes\n- Keep diffs minimal\n"
		got := string(mergeClaudeImportContent([]byte(in)))
		want := "@AGENTS.md\n\n# Custom Claude Notes\n- Keep diffs minimal\n"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("collapses_legacy_marker_only_file_to_single_line", func(t *testing.T) {
		legacy := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Old Protocol\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n"
		got := string(mergeClaudeImportContent([]byte(legacy)))
		if got != "@AGENTS.md\n" {
			t.Fatalf("expected collapsed '@AGENTS.md\\n', got %q", got)
		}
	})

	t.Run("collapses_legacy_marker_with_brownfield_separator_preserving_custom", func(t *testing.T) {
		legacyWithSuffix := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Old Protocol\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n\n---\n\n# Team Claude Rules\n- Output JSON\n"
		got := string(mergeClaudeImportContent([]byte(legacyWithSuffix)))
		want := "@AGENTS.md\n\n# Team Claude Rules\n- Output JSON\n"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestScaffoldClaudeConfig_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	claudePath := filepath.Join(dir, "CLAUDE.md")

	if err := scaffoldClaudeConfig(dir); err != nil {
		t.Fatalf("scaffoldClaudeConfig fresh failed: %v", err)
	}
	raw, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if string(raw) != "@AGENTS.md\n" {
		t.Fatalf("expected fresh CLAUDE.md to equal '@AGENTS.md\\n', got %q", string(raw))
	}

	legacy := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Legacy Copy\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n\n## Extra Claude Hint\nUse subagents sparingly.\n"
	if wErr := os.WriteFile(claudePath, []byte(legacy), 0644); wErr != nil {
		t.Fatalf("write legacy CLAUDE.md: %v", wErr)
	}
	if err := scaffoldClaudeConfig(dir); err != nil {
		t.Fatalf("scaffoldClaudeConfig upgrade failed: %v", err)
	}
	upgraded, uErr := os.ReadFile(claudePath)
	if uErr != nil {
		t.Fatalf("read upgraded CLAUDE.md: %v", uErr)
	}
	uStr := string(upgraded)
	if strings.Contains(uStr, protocolStartMarker) {
		t.Fatalf("expected legacy protocol block removed from CLAUDE.md, got %q", uStr)
	}
	if !hasClaudeAgentsImportLine(uStr) || !strings.Contains(uStr, "Extra Claude Hint") {
		t.Fatalf("expected '@AGENTS.md' and custom hint preserved, got %q", uStr)
	}
}
