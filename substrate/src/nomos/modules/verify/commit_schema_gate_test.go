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
			name: "valid full dual-layer",
			msg: `[SUB-101] feat(core): implement feature

**Architectural Rationale:**
Rationale details

**Blast Radius & Contracts:**
Blast radius details

**Resolution Mechanics:**
Mechanics details`,
			wantPassed: true,
		},
		{
			name: "missing prefix",
			msg: `feat(core): implement feature

**Architectural Rationale:**
Rationale details`,
			wantPassed: false,
		},
		{
			name: "missing conventional commit type",
			msg: `[SUB-101] Implement feature without conventional prefix

**Architectural Rationale:**
Rationale details`,
			wantPassed: false,
		},
		{
			name: "duplicate trailing bracketed key",
			msg: `[SUB-101] docs(readme): update benchmark summary [SUB-101]

**Architectural Rationale:**
Rationale details`,
			wantPassed: false,
		},
		{
			name: "duplicate trailing parenthesized key",
			msg: `[SUB-101] fix(cockpit): move header to server route (SUB-101)

**Architectural Rationale:**
Rationale details`,
			wantPassed: false,
		},
		{
			name: "canned boilerplate shim rejected",
			msg: `[SUB-101] feat(core): implement feature

**Architectural Rationale:**
satisfy SUB-101 deterministic execution requirements

**Blast Radius & Contracts:**
scoped strictly to SUB-101 implementation files

**Resolution Mechanics:**
implemented and verified via deterministic quality gates for SUB-101`,
			wantPassed: false,
		},
		{
			name: "empty section body rejected",
			msg: `[SUB-101] feat(core): implement feature

**Architectural Rationale:**

**Blast Radius & Contracts:**
Touches verify/commit_schema_gate.go`,
			wantPassed: false,
		},
		{
			name:       "missing headings",
			msg:        `[SUB-101] feat(core): implement feature without headings`,
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
	_ = workspace.SafeGitCommand(repoDir, "add", "a.txt").Run()
	_ = workspace.SafeGitCommand(repoDir, "commit", "-m", "rogue intermediate commit without schema").Run()

	f2 := filepath.Join(repoDir, "b.txt")
	if err := os.WriteFile(f2, []byte("second\n"), 0644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	_ = workspace.SafeGitCommand(repoDir, "add", "b.txt").Run()
	validHead := "[SUB-77] feat(core): add second file\n\n**Architectural Rationale:**\nAdd deterministic state file.\n\n**Blast Radius & Contracts:**\n- A b.txt\n\n**Resolution Mechanics:**\n- Wrote b.txt with clean content.\n"
	_ = workspace.SafeGitCommand(repoDir, "commit", "-m", validHead).Run()

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
