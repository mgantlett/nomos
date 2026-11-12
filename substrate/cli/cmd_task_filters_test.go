package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

func TestTaskListAndDAG_FilterFlagsAndJSONParity(t *testing.T) {
	_ = setupTaskTestDB(t)

	pairCmd := newTaskPairCmd()
	pairCmd.SetArgs([]string{
		"Filter Parity Pair",
		"--int-scope", "Document CLI filter contract in intent/architecture/specs/task_lifecycle_cli_spec.md.",
		"--sub-scope", "Implement CLI filter flags in substrate/cli/cmd_task_filters.go.",
		"--ac", "Verify filter flags and JSON output",
		"--verify", "go test ./substrate/cli/...",
		"--tier", "T1",
	})
	if err := pairCmd.Execute(); err != nil {
		t.Fatalf("task pair failed: %v", err)
	}

	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	doneTS := "2026-11-02T22:00:00Z"
	intTask, _ := store.GetTask("INT-1")
	intTask.Status = task.StatusDone
	intTask.CompletedAt = &doneTS
	if err := store.UpdateTask(*intTask); err != nil {
		t.Fatalf("update INT-1 DONE: %v", err)
	}

	// --pending excludes DONE tasks -> only SUB-1 returned
	pendingCmd := newTaskListCmd()
	pendingBuf := new(bytes.Buffer)
	pendingCmd.SetOut(pendingBuf)
	pendingCmd.SetArgs([]string{"--pending", "--json"})
	if err := pendingCmd.Execute(); err != nil {
		t.Fatalf("task list --pending --json failed: %v", err)
	}
	var pendingRecords []task.TaskRecord
	if err := json.Unmarshal(pendingBuf.Bytes(), &pendingRecords); err != nil {
		t.Fatalf("unmarshal pending JSON: %v", err)
	}
	if len(pendingRecords) != 1 || pendingRecords[0].Key != "SUB-1" {
		t.Fatalf("expected [SUB-1] for --pending, got %+v", pendingRecords)
	}

	// --status DONE --plane intent -> only INT-1 returned
	doneCmd := newTaskListCmd()
	doneBuf := new(bytes.Buffer)
	doneCmd.SetOut(doneBuf)
	doneCmd.SetArgs([]string{"--status", "DONE", "--plane", "intent", "--limit", "1", "--json"})
	if err := doneCmd.Execute(); err != nil {
		t.Fatalf("task list --status DONE --plane intent failed: %v", err)
	}
	var doneRecords []task.TaskRecord
	if err := json.Unmarshal(doneBuf.Bytes(), &doneRecords); err != nil {
		t.Fatalf("unmarshal done JSON: %v", err)
	}
	if len(doneRecords) != 1 || doneRecords[0].Key != "INT-1" {
		t.Fatalf("expected [INT-1] for --status DONE --plane intent, got %+v", doneRecords)
	}

	// Invalid --status fails closed with HATEOAS remedy
	badCmd := newTaskListCmd()
	badCmd.SilenceErrors = true
	badCmd.SilenceUsage = true
	badCmd.SetArgs([]string{"--status", "BOGUS"})
	if err := badCmd.Execute(); err == nil {
		t.Fatal("expected invalid --status BOGUS to fail closed")
	}

	// task show --json & task status --json & version --json
	showJSON := newTaskShowCmd()
	showBuf := new(bytes.Buffer)
	showJSON.SetOut(showBuf)
	showJSON.SetArgs([]string{"SUB-1", "--json"})
	if err := showJSON.Execute(); err != nil || !strings.Contains(showBuf.String(), `"SUB-1"`) {
		t.Fatalf("task show SUB-1 --json failed: err=%v out=%s", err, showBuf.String())
	}

	statusJSON := newTaskStatusCmd()
	statusBuf := new(bytes.Buffer)
	statusJSON.SetOut(statusBuf)
	statusJSON.SetArgs([]string{"--json"})
	if err := statusJSON.Execute(); err != nil || !strings.Contains(statusBuf.String(), `"phase"`) {
		t.Fatalf("task status --json failed: err=%v out=%s", err, statusBuf.String())
	}

	verJSON := selfupdate.NewVersionCmd()
	verBuf := new(bytes.Buffer)
	verJSON.SetOut(verBuf)
	verJSON.SetArgs([]string{"--json"})
	if err := verJSON.Execute(); err != nil || !strings.Contains(verBuf.String(), `"version"`) {
		t.Fatalf("version --json failed: err=%v out=%s", err, verBuf.String())
	}
}

func TestRootCommandTree_AllStaticCommandsHaveLongAndExample(t *testing.T) {
	t.Setenv("NOMOS_PLUGINS_ROOT", t.TempDir())
	root := NewRootCmd()
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if !c.DisableFlagParsing && c.Name() != "help" && c.Name() != "completion" {
			if strings.TrimSpace(c.Short) == "" || strings.TrimSpace(c.Long) == "" || strings.TrimSpace(c.Example) == "" {
				t.Errorf("command %q missing Short/Long/Example contract", c.CommandPath())
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
