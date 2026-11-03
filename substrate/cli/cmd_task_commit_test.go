package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func TestFormatCommitMessage_ExplicitFlags(t *testing.T) {
	opts := &commitCmdOptions{
		summary:     "feat(verify): activate git gates",
		rationale:   "enforce Law 1 and Law 3",
		blastRadius: "verify module",
		mechanics:   "registered gates",
	}

	msg := formatCommitMessage("SUB-1212", opts)

	if !strings.HasPrefix(msg, "[SUB-1212] feat(verify): activate git gates\n") {
		t.Errorf("unexpected subject line: %s", msg)
	}
	if !strings.Contains(msg, "**Architectural Rationale:**\nenforce Law 1 and Law 3") {
		t.Errorf("missing architectural rationale in: %s", msg)
	}
	if !strings.Contains(msg, "**Blast Radius & Contracts:**\nverify module") {
		t.Errorf("missing blast radius in: %s", msg)
	}
	if !strings.Contains(msg, "**Resolution Mechanics:**\nregistered gates") {
		t.Errorf("missing resolution mechanics in: %s", msg)
	}
}

func TestNormalizeCommitSubject_DeduplicationAndInference(t *testing.T) {
	cases := []struct {
		name        string
		taskKey     string
		raw         string
		rec         *task.TaskRecord
		stagedFiles []string
		want        string
	}{
		{
			name:        "strips duplicate leading and trailing bracketed key",
			taskKey:     "SUB-1559",
			raw:         "[SUB-1559] docs(readme): keep benchmark summary concise [SUB-1559]",
			stagedFiles: []string{"README.md"},
			want:        "docs(readme): keep benchmark summary concise",
		},
		{
			name:        "strips trailing parenthesized key",
			taskKey:     "SUB-1576",
			raw:         "fix(cockpit): move no-store header to server load (SUB-1576)",
			stagedFiles: []string{"substrate/cockpit-ui/src/routes/+page.server.ts"},
			want:        "fix(cockpit): move no-store header to server load",
		},
		{
			name:        "maps spec prefix to canonical docs",
			taskKey:     "INT-864",
			raw:         "spec(intent): codify 3-section task card contract",
			stagedFiles: []string{"intent/architecture/specs/core/card_spec.md"},
			want:        "docs(intent): codify 3-section task card contract",
		},
		{
			name:        "maps RELEASE prefix to chore(release)",
			taskKey:     "SUB-1552",
			raw:         "RELEASE: v4.1.0 - Native Hook CLI & Wazero AST",
			stagedFiles: []string{"substrate/cli/version.go"},
			want:        "chore(release): v4.1.0 - Native Hook CLI & Wazero AST",
		},
		{
			name:        "infers feat and module scope from plain title",
			taskKey:     "SUB-10",
			raw:         "Add ledger balance verification",
			stagedFiles: []string{"substrate/src/nomos/modules/verify/gates_standard.go"},
			want:        "feat(verify): Add ledger balance verification",
		},
		{
			name:        "falls back to TaskRecord Title when raw summary empty",
			taskKey:     "INT-12",
			raw:         "",
			rec:         &task.TaskRecord{Title: "Codify commit hydration contract"},
			stagedFiles: []string{"intent/architecture/specs/core/commit_rules_spec.md"},
			want:        "docs(specs): Codify commit hydration contract",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCommitSubject(tc.taskKey, tc.raw, tc.rec, tc.stagedFiles)
			if got != tc.want {
				t.Errorf("normalizeCommitSubject() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHydrateCommitMessage_FromCardWalkthroughAndGitDiff(t *testing.T) {
	repoDir := workspace.InitHermeticGitRepo(t)
	targetFile := filepath.Join(repoDir, "substrate", "cli", "widget.go")
	if err := os.MkdirAll(filepath.Dir(targetFile), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("package cli\n\nfunc Widget() string { return \"ok\" }\n"), 0644); err != nil {
		t.Fatalf("write widget.go: %v", err)
	}
	if out, err := workspace.SafeGitCommand(repoDir, "add", "-A").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}

	cardDesc, err := task.RenderTaskCardBody(task.TaskCardInput{
		Scope:              "Wire deterministic widget helper into substrate/cli/widget.go without placeholder shims.",
		AcceptanceCriteria: []string{"Implement Widget helper", "Verify zero boilerplate commit shims"},
		Verification:       "go test ./cli/...",
	})
	if err != nil {
		t.Fatalf("RenderTaskCardBody: %v", err)
	}

	walkPath := filepath.Join(repoDir, ".tmp", "walkthrough-SUB-88.md")
	if err := os.MkdirAll(filepath.Dir(walkPath), 0755); err != nil {
		t.Fatalf("mkdir .tmp: %v", err)
	}
	walkContent := "# Walkthrough SUB-88\n\n## Summary\n- Added `Widget()` in `substrate/cli/widget.go` returning deterministic status.\n- Verified CLI commit hydration from walkthrough and task card.\n"
	if err := os.WriteFile(walkPath, []byte(walkContent), 0644); err != nil {
		t.Fatalf("write walkthrough: %v", err)
	}

	opts := &commitCmdOptions{
		dir:             repoDir,
		walkthroughFile: walkPath,
		taskRecord: &task.TaskRecord{
			Key:         "SUB-88",
			Title:       "Implement CLI widget helper",
			Description: cardDesc,
		},
	}

	msg := formatCommitMessage("SUB-88", opts)
	if !strings.HasPrefix(msg, "[SUB-88] feat(cli): Implement CLI widget helper\n") {
		t.Errorf("unexpected hydrated subject:\n%s", msg)
	}
	if !strings.Contains(msg, "Wire deterministic widget helper into substrate/cli/widget.go") {
		t.Errorf("expected Architectural Rationale from task card Scope, got:\n%s", msg)
	}
	if !strings.Contains(msg, "- A substrate/cli/widget.go") {
		t.Errorf("expected Blast Radius to list staged file status, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Added `Widget()` in `substrate/cli/widget.go`") {
		t.Errorf("expected Resolution Mechanics from walkthrough bullets, got:\n%s", msg)
	}
	for _, banned := range []string{"deterministic execution requirements", "implementation files", "verified via deterministic quality gates"} {
		if strings.Contains(msg, banned) {
			t.Errorf("hydrated commit leaked banned canned shim phrase %q:\n%s", banned, msg)
		}
	}
}

func TestResolveTaskKeyForCommit_Explicit(t *testing.T) {
	opts := &commitCmdOptions{}
	key, err := resolveTaskKeyForCommit([]string{"sub-1212"}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "SUB-1212" {
		t.Errorf("expected SUB-1212, got %s", key)
	}
}
