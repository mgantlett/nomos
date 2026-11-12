package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
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
	t.Setenv(polyrun.EnvInToolchainGate, "")
	t.Setenv(polyrun.EnvSkipToolchainGate, "")
	t.Setenv(polyrun.EnvNoVerifyCache, "")

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
	ctx1 := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
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

	ctx2 := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
	tcRes2, err := tcGate.Evaluate(ctx2)
	if err != nil || !tcRes2.Passed || !strings.Contains(tcRes2.Message, "Go(receipt cache)") {
		t.Fatalf("expected committed identical snapshot to hit cryptographic receipt cache, got err=%v res=%+v", err, tcRes2)
	}

	// 3. Mutate 1 byte in math.go -> snapshot digest MUST change (cache miss)
	brokenGo := updatedGo + "\nfunc Broken() int { return missingSymbol }\n"
	if err := os.WriteFile(mathPath, []byte(brokenGo), 0644); err != nil {
		t.Fatalf("write broken math.go: %v", err)
	}
	ctx3 := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-1"}
	tcRes3, err := tcGate.Evaluate(ctx3)
	if err != nil || tcRes3.Passed {
		t.Fatalf("expected 1-line broken mutation to invalidate receipt cache and fail closed, got err=%v res=%+v", err, tcRes3)
	}
}

func TestSharedGoSuite_DiffScopesNonGoEditsInWorktree(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv(polyrun.EnvInToolchainGate, "")
	t.Setenv(polyrun.EnvSkipToolchainGate, "")

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

	ctx := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-2"}
	tcGate := &GoToolchainGate{}
	res, err := tcGate.Evaluate(ctx)
	if err != nil || !res.Passed || !strings.Contains(res.Message, "Go(diff-scoped skip)") {
		t.Fatalf("expected non-Go UI edit in worktree to skip Go test suite, got err=%v res=%+v", err, res)
	}

	// With AllFiles (-A), full Go suite must still execute
	ctxAll := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-2", AllFiles: true}
	resAll, err := tcGate.Evaluate(ctxAll)
	if err != nil || !resAll.Passed || !strings.Contains(resAll.Message, "Go(go test -cover)") {
		t.Fatalf("expected --all flag to force full go test -cover execution, got err=%v res=%+v", err, resAll)
	}
}

func TestTruncateToolchainOutput_AnchorsOnFailDiagnosticAfterLongStdout(t *testing.T) {
	prefix := strings.Repeat("Initialized Nomos workspace in /tmp/cleanroom-test\n", 40)
	tail := "--- FAIL: TestCleanroomLearn (0.04s)\n    learn_test.go:142: expected 0 graduation candidates\nFAIL\tgithub.com/mgantlett/nomos/substrate/cli\t0.81s"
	got := polyrun.TruncateToolchainOutput(prefix + tail)
	if !strings.HasPrefix(got, "--- FAIL: TestCleanroomLearn") {
		t.Fatalf("expected truncateToolchainOutput to anchor on --- FAIL block, got %q", got)
	}
}

func TestSharedGoSuite_CanonicalizesRelativeWorktreeAndFastForwardTrunk(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv(polyrun.EnvInToolchainGate, "")
	t.Setenv(polyrun.EnvSkipToolchainGate, "")
	t.Setenv(polyrun.EnvNoVerifyCache, "")

	repoDir := initGitTestRepo(t)
	wtDir := filepath.Join(repoDir, "worktrees", "test-SUB-3")
	if out, err := workspace.SafeGitCommand(repoDir, "worktree", "add", "-b", "feature/SUB-3", wtDir, "main").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v (%s)", err, string(out))
	}

	mathPath := filepath.Join(wtDir, "substrate", "math.go")
	updatedGo := "package testmod\n\n// Add sums two integers.\nfunc Add(a, b int) int { return a + b }\n\n// Mul multiplies two integers.\nfunc Mul(a, b int) int { return a * b }\n"
	if err := os.WriteFile(mathPath, []byte(updatedGo), 0644); err != nil {
		t.Fatalf("write math.go: %v", err)
	}

	// 1. Run from inside worktree using relative "." path (simulating 'nomos verify' inside worktree)
	t.Chdir(wtDir)
	tcGate := &GoToolchainGate{}
	ctxRel := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: ".", TaskID: "SUB-3"}
	resRel, err := tcGate.Evaluate(ctxRel)
	if err != nil || !resRel.Passed || !strings.Contains(resRel.Message, "Go(go test -cover)") {
		t.Fatalf("expected initial relative run to execute go test -cover, got err=%v res=%+v", err, resRel)
	}

	// 2. Run using absolute worktree path (simulating 'nomos task sync' executeGateCheckForTask) -> MUST hit receipt cache!
	ctxAbs := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: wtDir, TaskID: "SUB-3"}
	resAbs, err := tcGate.Evaluate(ctxAbs)
	if err != nil || !resAbs.Passed || !strings.Contains(resAbs.Message, "Go(receipt cache)") {
		t.Fatalf("expected absolute worktree path to hit canonicalized receipt cache, got err=%v res=%+v", err, resAbs)
	}

	// 3. Commit in worktree, fast-forward merge onto main, and verify trunk hits the exact same tree-hash receipt!
	if out, cErr := workspace.SafeGitCommand(wtDir, "add", "-A").CombinedOutput(); cErr != nil {
		t.Fatalf("git add: %v (%s)", cErr, string(out))
	}
	if out, cErr := workspace.SafeGitCommand(wtDir, "commit", "-m", "[SUB-3] add Mul").CombinedOutput(); cErr != nil {
		t.Fatalf("git commit: %v (%s)", cErr, string(out))
	}
	if out, mErr := workspace.SafeGitCommand(repoDir, "merge", "--ff-only", "feature/SUB-3").CombinedOutput(); mErr != nil {
		t.Fatalf("git merge --ff-only: %v (%s)", mErr, string(out))
	}
	ctxTrunk := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, IsBaseline: true}
	resTrunk, err := tcGate.Evaluate(ctxTrunk)
	if err != nil || !resTrunk.Passed || !strings.Contains(resTrunk.Message, "Go(receipt cache)") {
		t.Fatalf("expected fast-forwarded trunk to hit tree-hash receipt cache without re-running tests, got err=%v res=%+v", err, resTrunk)
	}
}

func TestParseAverageCoverage_CountsNoTestFilesAsZeroPercent(t *testing.T) {
	onlyUntested := "?   \texample.com/untested\t[no test files]\n"
	if got := polyrun.ParseAverageCoverage(onlyUntested); got != 0.0 {
		t.Fatalf("expected 0.0%% for [no test files] package, got %.1f%%", got)
	}
	mixed := strings.Join([]string{
		"ok  \texample.com/tested\t0.010s\tcoverage: 80.0% of statements",
		"?   \texample.com/untested\t[no test files]",
		"ok  \texample.com/consts\t0.004s\tcoverage: [no statements]",
	}, "\n")
	if got := polyrun.ParseAverageCoverage(mixed); got != 40.0 {
		t.Fatalf("expected 40.0%% across 80%% + 0%% (ignoring [no statements]), got %.1f%%", got)
	}
}

func TestSnapshotPackageScopeAndReverseImporterExpansion(t *testing.T) {
	repoDir := initGitTestRepo(t)
	subDir := filepath.Join(repoDir, "substrate")
	leafDir := filepath.Join(subDir, "pkg", "leaf")
	callerDir := filepath.Join(subDir, "pkg", "caller")
	if err := os.MkdirAll(leafDir, 0755); err != nil {
		t.Fatalf("mkdir leaf: %v", err)
	}
	if err := os.MkdirAll(callerDir, 0755); err != nil {
		t.Fatalf("mkdir caller: %v", err)
	}
	if err := os.WriteFile(filepath.Join(leafDir, "leaf.go"), []byte("package leaf\nfunc Val() int { return 1 }\n"), 0644); err != nil {
		t.Fatalf("write leaf.go: %v", err)
	}
	callerSrc := "package caller\nimport \"example.com/testmod/pkg/leaf\"\nfunc Use() int { return leaf.Val() }\n"
	if err := os.WriteFile(filepath.Join(callerDir, "caller.go"), []byte(callerSrc), 0644); err != nil {
		t.Fatalf("write caller.go: %v", err)
	}

	ctx := &vscope.VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir}
	fullHash := polyrun.ComputeWorkspaceSnapshotDigestWithPackages(ctx, repoDir, subDir, []string{"./..."})
	baseHash := polyrun.ComputeWorkspaceSnapshotDigest(ctx, repoDir, subDir)
	narrowHash := polyrun.ComputeWorkspaceSnapshotDigestWithPackages(ctx, repoDir, subDir, []string{"./pkg/leaf"})
	if fullHash == "" || fullHash != baseHash {
		t.Fatalf("expected default ./... hash parity: full=%q base=%q", fullHash, baseHash)
	}
	if narrowHash == fullHash {
		t.Fatalf("expected narrow package scope hash to differ from full ./... hash")
	}

	pkgSet := map[string]struct{}{"./pkg/leaf": {}}
	polyrun.ExpandModifiedGoCallerPackages(repoDir, subDir, []string{"substrate/pkg/leaf/leaf.go"}, pkgSet)
	if _, ok := pkgSet["./pkg/caller"]; !ok {
		t.Fatalf("expected 1-hop importer ./pkg/caller in pkgSet, got %v", pkgSet)
	}
}
