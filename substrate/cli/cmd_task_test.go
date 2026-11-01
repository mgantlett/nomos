package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func setupTaskTestDB(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_task_cmd.db")
	t.Setenv(constants.EnvNomosDB, dbPath)
	return dbPath
}

func TestTaskCmd_CreateAndList(t *testing.T) {
	dbPath := setupTaskTestDB(t)
	if dbPath == "" {
		t.Fatal("empty dbPath")
	}

	// Test task create
	createCmd := newTaskCreateCmd()
	createBuf := new(bytes.Buffer)
	createCmd.SetOut(createBuf)
	createCmd.SetErr(createBuf)
	createCmd.SetArgs([]string{"Build Pure Go Persistence", "--plane", "substrate", "--tier", "T1", "--priority", "85"})

	if err := createCmd.Execute(); err != nil {
		t.Fatalf("task create failed: %v", err)
	}

	// Test task list
	listCmd := newTaskListCmd()
	listBuf := new(bytes.Buffer)
	listCmd.SetOut(listBuf)
	listCmd.SetErr(listBuf)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	// Test task list JSON
	listJSONCmd := newTaskListCmd()
	listJSONBuf := new(bytes.Buffer)
	listJSONCmd.SetOut(listJSONBuf)
	listJSONCmd.SetErr(listJSONBuf)
	listJSONCmd.SetArgs([]string{"--json"})

	if err := listJSONCmd.Execute(); err != nil {
		t.Fatalf("task list --json failed: %v", err)
	}

	// Test task show
	showCmd := newTaskShowCmd()
	showBuf := new(bytes.Buffer)
	showCmd.SetOut(showBuf)
	showCmd.SetErr(showBuf)
	showCmd.SetArgs([]string{"SUB-1"})

	if err := showCmd.Execute(); err != nil {
		t.Fatalf("task show failed: %v", err)
	}
}

func TestTaskCmd_DAG(t *testing.T) {
	dbPath := setupTaskTestDB(t)
	if dbPath == "" {
		t.Fatal("empty dbPath")
	}

	// Create two tasks
	createCmd := newTaskCreateCmd()
	createCmd.SetArgs([]string{"Task Alpha", "--plane", "substrate"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("failed creating task alpha: %v", err)
	}

	dagCmd := newTaskDAGCmd()
	dagBuf := new(bytes.Buffer)
	dagCmd.SetOut(dagBuf)
	dagCmd.SetErr(dagBuf)

	if err := dagCmd.Execute(); err != nil {
		t.Fatalf("task dag failed: %v", err)
	}

	dagJSONCmd := newTaskDAGCmd()
	dagJSONBuf := new(bytes.Buffer)
	dagJSONCmd.SetOut(dagJSONBuf)
	dagJSONCmd.SetErr(dagJSONBuf)
	dagJSONCmd.SetArgs([]string{"--json"})

	if err := dagJSONCmd.Execute(); err != nil {
		t.Fatalf("task dag --json failed: %v", err)
	}
}

func TestTaskCmd_EditAndAbort(t *testing.T) {
	_ = setupTaskTestDB(t)

	// Create task
	createCmd := newTaskCreateCmd()
	createCmd.SetArgs([]string{"Task Beta", "--plane", "substrate"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("failed creating task beta: %v", err)
	}

	// Edit task
	editCmd := newTaskEditCmd()
	editCmd.SetArgs([]string{"SUB-1"})
	if err := editCmd.Execute(); err != nil {
		t.Fatalf("task edit failed: %v", err)
	}

	// Abort task
	abortCmd := newTaskAbortCmd()
	abortCmd.SetArgs([]string{"SUB-1"})
	if err := abortCmd.Execute(); err != nil {
		t.Fatalf("task abort failed: %v", err)
	}
}

func TestTaskCmd_PairAndTwinOf(t *testing.T) {
	_ = setupTaskTestDB(t)

	// 1. Test nomos task pair
	pairCmd := newTaskPairCmd()
	pairCmd.SetArgs([]string{"Bilateral Feature Pair"})
	if err := pairCmd.Execute(); err != nil {
		t.Fatalf("task pair failed: %v", err)
	}

	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("failed acquiring store: %v", err)
	}

	intTask, err := store.GetTask("INT-1")
	if err != nil {
		t.Fatalf("failed getting INT-1: %v", err)
	}
	if intTask.TwinKey != "SUB-1" {
		t.Errorf("expected INT-1 twin to be SUB-1, got %q", intTask.TwinKey)
	}

	subTask, err := store.GetTask("SUB-1")
	if err != nil {
		t.Fatalf("failed getting SUB-1: %v", err)
	}
	if subTask.TwinKey != "INT-1" {
		t.Errorf("expected SUB-1 twin to be INT-1, got %q", subTask.TwinKey)
	}

	// 2. Test nomos task create with --twin-of
	createCmd := newTaskCreateCmd()
	createCmd.SetArgs([]string{"Manual Twin Task", "--plane", "substrate", "--twin-of", "INT-1"})
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("task create --twin-of failed: %v", err)
	}

	sub2, err := store.GetTask("SUB-2")
	if err != nil {
		t.Fatalf("failed getting SUB-2: %v", err)
	}
	if sub2.TwinKey != "INT-1" {
		t.Errorf("expected SUB-2 twin to be INT-1, got %q", sub2.TwinKey)
	}
}

func TestTaskCmd_Prune(t *testing.T) {
	_ = setupTaskTestDB(t)

	pruneCmd := newTaskPruneCmd()
	buf := new(bytes.Buffer)
	pruneCmd.SetOut(buf)
	pruneCmd.SetErr(buf)
	pruneCmd.SetArgs([]string{"--dry-run", "--json"})

	if err := pruneCmd.Execute(); err != nil {
		t.Fatalf("task prune --dry-run --json failed: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected non-empty JSON prune report")
	}
}

