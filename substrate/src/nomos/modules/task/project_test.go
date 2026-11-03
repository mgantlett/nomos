package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoProject_AndProjectMatching(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme-ledger")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := ResolveRepoProject(dir); got != "acme-ledger" {
		t.Fatalf("expected directory basename acme-ledger, got %q", got)
	}

	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte("version: \"1.0\"\nproject: \"custom-proj\"\n"), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	if got := ResolveRepoProject(dir); got != "custom-proj" {
		t.Fatalf("expected nomos.yaml project custom-proj, got %q", got)
	}

	tasks := []TaskRecord{
		{Key: "SUB-1", Project: "custom-proj", Status: StatusTodo},
		{Key: "SUB-2", Project: "other-repo", Status: StatusReady},
		{Key: "SUB-3", Project: "", Status: StatusEdit},
	}

	autoScoped := FilterTasksByProject(tasks, "custom-proj", true)
	if len(autoScoped) != 2 || autoScoped[0].Key != "SUB-1" || autoScoped[1].Key != "SUB-3" {
		t.Fatalf("expected auto-scoped filter to retain matching and unassigned records, got %+v", autoScoped)
	}

	strictScoped := FilterTasksByProject(tasks, "custom-proj", false)
	if len(strictScoped) != 1 || strictScoped[0].Key != "SUB-1" {
		t.Fatalf("expected strict filter to retain only custom-proj, got %+v", strictScoped)
	}

	if !IsTerminalStatus(StatusDone) || !IsTerminalStatus(StatusAborted) || !IsTerminalStatus(StatusCancelled) || IsTerminalStatus(StatusTodo) {
		t.Fatal("unexpected terminal status classification")
	}
	if !IsInProgressStatus(StatusEdit) || !IsInProgressStatus(StatusReview) || IsInProgressStatus(StatusTodo) || IsInProgressStatus(StatusDone) {
		t.Fatal("unexpected in-progress status classification")
	}
}

