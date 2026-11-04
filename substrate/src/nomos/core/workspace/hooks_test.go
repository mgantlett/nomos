package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitMsgHookScript_EnforcesGoldenCommitAndTrailers(t *testing.T) {
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
			name: "valid Golden Commit with [KEY] prefix, ___ divider, and Task/Twin trailers passes",
			msg: `[SUB-1703] feat(cli): hydrate Golden Commits with section spacing and Task trailers

Eliminate redundant Blast Radius file-list dumps and ragged 72-column hard breaks while preserving instant 2-plane visibility in Source Control graphs.

- Hydrate flowing CommonMark rationale prose from Living Ledger Scope & Context.
- Separate Task and Twin trailers beneath a git-interpret-trailers-safe ___ divider.

___

Task: SUB-1703
Twin: INT-904`,
			wantErr: false,
		},
		{
			name: "missing leading [KEY] prefix blocked",
			msg: `feat(cli): hydrate commit sections from task card

Eliminate placeholder commit bodies in Git history.

___

Task: SUB-42`,
			wantErr: true,
		},
		{
			name: "mismatched subject key vs Task trailer blocked",
			msg: `[SUB-42] feat(cli): hydrate commit sections from task card

Eliminate placeholder commit bodies in Git history.

___

Task: SUB-99`,
			wantErr: true,
		},
		{
			name: "standalone --- mailinfo scissors divider blocked",
			msg: `[SUB-42] feat(cli): hydrate commit sections from task card

Eliminate placeholder commit bodies in Git history.

---

Task: SUB-42`,
			wantErr: true,
		},
		{
			name: "duplicate trailing task key token blocked",
			msg: `[SUB-42] docs(readme): clarify install instructions [SUB-42]

Update quickstart CLI verification snippet in README.

___

Task: SUB-42`,
			wantErr: true,
		},
		{
			name: "legacy Blast Radius header and raw diff dump blocked",
			msg: `[SUB-42] feat(cli): hydrate commit sections from task card

**Blast Radius & Contracts:**
- M substrate/cli/cmd_task_commit.go

Task: SUB-42`,
			wantErr: true,
		},
		{
			name: "canned boilerplate shim blocked",
			msg: `[SUB-42] feat(cli): update file

satisfy SUB-42 deterministic execution requirements

___

Task: SUB-42`,
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
		if rErr := mgr.RemoveWorktree("SUB-99"); rErr != nil {
			t.Logf("RemoveWorktree cleanup: %v", rErr)
		}
	}()

	if err := os.WriteFile(filepath.Join(wtPath, "note.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write note.txt: %v", err)
	}
	if err := SafeGitCommand(wtPath, "add", "note.txt").Run(); err != nil {
		t.Fatalf("git add note.txt: %v", err)
	}

	rawCommit := SafeGitCommand(wtPath, "commit", "-m", "[SUB-99] feat(core): attempt raw commit\n\nAttempt direct git commit bypassing nomos CLI.\n\n___\n\nTask: SUB-99")
	out, cErr := rawCommit.CombinedOutput()
	if cErr == nil {
		t.Fatalf("expected raw git commit without NOMOS_COMMIT_IN_PROGRESS=1 to be blocked")
	}
	if !strings.Contains(string(out), "Direct 'git commit' is forbidden") {
		t.Errorf("expected canonical CLI interception message, got:\n%s", string(out))
	}
}
