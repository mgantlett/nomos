package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func TestResolveRepoProjectName(t *testing.T) {
	// 1. Brownfield directory without nomos.yaml uses directory basename, not "nomos"
	brownfieldDir := filepath.Join(t.TempDir(), "payments-api")
	if err := os.MkdirAll(brownfieldDir, 0755); err != nil {
		t.Fatalf("mkdir brownfield: %v", err)
	}
	if got := resolveRepoProjectName(brownfieldDir); got != "payments-api" {
		t.Fatalf("expected brownfield project 'payments-api', got %q", got)
	}

	// 2. Explicit nomos.yaml project overrides directory basename
	sovDir := filepath.Join(t.TempDir(), "workstation-checkout")
	if err := os.MkdirAll(sovDir, 0755); err != nil {
		t.Fatalf("mkdir sovDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sovDir, "nomos.yaml"), []byte("project: \"gsi-sov\"\ndefault_branch: \"main\"\n"), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	if got := resolveRepoProjectName(sovDir); got != "gsi-sov" {
		t.Fatalf("expected configured project 'gsi-sov', got %q", got)
	}
}

func TestTaskCmd_ProjectAutoStampWorktreeAndReassign(t *testing.T) {
	_ = setupTaskTestDB(t)

	repoDir := workspace.InitHermeticGitRepo(t)
	if err := os.WriteFile(filepath.Join(repoDir, "nomos.yaml"), []byte("project: \"gsi-sov\"\ndefault_branch: \"main\"\n"), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	mgr, err := workspace.NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wtPath, err := mgr.CreateWorktree("SUB-99")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		if chErr := os.Chdir(origWD); chErr != nil {
			t.Errorf("restore wd: %v", chErr)
		}
	})
	if err := os.Chdir(wtPath); err != nil {
		t.Fatalf("chdir worktree: %v", err)
	}

	// Creating a pair from inside a transient worktree still stamps parent repo project ("gsi-sov")
	pairCmd := newTaskPairCmd()
	pairCmd.SetArgs([]string{
		"Automate Ship Release Subcommand",
		"--scope", "Implement nomos ship release in gsi-sov plugin.",
		"--ac", "Cut annotated semver tag",
		"--verify", "nomos verify",
	})
	if err := pairCmd.Execute(); err != nil {
		t.Fatalf("task pair in worktree failed: %v", err)
	}

	// Explicit --project override
	createCmd := newTaskCreateCmd()
	createCmd.SetArgs(append(validCardArgs("Open Core Kernel Gate", "substrate"), "--project", "nomos"))
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("task create --project nomos failed: %v", err)
	}

	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			t.Errorf("close store: %v", cErr)
		}
	}()

	tInt, err := store.GetTask("INT-1")
	if err != nil || tInt.Project != "gsi-sov" {
		t.Fatalf("expected INT-1 project 'gsi-sov', got %+v (err=%v)", tInt, err)
	}
	tSub2, err := store.GetTask("SUB-2")
	if err != nil || tSub2.Project != "nomos" {
		t.Fatalf("expected SUB-2 project 'nomos', got %+v (err=%v)", tSub2, err)
	}

	// Fail-closed pre-flight if any target key is missing
	badProjCmd := newTaskProjectCmd()
	badProjCmd.SilenceErrors = true
	badProjCmd.SilenceUsage = true
	badProjCmd.SetArgs([]string{"other-proj", "SUB-2", "SUB-999"})
	if err := badProjCmd.Execute(); err == nil {
		t.Fatal("expected missing key SUB-999 to fail closed without mutating SUB-2")
	}
	unchanged, _ := store.GetTask("SUB-2")
	if unchanged.Project != "nomos" {
		t.Fatalf("expected atomic pre-flight to leave SUB-2 at 'nomos', got %q", unchanged.Project)
	}

	// Reassign project via 'nomos task project'
	projCmd := newTaskProjectCmd()
	projBuf := new(bytes.Buffer)
	projCmd.SetOut(projBuf)
	projCmd.SetArgs([]string{"gsi-sov", "SUB-2"})
	if err := projCmd.Execute(); err != nil {
		t.Fatalf("task project reassign failed: %v", err)
	}
	if !strings.Contains(projBuf.String(), "gsi-sov") {
		t.Errorf("expected confirmation output mentioning gsi-sov, got %q", projBuf.String())
	}

	// Filter via 'nomos task list --project gsi-sov --json'
	listCmd := newTaskListCmd()
	listCmd.SetArgs([]string{"--project", "gsi-sov", "--json"})
	// Capture stdout isn't wired to SetOut in list, just verify non-error and check store
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("task list --project failed: %v", err)
	}
	filtered := filterTasksByProject(store.ListTasks(), "gsi-sov")
	rawJSON, _ := json.Marshal(filtered)
	if len(filtered) != 3 || !strings.Contains(string(rawJSON), `"project":"gsi-sov"`) {
		t.Fatalf("expected all 3 tasks under gsi-sov, got %d (%s)", len(filtered), string(rawJSON))
	}
}
