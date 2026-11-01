package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizedGitEnv(t *testing.T) {
	t.Setenv("GIT_DIR", "/some/git/dir")
	t.Setenv("GIT_WORK_TREE", "/some/work/tree")
	t.Setenv("GIT_INDEX_FILE", "/some/index")
	t.Setenv("GIT_OBJECT_DIRECTORY", "/some/objects")
	t.Setenv("GIT_PREFIX", "sub/dir")
	t.Setenv("CUSTOM_ENV_VAR", "preserved")

	env := SanitizedGitEnv()
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") ||
			strings.HasPrefix(e, "GIT_OBJECT_DIRECTORY=") ||
			strings.HasPrefix(e, "GIT_PREFIX=") {
			t.Errorf("SanitizedGitEnv failed to strip %s", e)
		}
	}

	foundCustom := false
	for _, e := range env {
		if e == "CUSTOM_ENV_VAR=preserved" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Errorf("SanitizedGitEnv failed to preserve non-git environment variable")
	}
}

func TestFindRepoRoot(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)

	subDir := filepath.Join(repoDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed creating subDir: %v", err)
	}

	tests := []struct {
		name     string
		startDir string
	}{
		{"root dir", repoDir},
		{"nested subdir", subDir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := FindRepoRoot(tt.startDir)
			if err != nil {
				t.Fatalf("FindRepoRoot(%s) failed: %v", tt.startDir, err)
			}
			if root != repoDir {
				t.Errorf("FindRepoRoot(%s) = %s; want %s", tt.startDir, root, repoDir)
			}
		})
	}
}

func TestFindRepoRoot_Error(t *testing.T) {
	tempDir, err := os.MkdirTemp("/tmp", "nomos-non-git-*")
	if err != nil {
		t.Fatalf("failed creating temp dir in /tmp: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root, err := FindRepoRoot(tempDir)
	if err == nil {
		t.Errorf("expected error when finding repo root in non-git directory %s, got root %s", tempDir, root)
	}
}

