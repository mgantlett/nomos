package task

import "testing"

func TestExtractTaskCardSections(t *testing.T) {
	cardBody, err := RenderTaskCardBody(TaskCardInput{
		Scope:              "Update substrate/cli/cmd_task_commit.go to hydrate commit sections from task cards.",
		AcceptanceCriteria: []string{"Strip duplicate task keys", "Hydrate rationale from scope"},
		Verification:       "go test ./cli/...",
	})
	if err != nil {
		t.Fatalf("RenderTaskCardBody failed: %v", err)
	}

	parsed := ExtractTaskCardSections(cardBody)
	if parsed.Scope != "Update substrate/cli/cmd_task_commit.go to hydrate commit sections from task cards." {
		t.Errorf("unexpected Scope: %q", parsed.Scope)
	}
	if len(parsed.AcceptanceCriteria) != 2 || parsed.AcceptanceCriteria[0] != "Strip duplicate task keys" {
		t.Errorf("unexpected AcceptanceCriteria: %#v", parsed.AcceptanceCriteria)
	}
	if parsed.Verification != "- go test ./cli/..." {
		t.Errorf("unexpected Verification: %q", parsed.Verification)
	}

	empty := ExtractTaskCardSections("Atomic 1-turn chore worktree")
	if empty.Scope != "" || len(empty.AcceptanceCriteria) != 0 || empty.Verification != "" {
		t.Errorf("expected empty sections for plain chore text, got: %+v", empty)
	}
}
