package commitmsg

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func TestNormalizeCommitSubject_AndHydration(t *testing.T) {
	staged := []string{"substrate/src/nomos/modules/verify/gates_standard.go", "substrate/cli/root.go"}
	rec := &task.TaskRecord{
		Key:         "SUB-101",
		Title:       "Enforce package fan-out ceiling",
		Description: "## Scope & Context\nBound horizontal package fan-out across substrate directories.\n\n## Acceptance Criteria\n- Enforce max_package_files ceiling\n- Decompose oversized packages\n\n## Verification\n`nomos verify`\n",
		TwinKey:     "INT-101",
	}

	subj := NormalizeCommitSubject("SUB-101", "[SUB-101] Enforce horizontal package fan-out ceiling across CLI and verify", rec, staged)
	if !strings.HasPrefix(subj, "refactor(") {
		t.Fatalf("expected inferred refactor(<scope>): prefix, got %q", subj)
	}
	full := "[SUB-101] " + subj
	if utf8.RuneCountInString(full) > TargetSubjectLineRunes {
		t.Fatalf("expected subject <= %d runes, got %d (%q)", TargetSubjectLineRunes, utf8.RuneCountInString(full), full)
	}

	ctx := BuildCommitHydrationContext("SUB-101", "", &CommitCmdOptions{
		Summary:    "feat(verify): enforce max_package_files gate",
		TaskRecord: rec,
	})
	ctx.stagedFiles = staged
	msg := HydrateCommitMessage(ctx, &CommitCmdOptions{
		Summary:    "feat(verify): enforce max_package_files gate",
		TaskRecord: rec,
	})
	for _, want := range []string{
		"[SUB-101] feat(verify): enforce max_package_files gate",
		"Bound horizontal package fan-out across substrate directories.",
		"Task: SUB-101",
		"Twin: INT-101",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected hydrated commit message to contain %q, got:\n%s", want, msg)
		}
	}
}
