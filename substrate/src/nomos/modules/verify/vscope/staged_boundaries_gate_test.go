package vscope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestStagedBoundariesGate_Name(t *testing.T) {
	gate := &StagedBoundariesGate{}
	if gate.Name() != "Worktree Boundaries Invariant (Law 3)" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}
}

func TestStagedBoundariesGate_EvaluateClean(t *testing.T) {
	gate := &StagedBoundariesGate{}
	ctx := &VerifyContext{
		RepoRoot:     t.TempDir(),
		WorktreePath: t.TempDir(),
	}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.Passed {
		t.Errorf("expected clean evaluate to pass, got: %+v", res)
	}
}

func TestStagedBoundariesGate_RejectsStagedAndCommittedScratchArtifacts(t *testing.T) {
	repoDir := workspace.InitHermeticGitRepo(t)
	mgr, err := workspace.NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wtPath, err := mgr.CreateWorktree("SUB-77")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer func() {
		_ = mgr.RemoveWorktree("SUB-77")
	}()

	gate := &StagedBoundariesGate{}
	ctx := &VerifyContext{RepoRoot: repoDir, WorktreePath: wtPath, TaskID: "SUB-77"}

	// 1. Force-staged .tmp artifact fails closed
	badTmp := filepath.Join(wtPath, "schema.json.tmp")
	if err := os.WriteFile(badTmp, []byte("scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := workspace.SafeGitCommand(wtPath, "add", "-f", "schema.json.tmp").CombinedOutput(); err != nil {
		t.Fatalf("git add -f: %v (%s)", err, string(out))
	}
	resStaged, err := gate.Evaluate(ctx)
	if err != nil || resStaged == nil || resStaged.Passed || !strings.Contains(resStaged.Message, "schema.json.tmp") {
		t.Fatalf("expected force-staged .tmp file to fail closed, got %+v (err=%v)", resStaged, err)
	}

	// 2. Even if committed on task branch, StagedBoundariesGate still fails closed until removed
	if out, err := workspace.SafeGitCommand(wtPath, "commit", "-m", "temp").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}
	resCommitted, err := gate.Evaluate(ctx)
	if err != nil || resCommitted == nil || resCommitted.Passed || !strings.Contains(resCommitted.Message, "schema.json.tmp") {
		t.Fatalf("expected committed .tmp file in branch diff to fail closed, got %+v (err=%v)", resCommitted, err)
	}

	// 3. Removing the .tmp file restores clean pass
	if out, err := workspace.SafeGitCommand(wtPath, "rm", "schema.json.tmp").CombinedOutput(); err != nil {
		t.Fatalf("git rm: %v (%s)", err, string(out))
	}
	if out, err := workspace.SafeGitCommand(wtPath, "commit", "-m", "cleanup").CombinedOutput(); err != nil {
		t.Fatalf("git commit cleanup: %v (%s)", err, string(out))
	}
	resClean, err := gate.Evaluate(ctx)
	if err != nil || resClean == nil || !resClean.Passed {
		t.Fatalf("expected clean pass once .tmp removed, got %+v (err=%v)", resClean, err)
	}
}
