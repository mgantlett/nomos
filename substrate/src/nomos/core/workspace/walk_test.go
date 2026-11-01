package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalk_Excludes(t *testing.T) {
	root := t.TempDir()

	// Create directories
	dirs := []string{
		filepath.Join(root, ".git"),
		filepath.Join(root, "worktrees", "nomos-SUB-1"),
		filepath.Join(root, "vendor", "somepkg"),
		filepath.Join(root, "node_modules", "express"),
		filepath.Join(root, "substrate", "src"),
		filepath.Join(root, "intent", "specs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed creating dir %s: %v", d, err)
		}
	}

	// Create files
	files := []string{
		filepath.Join(root, ".git", "config"),
		filepath.Join(root, "worktrees", "nomos-SUB-1", "file.go"),
		filepath.Join(root, "vendor", "somepkg", "lib.go"),
		filepath.Join(root, "node_modules", "express", "index.js"),
		filepath.Join(root, "substrate", "src", "main.go"),
		filepath.Join(root, "intent", "specs", "spec.md"),
		filepath.Join(root, "custom_ignored.txt"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatalf("failed creating file %s: %v", f, err)
		}
	}

	customIgnored := filepath.Join(root, "custom_ignored.txt")
	excludes := []string{customIgnored}

	var visited []string
	err := WalkWithExcludes(root, excludes, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(root, path)
			visited = append(visited, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk failed: %v", err)
	}

	for _, v := range visited {
		if strings.HasPrefix(v, ".git") {
			t.Errorf("Walk visited .git file: %s", v)
		}
		if strings.HasPrefix(v, "worktrees") {
			t.Errorf("Walk visited worktree file: %s", v)
		}
		if strings.HasPrefix(v, "vendor") {
			t.Errorf("Walk visited vendor file: %s", v)
		}
		if strings.HasPrefix(v, "node_modules") {
			t.Errorf("Walk visited node_modules file: %s", v)
		}
		if v == "custom_ignored.txt" {
			t.Errorf("Walk visited explicit excluded file: %s", v)
		}
	}

	foundSubstrate := false
	foundIntent := false
	for _, v := range visited {
		if v == filepath.Join("substrate", "src", "main.go") {
			foundSubstrate = true
		}
		if v == filepath.Join("intent", "specs", "spec.md") {
			foundIntent = true
		}
	}

	if !foundSubstrate {
		t.Errorf("Walk did not visit substrate/src/main.go")
	}
	if !foundIntent {
		t.Errorf("Walk did not visit intent/specs/spec.md")
	}
}
