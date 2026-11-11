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

// @verifies TC-CMT-005
func TestCheckCommitCompliance(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		wantPassed bool
	}{
		{
			name: "valid golden commit with [KEY] prefix, ___ divider, and Task/Twin trailers",
			msg: `[SUB-1703] feat(verify): enforce [KEY] prefix and flowing section spacing

Replace ragged 72-column hard line breaks with flowing CommonMark paragraphs so IDE hover cards render crisp inter-section whitespace while preserving instant 2-plane visibility in Source Control graphs.

- Enforce subject-to-trailer Task key parity across unmerged branch commits.
- Separate RFC 822 Task and Twin trailers beneath a git-interpret-trailers-safe ___ divider.

___

Task: SUB-1703
Twin: INT-904`,
			wantPassed: true,
		},
		{
			name: "valid single-paragraph golden commit without bullets",
			msg: `[SUB-1698] fix(cli): preserve Go directive version when scaffolding task worktrees

Read the workspace go.mod version dynamically instead of emitting a static toolchain string that breaks older downstream repositories.

___

Task: SUB-1698`,
			wantPassed: true,
		},
		{
			name: "valid perf golden commit for throughput and cache alignment",
			msg: `[SUB-2177] perf(verify): memoize workspace config and diff state across gates

Cache parsed ecosystem configuration and modified worktree paths on VerifyContext to avoid redundant subprocess forks.

___

Task: SUB-2177`,
			wantPassed: true,
		},
		{
			name: "euphemistic self-healing buzzword in commit subject rejected",
			msg: `[SUB-2177] refactor(hooks): add self-healing loop guard recovery

Anchor loop tracking state to the canonical worktree root directory.

___

Task: SUB-2177`,
			wantPassed: false,
		},
		{
			name: "missing leading [KEY] prefix rejected",
			msg: `feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "non-canonical [ARC-9] prefix rejected",
			msg: `[ARC-9] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "subject key mismatching Task trailer rejected",
			msg: `[SUB-101] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

___

Task: SUB-202`,
			wantPassed: false,
		},
		{
			name: "duplicate trailing bracketed key rejected",
			msg: `[SUB-101] docs(readme): update benchmark summary [SUB-101]

Clarify cold-start verification latency figures in benchmark table.

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "mailinfo --- scissors divider rejected in favor of ___",
			msg: `[SUB-101] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

---

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "missing conventional commit type rejected",
			msg: `[SUB-101] Implement feature without conventional prefix

Provide deterministic state validation across worktree boundaries.

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "missing Task trailer rejected",
			msg: `[SUB-101] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.`,
			wantPassed: false,
		},
		{
			name: "missing rationale prose above divider and Task trailer rejected",
			msg: `[SUB-101] feat(core): implement feature

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "legacy Dual-Layer bold headers rejected",
			msg: `[SUB-101] feat(core): implement feature

**Architectural Rationale:**
Provide deterministic state validation across worktree boundaries.

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "raw git diff name-status file dump rejected",
			msg: `[SUB-101] feat(core): implement feature

Provide deterministic state validation across worktree boundaries.

- M substrate/cli/cmd_task_commit.go

___

Task: SUB-101`,
			wantPassed: false,
		},
		{
			name: "canned boilerplate shim rejected",
			msg: `[SUB-101] feat(core): implement feature

satisfy SUB-101 deterministic execution requirements

___

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
	validHead := "[SUB-77] feat(core): add deterministic state file\n\nPersist state snapshot atomically across worktree verification runs.\n\n___\n\nTask: SUB-77\n"
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
