package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookBrainPathScopeAndBypassCommandGuards(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	wsRoot := t.TempDir()

	// Source path containing '/brain/' inside a repository must still be blocked on trunk.
	repoBrainFile := filepath.Join(wsRoot, "pkg", "brain", "service.go")
	denyDec := EvaluatePreToolHook(&HookPayload{
		ToolName:   "Write",
		TargetFile: repoBrainFile,
		Cwd:        wsRoot,
	}, "")
	if denyDec.Decision != hookDecisionDeny {
		t.Fatalf("expected repo /pkg/brain/service.go on trunk to be denied, got %+v", denyDec)
	}

	// Genuine Antigravity brain artifact directory remains permitted.
	agyBrainFile := filepath.Join(t.TempDir(), "antigravity", "brain", "conv-1", "plan.md")
	allowDec := EvaluatePreToolHook(&HookPayload{
		ToolName:   "write_file",
		TargetFile: agyBrainFile,
		Cwd:        wsRoot,
	}, "")
	if allowDec.Decision != hookDecisionAllow {
		t.Fatalf("expected Antigravity brain artifact path to be permitted, got %+v", allowDec)
	}

	for _, forbiddenCmd := range []string{
		`git commit -m "bypass" --no-verify`,
		`git commit -n -m "bypass"`,
		`NOMOS_COMMIT_IN_PROGRESS=1 git commit -m "spoof"`,
		`NOMOS_INIT_IN_PROGRESS=1 git commit -m "spoof"`,
	} {
		dec := EvaluatePreToolHook(&HookPayload{
			ToolName:    "Bash",
			CommandLine: forbiddenCmd,
			Cwd:         wsRoot,
		}, "")
		if dec.Decision != hookDecisionDeny {
			t.Fatalf("expected %q to be denied by command guard, got %+v", forbiddenCmd, dec)
		}
	}
}

func TestHookVerifyState_RejectsEchoSpoofAndFailedVerifyKeepsStopBlocked(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	wtDir := filepath.Join(t.TempDir(), "worktrees", "repo-SUB-501")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}
	wtFile := filepath.Join(wtDir, "engine.go")
	if err := os.WriteFile(wtFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	recordPreToolSessionTrace(&HookPayload{
		ToolName:   "Write",
		TargetFile: wtFile,
		Cwd:        wtDir,
	}, "", false)

	// Read-only --impact or echo must NOT clear DirtyUnverifiedEdits.
	for _, nonVerify := range []string{"echo nomos verify", "nomos verify --impact"} {
		recordPreToolSessionTrace(&HookPayload{
			ToolName:    "Bash",
			CommandLine: nonVerify,
			Cwd:         wtDir,
		}, "", false)
		stopDec := EvaluateStopHook(&HookPayload{Cwd: wtDir}, "", false)
		if stopDec.Decision != hookDecisionBlock {
			t.Fatalf("expected %q not to clear Stop block, got %+v", nonVerify, stopDec)
		}
	}

	// Executing 'nomos verify' that FAILS in PostToolUse must re-mark DirtyUnverifiedEdits=true.
	recordPreToolSessionTrace(&HookPayload{
		ToolName:    "Bash",
		CommandLine: "nomos verify",
		Cwd:         wtDir,
	}, "", false)
	adv := evaluateWorktreeChurnAdvisory(&HookPayload{
		ToolName:         "Bash",
		CommandLine:      "nomos verify",
		Cwd:              wtDir,
		ToolResponseText: "❌ Verification Failed · 41/42 Gates Passed",
	}, "", "")
	if adv != "" {
		t.Fatalf("expected silent churn advisory on nomos verify, got %q", adv)
	}
	failedStop := EvaluateStopHook(&HookPayload{Cwd: wtDir}, "", false)
	if failedStop.Decision != hookDecisionBlock {
		t.Fatalf("expected failed nomos verify to keep Stop blocked, got %+v", failedStop)
	}

	// Subsequent clean 'nomos verify' clears unverified state and allows Stop.
	cleanAdv := evaluateWorktreeChurnAdvisory(&HookPayload{
		ToolName:         "Bash",
		CommandLine:      "nomos verify",
		Cwd:              wtDir,
		ToolResponseText: "✅ All Laws Enforced · 42/42 Gates Satisfied",
	}, "", "")
	if cleanAdv != "" {
		t.Fatalf("expected silent churn advisory on clean nomos verify, got %q", cleanAdv)
	}
	passedStop := EvaluateStopHook(&HookPayload{Cwd: wtDir}, "", false)
	if passedStop.Decision != hookDecisionAllow {
		t.Fatalf("expected clean nomos verify to allow Stop, got %+v", passedStop)
	}
}

func TestReceiptTraceParity_ZeroDiffChangesSectionFailsClosed(t *testing.T) {
	wtDir := filepath.Join(t.TempDir(), "worktrees", "repo-SUB-502")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}
	walkPath := filepath.Join(wtDir, "walkthrough.md")
	body := "# Walkthrough\n\n## Changes\n- `substrate/cli/phantom.go`: Claimed edit with zero worktree diff.\n"
	if err := os.WriteFile(walkPath, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	err := validateReceiptTraceParity(wtDir, walkPath, nil)
	if err == nil || !strings.Contains(err.Error(), "phantom.go") {
		t.Fatalf("expected 0-diff task claiming ## Changes bullet to fail closed, got %v", err)
	}
}

func TestPreToolHook_CredentialReadModeAndBlockedReadPaths(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	wsRoot := t.TempDir()

	// Default host_deny blocks ~/.aws and cat ~/.aws/credentials while permitting & tainting workspace .env.
	for _, p := range []*HookPayload{
		{ToolName: "Read", TargetFile: "~/.aws/credentials", Cwd: wsRoot},
		{ToolName: "Read", TargetFile: "/root/.ssh/id_ed25519", Cwd: wsRoot},
		{ToolName: "Bash", CommandLine: "cat ~/.aws/credentials", Cwd: wsRoot},
	} {
		dec := EvaluatePreToolHook(p, "")
		if dec.Decision != hookDecisionDeny || !strings.Contains(dec.Reason, "Credential Read Airlock") {
			t.Fatalf("expected default host_deny PreToolUse block for %+v, got %+v", p, dec)
		}
	}

	envPath := filepath.Join(wsRoot, ".env")
	if err := os.WriteFile(envPath, []byte("PORT=8080\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if dec := EvaluatePreToolHook(&HookPayload{ToolName: "Read", TargetFile: envPath, Cwd: wsRoot}, ""); dec.Decision != hookDecisionAllow {
		t.Fatalf("expected workspace .env read to be allowed under default host_deny, got %+v", dec)
	}

	// Configure strict mode + blocked_read_paths in nomos.yaml.
	strictYAML := "project: demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n    credential_read_mode: strict\n    blocked_read_paths:\n      - \"secrets/**\"\n"
	if err := os.WriteFile(filepath.Join(wsRoot, "nomos.yaml"), []byte(strictYAML), 0600); err != nil {
		t.Fatal(err)
	}
	if dec := EvaluatePreToolHook(&HookPayload{ToolName: "Read", TargetFile: envPath, Cwd: wsRoot}, ""); dec.Decision != hookDecisionDeny {
		t.Fatalf("expected strict mode to deny workspace .env read, got %+v", dec)
	}
	secretFile := filepath.Join(wsRoot, "secrets", "prod.json")
	if dec := EvaluatePreToolHook(&HookPayload{ToolName: "Read", TargetFile: secretFile, Cwd: wsRoot}, ""); dec.Decision != hookDecisionDeny {
		t.Fatalf("expected blocked_read_paths 'secrets/**' to deny read, got %+v", dec)
	}
}
