package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

type hookDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func TestCleanroomDownstreamVectors_AmbientHookPayloadsAndTraversal(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repoDir := filepath.Join(t.TempDir(), "cdv-hooks-repo")
	seedCleanroomDualPlaneRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}

	wtGuard := filepath.Join(repoDir, ".agents", "scripts", "worktree-guard.sh")
	buildGuard := filepath.Join(repoDir, ".agents", "scripts", "nomos-build-guard.sh")
	wtFile := filepath.Join(repoDir, "worktrees", "cdv-hooks-repo-SUB-1", "src", "app.py")
	trunkFile := filepath.Join(repoDir, "substrate", "src", "app.py")
	traversalFile := filepath.Join(repoDir, "worktrees", "cdv-hooks-repo-SUB-1") + "/../../substrate/src/app.py"
	geminiArtifact := filepath.Join(t.TempDir(), ".gemini", "brain", "plan.md")

	verifyWorktreeGuardVectors(t, wtGuard, repoDir, wtFile, trunkFile, traversalFile, geminiArtifact)
	verifyBuildGuardVectors(t, buildGuard, repoDir)
}

func verifyWorktreeGuardVectors(t *testing.T, script, repoDir, wtFile, trunkFile, traversalFile, geminiArtifact string) {
	t.Helper()
	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{
			name:    "AGY direct trunk Path denied",
			payload: map[string]any{"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": trunkFile}}},
			want:    "deny",
		},
		{
			name:    "AGY worktree TargetFile allowed",
			payload: map[string]any{"toolCall": map[string]any{"name": "edit_file", "args": map[string]any{"TargetFile": wtFile}}},
			want:    "allow",
		},
		{
			name:    "Claude Code direct trunk file_path denied",
			payload: map[string]any{"tool_name": "Write", "tool_input": map[string]any{"file_path": trunkFile}},
			want:    "deny",
		},
		{
			name:    "Claude Code worktree file_path allowed",
			payload: map[string]any{"tool_name": "Edit", "tool_input": map[string]any{"file_path": wtFile}},
			want:    "allow",
		},
		{
			name:    "Adversarial dot-dot path traversal escaping worktree denied",
			payload: map[string]any{"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": traversalFile}}},
			want:    "deny",
		},
		{
			name:    "Relative path inside active worktree cwd allowed",
			payload: map[string]any{"cwd": filepath.Dir(wtFile), "tool_name": "Write", "tool_input": map[string]any{"file_path": "app.py"}},
			want:    "allow",
		},
		{
			name:    "Relative path from trunk cwd denied",
			payload: map[string]any{"cwd": repoDir, "tool_name": "Write", "tool_input": map[string]any{"file_path": "substrate/src/app.py"}},
			want:    "deny",
		},
		{
			name:    "Agent scratch artifact path allowed",
			payload: map[string]any{"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": geminiArtifact}}},
			want:    "allow",
		},
	}

	for _, tc := range cases {
		got := invokeHookScript(t, script, repoDir, tc.payload)
		if got.Decision != tc.want {
			t.Fatalf("%s: expected decision %q, got %q (reason: %s)", tc.name, tc.want, got.Decision, got.Reason)
		}
	}
}

func verifyBuildGuardVectors(t *testing.T, script, repoDir string) {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "state.db")
	hookTarget := filepath.Join(".git", "hooks", "pre-commit")
	cases := []struct {
		name    string
		payload map[string]any
		want    string
	}{
		{
			name:    "AGY manual go install nomos denied",
			payload: map[string]any{"toolCall": map[string]any{"name": "run_command", "args": map[string]any{"CommandLine": "go install ./cmd/nomos"}}},
			want:    "deny",
		},
		{
			name:    "Claude Code direct sqlite3 query denied",
			payload: map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "sqlite3 " + dbFile + " 'SELECT * FROM tasks;'"}},
			want:    "deny",
		},
		{
			name:    "Git stash mutation denied",
			payload: map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "git stash push -m wip"}},
			want:    "deny",
		},
		{
			name:    "Git stash list read-only allowed",
			payload: map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "git stash list"}},
			want:    "allow",
		},
		{
			name:    "Deleting git pre-commit hook denied",
			payload: map[string]any{"toolCall": map[string]any{"name": "run_command", "args": map[string]any{"CommandLine": "rm -f " + hookTarget}}},
			want:    "deny",
		},
		{
			name:    "Standard test execution allowed",
			payload: map[string]any{"tool_name": "Bash", "tool_input": map[string]any{"command": "go test ./..."}},
			want:    "allow",
		},
	}

	for _, tc := range cases {
		got := invokeHookScript(t, script, repoDir, tc.payload)
		if got.Decision != tc.want {
			t.Fatalf("%s: expected decision %q, got %q (reason: %s)", tc.name, tc.want, got.Decision, got.Reason)
		}
	}
}

func invokeHookScript(t *testing.T, scriptPath, workDir string, payload map[string]any) hookDecision {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}
	cmd := exec.Command("bash", scriptPath)
	cmd.Dir, cmd.Env = workDir, ensureCompiledHookTestEnv(t)
	cmd.Stdin = bytes.NewReader(raw)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hook script %s exited with error %v: %s", scriptPath, err, string(out))
	}
	var dec hookDecision
	if uErr := json.Unmarshal(out, &dec); uErr != nil {
		t.Fatalf("hook script %s returned invalid JSON %q: %v", scriptPath, string(out), uErr)
	}
	return dec
}

func TestCleanroomDownstreamVectors_ConcurrentWorktreesAndConflictRollback(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repoDir := filepath.Join(t.TempDir(), "cdv-concurrent-repo")
	seedCleanroomDualPlaneRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	verifyDivergedNonOverlappingRebaseSync(t, repoDir)
	verifyConflictingConcurrentSyncAbortsCleanly(t, repoDir)
	verifyConcurrentHandshakeWALBurst(t, repoDir)
}

func verifyDivergedNonOverlappingRebaseSync(t *testing.T, repoDir string) {
	t.Helper()
	wt1 := startCleanroomTaskInEdit(t, repoDir, "SUB", "Add billing module")
	wt2 := startCleanroomTaskInEdit(t, repoDir, "SUB", "Add invoicing module")

	billingPy := "\"\"\"Billing service.\"\"\"\n\ndef bill_amount(cents: int) -> int:\n    \"\"\"Compute billed cents.\"\"\"\n    return cents if cents > 0 else 0\n"
	writeWorktreeFile(t, wt1, filepath.Join("substrate", "src", "billing.py"), billingPy)

	invoicePy := "\"\"\"Invoice service.\"\"\"\n\ndef invoice_label(inv_id: str) -> str:\n    \"\"\"Format invoice label.\"\"\"\n    return f\"INV-{inv_id}\" if inv_id else \"INV-NONE\"\n"
	writeWorktreeFile(t, wt2, filepath.Join("substrate", "src", "invoicing.py"), invoicePy)

	syncCleanroomTask(t, repoDir, "SUB-1", "Synced billing module")
	syncCleanroomTask(t, repoDir, "SUB-2", "Synced invoicing module after concurrent rebase")

	for _, rel := range []string{"billing.py", "invoicing.py"} {
		if _, err := os.Stat(filepath.Join(repoDir, "substrate", "src", rel)); err != nil {
			t.Fatalf("expected %s on main after concurrent diverged sync: %v", rel, err)
		}
	}
	noteCmd := workspace.SafeGitCommand(repoDir, "notes", "--ref", "refs/notes/nomos/releases", "show", "HEAD")
	if out, err := noteCmd.CombinedOutput(); err != nil {
		t.Fatalf("expected release note receipt on HEAD after rebased sync: %v (%s)", err, string(out))
	}
}

func verifyConflictingConcurrentSyncAbortsCleanly(t *testing.T, repoDir string) {
	t.Helper()
	wt3 := startCleanroomTaskInEdit(t, repoDir, "SUB", "Update billing v1")
	wt4 := startCleanroomTaskInEdit(t, repoDir, "SUB", "Conflicting billing v2")

	v1 := "\"\"\"Billing service.\"\"\"\n\ndef bill_amount(cents: int) -> int:\n    \"\"\"Compute billed cents with fee.\"\"\"\n    return cents + 10 if cents > 0 else 0\n"
	v2 := "\"\"\"Billing service.\"\"\"\n\ndef bill_amount(cents: int) -> int:\n    \"\"\"Compute billed cents with discount.\"\"\"\n    return cents - 5 if cents > 10 else 0\n"
	writeWorktreeFile(t, wt3, filepath.Join("substrate", "src", "billing.py"), v1)
	writeWorktreeFile(t, wt4, filepath.Join("substrate", "src", "billing.py"), v2)

	syncCleanroomTask(t, repoDir, "SUB-3", "Applied billing fee")

	walk4 := writeWalkthroughTemp(t, repoDir, "SUB-4", "Conflicting billing discount")
	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{"SUB-4", "-F", walk4})
	if err := syncCmd.Execute(); err == nil {
		t.Fatalf("expected conflicting concurrent sync for SUB-4 to fail closed")
	}

	mainData, err := os.ReadFile(filepath.Join(repoDir, "substrate", "src", "billing.py"))
	if err != nil || !strings.Contains(string(mainData), "cents + 10") {
		t.Fatalf("expected main billing.py to remain untouched after SUB-4 conflict")
	}

	statusOut, sErr := workspace.SafeGitCommand(wt4, "status", "--porcelain").Output()
	if sErr != nil || strings.TrimSpace(string(statusOut)) != "" {
		t.Fatalf("expected SUB-4 worktree rebase to abort cleanly with clean status, got: %q (err=%v)", string(statusOut), sErr)
	}

	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("open task store: %v", err)
	}
	defer func() { _ = store.Close() }()
	rec, err := store.GetTask("SUB-4")
	if err != nil || rec.Status != task.StatusEdit {
		t.Fatalf("expected SUB-4 to remain in EDIT status after conflict abort, got %+v (err=%v)", rec, err)
	}
}

func verifyConcurrentHandshakeWALBurst(t *testing.T, repoDir string) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := handshake.SynthesizeDynamicHandshake(repoDir); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent SynthesizeDynamicHandshake failed under WAL burst: %v", err)
	}
}

func TestCleanroomDownstreamVectors_RogueNoVerifySymlinkCrossPlaneAndCrashRecovery(t *testing.T) {
	useCompiledNomosInPath(t)
	t.Setenv("NOMOS_HOME", t.TempDir())
	repoDir := filepath.Join(t.TempDir(), "cdv-adversarial-repo")
	seedCleanroomDualPlaneRepo(t, repoDir)

	wfDir := filepath.Join(repoDir, ".agents", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	writeRawFile(t, filepath.Join(wfDir, "nomos-handshake.md"), "# Deprecated Nomos Handshake\n")
	writeRawFile(t, filepath.Join(wfDir, "deploy.md"), "# User Custom Deploy Playbook\n")
	writeRawFile(t, filepath.Join(repoDir, "dirty-notes.txt"), "uncommitted developer scratchpad\n")

	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}
	verifyBrownfieldPreservationAndHookSelfHeal(t, repoDir, wfDir)

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	wt1 := startCleanroomTaskInEdit(t, repoDir, "SUB", "Hardened ledger service")
	verifyRogueNoVerifySymlinkAndStubBlocked(t, repoDir, wt1)
	verifyTornWorktreeCrashSelfHeal(t, repoDir, wt1)
	verifyCrossPlaneNoVerifyBlocked(t, repoDir)
}

func verifyBrownfieldPreservationAndHookSelfHeal(t *testing.T, repoDir, wfDir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(wfDir, "nomos-handshake.md")); !os.IsNotExist(err) {
		t.Fatalf("expected deprecated nomos-handshake.md to be pruned on brownfield init")
	}
	if _, err := os.Stat(filepath.Join(wfDir, "deploy.md")); err != nil {
		t.Fatalf("expected non-Nomos user workflow deploy.md to survive brownfield init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "dirty-notes.txt")); err != nil {
		t.Fatalf("expected uncommitted developer dirty-notes.txt to survive brownfield init: %v", err)
	}
	_ = os.Remove(filepath.Join(repoDir, "dirty-notes.txt"))

	preCommitPath := filepath.Join(repoDir, ".git", "hooks", "pre-commit")
	if err := os.Remove(preCommitPath); err != nil {
		t.Fatalf("remove pre-commit: %v", err)
	}
	reinstalled, err := workspace.EnsureHooksInstalled(repoDir)
	if err != nil || !reinstalled {
		t.Fatalf("expected EnsureHooksInstalled to detect missing pre-commit and reinstall (reinstalled=%v, err=%v)", reinstalled, err)
	}
}

func verifyRogueNoVerifySymlinkAndStubBlocked(t *testing.T, repoDir, wt1 string) {
	t.Helper()
	symPath := filepath.Join(wt1, "substrate", "src", "alias.py")
	if err := os.Symlink("seed.py", symPath); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}
	_ = workspace.SafeGitCommand(wt1, "add", "substrate/src/alias.py").Run()
	_ = workspace.SafeGitCommand(wt1, "commit", "--no-verify", "-m", "rogue symlink bypass").Run()

	walk := writeWalkthroughTemp(t, repoDir, "SUB-1", "Attempt rogue symlink sync")
	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{"SUB-1", "-F", walk})
	if err := syncCmd.Execute(); err == nil {
		t.Fatalf("expected nomos task sync to block --no-verify symlink commit (Law 6)")
	}

	_ = os.Remove(symPath)
	writeWorktreeFile(t, wt1, filepath.Join("substrate", "src", "shim.py"), "def broken_stub():\n    pass\n")
	_ = workspace.SafeGitCommand(wt1, "add", "-A").Run()
	_ = workspace.SafeGitCommand(wt1, "commit", "--no-verify", "-m", "rogue empty stub bypass").Run()

	syncCmd2 := newTaskSyncCmd()
	syncCmd2.SetArgs([]string{"SUB-1", "-F", walk})
	if err := syncCmd2.Execute(); err == nil {
		t.Fatalf("expected nomos task sync to block --no-verify stub commit (Law 2)")
	}
	_ = os.Remove(filepath.Join(wt1, "substrate", "src", "shim.py"))
}

func verifyTornWorktreeCrashSelfHeal(t *testing.T, repoDir, wt1 string) {
	t.Helper()
	_ = workspace.SafeGitCommand(wt1, "reset", "--hard", "main").Run()
	cleanLedger := "\"\"\"Ledger module.\"\"\"\n\ndef verify_balance(balance: int) -> bool:\n    \"\"\"Verify non-negative balance.\"\"\"\n    return balance >= 0\n"
	writeWorktreeFile(t, wt1, filepath.Join("substrate", "src", "ledger.py"), cleanLedger)
	_ = workspace.SafeGitCommand(wt1, "add", "-A").Run()
	opts := &commitCmdOptions{dir: wt1, summary: "Add ledger balance verification", autoStage: true}
	if err := runTaskCommitCmd([]string{"SUB-1"}, opts); err != nil {
		t.Fatalf("valid task commit failed: %v", err)
	}

	// Simulate abrupt crash / manual rm -rf of active worktree without git worktree prune
	if err := os.RemoveAll(wt1); err != nil {
		t.Fatalf("removeAll wt1: %v", err)
	}

	startCmd := newTaskStartCmd()
	if err := startCmd.RunE(startCmd, []string{"SUB-1"}); err != nil {
		t.Fatalf("expected nomos task start SUB-1 to self-heal torn worktree lock, got: %v", err)
	}
	editCmd := newTaskEditCmd()
	if err := editCmd.RunE(editCmd, []string{"SUB-1"}); err != nil {
		t.Fatalf("task edit SUB-1 failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wt1, "substrate", "src", "ledger.py")); err != nil {
		t.Fatalf("expected committed ledger.py to be restored when torn worktree re-attached: %v", err)
	}
	syncCleanroomTask(t, repoDir, "SUB-1", "Recovered and synced ledger service")
}

func verifyCrossPlaneNoVerifyBlocked(t *testing.T, repoDir string) {
	t.Helper()
	wtInt := startCleanroomTaskInEdit(t, repoDir, "INT", "Architecture spec update")
	writeWorktreeFile(t, wtInt, filepath.Join("substrate", "src", "illegal.py"), "\"\"\"Illegal cross-plane file.\"\"\"\n\ndef x() -> int:\n    \"\"\"Return 1.\"\"\"\n    return 1\n")
	_ = workspace.SafeGitCommand(wtInt, "add", "-A").Run()
	_ = workspace.SafeGitCommand(wtInt, "commit", "--no-verify", "-m", "rogue cross-plane commit").Run()

	walk := writeWalkthroughTemp(t, repoDir, "INT-1", "Attempt cross-plane sync")
	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{"INT-1", "-F", walk})
	err := syncCmd.Execute()
	if err == nil {
		t.Fatalf("expected INT-1 modifying substrate/ to fail PlaneSeparationGate at sync (Law 7)")
	}
	if nErr, ok := err.(*types.NomosError); !ok || nErr.Code != types.ErrCodeCrossPlaneMutation {
		t.Fatalf("expected ErrCodeCrossPlaneMutation, got: %T (%v)", err, err)
	}
}

func seedCleanroomDualPlaneRepo(t *testing.T, repoDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoDir, "substrate", "src"), 0755); err != nil {
		t.Fatalf("mkdir substrate/src: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "intent", "specs"), 0755); err != nil {
		t.Fatalf("mkdir intent/specs: %v", err)
	}
	_ = workspace.SafeGitCommand(repoDir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.email", "dev@acme.io").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.name", "Acme Dev").Run()

	writeRawFile(t, filepath.Join(repoDir, "substrate", "src", "seed.py"), "\"\"\"Seed module.\"\"\"\n\ndef seed_ok() -> bool:\n    \"\"\"Seed status.\"\"\"\n    return True\n")
	writeRawFile(t, filepath.Join(repoDir, "intent", "specs", "overview.md"), "# Architecture Overview\n")
	_ = workspace.SafeGitCommand(repoDir, "add", ".").Run()
	_ = workspace.SafeGitCommand(repoDir, "commit", "-m", "Initial cleanroom baseline").Run()
}

func startCleanroomTaskInEdit(t *testing.T, repoDir, plane, title string) string {
	t.Helper()
	card := taskCardFlags{
		scope:  "Cleanroom stress target for " + title + " in " + plane + " plane.",
		ac:     []string{"Complete cleanroom mutation and pass DoD gates"},
		verify: "nomos verify",
	}
	if err := executeTaskCreate(title, taskCreateOpts{plane: plane, tier: "T1", priority: 50, card: card}); err != nil {
		t.Fatalf("executeTaskCreate(%s) failed: %v", title, err)
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("open task store: %v", err)
	}
	tasks := store.ListTasks()
	_ = store.Close()
	var key string
	for _, rec := range tasks {
		if rec.Title == title {
			key = rec.Key
			break
		}
	}
	if key == "" {
		t.Fatalf("failed discovering created task key for %q", title)
	}
	startCmd := newTaskStartCmd()
	if err := startCmd.RunE(startCmd, []string{key}); err != nil {
		t.Fatalf("task start %s failed: %v", key, err)
	}
	editCmd := newTaskEditCmd()
	if err := editCmd.RunE(editCmd, []string{key}); err != nil {
		t.Fatalf("task edit %s failed: %v", key, err)
	}
	mgr, err := workspace.NewManager(repoDir)
	if err != nil {
		t.Fatalf("workspace.NewManager: %v", err)
	}
	return mgr.WorktreePath(key)
}

func syncCleanroomTask(t *testing.T, repoDir, key, summary string) {
	t.Helper()
	walk := writeWalkthroughTemp(t, repoDir, key, summary)
	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{key, "-F", walk})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("nomos task sync %s failed: %v", key, err)
	}
}

func writeWalkthroughTemp(t *testing.T, repoDir, key, summary string) string {
	t.Helper()
	p := filepath.Join(repoDir, ".tmp", "walkthrough-"+key+".md")
	writeRawFile(t, p, "# Walkthrough "+key+"\n\n"+summary+"\n")
	return p
}

func writeWorktreeFile(t *testing.T, wtPath, relPath, content string) {
	t.Helper()
	writeRawFile(t, filepath.Join(wtPath, relPath), content)
}

func writeRawFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
