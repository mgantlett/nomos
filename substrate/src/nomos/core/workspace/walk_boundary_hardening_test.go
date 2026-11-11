package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsExcluded_DoubleStarAndDirScopedGlobsStayContained(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	excludes := []string{
		filepath.Join(repoRoot, "legacy") + "/**",
		filepath.Join(repoRoot, "docs", "archive", "*.md"),
		filepath.Join(repoRoot, "*.pb.go"),
	}

	legacyFile := filepath.Join(repoRoot, "legacy", "sub", "old.go")
	if !IsExcluded(legacyFile, false, "old.go", excludes) {
		t.Fatalf("expected %s to be excluded by legacy/**", legacyFile)
	}

	substrateDir := filepath.Join(repoRoot, "substrate")
	if IsExcluded(substrateDir, true, "substrate", excludes) {
		t.Fatalf("expected %s NOT to be vacuously excluded by legacy/**", substrateDir)
	}

	archiveDoc := filepath.Join(repoRoot, "docs", "archive", "old.md")
	if !IsExcluded(archiveDoc, false, "old.md", excludes) {
		t.Fatalf("expected %s to be excluded by docs/archive/*.md", archiveDoc)
	}

	intentDoc := filepath.Join(repoRoot, "intent", "intent.md")
	if IsExcluded(intentDoc, false, "intent.md", excludes) {
		t.Fatalf("expected %s NOT to be excluded by docs/archive/*.md", intentDoc)
	}

	nestedPB := filepath.Join(repoRoot, "pkg", "api", "service.pb.go")
	if !IsExcluded(nestedPB, false, "service.pb.go", excludes) {
		t.Fatalf("expected %s to be excluded by root *.pb.go rule", nestedPB)
	}
}

func TestValidateUncommittedBoundaries_DeletedFileOnMainFailsClosed(t *testing.T) {
	repoRoot := t.TempDir()
	// Deleted file does not exist on disk anymore, yet deleting on protected main must still fail closed.
	err := ValidateUncommittedTaskBoundaries(repoRoot, "main", []string{"README.md"}, "")
	if err == nil {
		t.Fatalf("expected deleting tracked file on protected main branch to fail closed")
	}
}

func TestPruneOrphanWorktrees_PreservesUnregisteredDirtyGitTree(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	unregDir := filepath.Join(mgr.WorktreesDir(), "manual-SUB-999")
	if err := os.MkdirAll(unregDir, 0755); err != nil {
		t.Fatalf("mkdir unregDir: %v", err)
	}
	if out, gErr := SafeGitCommand(unregDir, "init", "-b", "main").CombinedOutput(); gErr != nil {
		t.Fatalf("git init unregDir: %v (%s)", gErr, string(out))
	}
	dirtyFile := filepath.Join(unregDir, "unsaved.go")
	if err := os.WriteFile(dirtyFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write unsaved.go: %v", err)
	}

	rep, pErr := mgr.PruneOrphanWorktrees(false, map[string]bool{})
	if pErr != nil {
		t.Fatalf("PruneOrphanWorktrees: %v", pErr)
	}
	if _, sErr := os.Stat(dirtyFile); sErr != nil {
		t.Fatalf("expected dirty unregistered git worktree to be preserved, stat err: %v (rep=%+v)", sErr, rep)
	}
}
