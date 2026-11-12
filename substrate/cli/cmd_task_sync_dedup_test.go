// Package cli tests pre-verify trunk rebase ordering and short-lived clean-tree
// verification receipt reuse across pre-commit and task sync transitions.
// @intent task_lifecycle_cli_spec.md
// @verifies TC-SYNC-020
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

func initDedupTestRepoAndWorktree(t *testing.T, taskKey string) (string, string) {
	t.Helper()
	t.Setenv(constants.EnvNomosHome, t.TempDir())
	t.Setenv(envDisableVerifyReceiptCache, "")

	repoDir := t.TempDir()
	mustRunGit(t, repoDir, "init", "-b", constants.ValMain)
	mustRunGit(t, repoDir, "config", "user.email", "test@nomos.local")
	mustRunGit(t, repoDir, "config", "user.name", "Nomos Test")

	subDir := filepath.Join(repoDir, constants.DirSubstrate)
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir substrate: %v", err)
	}
	mustWriteTestFile(t, filepath.Join(repoDir, ".gitignore"), "go.work\ngo.work.sum\n.tmp/\nworktrees/\n")
	mustWriteTestFile(t, filepath.Join(subDir, "go.mod"), "module example.com/deduptest\n\ngo 1.22\n")
	mustWriteTestFile(t, filepath.Join(subDir, "calc.go"), "package deduptest\n\n// Double multiplies v by two.\nfunc Double(v int) int { return v * 2 }\n")
	mustRunGit(t, repoDir, "add", "-A")
	mustRunGit(t, repoDir, "commit", "-m", "chore: initial baseline")

	mgr, err := workspace.NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wtDir, err := mgr.CreateWorktree(taskKey)
	if err != nil {
		t.Fatalf("CreateWorktree(%s): %v", taskKey, err)
	}
	return repoDir, wtDir
}

func mustRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := workspace.RunSafeGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v in %s failed: %v (%s)", args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func mustWriteTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func syntheticPassingPipelineReport() *verify.PipelineReport {
	return &verify.PipelineReport{
		Passed:      true,
		GatesTotal:  42,
		GatesPassed: 42,
	}
}

// TestVerifyReceiptCache_StagedWriteTreeHitOnCleanPostCommitHead verifies that a receipt
// stamped during pre-commit (staged index via git write-tree + trunkHeadSHA) matches the
// clean post-commit HEAD^{tree} inside executeGateCheckForTask.
// @verifies TC-SYNC-020
func TestVerifyReceiptCache_StagedWriteTreeHitOnCleanPostCommitHead(t *testing.T) {
	repoDir, wtDir := initDedupTestRepoAndWorktree(t, "SUB-101")
	t.Chdir(wtDir)

	updatedSrc := "package deduptest\n\n// Double multiplies v by two.\nfunc Double(v int) int { return v * 2 }\n\n// Triple multiplies v by three.\nfunc Triple(v int) int { return v * 3 }\n"
	mustWriteTestFile(t, filepath.Join(wtDir, constants.DirSubstrate, "calc.go"), updatedSrc)
	mustRunGit(t, wtDir, "add", "-A")

	stagedWriteTree := mustRunGit(t, wtDir, "write-tree")
	baseNow := time.Now()
	saveWorktreeVerifyReceiptAt(repoDir, wtDir, "SUB-101", syntheticPassingPipelineReport(), baseNow)

	cacheFile := verifyReceiptCachePath(repoDir, wtDir)
	if !strings.HasPrefix(cacheFile, filepath.Join(config.GetNomosHome(), constants.DirCache, constants.DirVerifyReceipts)) {
		t.Fatalf("expected receipt under NOMOS_HOME cache/verify_receipts, got %s", cacheFile)
	}
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-101", baseNow.Add(time.Second)) != nil {
		t.Fatal("expected uncommitted staged index to miss clean-only receipt lookup before commit completes")
	}

	commitMsg := "[SUB-101] feat(calc): add Triple helper\n\nAdds Triple helper for deterministic sync receipt testing.\n\n___\nTask: SUB-101\n"
	mustRunGit(t, wtDir, "commit", "--no-verify", "-m", commitMsg)

	if headTree := mustRunGit(t, wtDir, "rev-parse", "HEAD^{tree}"); headTree != stagedWriteTree {
		t.Fatalf("expected post-commit HEAD^{tree} %s to equal pre-commit write-tree %s", headTree, stagedWriteTree)
	}
	rep, err := executeGateCheckForTask("SUB-101")
	if err != nil || rep == nil || !rep.Passed || rep.GatesPassed != 42 {
		t.Fatalf("expected executeGateCheckForTask to reuse 42/42 clean-tree receipt, got rep=%+v err=%v", rep, err)
	}
}

// TestVerifyReceiptCache_InvalidatesOnDirtyUntrackedTrunkDriftAndTTL verifies fail-closed
// cache invalidation on modified tracked bytes, untracked files, staged diffs, TTL expiry, and trunk SHA drift.
// @verifies TC-SYNC-020
func TestVerifyReceiptCache_InvalidatesOnDirtyUntrackedTrunkDriftAndTTL(t *testing.T) {
	repoDir, wtDir := initDedupTestRepoAndWorktree(t, "SUB-102")
	calcFile := filepath.Join(wtDir, constants.DirSubstrate, "calc.go")
	baseSrc := "package deduptest\n\n// Double multiplies v by two.\nfunc Double(v int) int { return v * 2 }\n"

	baseNow := time.Now()
	saveWorktreeVerifyReceiptAt(repoDir, wtDir, "SUB-102", syntheticPassingPipelineReport(), baseNow)
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(10*time.Second)) == nil {
		t.Fatal("expected fresh clean worktree receipt hit")
	}
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(6*time.Minute)) != nil {
		t.Fatal("expected receipt older than 5m TTL to expire")
	}

	assertDirtyAndUntrackedInvalidate(t, repoDir, wtDir, calcFile, baseSrc, baseNow)

	mustWriteTestFile(t, filepath.Join(repoDir, "README.md"), "# Trunk advanced\n")
	mustRunGit(t, repoDir, "add", "README.md")
	mustRunGit(t, repoDir, "commit", "--no-verify", "-m", "docs: advance trunk")
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(20*time.Second)) != nil {
		t.Fatal("expected advanced trunkHeadSHA to invalidate cached worktree receipt")
	}
}

func assertDirtyAndUntrackedInvalidate(t *testing.T, repoDir, wtDir, calcFile, baseSrc string, baseNow time.Time) {
	t.Helper()
	mustWriteTestFile(t, calcFile, baseSrc+"\n// 1-byte dirty comment\n")
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(12*time.Second)) != nil {
		t.Fatal("expected modified tracked file byte to invalidate receipt")
	}
	mustRunGit(t, wtDir, "add", "-A")
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(13*time.Second)) != nil {
		t.Fatal("expected staged uncommitted diff to invalidate receipt")
	}
	mustRunGit(t, wtDir, "reset", "--hard", constants.ValHead)

	untracked := filepath.Join(wtDir, "untracked_note.txt")
	mustWriteTestFile(t, untracked, "untracked\n")
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(14*time.Second)) != nil {
		t.Fatal("expected untracked file in worktree to invalidate receipt")
	}
	if err := os.Remove(untracked); err != nil {
		t.Fatalf("remove untracked: %v", err)
	}
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-102", baseNow.Add(15*time.Second)) == nil {
		t.Fatal("expected restored clean worktree to hit receipt again")
	}
}

// TestVerifyReceiptCache_NoVerifyCommitMissesAndFailsClosed verifies that a --no-verify
// commit bypassing pre-commit never hits the receipt cache and fails closed during gate evaluation.
// @verifies TC-SYNC-020
func TestVerifyReceiptCache_NoVerifyCommitMissesAndFailsClosed(t *testing.T) {
	repoDir, wtDir := initDedupTestRepoAndWorktree(t, "SUB-103")
	t.Chdir(wtDir)

	saveWorktreeVerifyReceipt(repoDir, wtDir, "SUB-103", syntheticPassingPipelineReport())

	brokenSrc := "package deduptest\n\nfunc BrokenSyntax( {\n"
	mustWriteTestFile(t, filepath.Join(wtDir, constants.DirSubstrate, "calc.go"), brokenSrc)
	mustRunGit(t, wtDir, "add", "-A")
	mustRunGit(t, wtDir, "commit", "--no-verify", "-m", "[SUB-103] feat(calc): broken bypass")

	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-103", time.Now()) != nil {
		t.Fatal("expected --no-verify commit with unverified HEAD^{tree} to miss receipt cache")
	}
	if _, err := executeGateCheckForTask("SUB-103"); err == nil {
		t.Fatal("expected executeGateCheckForTask to run full gates and fail closed on --no-verify broken commit")
	}
}

// TestRebaseOntoTrunkBeforeVerify_DivergedRebaseAndCleanConflictAbort verifies that
// rebaseOntoTrunkIfDiverged rebases diverged branches cleanly and aborts conflicts cleanly.
// @verifies TC-SYNC-020
func TestRebaseOntoTrunkBeforeVerify_DivergedRebaseAndCleanConflictAbort(t *testing.T) {
	repoDir, wtDir := initDedupTestRepoAndWorktree(t, "SUB-104")
	mustWriteTestFile(t, filepath.Join(wtDir, constants.DirSubstrate, "feature.go"), "package deduptest\n\n// One returns 1.\nfunc One() int { return 1 }\n")
	mustRunGit(t, wtDir, "add", "-A")
	mustRunGit(t, wtDir, "commit", "--no-verify", "-m", "[SUB-104] feat(calc): add One")
	saveWorktreeVerifyReceipt(repoDir, wtDir, "SUB-104", syntheticPassingPipelineReport())

	mustWriteTestFile(t, filepath.Join(repoDir, "CHANGELOG.md"), "v1.0.1\n")
	mustRunGit(t, repoDir, "add", "CHANGELOG.md")
	mustRunGit(t, repoDir, "commit", "--no-verify", "-m", "docs: changelog on trunk")

	rebased, err := rebaseOntoTrunkIfDiverged(repoDir, wtDir, "feature/SUB-104")
	if err != nil || !rebased {
		t.Fatalf("expected clean rebase onto diverged trunk, got rebased=%v err=%v", rebased, err)
	}
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-104", time.Now()) != nil {
		t.Fatal("expected post-rebase tree and trunk SHA to miss stale pre-rebase receipt")
	}

	mustWriteTestFile(t, filepath.Join(wtDir, "CONFLICT.txt"), "branch-line\n")
	mustRunGit(t, wtDir, "add", "CONFLICT.txt")
	mustRunGit(t, wtDir, "commit", "--no-verify", "-m", "[SUB-104] feat(calc): conflict branch")
	preConflictHead := mustRunGit(t, wtDir, "rev-parse", constants.ValHead)

	mustWriteTestFile(t, filepath.Join(repoDir, "CONFLICT.txt"), "trunk-line\n")
	mustRunGit(t, repoDir, "add", "CONFLICT.txt")
	mustRunGit(t, repoDir, "commit", "--no-verify", "-m", "feat: conflict on trunk")

	if _, cErr := rebaseOntoTrunkIfDiverged(repoDir, wtDir, "feature/SUB-104"); cErr == nil {
		t.Fatal("expected conflicting trunk divergence to fail closed with boundary error")
	}
	if postHead := mustRunGit(t, wtDir, "rev-parse", constants.ValHead); postHead != preConflictHead {
		t.Fatalf("expected clean git rebase --abort rollback to %s, got %s", preConflictHead, postHead)
	}
	if !isWorktreeStatusCompletelyClean(wtDir) {
		t.Fatal("expected worktree status to remain clean after rebase --abort rollback")
	}
}

// TestWorkingTreeVerifyReceipt_UnstagedToPreCommitToSync verifies that an unstaged
// 'nomos verify' pass stamps a temp-index working-tree SHA receipt that is reused across
// staged pre-commit and clean post-commit sync, while rejecting partial staging.
// @verifies TC-SYNC-024
func TestWorkingTreeVerifyReceipt_UnstagedToPreCommitToSync(t *testing.T) {
	repoDir, wtDir := initDedupTestRepoAndWorktree(t, "SUB-105")
	t.Chdir(wtDir)

	updatedSrc := "package deduptest\n\n// Double multiplies v by two.\nfunc Double(v int) int { return v * 2 }\n\n// Quad multiplies v by four.\nfunc Quad(v int) int { return v * 4 }\n"
	mustWriteTestFile(t, filepath.Join(wtDir, constants.DirSubstrate, "calc.go"), updatedSrc)

	baseNow := time.Now()
	saveWorktreeVerifyReceiptWithStrict(repoDir, wtDir, "SUB-105", true, syntheticPassingPipelineReport(), baseNow)

	if loadValidWorkingTreeVerifyReceipt(repoDir, wtDir, "SUB-105", true, baseNow.Add(time.Second)) == nil {
		t.Fatal("expected unstaged working-tree receipt hit immediately after unstaged verify")
	}

	mustRunGit(t, wtDir, "add", "-A")
	if loadValidWorkingTreeVerifyReceipt(repoDir, wtDir, "SUB-105", false, baseNow.Add(2*time.Second)) == nil {
		t.Fatal("expected staged pre-commit index to reuse identical working-tree SHA receipt")
	}

	mustWriteTestFile(t, filepath.Join(wtDir, constants.DirSubstrate, "extra.go"), "package deduptest\n")
	if loadValidWorkingTreeVerifyReceipt(repoDir, wtDir, "SUB-105", false, baseNow.Add(3*time.Second)) != nil {
		t.Fatal("expected partially staged worktree with untracked file to miss receipt cache")
	}
	if err := os.Remove(filepath.Join(wtDir, constants.DirSubstrate, "extra.go")); err != nil {
		t.Fatal(err)
	}

	commitMsg := "[SUB-105] feat(calc): add Quad helper\n\nAdds Quad helper for working-tree receipt cache testing.\n\n___\nTask: SUB-105\n"
	mustRunGit(t, wtDir, "commit", "--no-verify", "-m", commitMsg)
	if loadValidWorktreeVerifyReceipt(repoDir, wtDir, "SUB-105", baseNow.Add(4*time.Second)) == nil {
		t.Fatal("expected post-commit clean HEAD^{tree} to reuse unstaged verify receipt during sync")
	}
}
