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

func TestCommitMsgHookScript_ZeroFHSSedFreePureNixOSCompatibility(t *testing.T) {
	if strings.Contains(CommitMsgHookScript, "sed ") || strings.Contains(CommitMsgHookScript, "awk ") {
		t.Fatalf("CommitMsgHookScript must not depend on sed or awk in Zero-FHS Pure NixOS containers")
	}
	repoDir := InitHermeticGitRepo(t)
	if err := InstallHooks(repoDir); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	minBin := t.TempDir()
	for _, tool := range []string{"bash", "git", "head", "tail", "tr", "grep"} {
		src, err := exec.LookPath(tool)
		if err != nil {
			t.Fatalf("LookPath(%s): %v", tool, err)
		}
		data, rErr := os.ReadFile(src)
		if rErr != nil {
			t.Fatalf("read %s: %v", src, rErr)
		}
		if wErr := os.WriteFile(filepath.Join(minBin, tool), data, 0755); wErr != nil {
			t.Fatalf("stage %s: %v", tool, wErr)
		}
	}
	msgFile := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
	valid := "[SUB-1847] fix(workspace): remove sed dependency from commit-msg\n\nEliminate external sed subprocess so Pure NixOS containers validate Golden Commits natively.\n\n___\n\nTask: SUB-1847\n"
	if err := os.WriteFile(msgFile, []byte(valid), 0644); err != nil {
		t.Fatalf("write msgFile: %v", err)
	}
	cmd := exec.Command(filepath.Join(minBin, "bash"), filepath.Join(repoDir, ".git", "hooks", "commit-msg"), msgFile)
	cmd.Dir = repoDir
	cmd.Env = []string{"PATH=" + minBin, "HOME=" + t.TempDir()}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit-msg failed in sed/awk-free PATH: %v\n%s", err, string(out))
	}
}

func TestEnsureOriginNotesRefspecs_AdditivePushAndFetch(t *testing.T) {
	remoteBare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := SafeGitCommand(t.TempDir(), "init", "--bare", remoteBare).CombinedOutput(); err != nil {
		t.Fatalf("bare init failed: %v\n%s", err, string(out))
	}
	repoDir := InitHermeticGitRepo(t)
	if out, err := SafeGitCommand(repoDir, "remote", "add", "origin", remoteBare).CombinedOutput(); err != nil {
		t.Fatalf("remote add failed: %v\n%s", err, string(out))
	}
	if err := InstallHooks(repoDir); err != nil {
		t.Fatalf("InstallHooks failed: %v", err)
	}
	// Repeat call to verify idempotency.
	if _, err := EnsureHooksInstalled(repoDir); err != nil {
		t.Fatalf("EnsureHooksInstalled failed: %v", err)
	}
	pushOut, err := SafeGitCommand(repoDir, "config", "--get-all", "remote.origin.push").Output()
	if err != nil {
		t.Fatalf("expected remote.origin.push entries: %v", err)
	}
	pushLines := strings.Fields(string(pushOut))
	if len(pushLines) != 2 || pushLines[0] != "HEAD" || pushLines[1] != "refs/notes/nomos/*:refs/notes/nomos/*" {
		t.Fatalf("unexpected remote.origin.push lines: %v", pushLines)
	}
	fetchOut, fErr := SafeGitCommand(repoDir, "config", "--get-all", "remote.origin.fetch").Output()
	if fErr != nil || !strings.Contains(string(fetchOut), "+refs/notes/nomos/*:refs/notes/nomos/*") {
		t.Fatalf("expected notes fetch refspec in remote.origin.fetch, got %q (err=%v)", string(fetchOut), fErr)
	}
}
