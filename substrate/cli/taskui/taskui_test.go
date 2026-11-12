package taskui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func TestApplyTaskListFilters_AndRenderNextActions(t *testing.T) {
	records := []task.TaskRecord{
		{Key: "INT-1", Plane: "intent", Project: "nomos", Status: task.StatusDone, Tier: task.Tier1, Priority: 90, Title: "Spec alpha"},
		{Key: "SUB-1", Plane: "substrate", Project: "nomos", Status: task.StatusTodo, Tier: task.Tier1, Priority: 90, Title: "Impl alpha"},
		{Key: "SUB-2", Plane: "substrate", Project: "other", Status: task.StatusEdit, Tier: task.Tier2, Priority: 50, Title: "Impl beta"},
	}

	got, err := ApplyTaskListFilters(records, TaskListFilterOpts{
		Project: "nomos",
		Pending: true,
		Plane:   "SUB",
		Tier:    "T1",
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("ApplyTaskListFilters failed: %v", err)
	}
	if len(got) != 1 || got[0].Key != "SUB-1" {
		t.Fatalf("expected [SUB-1], got %+v", got)
	}

	if _, badErr := ApplyTaskListFilters(records, TaskListFilterOpts{Status: "BOGUS"}); badErr == nil {
		t.Fatalf("expected invalid status filter to fail closed")
	}

	var buf bytes.Buffer
	RenderTaskListNextActions(&buf, got, 2)
	if !strings.Contains(buf.String(), "SUB-1") {
		t.Fatalf("expected Next Actions to reference SUB-1, got:\n%s", buf.String())
	}

	if cmd := NewTaskProjectCmd(); cmd == nil || cmd.Use == "" {
		t.Fatalf("expected valid NewTaskProjectCmd")
	}
	if pCmd := NewTaskPruneCmd(); pCmd == nil || pCmd.Use == "" {
		t.Fatalf("expected valid NewTaskPruneCmd")
	}
}

