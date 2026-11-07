package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestScaffoldNomosBoundaries_AnchorsAndCreatesGitExclude(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, constants.DirGit), 0755); err != nil {
		t.Fatalf("failed creating .git dir: %v", err)
	}

	if err := scaffoldNomosBoundaries(tempDir); err != nil {
		t.Fatalf("scaffoldNomosBoundaries failed: %v", err)
	}

	localNomosPattern := "/" + constants.DirNomosHome + "/"
	for _, rel := range []string{".gitignore", filepath.Join(constants.DirGit, "info", "exclude")} {
		raw, err := os.ReadFile(filepath.Join(tempDir, rel))
		if err != nil {
			t.Fatalf("failed reading %s: %v", rel, err)
		}
		content := string(raw)
		for _, expected := range []string{"/worktrees/", "/.tmp/", localNomosPattern, "/go.work", "/go.work.sum", "*.tmp", "*.bak", "*.orig", "*.swp"} {
			if !strings.Contains(content, expected) {
				t.Errorf("expected %s in %s, got:\n%s", expected, rel, content)
			}
		}
	}
}

func TestAppendMissingPatterns_IdempotentWithUnanchoredLegacy(t *testing.T) {
	tempDir := t.TempDir()
	giPath := filepath.Join(tempDir, ".gitignore")
	initial := "worktrees/\n.tmp/\n"
	if err := os.WriteFile(giPath, []byte(initial), 0644); err != nil {
		t.Fatalf("failed writing initial .gitignore: %v", err)
	}

	localNomosPattern := "/" + constants.DirNomosHome + "/"
	if err := appendMissingPatterns(giPath, "# Nomos Operational Boundaries", []string{"/worktrees/", "/.tmp/", localNomosPattern}); err != nil {
		t.Fatalf("appendMissingPatterns failed: %v", err)
	}

	raw, err := os.ReadFile(giPath)
	if err != nil {
		t.Fatalf("failed reading .gitignore: %v", err)
	}
	content := string(raw)
	if strings.Contains(content, "\n/worktrees/\n") {
		t.Errorf("expected existing unanchored worktrees/ not to be duplicated, got:\n%s", content)
	}
	if !strings.Contains(content, localNomosPattern) {
		t.Errorf("expected missing %s to be appended, got:\n%s", localNomosPattern, content)
	}
}
