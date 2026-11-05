package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func initGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@nomos.local")
	runGit("config", "user.name", "Nomos Test")

	subDir := filepath.Join(dir, "substrate")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	goMod := "module example.com/testmod\n\ngo 1.22\n"
	goSrc := "package testmod\n\n// Add sums two integers.\nfunc Add(a, b int) int { return a + b }\n"
	goTest := "package testmod\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"bad sum\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(subDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "math.go"), []byte(goSrc), 0644); err != nil {
		t.Fatalf("write math.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "math_test.go"), []byte(goTest), 0644); err != nil {
		t.Fatalf("write math_test.go: %v", err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "initial baseline")
	return dir
}

func TestSharedGoSuite_DeduplicatesAndCachesReceiptsAcrossLifecycle(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv(envInToolchainGate, "")
	t.Setenv(envSkipToolchainGate, "")
	t.Setenv(envNoVerifyCache, "")

	repoDir := initGitTestRepo(t)
	if out, err := workspace.SafeGitCommand(repoDir, "checkout", "-b", "feature/SUB-1").CombinedOutput(); err != nil {
		t.Fatalf("checkout feature/SUB-1: %v (%s)", err, string(out))
	}

	mathPath := filepath.Join(repoDir, "substrate", "math.go")
	updatedGo := "package testmod\n\n// Add sums two integers.\nfunc Add(a, b int) int { return a + b }\n\n// Sub subtracts b from a.\nfunc Sub(a, b int) int { return a - b }\n"
	if err := os.WriteFile(mathPath, []byte(updatedGo), 0644); err != nil {
		t.Fatalf("update math.go: %v", err)
	}
	testPath := filepath.Join(repoDir, "substrate", "math_test.go")
	updatedTest := "package testmod\n\nimport \"testing\"\n\nfunc TestAddSub(t *testing.T) {\n\tif Add(2, 3) != 5 || Sub(5, 2) != 3 {\n\t\tt.Fatal(\"bad math\")\n\t}\n}\n"
	if err := os.WriteFile(testPath, []byte(updatedTest), 0644); err != nil {
		t.Fatalf("update math_test.go: %v", err)
	}

	// 1. First pipeline pass (unstaged): CoverageGate runs go test -cover, GoToolchainGate reuses ctx.CoverageRep
	ctx1 := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
	covGate := &CoverageGate{}
	tcGate := &GoToolchainGate{}

	covRes1, err := covGate.Evaluate(ctx1)
	if err != nil || !covRes1.Passed {
		t.Fatalf("expected CoverageGate pass on run 1, got err=%v res=%+v", err, covRes1)
	}
	tcRes1, err := tcGate.Evaluate(ctx1)
	if err != nil || !tcRes1.Passed || !strings.Contains(tcRes1.Message, "Go(go test -cover)") {
		t.Fatalf("expected GoToolchainGate to share run 1 result, got err=%v res=%+v", err, tcRes1)
	}

	// 2. Stage and commit the exact same file bytes -> new VerifyContext must hit cryptographic receipt cache!
	if out, cErr := workspace.SafeGitCommand(repoDir, "add", "-A").CombinedOutput(); cErr != nil {
		t.Fatalf("git add: %v (%s)", cErr, string(out))
	}
	if out, cErr := workspace.SafeGitCommand(repoDir, "commit", "-m", "[SUB-1] add Sub").CombinedOutput(); cErr != nil {
		t.Fatalf("git commit: %v (%s)", cErr, string(out))
	}

	ctx2 := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
	tcRes2, err := tcGate.Evaluate(ctx2)
	if err != nil || !tcRes2.Passed || !strings.Contains(tcRes2.Message, "Go(receipt cache)") {
		t.Fatalf("expected committed identical snapshot to hit cryptographic receipt cache, got err=%v res=%+v", err, tcRes2)
	}

	// 3. Mutate 1 byte in math.go -> snapshot digest MUST change (cache miss)
	brokenGo := updatedGo + "\nfunc Broken() int { return missingSymbol }\n"
	if err := os.WriteFile(mathPath, []byte(brokenGo), 0644); err != nil {
		t.Fatalf("write broken math.go: %v", err)
	}
	ctx3 := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
	tcRes3, err := tcGate.Evaluate(ctx3)
	if err != nil || tcRes3.Passed {
		t.Fatalf("expected 1-line broken mutation to invalidate receipt cache and fail closed, got err=%v res=%+v", err, tcRes3)
	}
}

func TestSharedGoSuite_DiffScopesNonGoEditsInWorktree(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv(envInToolchainGate, "")
	t.Setenv(envSkipToolchainGate, "")

	repoDir := initGitTestRepo(t)
	if out, err := workspace.SafeGitCommand(repoDir, "checkout", "-b", "feature/SUB-2").CombinedOutput(); err != nil {
		t.Fatalf("checkout feature/SUB-2: %v (%s)", err, string(out))
	}

	// Touch only a Svelte UI file inside worktree scope -> Go suite should be diff-scoped skipped
	uiPath := filepath.Join(repoDir, "substrate", "membrane", "Widget.svelte")
	if err := os.MkdirAll(filepath.Dir(uiPath), 0755); err != nil {
		t.Fatalf("mkdir membrane: %v", err)
	}
	if err := os.WriteFile(uiPath, []byte("<p>Hello</p>\n"), 0644); err != nil {
		t.Fatalf("write Widget.svelte: %v", err)
	}

	ctx := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-2"}
	tcGate := &GoToolchainGate{}
	res, err := tcGate.Evaluate(ctx)
	if err != nil || !res.Passed || !strings.Contains(res.Message, "Go(diff-scoped skip)") {
		t.Fatalf("expected non-Go UI edit in worktree to skip Go test suite, got err=%v res=%+v", err, res)
	}

	// With AllFiles (-A), full Go suite must still execute
	ctxAll := &VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-2", AllFiles: true}
	resAll, err := tcGate.Evaluate(ctxAll)
	if err != nil || !resAll.Passed || !strings.Contains(resAll.Message, "Go(go test -cover)") {
		t.Fatalf("expected --all flag to force full go test -cover execution, got err=%v res=%+v", err, resAll)
	}
}

func TestTruncateToolchainOutput_AnchorsOnFailDiagnosticAfterLongStdout(t *testing.T) {
	prefix := strings.Repeat("Initialized Nomos workspace in /tmp/cleanroom-test\n", 40)
	tail := "--- FAIL: TestCleanroomLearn (0.04s)\n    learn_test.go:142: expected 0 graduation candidates\nFAIL\tgithub.com/mgantlett/nomos/substrate/cli\t0.81s"
	got := truncateToolchainOutput(prefix + tail)
	if !strings.HasPrefix(got, "--- FAIL: TestCleanroomLearn") {
		t.Fatalf("expected truncateToolchainOutput to anchor on --- FAIL block, got %q", got)
	}
}

