package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func setupTaskTestDB(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_task_cmd.db")
	t.Setenv(constants.EnvNomosDB, dbPath)
	return dbPath
}

func validCardArgs(title, plane string) []string {
	return []string{
		title,
		"--plane", plane,
		"--scope", "Implement " + title + " in substrate/cli/ and substrate/src/nomos/modules/task/.",
		"--ac", "Enforce 3-section backlog card contract",
		"--verify", "go test ./substrate/cli/...",
	}
}

func TestTaskCmd_CreateAndList(t *testing.T) {
	dbPath := setupTaskTestDB(t)
	if dbPath == "" {
		t.Fatal("empty dbPath")
	}

	createCmd := newTaskCreateCmd()
	createBuf := new(bytes.Buffer)
	createCmd.SetOut(createBuf)
	createCmd.SetErr(createBuf)
	args := append(validCardArgs("Build Pure Go Persistence", "substrate"), "--tier", "T1", "--priority", "85")
	createCmd.SetArgs(args)

	if err := createCmd.Execute(); err != nil {
		t.Fatalf("task create failed: %v", err)
	}

	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	created, err := store.GetTask("SUB-1")
	if err != nil {
		t.Fatalf("get SUB-1: %v", err)
	}
	if created.Burden < 2 {
		t.Errorf("expected auto-derived burden >= 2, got %d", created.Burden)
	}
	if !strings.Contains(created.Description, "## Acceptance Criteria") {
		t.Errorf("expected rendered 3-section card, got:\n%s", created.Description)
	}

	listCmd := newTaskListCmd()
	listBuf := new(bytes.Buffer)
	listCmd.SetOut(listBuf)
	listCmd.SetErr(listBuf)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	listJSONCmd := newTaskListCmd()
	listJSONBuf := new(bytes.Buffer)
	listJSONCmd.SetOut(listJSONBuf)
	listJSONCmd.SetErr(listJSONBuf)
	listJSONCmd.SetArgs([]string{"--json"})
	if err := listJSONCmd.Execute(); err != nil {
		t.Fatalf("task list --json failed: %v", err)
	}

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

	createCmd := newTaskCreateCmd()
	createCmd.SetArgs(validCardArgs("Task Alpha", "substrate"))
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

	createCmd := newTaskCreateCmd()
	createCmd.SetArgs(validCardArgs("Task Beta", "substrate"))
	if err := createCmd.Execute(); err != nil {
		t.Fatalf("failed creating task beta: %v", err)
	}

	editCmd := newTaskEditCmd()
	editCmd.SetArgs([]string{"SUB-1"})
	if err := editCmd.Execute(); err != nil {
		t.Fatalf("task edit failed: %v", err)
	}

	abortCmd := newTaskAbortCmd()
	abortCmd.SetArgs([]string{"SUB-1"})
	if err := abortCmd.Execute(); err != nil {
		t.Fatalf("task abort failed: %v", err)
	}
}

func TestTaskCmd_PairAndTwinOf(t *testing.T) {
	_ = setupTaskTestDB(t)

	pairCmd := newTaskPairCmd()
	pairCmd.SetArgs([]string{
		"Bilateral Feature Pair",
		"--int-scope", "Codify bilateral contract in intent/contracts/cue/task.cue.",
		"--sub-scope", "Enforce bilateral contract in substrate/cli/cmd_task.go.",
		"--ac", "Pass 42 DoD gates across both planes",
		"--verify", "nomos verify",
		"--burden", "4",
	})
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
	if intTask.TwinKey != "SUB-1" || intTask.Burden != 4 || !strings.Contains(intTask.Description, "intent/contracts/cue/task.cue") {
		t.Errorf("unexpected INT-1 state: twin=%q burden=%d desc=%q", intTask.TwinKey, intTask.Burden, intTask.Description)
	}

	subTask, err := store.GetTask("SUB-1")
	if err != nil {
		t.Fatalf("failed getting SUB-1: %v", err)
	}
	if subTask.TwinKey != "INT-1" || subTask.Burden != 4 || !strings.Contains(subTask.Description, "substrate/cli/cmd_task.go") {
		t.Errorf("unexpected SUB-1 state: twin=%q burden=%d desc=%q", subTask.TwinKey, subTask.Burden, subTask.Description)
	}

	createCmd := newTaskCreateCmd()
	args := append(validCardArgs("Manual Twin Task", "substrate"), "--twin-of", "INT-1")
	createCmd.SetArgs(args)
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

func TestTaskCmd_StdinMarkdownAndFailClosedRemedy(t *testing.T) {
	_ = setupTaskTestDB(t)

	// TC-TSK-10: Plain-text -d fails closed with HATEOAS remedy
	badCmd := newTaskCreateCmd()
	badCmd.SilenceErrors = true
	badCmd.SilenceUsage = true
	badCmd.SetArgs([]string{"Unstructured Backlog Task", "--plane", "substrate", "-d", "Implement (1) foo; (2) bar; (3) baz."})
	err := badCmd.Execute()
	if err == nil {
		t.Fatal("TC-TSK-10: expected plain-text -d on task create to fail closed")
	}
	nErr, ok := err.(*types.NomosError)
	if !ok || !strings.Contains(nErr.Remedy, "--scope") || !strings.Contains(nErr.Remedy, "nomos task chore") {
		t.Fatalf("expected NomosError with --scope HATEOAS remedy, got: %T (%v)", err, err)
	}

	// TV-SCH-02: Stdin Markdown (-F -) succeeds on both create and pair
	stdinCard := "## Scope & Context\nWire native hooks in substrate/cli/.\n\n## Acceptance Criteria\n- [ ] Add pre-tool guard\n- [ ] Add post-tool AST check\n\n## Verification\n- go test ./substrate/cli/...\n"
	stdinPair := newTaskPairCmd()
	stdinPair.SetIn(strings.NewReader(stdinCard))
	stdinPair.SetArgs([]string{"Stdin Pair Feature", "-F", "-"})
	if err := stdinPair.Execute(); err != nil {
		t.Fatalf("TV-SCH-02: task pair -F - failed: %v", err)
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
