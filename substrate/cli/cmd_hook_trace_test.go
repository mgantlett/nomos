package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runInternalHook(t *testing.T, args []string, payload map[string]any) string {
	t.Helper()
	root := NewRootCmd()
	outBuf := new(bytes.Buffer)
	root.SetOut(outBuf)
	root.SetErr(new(bytes.Buffer))
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	root.SetIn(bytes.NewReader(raw))
	root.SetArgs(append([]string{"hook"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("hook %v failed: %v", args, err)
	}
	return strings.TrimSpace(outBuf.String())
}

func TestHookPostTool_VerifiedHostHealthSnapshotsAndDeduplication(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	sandboxDir := t.TempDir()

	// 1. Clean Bash output remains 100% silent
	cleanOut := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, map[string]any{
		"session_id":      "sess-health-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "echo ok"},
		"tool_response":   map[string]any{"stdout": "ok\n", "stderr": ""},
	})
	if cleanOut != "{}" {
		t.Fatalf("expected silent '{}' on clean Bash response, got %s", cleanOut)
	}

	cases := []struct {
		name      string
		eventName string
		toolName  string
		cmdLine   string
		stderr    string
		wantSub   string
	}{
		{
			name:      "dns_offline",
			eventName: "PostToolUse",
			toolName:  "Bash",
			cmdLine:   "pip install numpy",
			stderr:    "WARNING: Retrying: Temporary failure in name resolution",
			wantSub:   "Network egress or DNS resolution is unavailable",
		},
		{
			name:      "apt_archive",
			eventName: "AfterTool",
			toolName:  "run_shell_command",
			cmdLine:   "apt-get update",
			stderr:    "Err:4 http://deb.debian.org/debian bullseye Release 404 Not Found",
			wantSub:   "archive.debian.org",
		},
		{
			name:      "readonly_fs",
			eventName: "PostToolUse",
			toolName:  "Bash",
			cmdLine:   "mkdir /opt/out",
			stderr:    "mkdir: cannot create directory '/opt/out': Read-only file system",
			wantSub:   "read-only (EROFS)",
		},
		{
			name:      "resource_exhausted",
			eventName: "PostToolUse",
			toolName:  "Bash",
			cmdLine:   "tar -xf data.tar",
			stderr:    "tar: write error: No space left on device",
			wantSub:   "ENOSPC/OOM",
		},
		{
			name:      "missing_binary",
			eventName: "AfterTool",
			toolName:  "run_shell_command",
			cmdLine:   "sudo rg pattern src/",
			stderr:    "sh: 1: rg: command not found",
			wantSub:   "Executable 'rg' is not installed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{
				"session_id":      "sess-health-dedup",
				"hook_event_name": tc.eventName,
				"tool_name":       tc.toolName,
				"tool_input":      map[string]any{"command": tc.cmdLine},
				"tool_response":   map[string]any{"stderr": tc.stderr},
			}
			first := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, payload)
			if !strings.Contains(first, tc.wantSub) {
				t.Fatalf("TV-LRN-11 (%s): expected %q in first fault response, got %s", tc.name, tc.wantSub, first)
			}
			repeat := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, payload)
			if repeat != "{}" {
				t.Fatalf("TV-LRN-11 (%s): expected deduplicated silent '{}' on repeat fault, got %s", tc.name, repeat)
			}
		})
	}
}

func TestReceiptTraceParity_GracefulManualSyncAndContradictionGuards(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "demo-SUB-77")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("mkdir wtDir: %v", err)
	}
	walkPath := filepath.Join(repoDir, "walkthrough-SUB-77.md")
	writeWalk := func(body string) {
		t.Helper()
		if err := os.WriteFile(walkPath, []byte(body), 0644); err != nil {
			t.Fatalf("write walkthrough: %v", err)
		}
	}

	// 1. Graceful degradation: manual CLI sync with NO hook session log never false-fails
	writeWalk("# Walkthrough SUB-77\n- Ran go test ./cli/... and verified output.\n")
	if err := validateReceiptTraceParity(wtDir, walkPath, []string{"substrate/cli/a.go"}); err != nil {
		t.Fatalf("TC-TSK-17: expected graceful pass when no hook trace exists, got %v", err)
	}

	// 2. False zero-mutation claim fails closed when files were touched
	writeWalk("# Walkthrough SUB-77\n- No files were modified during this audit.\n")
	if err := validateReceiptTraceParity(wtDir, walkPath, []string{"substrate/cli/a.go"}); err == nil {
		t.Fatalf("TC-TSK-17: expected fail-closed error on false zero-mutation claim")
	}

	// 3. False exclusive single-file claim fails closed when multiple files changed
	writeWalk("# Walkthrough SUB-77\n- Only modified `substrate/cli/a.go` to tighten validation.\n")
	if err := validateReceiptTraceParity(wtDir, walkPath, []string{"substrate/cli/a.go", "substrate/cli/b.go"}); err == nil {
		t.Fatalf("TC-TSK-17: expected fail-closed error on false exclusive-scope claim")
	}

	// 4. Active hook session trace with shell activity and 0 test commands rejects fabricated test run claim
	targetA := filepath.Join(wtDir, "substrate", "cli", "a.go")
	_ = runInternalHook(t, []string{"pre-tool"}, map[string]any{
		"session_id":      "sess-trace-77",
		"hook_event_name": "PreToolUse",
		"cwd":             wtDir,
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": targetA},
	})
	_ = runInternalHook(t, []string{"pre-tool"}, map[string]any{
		"session_id":      "sess-trace-77",
		"hook_event_name": "PreToolUse",
		"cwd":             wtDir,
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "git status --short"},
	})

	writeWalk("# Walkthrough SUB-77\n- Updated a.go and b.go.\n- Ran pytest across unit suite.\n")
	if err := validateReceiptTraceParity(wtDir, walkPath, []string{"substrate/cli/a.go", "substrate/cli/b.go"}); err == nil {
		t.Fatalf("TC-TSK-17: expected fail-closed error on fabricated pytest claim with 0 recorded test commands")
	}

	// 5. Recording an actual test command in the worktree hook trace satisfies Receipt-Trace Parity
	_ = runInternalHook(t, []string{"pre-tool"}, map[string]any{
		"session_id":      "sess-trace-77",
		"hook_event_name": "PreToolUse",
		"cwd":             wtDir,
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "pytest -q"},
	})
	if err := validateReceiptTraceParity(wtDir, walkPath, []string{"substrate/cli/a.go", "substrate/cli/b.go"}); err != nil {
		t.Fatalf("TC-TSK-17: expected parity check to pass once pytest was recorded in hook trace, got %v", err)
	}

	clearHookSessionTrace(wtDir)
	if tr := loadHookSessionTrace(wtDir); tr != nil {
		t.Fatalf("expected clearHookSessionTrace to remove worktree trace file")
	}
}

func TestHookPostTool_SlowStepTimeoutGuardAndExplicitTimeoutSkip(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	sandboxDir := t.TempDir()
	now := time.Now().Unix()

	p := &HookPayload{SessionID: "sess-slow-1", CommandLine: "pytest -q"}
	statePath := resolveBudgetStateFilePath(p, sandboxDir)
	saveBudgetState(statePath, sessionBudgetState{
		StartedUnix:     now - 60,
		LastPreToolUnix: now - 50,
	})

	// 1. Explicit 'timeout' command skips slow-step warning
	skipOut := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir, "--budget-sec", "180"}, map[string]any{
		"session_id":      "sess-slow-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "timeout 60s pytest -q"},
		"tool_response":   map[string]any{"stdout": "ok\n"},
	})
	if skipOut != "{}" {
		t.Fatalf("TC-LRN-13: expected explicit timeout command to skip slow-step advisory, got %s", skipOut)
	}

	// 2. Unbounded 50s shell step emits slow-step timeout radar once and deduplicates on repeat
	saveBudgetState(statePath, sessionBudgetState{
		StartedUnix:     now - 60,
		LastPreToolUnix: now - 50,
	})
	first := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir, "--budget-sec", "180"}, map[string]any{
		"session_id":      "sess-slow-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "pytest -q"},
		"tool_response":   map[string]any{"stdout": "ok\n"},
	})
	if !strings.Contains(first, "[Nomos Slow-Step Timeout Radar]") || !strings.Contains(first, "timeout ") {
		t.Fatalf("TC-LRN-13: expected slow-step timeout advisory, got %s", first)
	}

	st := loadBudgetState(statePath)
	st.LastPreToolUnix = now - 55
	saveBudgetState(statePath, st)
	repeat := runInternalHook(t, []string{"post-tool", "--hermetic-dir", sandboxDir, "--budget-sec", "180"}, map[string]any{
		"session_id":      "sess-slow-1",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "pytest -q"},
		"tool_response":   map[string]any{"stdout": "ok\n"},
	})
	if repeat != "{}" {
		t.Fatalf("TC-LRN-13: expected deduplicated '{}' on second slow step, got %s", repeat)
	}
}

func TestHookPostTool_WorktreeChurnCheckpointsAndVerifyReset(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	wtDir := filepath.Join(t.TempDir(), "worktrees", "demo-SUB-88")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatalf("mkdir wtDir: %v", err)
	}
	payload := map[string]any{
		"session_id":      "sess-churn-88",
		"hook_event_name": "PostToolUse",
		"cwd":             wtDir,
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "git status --short"},
		"tool_response":   map[string]any{"stdout": ""},
	}
	for i := 1; i <= 74; i++ {
		if out := runInternalHook(t, []string{"post-tool"}, payload); out != "{}" {
			t.Fatalf("expected silent '{}' at call %d, got %s", i, out)
		}
	}
	warn75 := runInternalHook(t, []string{"post-tool"}, payload)
	if !strings.Contains(warn75, "[Nomos Worktree Churn Radar]") || !strings.Contains(warn75, "75") {
		t.Fatalf("TC-LRN-13: expected 75-call churn advisory, got %s", warn75)
	}
	if out76 := runInternalHook(t, []string{"post-tool"}, payload); out76 != "{}" {
		t.Fatalf("TC-LRN-13: expected silent '{}' at call 76, got %s", out76)
	}
	verifyPayload := map[string]any{
		"session_id":      "sess-churn-88",
		"hook_event_name": "PostToolUse",
		"cwd":             wtDir,
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "nomos verify"},
		"tool_response":   map[string]any{"stdout": "42/42 passed"},
	}
	if vOut := runInternalHook(t, []string{"post-tool"}, verifyPayload); vOut != "{}" {
		t.Fatalf("expected silent '{}' on nomos verify, got %s", vOut)
	}
	if tr := loadHookSessionTrace(wtDir); tr == nil || tr.UnverifiedCalls != 0 || tr.WarnedChurn75 {
		t.Fatalf("TC-LRN-13: expected nomos verify to reset churn state, got %+v", tr)
	}
}
