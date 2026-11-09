package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestMergeGoCoverProfilesAndLivePersistence(t *testing.T) {
	base := []byte("mode: set\nexample.com/mod/a.go:3.10,5.2 2 1\nexample.com/mod/b.go:4.12,8.2 3 0\n")
	overlay := []byte("mode: set\nexample.com/mod/b.go:4.12,8.2 3 1\nexample.com/mod/c.go:2.8,4.2 1 1\n")

	merged := string(mergeGoCoverProfiles(base, overlay))
	if !strings.HasPrefix(merged, "mode: set\n") {
		t.Fatalf("expected mode header, got %q", merged)
	}
	if !strings.Contains(merged, "example.com/mod/a.go:3.10,5.2 2 1") {
		t.Errorf("expected untouched baseline block preserved, got %q", merged)
	}
	if strings.Contains(merged, "example.com/mod/b.go:4.12,8.2 3 0") || !strings.Contains(merged, "example.com/mod/b.go:4.12,8.2 3 1") {
		t.Errorf("expected updated file block replaced by overlay, got %q", merged)
	}
	if !strings.Contains(merged, "example.com/mod/c.go:2.8,4.2 1 1") {
		t.Errorf("expected new overlay block appended, got %q", merged)
	}

	repoDir := initGitTestRepo(t)
	ctx := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, AllFiles: true}
	rec := executeGoTestWithCoverage(ctx, repoDir, filepath.Join(repoDir, "substrate"), "snap-cov")
	if !rec.GoPassed {
		t.Fatalf("expected Go suite pass: %s", rec.GoOutput)
	}
	profileBytes, err := os.ReadFile(filepath.Join(repoDir, ".tmp", "coverage.out"))
	if err != nil || !strings.Contains(string(profileBytes), "math.go:") {
		t.Fatalf("expected .tmp/coverage.out persisted with math.go blocks, err=%v body=%q", err, string(profileBytes))
	}
}

func buildBloatedGoTestSource(funcName string, branches int) string {
	var b strings.Builder
	b.WriteString("package demo\n\nimport \"testing\"\n\nfunc ")
	b.WriteString(funcName)
	b.WriteString("(t *testing.T) {\n\tx := Val()\n")
	for i := 0; i < branches; i++ {
		fmt.Fprintf(&b, "\tif x == %d { t.Log(%d) }\n", i+100, i)
	}
	b.WriteString("}\n")
	return b.String()
}

func TestComplexityGate_EnforcesAndRatchetsTestComplexity(t *testing.T) {
	repo := t.TempDir()
	_ = workspace.SafeGitCommand(repo, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.email", "test@nomos.dev").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.name", "Nomos Test").Run()

	mustWriteGoFixture(t, filepath.Join(repo, "nomos.yaml"), "project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n")
	mustWriteGoFixture(t, filepath.Join(repo, "val.go"), "package demo\n\n// Val returns baseline constant.\nfunc Val() int { return 1 }\n")
	mustWriteGoFixture(t, filepath.Join(repo, "val_test.go"), buildBloatedGoTestSource("TestLegacyMega", 28))
	_ = workspace.SafeGitCommand(repo, "add", "-A").Run()
	_ = workspace.SafeGitCommand(repo, "commit", "-m", "chore: baseline").Run()

	gate := &ComplexityGate{}
	// 1. Brownfield ratchet allows untouched baseline test > 25 on main.
	resBase, err := gate.Evaluate(&VerifyContext{RepoRoot: repo, AllFiles: true})
	if err != nil || !resBase.Passed {
		t.Fatalf("expected untouched brownfield mega-test to ratchet cleanly, got err=%v res=%+v", err, resBase)
	}

	// 2. Strict mode rejects mega-test > 25 immediately.
	resStrict, err := gate.Evaluate(&VerifyContext{RepoRoot: repo, AllFiles: true, Strict: true})
	if err != nil || resStrict.Passed || !strings.Contains(resStrict.Message, "TestLegacyMega") {
		t.Fatalf("expected strict mode to reject test CCN > 25, got err=%v res=%+v", err, resStrict)
	}

	// 3. Net-new bloated test function in brownfield mode fails closed.
	mustWriteGoFixture(t, filepath.Join(repo, "new_test.go"), buildBloatedGoTestSource("TestNetNewMega", 27))
	resNew, err := gate.Evaluate(&VerifyContext{RepoRoot: repo, AllFiles: true})
	if err != nil || resNew.Passed || !strings.Contains(resNew.Message, "TestNetNewMega") {
		t.Fatalf("expected net-new bloated test to fail closed in brownfield mode, got err=%v res=%+v", err, resNew)
	}
}

func TestHasCoupledGoPackageTests(t *testing.T) {
	dir := t.TempDir()
	mustWriteGoFixture(t, filepath.Join(dir, "svc.go"), "package svc\n\ntype Config struct{ Port int }\nfunc Start(c Config) int { return c.Port }\n")
	mustWriteGoFixture(t, filepath.Join(dir, "svc_test.go"), "package svc\n\nimport \"testing\"\nfunc TestNothing(t *testing.T) { if 1 != 1 { t.Fatal() } }\n")
	if hasCoupledGoPackageTests(dir) {
		t.Fatalf("expected hollow test referencing zero package symbols to return false")
	}

	mustWriteGoFixture(t, filepath.Join(dir, "svc_test.go"), "package svc\n\nimport \"testing\"\nfunc TestStart(t *testing.T) { if Start(Config{Port: 80}) != 80 { t.Fatal() } }\n")
	if !hasCoupledGoPackageTests(dir) {
		t.Fatalf("expected coupled test referencing Start and Config to return true")
	}
}
