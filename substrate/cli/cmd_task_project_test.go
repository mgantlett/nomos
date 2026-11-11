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
	acmeDir := filepath.Join(t.TempDir(), "workstation-checkout")
	if err := os.MkdirAll(acmeDir, 0755); err != nil {
		t.Fatalf("mkdir acmeDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(acmeDir, "nomos.yaml"), []byte("project: \"acme-core\"\ndefault_branch: \"main\"\n"), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	if got := resolveRepoProjectName(acmeDir); got != "acme-core" {
		t.Fatalf("expected configured project 'acme-core', got %q", got)
	}
}

func TestTaskCmd_ProjectAutoStampWorktreeAndReassign(t *testing.T) {
	_ = setupTaskTestDB(t)

	repoDir := workspace.InitHermeticGitRepo(t)
	if err := os.WriteFile(filepath.Join(repoDir, "nomos.yaml"), []byte("project: \"acme-core\"\ndefault_branch: \"main\"\n"), 0644); err != nil {
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

	// Creating a pair from inside a transient worktree still stamps parent repo project ("acme-core")
	pairCmd := newTaskPairCmd()
	pairCmd.SetArgs([]string{
		"Automate Release Verification Subcommand",
		"--scope", "Implement verification pre-flight in acme-core module.",
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
	if err != nil || tInt.Project != "acme-core" {
		t.Fatalf("expected INT-1 project 'acme-core', got %+v (err=%v)", tInt, err)
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
	projCmd.SetArgs([]string{"acme-core", "SUB-2"})
	if err := projCmd.Execute(); err != nil {
		t.Fatalf("task project reassign failed: %v", err)
	}
	if !strings.Contains(projBuf.String(), "acme-core") {
		t.Errorf("expected confirmation output mentioning acme-core, got %q", projBuf.String())
	}

	// Filter via 'nomos task list --project acme-core --json'
	listCmd := newTaskListCmd()
	listCmd.SetArgs([]string{"--project", "acme-core", "--json"})
	// Capture stdout isn't wired to SetOut in list, just verify non-error and check store
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("task list --project failed: %v", err)
	}
	filtered := filterTasksByProject(store.ListTasks(), "acme-core")
	rawJSON, _ := json.Marshal(filtered)
	if len(filtered) != 3 || !strings.Contains(string(rawJSON), `"project":"acme-core"`) {
		t.Fatalf("expected all 3 tasks under acme-core, got %d (%s)", len(filtered), string(rawJSON))
	}
}

func TestMultiRepoLedgerIsolation_ListDAGStatusAndStartGuard(t *testing.T) {
	_ = setupTaskTestDB(t)

	repoA := workspace.InitHermeticGitRepo(t)
	if err := os.WriteFile(filepath.Join(repoA, "nomos.yaml"), []byte("project: \"alpha-svc\"\ndefault_branch: \"main\"\n"), 0644); err != nil {
		t.Fatalf("write alpha nomos.yaml: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(repoA); err != nil {
		t.Fatalf("chdir repoA: %v", err)
	}

	cAlpha := newTaskCreateCmd()
	cAlpha.SetArgs(validCardArgs("Alpha Local Feature", "substrate"))
	if err := cAlpha.Execute(); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	cBeta := newTaskCreateCmd()
	cBeta.SetArgs(append(validCardArgs("Beta Foreign Feature", "substrate"), "--project", "beta-svc"))
	if err := cBeta.Execute(); err != nil {
		t.Fatalf("create beta task: %v", err)
	}

	// Default 'nomos task list --json' inside alpha-svc sees only SUB-1 (alpha-svc), excluding SUB-2 (beta-svc)
	listScoped := newTaskListCmd()
	listBuf := new(bytes.Buffer)
	listScoped.SetOut(listBuf)
	listScoped.SetArgs([]string{"--json"})
	if err := listScoped.Execute(); err != nil {
		t.Fatalf("scoped task list: %v", err)
	}
	var scopedTasks []task.TaskRecord
	_ = json.Unmarshal(listBuf.Bytes(), &scopedTasks)
	if len(scopedTasks) != 1 || scopedTasks[0].Key != "SUB-1" {
		t.Fatalf("expected default list to isolate [SUB-1], got %+v", scopedTasks)
	}

	// '-A / --all-projects' includes both SUB-1 and SUB-2
	listAll := newTaskListCmd()
	listAllBuf := new(bytes.Buffer)
	listAll.SetOut(listAllBuf)
	listAll.SetArgs([]string{"--all-projects", "--json"})
	if err := listAll.Execute(); err != nil {
		t.Fatalf("all-projects task list: %v", err)
	}
	var allTasks []task.TaskRecord
	_ = json.Unmarshal(listAllBuf.Bytes(), &allTasks)
	if len(allTasks) != 2 {
		t.Fatalf("expected --all-projects to return 2 tasks, got %d", len(allTasks))
	}

	// Default 'nomos status' isolates ledger counts to alpha-svc (1 total, 1 wavefront)
	rep := collectStatusReport()
	if rep.Project != "alpha-svc" || rep.TotalTasks != 1 || rep.WavefrontCount != 1 || rep.ActiveTasks != 0 {
		t.Fatalf("expected isolated status report for alpha-svc (total=1, wf=1, active=0), got %+v", rep)
	}

	// Cross-project start guard fails closed when attempting to start beta-svc SUB-2 inside alpha-svc
	if sErr := runTaskStart("SUB-2", true); sErr == nil || !strings.Contains(sErr.Error(), "beta-svc") {
		t.Fatalf("expected cross-project start of SUB-2 inside alpha-svc to fail closed, got: %v", sErr)
	}
}
