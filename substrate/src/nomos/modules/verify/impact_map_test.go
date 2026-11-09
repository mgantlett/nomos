package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestListModifiedWorktreeFiles_AnchorsTaskKeyPrefix(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := workspace.SafeGitCommand(repo, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Nomos Test")

	if err := os.WriteFile(filepath.Join(repo, "target.go"), []byte("package p\n"), 0644); err != nil {
		t.Fatalf("write target.go: %v", err)
	}
	runGit("add", "target.go")
	runGit("commit", "-m", "[SUB-195] feat(verify): target change")

	if err := os.WriteFile(filepath.Join(repo, "collision.go"), []byte("package p\n"), 0644); err != nil {
		t.Fatalf("write collision.go: %v", err)
	}
	runGit("add", "collision.go")
	runGit("commit", "-m", "[SUB-1950] feat(verify): unrelated prefix collision")

	got := ListModifiedWorktreeFiles(repo, "SUB-195")
	if len(got) != 1 || got[0] != "target.go" {
		t.Fatalf("expected only target.go for SUB-195 without SUB-1950 prefix leak, got %v", got)
	}
}
