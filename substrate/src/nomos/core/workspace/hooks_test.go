package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitMsgHookScript_EnforcesSchemaAndAntiShim(t *testing.T) {
	t.Setenv("NOMOS_INIT_IN_PROGRESS", "")
	t.Setenv("NOMOS_COMMIT_IN_PROGRESS", "")
	repoDir := InitHermeticGitRepo(t)
	if err := InstallHooks(repoDir); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	hookPath := filepath.Join(repoDir, ".git", "hooks", "commit-msg")

	cases := []struct {
		name    string
		msg     string
		wantErr bool
	}{
		{
			name: "valid Dual-Layer commit passes",
			msg: `[SUB-42] feat(cli): hydrate commit sections from task card

**Architectural Rationale:**
Eliminate placeholder commit bodies in Git history.

**Blast Radius & Contracts:**
- M substrate/cli/cmd_task_commit.go

**Resolution Mechanics:**
- Hydrated sections from 3-section Living Ledger card and staged diff.`,
			wantErr: false,
		},
		{
			name: "missing conventional commit type blocked",
			msg: `[SUB-42] Hydrate commit sections without type

**Architectural Rationale:**
Details here.`,
			wantErr: true,
		},
		{
			name: "duplicate trailing task key blocked",
			msg: `[SUB-42] docs(readme): clarify install instructions [SUB-42]

**Architectural Rationale:**
Details here.`,
			wantErr: true,
		},
		{
			name: "canned boilerplate shim blocked",
			msg: `[SUB-42] feat(cli): update file

**Architectural Rationale:**
satisfy SUB-42 deterministic execution requirements

**Blast Radius & Contracts:**
scoped strictly to SUB-42 implementation files

**Resolution Mechanics:**
implemented and verified via deterministic quality gates for SUB-42`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
			if err := os.WriteFile(msgFile, []byte(tc.msg+"\n"), 0644); err != nil {
				t.Fatalf("write msgFile: %v", err)
			}
			cmd := exec.Command("bash", hookPath, msgFile)
			cmd.Dir = repoDir
			cmd.Env = SanitizedGitEnv()
			out, err := cmd.CombinedOutput()
			if (err != nil) != tc.wantErr {
				t.Errorf("commit-msg hook err=%v (wantErr=%v), output:\n%s", err, tc.wantErr, string(out))
			}
		})
	}
}

func TestPreCommitHookScript_BlocksRawWorktreeGitCommitWithoutEnvToken(t *testing.T) {
	t.Setenv("NOMOS_INIT_IN_PROGRESS", "")
	t.Setenv("NOMOS_COMMIT_IN_PROGRESS", "")
	repoDir := InitHermeticGitRepo(t)
	if err := InstallHooks(repoDir); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	wtPath, err := mgr.CreateWorktree("SUB-99")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}
	defer func() {
		_ = mgr.RemoveWorktree("SUB-99")
	}()

	if err := os.WriteFile(filepath.Join(wtPath, "note.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}
	_ = SafeGitCommand(wtPath, "add", "note.txt").Run()

	rawCommit := SafeGitCommand(wtPath, "commit", "-m", "[SUB-99] feat: raw commit\n\n**Architectural Rationale:**\nRaw commit attempt.")
	out, cErr := rawCommit.CombinedOutput()
	if cErr == nil {
		t.Fatalf("expected raw git commit without NOMOS_COMMIT_IN_PROGRESS=1 to be blocked")
	}
	if !strings.Contains(string(out), "Direct 'git commit' is forbidden") {
		t.Errorf("expected canonical CLI interception message, got:\n%s", string(out))
	}
}
