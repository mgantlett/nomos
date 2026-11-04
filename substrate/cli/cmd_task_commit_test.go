package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func TestFormatCommitMessage_ExplicitFlagsAndTwinTrailer(t *testing.T) {
	opts := &commitCmdOptions{
		summary:    "feat(verify): activate git trailer gate",
		rationale:  "Enforce Law 1 and Law 6 with standard RFC 822 Task trailers instead of file-list noise.",
		mechanics:  "- Parse trailing Task and Twin Git trailers.\n- Reject raw git diff file dumps.",
		taskRecord: &task.TaskRecord{Key: "SUB-1212", TwinKey: "INT-734"},
	}

	msg := formatCommitMessage("SUB-1212", opts)

	if !strings.HasPrefix(msg, "feat(verify): activate git trailer gate\n\n") {
		t.Errorf("unexpected subject line: %s", msg)
	}
	if strings.Contains(msg, "[SUB-1212]") || strings.Contains(msg, "**Blast Radius") || strings.Contains(msg, "**Architectural Rationale") {
		t.Errorf("expected zero legacy prefixes or Dual-Layer bold headers, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Enforce Law 1 and Law 6 with standard RFC 822 Task trailers") {
		t.Errorf("missing rationale prose in:\n%s", msg)
	}
	if !strings.Contains(msg, "- Parse trailing Task and Twin Git trailers.") {
		t.Errorf("missing mechanics bullets in:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "\n\nTask: SUB-1212\nTwin: INT-734\n") {
		t.Errorf("expected RFC 822 Task and Twin trailers at bottom, got:\n%s", msg)
	}
}

func TestNormalizeCommitSubject_DeduplicationCasingAnd72RuneBudget(t *testing.T) {
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
			name:        "maps RELEASE prefix to chore(release) preserving version token",
			taskKey:     "SUB-1552",
			raw:         "RELEASE: v4.1.0 - Native Hook CLI & Wazero AST",
			stagedFiles: []string{"substrate/cli/version.go"},
			want:        "chore(release): v4.1.0 - Native Hook CLI & Wazero AST",
		},
		{
			name:        "infers feat, module scope, and lowercase imperative start",
			taskKey:     "SUB-10",
			raw:         "Add ledger balance verification",
			stagedFiles: []string{"substrate/src/nomos/modules/verify/gates_standard.go"},
			want:        "feat(verify): add ledger balance verification",
		},
		{
			name:        "preserves leading acronym tokens like AST and CapBAC",
			taskKey:     "SUB-11",
			raw:         "CapBAC flow taint enforcement across MCP tools",
			stagedFiles: []string{"substrate/src/nomos/core/capbac/flow.go"},
			want:        "feat(capbac): CapBAC flow taint enforcement across MCP tools",
		},
		{
			name:        "falls back to TaskRecord Title with lowercase verb",
			taskKey:     "INT-12",
			raw:         "",
			rec:         &task.TaskRecord{Title: "Codify commit hydration contract"},
			stagedFiles: []string{"intent/architecture/specs/core/commit_rules_spec.md"},
			want:        "docs(specs): codify commit hydration contract",
		},
		{
			name:        "truncates verbose subject cleanly within 72-rune target",
			taskKey:     "SUB-99",
			raw:         "Implement comprehensive multi-stage cryptographic attestation across every nested workspace boundary",
			stagedFiles: []string{"substrate/src/nomos/modules/verify/gates_standard.go"},
			want:        "feat(verify): implement comprehensive multi-stage cryptographic...",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCommitSubject(tc.taskKey, tc.raw, tc.rec, tc.stagedFiles)
			if got != tc.want {
				t.Errorf("normalizeCommitSubject() = %q, want %q", got, tc.want)
			}
			if n := utf8.RuneCountInString(got); n > targetSubjectLineRunes {
				t.Errorf("subject exceeded %d runes (%d): %q", targetSubjectLineRunes, n, got)
			}
		})
	}
}

func TestHydrateCommitMessage_DeduplicatesWalkthroughAndOmitsFileDump(t *testing.T) {
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
	walkContent := "# Walkthrough SUB-88\n\n## Summary\n- Wire deterministic widget helper into substrate/cli/widget.go without placeholder shims.\n- Added `Widget()` in `substrate/cli/widget.go` returning deterministic status.\n- Verified CLI commit hydration from walkthrough and task card.\n"
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
			TwinKey:     "INT-44",
		},
	}

	msg := formatCommitMessage("SUB-88", opts)
	if !strings.HasPrefix(msg, "feat(cli): implement CLI widget helper\n\n") {
		t.Errorf("unexpected hydrated subject:\n%s", msg)
	}
	if strings.Contains(msg, "- A substrate/cli/widget.go") || strings.Contains(msg, "Blast Radius") {
		t.Errorf("hydrated commit must never dump staged file-status lists:\n%s", msg)
	}
	if strings.Count(collapseSingleLine(msg), "without placeholder shims") != 1 {
		t.Errorf("expected walkthrough bullet identical to rationale to be deduplicated once, got:\n%s", msg)
	}
	if !strings.Contains(msg, "- Added `Widget()` in `substrate/cli/widget.go`") {
		t.Errorf("expected distinct walkthrough mechanics bullet in:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "\n\nTask: SUB-88\nTwin: INT-44\n") {
		t.Errorf("expected Task and Twin trailers at end, got:\n%s", msg)
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
