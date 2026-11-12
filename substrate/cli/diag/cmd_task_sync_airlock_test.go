package diag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

)

// @verifies TC-SYNC-021 TC-SYNC-023
func TestSanitizeAndValidateWalkthrough_AirlockAndNormalization(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "acme-repo")
	wtPath := filepath.Join(repoRoot, "worktrees", "acme-repo-SUB-42")
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	nomosYAML := "project: acme-repo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n    prohibited_packages:\n      - \"pkg/proprietary\"\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("WriteFile nomos.yaml: %v", err)
	}

	raw := "## Summary\nUpdated [README.md](file://" + filepath.ToSlash(repoRoot) + "/README.md) \u2014 inside " + filepath.ToSlash(wtPath) + "/substrate/app.go and /home/alice/custom/note.txt.\n"
	clean, err := SanitizeAndValidateWalkthrough(repoRoot, wtPath, raw, []string{"README.md", "substrate/app.go"})
	if err != nil {
		t.Fatalf("expected clean normalization, got error: %v", err)
	}
	if strings.Contains(clean, "file://") || strings.Contains(clean, repoRoot) || strings.Contains(clean, "/home/alice/") || strings.ContainsRune(clean, '\u2014') {
		t.Fatalf("expected file://, repoRoot, /home/alice/, and em-dash scrubbed, got: %s", clean)
	}
	if !strings.Contains(clean, "[README.md](README.md)") || !strings.Contains(clean, "substrate/app.go") {
		t.Fatalf("expected repo-relative paths preserved, got: %s", clean)
	}

	prohibited := "## Summary\nLeaked internal module pkg/proprietary/engine in release note.\n"
	if _, bErr := SanitizeAndValidateWalkthrough(repoRoot, wtPath, prohibited, []string{"README.md"}); bErr == nil {
		t.Fatalf("expected prohibited boundary package in walkthrough to fail closed")
	}

	fakePAT := "ghp_" + strings.Repeat("A1b2C3", 6)
	if _, sErr := SanitizeAndValidateWalkthrough(repoRoot, wtPath, "token "+fakePAT, nil); sErr == nil {
		t.Fatalf("expected raw GitHub PAT in walkthrough to fail closed")
	}
}
