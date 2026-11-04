package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

func TestCleanroomDownstreamVectors_BrownfieldLifecycleAndSubdirContext(t *testing.T) {
	useCompiledNomosInPath(t)
	nomosHome := t.TempDir()
	t.Setenv("NOMOS_HOME", nomosHome)

	repoDir := filepath.Join(t.TempDir(), "acme-payments")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed creating repo dir: %v", err)
	}

	seedBrownfieldMasterRepo(t, repoDir)

	if err := executeInit(repoDir); err != nil {
		t.Fatalf("Vector 1 failed: executeInit on brownfield master repo: %v", err)
	}

	if err := runVerification(repoDir, "", true, true, false); err != nil {
		t.Fatalf("Vector 1 failed: brownfield master repo failed 42-gate verification: %v", err)
	}

	verifyDirectTrunkCommitBlocked(t, repoDir)
	wtPath := verifyTaskLifecycleAndSubdirResolution(t, repoDir)
	verifyMemoryRepoScopingAcrossWorktreeAndSubdir(t, repoDir, wtPath)
	verifyAdversarialShimRejectedAndRootTaskCommit(t, repoDir, wtPath)
}

func seedBrownfieldMasterRepo(t *testing.T, repoDir string) {
	t.Helper()
	_ = workspace.SafeGitCommand(repoDir, "init", "-b", "master").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.email", "dev@acme.io").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.name", "Acme Dev").Run()

	srcDir := filepath.Join(repoDir, "substrate", "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed creating srcDir: %v", err)
	}
	pySrc := "\"\"\"Acme payment engine.\"\"\"\n\ndef authorize_charge(cents: int) -> bool:\n    \"\"\"Authorize charge.\"\"\"\n    return cents > 0\n"
	if err := os.WriteFile(filepath.Join(srcDir, "payments.py"), []byte(pySrc), 0644); err != nil {
		t.Fatalf("failed writing payments.py: %v", err)
	}
	agentsMd := "# Acme Engineering Rules\n- Always validate currency amounts in integer cents\n"
	if err := os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte(agentsMd), 0644); err != nil {
		t.Fatalf("failed writing AGENTS.md: %v", err)
	}
	_ = workspace.SafeGitCommand(repoDir, "add", ".").Run()
	_ = workspace.SafeGitCommand(repoDir, "commit", "-m", "Initial brownfield master commit").Run()
}

func verifyDirectTrunkCommitBlocked(t *testing.T, repoDir string) {
	t.Helper()
	 rogueFile := filepath.Join(repoDir, "substrate", "src", "rogue.py")
	if err := os.WriteFile(rogueFile, []byte("x = 1\n"), 0644); err != nil {
		t.Fatalf("failed writing rogue.py: %v", err)
	}
	_ = workspace.SafeGitCommand(repoDir, "add", "substrate/src/rogue.py").Run()
	cmd := workspace.SafeGitCommand(repoDir, "commit", "-m", "unauthorized trunk commit")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Vector 2 failed: expected direct commit on master to be blocked by pre-commit hook, but succeeded: %s", string(out))
	}
	_ = workspace.SafeGitCommand(repoDir, "reset", "--hard", "HEAD").Run()
}

func verifyTaskLifecycleAndSubdirResolution(t *testing.T, repoDir string) string {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed getting cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed chdir to repoDir: %v", err)
	}

	// Test shorthand plane normalization (--plane SUB) with 3-section card synthesis
	card := taskCardFlags{
		scope:  "Implement refund handler in substrate/src/refund.go.",
		ac:     []string{"Handle idempotent refund requests"},
		verify: "go test ./...",
	}
	if err := executeTaskCreate("Add refund handler", taskCreateOpts{plane: "SUB", tier: "T1", priority: 50, card: card}); err != nil {
		t.Fatalf("Vector 3 failed: executeTaskCreate with shorthand plane 'SUB' failed: %v", err)
	}

	startCmd := newTaskStartCmd()
	if err := startCmd.RunE(startCmd, []string{"SUB-1"}); err != nil {
		t.Fatalf("Vector 3 failed: task start SUB-1 failed: %v", err)
	}

	editCmd := newTaskEditCmd()
	if err := editCmd.RunE(editCmd, []string{"SUB-1"}); err != nil {
		t.Fatalf("Vector 3 failed: task edit SUB-1 failed: %v", err)
	}

	mgr, err := workspace.NewManager(repoDir)
	if err != nil {
		t.Fatalf("failed creating workspace manager: %v", err)
	}
	wtPath := mgr.WorktreePath("SUB-1")
	nestedDir := filepath.Join(wtPath, "substrate", "src", "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed creating nestedDir: %v", err)
	}

	// Verify handshake from nested subdirectory inside worktree resolves canonical repo name & EDIT phase
	ev, err := handshake.SynthesizeDynamicHandshake(nestedDir)
	if err != nil {
		t.Fatalf("Vector 4 failed: SynthesizeDynamicHandshake from nested worktree dir failed: %v", err)
	}
	if ev.Repository != "acme-payments" {
		t.Errorf("Vector 4 failed: expected Repository 'acme-payments' from nested worktree dir, got %q", ev.Repository)
	}
	if ev.ActiveTask != "SUB-1" || string(ev.Phase) != "EDIT" {
		t.Errorf("Vector 4 failed: expected ActiveTask=SUB-1 and Phase=EDIT, got task=%q phase=%q", ev.ActiveTask, ev.Phase)
	}
	return wtPath
}

func verifyMemoryRepoScopingAcrossWorktreeAndSubdir(t *testing.T, repoDir, wtPath string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed getting cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()

	nestedWorktreeDir := filepath.Join(wtPath, "substrate", "src", "nested")
	if err := os.Chdir(nestedWorktreeDir); err != nil {
		t.Fatalf("failed chdir to nestedWorktreeDir: %v", err)
	}
	wtRepoID := resolveCurrentRepoID()
	if wtRepoID != "acme-payments" {
		t.Fatalf("Vector 4 failed: expected resolveCurrentRepoID() inside worktree subdir to be 'acme-payments', got %q", wtRepoID)
	}

	saveCmd := newMemSaveCmd()
	if err := saveCmd.RunE(saveCmd, []string{"Refunds must be idempotent per transaction ID"}); err != nil {
		t.Fatalf("failed saving memory from worktree subdir: %v", err)
	}

	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed chdir to repoDir: %v", err)
	}
	trunkRepoID := resolveCurrentRepoID()
	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err != nil {
		t.Fatalf("failed opening memory store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()
	recalled, err := store.SearchWithOptions("idempotent", mem.SearchOptions{RepoID: trunkRepoID, Limit: 5})
	if err != nil || len(recalled) != 1 {
		t.Fatalf("Vector 4 failed: expected memory saved in worktree to be recallable from trunk under repo %q, got %d results (err=%v)", trunkRepoID, len(recalled), err)
	}
	if !strings.Contains(recalled[0].Memory.Content, "idempotent") {
		t.Errorf("unexpected recalled memory content: %s", recalled[0].Memory.Content)
	}
}

func verifyAdversarialShimRejectedAndRootTaskCommit(t *testing.T, repoDir, wtPath string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed getting cwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(origDir)
	}()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("failed chdir to repoDir: %v", err)
	}

	shimFile := filepath.Join(wtPath, "substrate", "src", "refunds.py")
	if err := os.WriteFile(shimFile, []byte("def empty_shim():\n    pass\n"), 0644); err != nil {
		t.Fatalf("failed writing shimFile: %v", err)
	}
	_ = workspace.SafeGitCommand(wtPath, "add", "substrate/src/refunds.py").Run()

	if _, gateErr := executeGateCheckForTask("SUB-1"); gateErr == nil {
		t.Fatalf("Vector 5 failed: expected adversarial Python shim to fail gate check for SUB-1")
	}

	validPy := "\"\"\"Refund processor.\"\"\"\n\ndef process_refund(amount_cents: int) -> int:\n    \"\"\"Process refund.\"\"\"\n    if amount_cents <= 0:\n        return 0\n    return amount_cents\n"
	if err := os.WriteFile(shimFile, []byte(validPy), 0644); err != nil {
		t.Fatalf("failed writing valid refunds.py: %v", err)
	}

	// Execute nomos task commit from repoRoot (without -d) to verify automatic worktree resolution
	opts := &commitCmdOptions{
		summary:   "Implement refund processor",
		rationale: "Support idempotent customer refunds without duplicate settlement charges.",
		mechanics: "- Added process_refund with positive cents guard.",
		autoStage: true,
	}
	if err := runTaskCommitCmd([]string{"SUB-1"}, opts); err != nil {
		t.Fatalf("Vector 5 failed: expected runTaskCommitCmd from repoRoot to auto-resolve worktree and succeed, got: %v", err)
	}
	verifyDirtyWorktreeSyncAutoCommit(t, repoDir, wtPath)
}

func verifyDirtyWorktreeSyncAutoCommit(t *testing.T, repoDir, wtPath string) {
	t.Helper()
	extraFile := filepath.Join(wtPath, "substrate", "src", "receipt.py")
	extraPy := "\"\"\"Receipt formatter.\"\"\"\n\ndef format_receipt(tx_id: str) -> str:\n    \"\"\"Format receipt string.\"\"\"\n    if not tx_id:\n        return \"invalid\"\n    return f\"rcpt:{tx_id}\"\n"
	if err := os.WriteFile(extraFile, []byte(extraPy), 0644); err != nil {
		t.Fatalf("failed writing extraFile: %v", err)
	}

	walkPath := filepath.Join(repoDir, ".tmp", "walkthrough.md")
	if err := os.MkdirAll(filepath.Dir(walkPath), 0755); err != nil {
		t.Fatalf("failed creating .tmp dir: %v", err)
	}
	if err := os.WriteFile(walkPath, []byte("# Walkthrough SUB-1\nVerified refund and receipt handlers.\n"), 0644); err != nil {
		t.Fatalf("failed writing walkthrough.md: %v", err)
	}

	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{"SUB-1", "-F", walkPath})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("Vector 5 failed: expected nomos task sync SUB-1 to auto-commit dirty worktree and merge to master: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "substrate", "src", "receipt.py")); err != nil {
		t.Fatalf("Vector 5 failed: expected uncommitted receipt.py to be auto-committed and merged to master on sync: %v", err)
	}
}

