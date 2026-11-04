package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestCommitSchemaGate_Name(t *testing.T) {
	gate := &CommitSchemaGate{}
	if gate.Name() != "Commit Schema Invariant Gate" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}
}

func TestCheckCommitCompliance(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		wantPassed bool
	}{
		{
			name: "valid golden commit with Task and Twin trailers",
			msg: `feat(verify): enforce RFC 822 Task trailer on branch commits

Replace noisy Blast Radius file-list dumps with 72-column rationale prose
and machine-parsable Git trailers so git log --oneline stays high-signal.

- Reject raw staged diff dumps and legacy bold section headers.
- Validate canonical INT-<N> and SUB-<N> keys in Task and Twin trailers.

Task: SUB-1700
Twin: INT-903`,
			wantPassed: true,
		},
		{
			name: "valid single-paragraph golden commit",
			msg: `fix(cli): preserve Go directive version when scaffolding task worktrees

Read the workspace go.mod version dynamically instead of emitting a static
toolchain string that breaks older downstream repositories.

Task: SUB-1698`,
			wantPassed: true,
		},
		{
			name: "legacy leading bracketed TaskID prefix rejected",
			msg: `[SUB-101] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "trailing bracketed task key rejected",
			msg: `docs(readme): update benchmark summary [SUB-101]

Clarify cold-start verification latency figures in benchmark table.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "trailing parenthesized task key rejected",
			msg: `fix(cli): normalize worktree path resolution (SUB-101)

Resolve nested subdirectory invocations back to the worktree root.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "missing conventional commit type rejected",
			msg: `Implement feature without conventional prefix

Provide deterministic state validation across worktree boundaries.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "subject exceeding 100 runes rejected",
			msg: `feat(verify): implement an excessively verbose subject line that stretches well past one hundred runes in length for no good reason

Provide deterministic state validation across worktree boundaries.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "missing Task trailer rejected",
			msg: `feat(core): implement feature

Provide deterministic state validation across worktree boundaries.`,
			wantPassed: false,
		},
		{
			name: "non-canonical ARC Task trailer rejected",
			msg: `feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

Task: ARC-99`,
			wantPassed: false,
		},
		{
			name: "missing rationale prose above Task trailer rejected",
			msg: `feat(core): implement feature

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "legacy Dual-Layer bold headers rejected",
			msg: `feat(core): implement feature

**Architectural Rationale:**
Provide deterministic state validation across worktree boundaries.

**Blast Radius & Contracts:**
- M substrate/cli/cmd_task_commit.go

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "raw git diff name-status file dump rejected",
			msg: `feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

- M substrate/cli/cmd_task_commit.go
- A substrate/cli/commit_hydration.go

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "truncation filler bullet rejected",
			msg: `feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

- Updated CLI flags
- ... (+4 additional staged file(s))

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "canned boilerplate shim rejected",
			msg: `feat(core): implement feature

satisfy SUB-101 deterministic execution requirements

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name:       "merge commit exempt",
			msg:        `Merge branch 'feature/SUB-101' into main`,
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := checkCommitCompliance("Commit Schema Invariant Gate", tt.msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Passed != tt.wantPassed {
				t.Errorf("checkCommitCompliance() passed = %v, want %v (msg: %s)", res.Passed, tt.wantPassed, res.Message)
			}
		})
	}
}

func TestCommitSchemaGate_DetectsIntermediateRogueBranchCommit(t *testing.T) {
	repoDir := workspace.InitHermeticGitRepo(t)
	if out, err := workspace.SafeGitCommand(repoDir, "checkout", "-b", "feature/SUB-77").CombinedOutput(); err != nil {
		t.Fatalf("checkout feature/SUB-77: %v (%s)", err, string(out))
	}

	f1 := filepath.Join(repoDir, "a.txt")
	if err := os.WriteFile(f1, []byte("first\n"), 0644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := workspace.SafeGitCommand(repoDir, "add", "a.txt").Run(); err != nil {
		t.Fatalf("git add a.txt: %v", err)
	}
	if err := workspace.SafeGitCommand(repoDir, "commit", "-m", "rogue intermediate commit without schema").Run(); err != nil {
		t.Fatalf("git commit a.txt: %v", err)
	}

	f2 := filepath.Join(repoDir, "b.txt")
	if err := os.WriteFile(f2, []byte("second\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	if err := workspace.SafeGitCommand(repoDir, "add", "b.txt").Run(); err != nil {
		t.Fatalf("git add b.txt: %v", err)
	}
	validHead := "feat(core): add deterministic state file\n\nPersist state snapshot atomically across worktree verification runs.\n\nTask: SUB-77\n"
	if err := workspace.SafeGitCommand(repoDir, "commit", "-m", validHead).Run(); err != nil {
		t.Fatalf("git commit b.txt: %v", err)
	}

	gate := &CommitSchemaGate{}
	res, err := gate.Evaluate(&VerifyContext{RepoRoot: repoDir, WorktreePath: repoDir, TaskID: "SUB-77"})
	if err != nil {
		t.Fatalf("Evaluate error: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected intermediate rogue commit on branch to fail closed even when HEAD is valid")
	}
	if !strings.Contains(res.Message, "commit ") {
		t.Errorf("expected offending commit SHA in failure message, got: %s", res.Message)
	}
}
