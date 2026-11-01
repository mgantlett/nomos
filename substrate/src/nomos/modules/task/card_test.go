package task

import (
	"strings"
	"testing"
)

func TestRenderAndValidateTaskCardBody_Vectors(t *testing.T) {
	// TV-SCH-01 & TV-SCH-05: Structured Flag Synthesis + Domain Token Tolerance (StatusTodo, <task-key>)
	rendered, err := RenderTaskCardBody(TaskCardInput{
		Scope: "Update substrate/cli/cmd_task.go for <task-key> transitions in StatusTodo.",
		AcceptanceCriteria: []string{
			"Add --scope, --ac, and --verify flags",
			"- [ ] Preserve TODO status compatibility across Living Ledger",
		},
		Verification: "go test ./substrate/cli/... -run TestTaskCmd",
	})
	if err != nil {
		t.Fatalf("TV-SCH-01/05 failed rendering valid card: %v", err)
	}
	if strings.Contains(rendered, "- [ ] - [ ]") {
		t.Errorf("unexpected duplicate checkbox prefix in:\n%s", rendered)
	}
	if err := ValidateTaskCardBody(rendered); err != nil {
		t.Fatalf("ValidateTaskCardBody rejected rendered card: %v", err)
	}

	burden, err := DeriveTaskBurden(0, rendered)
	if err != nil || burden < 2 || burden > 8 {
		t.Fatalf("expected auto-derived burden in [2,8], got %d (err=%v)", burden, err)
	}
}

func TestValidateTaskCardBody_FailClosedVectors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "TV-SCH-03 plain paragraph",
			body: "Implement INT-863 in substrate/cli/: (1) Add subcommands; (2) Enforce guards.",
		},
		{
			name: "empty body",
			body: "   ",
		},
		{
			name: "TV-SCH-04 empty checkbox",
			body: "## Scope & Context\nValid target scope description.\n\n## Acceptance Criteria\n- [ ] \n\n## Verification\n- nomos verify\n",
		},
		{
			name: "missing verification section",
			body: "## Scope & Context\nValid target scope description.\n\n## Acceptance Criteria\n- [ ] Ship feature\n",
		},
		{
			name: "unexpanded escaped newlines",
			body: `## Scope & Context\nValid scope text\n\n## Acceptance Criteria\n- [ ] Item one\n\n## Verification\n- go test ./...`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateTaskCardBody(tc.body); err == nil {
				t.Fatalf("expected fail-closed error for %s, got nil", tc.name)
			}
		})
	}
}

func TestDeriveTaskBurden_Bounds(t *testing.T) {
	if b, err := DeriveTaskBurden(5, "body"); err != nil || b != 5 {
		t.Fatalf("expected explicit burden 5, got %d (%v)", b, err)
	}
	if _, err := DeriveTaskBurden(11, "body"); err == nil {
		t.Fatal("TV-SCH-07: expected burden 11 to fail closed")
	}
	if _, err := DeriveTaskBurden(-1, "body"); err == nil {
		t.Fatal("TV-SCH-07: expected burden -1 to fail closed")
	}
}

func TestTaskCardRemedy(t *testing.T) {
	rem := TaskCardRemedy("create", "Ship Hook CLI", "substrate")
	if !strings.Contains(rem, "--scope") || !strings.Contains(rem, "nomos task chore") {
		t.Fatalf("unexpected remedy text:\n%s", rem)
	}
}

