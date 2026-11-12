package polyrun

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

func TestMergeGoCoverProfiles(t *testing.T) {
	base := "mode: set\npkg/a.go:1.1,2.2 1 1\npkg/b.go:1.1,2.2 1 0\n"
	overlay := "mode: set\npkg/b.go:1.1,2.2 1 1\npkg/c.go:1.1,2.2 1 1\n"

	merged := string(MergeGoCoverProfiles([]byte(base), []byte(overlay)))
	if !strings.HasPrefix(merged, "mode: set\n") {
		t.Fatalf("expected mode header preserved, got:\n%s", merged)
	}
	if !strings.Contains(merged, "pkg/a.go:1.1,2.2 1 1") {
		t.Fatalf("expected untouched base entry retained, got:\n%s", merged)
	}
	if strings.Contains(merged, "pkg/b.go:1.1,2.2 1 0") || !strings.Contains(merged, "pkg/b.go:1.1,2.2 1 1") {
		t.Fatalf("expected overlaid entry replaced, got:\n%s", merged)
	}
	if !strings.Contains(merged, "pkg/c.go:1.1,2.2 1 1") {
		t.Fatalf("expected new overlay entry appended, got:\n%s", merged)
	}
}

func TestPersistGoCoverProfile_FullAndIncremental(t *testing.T) {
	root := t.TempDir()
	runPath := prepareGoCoverProfileRunPath(root)
	if runPath == "" {
		t.Fatal("expected non-empty run profile path")
	}
	fullData := []byte("mode: set\npkg/a.go:1.1,2.2 1 1\npkg/b.go:1.1,2.2 1 0\n")
	if err := os.WriteFile(runPath, fullData, 0644); err != nil {
		t.Fatal(err)
	}

	ctx := &vscope.VerifyContext{RepoRoot: root}
	persistGoCoverProfile(ctx, root, runPath, []string{"./..."})

	destPath := filepath.Join(root, coverProfileDirName, coverProfileFileName)
	got, err := os.ReadFile(destPath)
	if err != nil || string(got) != string(fullData) {
		t.Fatalf("expected full coverprofile persisted, err=%v got=%q", err, string(got))
	}

	runPath2 := prepareGoCoverProfileRunPath(root)
	if err := os.WriteFile(runPath2, []byte("mode: set\npkg/b.go:1.1,2.2 1 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	persistGoCoverProfile(ctx, root, runPath2, []string{"./pkg"})
	merged, err := os.ReadFile(destPath)
	if err != nil || !strings.Contains(string(merged), "pkg/b.go:1.1,2.2 1 1") || !strings.Contains(string(merged), "pkg/a.go:1.1,2.2 1 1") {
		t.Fatalf("expected incremental merge to preserve pkg/a.go and update pkg/b.go, err=%v got=%q", err, string(merged))
	}
}

func TestComputeWorkspaceSnapshotDigestWithPackages_DirtySensitive(t *testing.T) {
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := workspace.SafeGitCommand(repo, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	filePath := filepath.Join(repo, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "main.go")
	runGit("commit", "-m", "init")

	ctx := &vscope.VerifyContext{RepoRoot: repo}
	d1 := ComputeWorkspaceSnapshotDigestWithPackages(ctx, repo, repo, []string{"./..."})
	if d1 == "" {
		t.Fatal("expected non-empty snapshot digest")
	}
	if err := os.WriteFile(filePath, []byte("package main\nfunc x() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	d2 := ComputeWorkspaceSnapshotDigestWithPackages(ctx, repo, repo, []string{"./..."})
	if d2 == "" || d2 == d1 {
		t.Fatalf("expected dirty file modification to change snapshot digest (%s vs %s)", d1, d2)
	}
}
