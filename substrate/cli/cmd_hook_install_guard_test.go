package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func seedCleanNomosTrunkRepo(t *testing.T) (string, string) {
	t.Helper()
	repoDir := filepath.Join(t.TempDir(), "clean-trunk-repo")
	subDir := filepath.Join(repoDir, "substrate")
	cmdNomos := filepath.Join(subDir, "cmd", "nomos")
	cmdPlugin := filepath.Join(subDir, "cmd", "nomos-plugin-demo")
	for _, dir := range []string{cmdNomos, cmdPlugin} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	files := map[string]string{
		filepath.Join(repoDir, ".gitignore"):           "worktrees/\n",
		filepath.Join(subDir, "go.mod"):                "module github.com/mgantlett/nomos/substrate\n\ngo 1.24\n",
		filepath.Join(cmdNomos, "main.go"):             "package main\n\nfunc main() {}\n",
		filepath.Join(cmdPlugin, "main.go"):            "package main\n\nfunc main() {}\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	runGitForInstallGuardTest(t, repoDir, "init", "-b", "main")
	runGitForInstallGuardTest(t, repoDir, "config", "user.email", "verify@example.com")
	runGitForInstallGuardTest(t, repoDir, "config", "user.name", "Nomos Test")
	runGitForInstallGuardTest(t, repoDir, "add", "-A")
	runGitForInstallGuardTest(t, repoDir, "commit", "-m", "chore: initial clean main trunk")
	return repoDir, subDir
}

func runGitForInstallGuardTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
	}
}

// TestHookPreTool_GoInstallGuardQuotedProseAndUnrelatedPackage verifies that quoted CLI prose
// and unrelated Go modules inside 'nomos'-named parent paths never false-trigger Law 4 denials.
func TestHookPreTool_GoInstallGuardQuotedProseAndUnrelatedPackage(t *testing.T) {
	t.Setenv(constants.EnvNomosDB, filepath.Join(t.TempDir(), "isolated_global.db"))

	parentWithNomos := filepath.Join(t.TempDir(), "nomos-parent-dir", "hello-svc")
	helloCmd := filepath.Join(parentWithNomos, "cmd", "hello")
	if err := os.MkdirAll(helloCmd, 0755); err != nil {
		t.Fatalf("mkdir helloCmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parentWithNomos, "go.mod"), []byte("module example.com/hello\n\ngo 1.24\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(helloCmd, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	allowed := []struct {
		name string
		cwd  string
		cmd  string
	}{
		{
			name: "double-quoted task create prose mentioning go install and nomos",
			cwd:  parentWithNomos,
			cmd:  `nomos task create "Scope go install guard for nomos" --scope "Prevent dirty go install ./cmd/nomos"`,
		},
		{
			name: "single-quoted git commit message mentioning go install and nomos",
			cwd:  parentWithNomos,
			cmd:  `git commit -m 'fix(cli): tighten go install ./cmd/nomos guard'`,
		},
		{
			name: "go test on nomos package is not go install",
			cwd:  parentWithNomos,
			cmd:  `go test ./cmd/nomos/...`,
		},
		{
			name: "unrelated Go binary inside nomos-named parent directory",
			cwd:  parentWithNomos,
			cmd:  `go install ./cmd/hello`,
		},
	}

	for _, tc := range allowed {
		out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"cwd":        tc.cwd,
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": tc.cmd},
		})
		if dec := parseDecision(t, out); dec.Decision != "allow" {
			t.Fatalf("%s: expected allow, got %+v", tc.name, dec)
		}
	}
}

// TestHookPreTool_GoInstallGuardCleanIdleMainTrunkAllowed verifies that local relative
// binary rebuilds succeed strictly when the repository is IDLE on a clean primary 'main' trunk.
func TestHookPreTool_GoInstallGuardCleanIdleMainTrunkAllowed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "isolated_global.db")
	t.Setenv(constants.EnvNomosDB, dbPath)
	repoDir, subDir := seedCleanNomosTrunkRepo(t)
	otherDir := t.TempDir()

	allowed := []struct {
		name string
		cwd  string
		cmd  string
	}{
		{
			name: "direct cwd on clean main substrate",
			cwd:  subDir,
			cmd:  "go install ./cmd/nomos",
		},
		{
			name: "chained cd into clean main substrate with standard build flags",
			cwd:  otherDir,
			cmd:  `cd "` + subDir + `" && GOOS=linux go install -trimpath -ldflags "-s -w" ./cmd/nomos`,
		},
		{
			name: "plugin wildcard install on clean main substrate",
			cwd:  repoDir,
			cmd:  "cd substrate && go install ./cmd/...",
		},
	}

	for _, tc := range allowed {
		out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"cwd":        tc.cwd,
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": tc.cmd},
		})
		if dec := parseDecision(t, out); dec.Decision != "allow" {
			t.Fatalf("%s: expected allow on clean IDLE main trunk, got %+v", tc.name, dec)
		}
	}
}

// TestHookPreTool_GoInstallGuardRejectsUnsafeFlagsSubshellsAndHermetic verifies fail-closed
// rejection of @latest specifiers, unsafe flags, subshell wrappers, and --hermetic-dir containers.
func TestHookPreTool_GoInstallGuardRejectsUnsafeFlagsSubshellsAndHermetic(t *testing.T) {
	t.Setenv(constants.EnvNomosDB, filepath.Join(t.TempDir(), "isolated_global.db"))
	_, subDir := seedCleanNomosTrunkRepo(t)

	denied := []string{
		"go install github.com/mgantlett/nomos/substrate/cmd/nomos@latest",
		"go install ./cmd/nomos@latest",
		"go install -overlay=hack.json ./cmd/nomos",
		"go install -modfile=alt.mod ./cmd/nomos",
		"go -C /tmp install ./cmd/nomos",
		"GOFLAGS=-overlay=hack.json go install ./cmd/nomos",
		"bash -lc 'go install ./cmd/nomos@latest'",
		`echo "$(go install ./cmd/nomos@latest)"`,
	}
	for _, cmdStr := range denied {
		out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"cwd":        subDir,
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": cmdStr},
		})
		if dec := parseDecision(t, out); dec.Decision != "deny" {
			t.Fatalf("expected %q to be denied, got %+v", cmdStr, dec)
		}
	}

	hermeticOut := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", subDir}, map[string]any{
		"cwd":        subDir,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "go install ./cmd/nomos"},
	})
	if dec := parseDecision(t, hermeticOut); dec.Decision != "deny" {
		t.Fatalf("expected --hermetic-dir to deny nomos go install, got %+v", dec)
	}
}

// TestHookPreTool_GoInstallGuardRejectsActiveTasksWorktreesDirtyAndBranches verifies fail-closed
// rejection whenever an active task, linked worktree, dirty file, or non-main branch is present.
func TestHookPreTool_GoInstallGuardRejectsActiveTasksWorktreesDirtyAndBranches(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "isolated_global.db")
	t.Setenv(constants.EnvNomosDB, dbPath)
	repoDir, subDir := seedCleanNomosTrunkRepo(t)

	assertDenied := func(label, cwd, cmd string) {
		t.Helper()
		out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"cwd":        cwd,
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": cmd},
		})
		if dec := parseDecision(t, out); dec.Decision != "deny" {
			t.Fatalf("%s: expected deny, got %+v", label, dec)
		}
	}

	// 1. Active IN_PROGRESS task in Living Ledger blocks install even when main tree is clean
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store := task.NewSQLiteTaskStore(db)
	rec := task.TaskRecord{
		Key: "SUB-77", Plane: "substrate", SchemaType: task.SchemaSubstrate,
		Title: "Active Task", Status: task.StatusInProgress, Tier: task.Tier1,
		Project: task.ResolveRepoProject(repoDir),
	}
	if err := store.CreateTask(rec); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	assertDenied("active IN_PROGRESS task on clean main", subDir, "go install ./cmd/nomos")

	// Reset DB to empty so subsequent checks isolate git state vectors
	t.Setenv(constants.EnvNomosDB, filepath.Join(t.TempDir(), "empty_global.db"))

	// 2. Linked Git worktree blocks both worktree cwd and trunk cwd
	wtPath := filepath.Join(repoDir, "worktrees", "clean-trunk-repo-SUB-77")
	runGitForInstallGuardTest(t, repoDir, "worktree", "add", "-b", "SUB-77", wtPath, "HEAD")
	assertDenied("inside linked worktree", filepath.Join(wtPath, "substrate"), "go install ./cmd/nomos")
	assertDenied("trunk cwd while linked worktree exists", subDir, "go install ./cmd/nomos")
	runGitForInstallGuardTest(t, repoDir, "worktree", "remove", "--force", wtPath)

	// 3. Dirty file on main trunk blocks install
	dirtyFile := filepath.Join(subDir, "untracked.tmp")
	if err := os.WriteFile(dirtyFile, []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}
	assertDenied("dirty untracked file on main", subDir, "go install ./cmd/nomos")
	if err := os.Remove(dirtyFile); err != nil {
		t.Fatalf("remove dirty: %v", err)
	}

	// 4. Non-main branch on primary checkout blocks install
	runGitForInstallGuardTest(t, repoDir, "checkout", "-b", "feature/drift")
	assertDenied("non-main branch on primary checkout", subDir, "go install ./cmd/nomos")
}
