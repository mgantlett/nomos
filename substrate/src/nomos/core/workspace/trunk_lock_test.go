package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockAndUnlockTrunk(t *testing.T) {
	repoRoot := t.TempDir()

	intentDir := filepath.Join(repoRoot, "intent")
	substrateDir := filepath.Join(repoRoot, "substrate")
	gitDir := filepath.Join(repoRoot, ".git")
	worktreesDir := filepath.Join(repoRoot, "worktrees")

	for _, d := range []string{intentDir, substrateDir, gitDir, worktreesDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed creating dir %s: %v", d, err)
		}
	}

	testFile := filepath.Join(intentDir, "spec.md")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("failed creating testFile: %v", err)
	}

	// 1. Lock trunk
	if err := LockTrunk(repoRoot); err != nil {
		t.Fatalf("LockTrunk failed: %v", err)
	}

	// 2. Unlock trunk
	if err := UnlockTrunk(repoRoot); err != nil {
		t.Fatalf("UnlockTrunk failed: %v", err)
	}

	// 3. Fails on non-existent repo
	if err := LockTrunk(filepath.Join(repoRoot, "non-existent")); err == nil {
		t.Errorf("expected LockTrunk to fail on non-existent path")
	}
}
