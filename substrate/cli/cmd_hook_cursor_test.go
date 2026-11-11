// Package cli_test verifies Cursor hook manifest scaffolding, 5-provider install
// parity, additive JSON merging, and Cursor lifecycle payload normalization.
// @intent intent.md
// @intent nomos_hermetic_worktree_spec.md
package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func unmarshalJSONObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return m
}

// TestHookInstall_FiveProviderDefaultParity verifies default nomos hook install scaffolds all 5 providers.
// @verifies TC-HOOK-002
func TestHookInstall_FiveProviderDefaultParity(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, "xdg"))

	projDir := t.TempDir()
	rawAll := executeHookCmd(t, []string{"install", "--dir", projDir, "--json"}, map[string]any{})
	var allReceipt cli.HookInstallReceipt
	if err := json.Unmarshal([]byte(rawAll), &allReceipt); err != nil {
		t.Fatalf("unmarshal all-provider receipt: %v", err)
	}
	if allReceipt.Scope != "project" || len(allReceipt.Targets) != 5 {
		t.Fatalf("expected 5 project targets, got %+v", allReceipt)
	}
	expectedSuffixes := []string{
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".gemini", "settings.json"),
		filepath.Join(".opencode", "plugins", "nomos.ts"),
		filepath.Join(".cursor", "hooks.json"),
		filepath.Join(".agents", "hooks.json"),
	}
	for _, suffix := range expectedSuffixes {
		matched := false
		for _, target := range allReceipt.Targets {
			if strings.HasSuffix(target, suffix) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("expected default install targets %v to contain suffix %q", allReceipt.Targets, suffix)
		}
	}
}

// TestHookInstall_CursorAdditiveMergeAndFallback verifies --cursor project/user installation, idempotency, and missing-binary '{}' grace.
// @verifies TC-HOOK-002
func TestHookInstall_CursorAdditiveMergeAndFallback(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cursorProj := t.TempDir()
	cursorManifestPath := filepath.Join(cursorProj, ".cursor", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(cursorManifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	brownfieldCursor := `{"version":1,"hooks":{"afterFileEdit":[{"command":"./scripts/prettier.sh","timeout":5}]}}`
	if err := os.WriteFile(cursorManifestPath, []byte(brownfieldCursor), 0600); err != nil {
		t.Fatal(err)
	}

	rawProj := executeHookCmd(t, []string{"install", "--cursor", "--dir", cursorProj, "--json"}, map[string]any{})
	var projReceipt cli.HookInstallReceipt
	if err := json.Unmarshal([]byte(rawProj), &projReceipt); err != nil {
		t.Fatal(err)
	}
	if projReceipt.Scope != "project" || len(projReceipt.Targets) != 1 || projReceipt.Targets[0] != cursorManifestPath {
		t.Fatalf("unexpected project --cursor receipt: %+v", projReceipt)
	}
	mergedOnce, err := os.ReadFile(cursorManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = executeHookCmd(t, []string{"install", "--cursor", "--dir", cursorProj, "--json"}, map[string]any{})
	mergedTwice, err := os.ReadFile(cursorManifestPath)
	if err != nil || string(mergedOnce) != string(mergedTwice) {
		t.Fatalf("expected --cursor merge idempotency, err=%v", err)
	}
	assertCursorManifestStructure(t, mergedTwice)

	rawUser := executeHookCmd(t, []string{"install", "--cursor", "--user", "--json"}, map[string]any{})
	var userReceipt cli.HookInstallReceipt
	if err := json.Unmarshal([]byte(rawUser), &userReceipt); err != nil {
		t.Fatal(err)
	}
	expectedUserPath := filepath.Join(homeDir, ".cursor", "hooks.json")
	if userReceipt.Scope != "user" || len(userReceipt.Targets) != 1 || userReceipt.Targets[0] != expectedUserPath {
		t.Fatalf("unexpected user --cursor receipt: %+v", userReceipt)
	}
	verifyCursorMissingBinaryFallback(t, expectedUserPath)
}

func assertCursorManifestStructure(t *testing.T, raw []byte) {
	t.Helper()
	parsed := unmarshalJSONObject(t, string(raw))
	if v, ok := parsed["version"].(float64); !ok || int(v) != 1 {
		t.Fatalf("expected version=1 in .cursor/hooks.json, got %v", parsed["version"])
	}
	if !strings.Contains(string(raw), "./scripts/prettier.sh") {
		t.Fatalf("expected brownfield prettier hook preserved:\n%s", string(raw))
	}
	hooksObj, _ := parsed["hooks"].(map[string]any)
	for _, ev := range []string{"sessionStart", "preToolUse", "beforeShellExecution", "postToolUse", "afterFileEdit", "stop"} {
		if entries, ok := hooksObj[ev].([]any); !ok || len(entries) == 0 {
			t.Fatalf("expected non-empty Cursor %q hook array, got %v", ev, hooksObj[ev])
		}
	}
}

func verifyCursorMissingBinaryFallback(t *testing.T, manifestPath string) {
	t.Helper()
	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("LookPath(sh): %v", err)
	}
	rawBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed := unmarshalJSONObject(t, string(rawBytes))
	hooksObj := parsed["hooks"].(map[string]any)
	beforeShell := hooksObj["beforeShellExecution"].([]any)[0].(map[string]any)
	shellCmd := beforeShell["command"].(string)

	cmd := exec.Command(shBin, "-c", shellCmd)
	cmd.Env = []string{"PATH=" + filepath.Dir(shBin), "HOME=" + t.TempDir()}
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"beforeShellExecution","command":"ls"}`)
	out, cErr := cmd.CombinedOutput()
	if cErr != nil || strings.TrimSpace(string(out)) != "{}" {
		t.Fatalf("expected missing nomos binary to exit 0 with '{}', err=%v out=%q", cErr, string(out))
	}
}

// TestHookCursor_PreToolAndBeforeShellBlocking verifies table-driven Cursor beforeShellExecution and preToolUse permission gating.
// @verifies TC-HOOK-003
func TestHookCursor_PreToolAndBeforeShellBlocking(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "cursor-demo-SUB-7")
	if err := os.MkdirAll(filepath.Join(wtDir, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	trunkFile := filepath.Join(repoDir, "substrate", "main.go")
	wtFile := filepath.Join(wtDir, "pkg", "handler.go")

	cases := []struct {
		name           string
		payload        map[string]any
		wantPermission string
		wantMsgSubstr  string
	}{
		{
			name:           "beforeShellExecution_denies_wildcard_rm",
			payload:        map[string]any{"conversation_id": "c1", "hook_event_name": "beforeShellExecution", "workspace_roots": []string{repoDir}, "cwd": wtDir, "command": "rm -rf /"},
			wantPermission: "deny",
			wantMsgSubstr:  "Wildcard or root/parent deletion",
		},
		{
			name:           "beforeShellExecution_denies_git_stash",
			payload:        map[string]any{"conversation_id": "c1", "hook_event_name": "beforeShellExecution", "workspace_roots": []string{repoDir}, "cwd": wtDir, "command": "git stash push"},
			wantPermission: "deny",
			wantMsgSubstr:  "git stash",
		},
		{
			name:           "beforeShellExecution_allows_safe_test",
			payload:        map[string]any{"conversation_id": "c1", "hook_event_name": "beforeShellExecution", "workspace_roots": []string{repoDir}, "cwd": wtDir, "command": "go test ./..."},
			wantPermission: "allow",
		},
		{
			name:           "preToolUse_denies_trunk_write",
			payload:        map[string]any{"conversation_id": "c1", "hook_event_name": "preToolUse", "workspace_roots": []string{repoDir}, "tool_name": "Write", "tool_input": map[string]any{"file_path": trunkFile}},
			wantPermission: "deny",
			wantMsgSubstr:  "Hermetic Worktree Violation",
		},
		{
			name:           "preToolUse_allows_worktree_write",
			payload:        map[string]any{"conversation_id": "c1", "hook_event_name": "preToolUse", "workspace_roots": []string{repoDir}, "tool_name": "Write", "tool_input": map[string]any{"file_path": wtFile}},
			wantPermission: "allow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := executeHookCmd(t, []string{"pre-tool"}, tc.payload)
			res := unmarshalJSONObject(t, out)
			if got, _ := res["permission"].(string); got != tc.wantPermission {
				t.Fatalf("expected permission=%q, got %q (raw=%s)", tc.wantPermission, got, out)
			}
			if tc.wantPermission == "deny" {
				if !strings.Contains(res["agent_message"].(string), tc.wantMsgSubstr) || !strings.Contains(res["user_message"].(string), tc.wantMsgSubstr) {
					t.Fatalf("expected agent_message and user_message to contain %q, got %s", tc.wantMsgSubstr, out)
				}
			}
		})
	}
}

// TestHookCursor_AfterFileEditAndStopLifecycle verifies Cursor sessionStart, afterFileEdit AST radar, and stop followup_message gating.
// @verifies TC-HOOK-003
func TestHookCursor_AfterFileEditAndStopLifecycle(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "nomos.yaml"), []byte("project: cursor-demo\ndefault_branch: main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	wtDir := filepath.Join(repoDir, "worktrees", "cursor-demo-SUB-7")
	wtPkgDir := filepath.Join(wtDir, "pkg")
	if err := os.MkdirAll(wtPkgDir, 0755); err != nil {
		t.Fatal(err)
	}

	sessOut := executeHookCmd(t, []string{"session-start"}, map[string]any{
		"conversation_id": "cur-conv-001", "generation_id": "cur-gen-001", "hook_event_name": "sessionStart", "workspace_roots": []string{repoDir},
	})
	if ctxStr, _ := unmarshalJSONObject(t, sessOut)["additional_context"].(string); !strings.Contains(ctxStr, "Nomos JIT Session Handshake") {
		t.Fatalf("expected Cursor sessionStart additional_context handshake, got %s", sessOut)
	}

	brokenGo := filepath.Join(wtPkgDir, "broken.go")
	if err := os.WriteFile(brokenGo, []byte("package pkg\nfunc Bad( {\n"), 0600); err != nil {
		t.Fatal(err)
	}
	afterOut := executeHookCmd(t, []string{"post-tool"}, map[string]any{
		"conversation_id": "cur-conv-001", "hook_event_name": "afterFileEdit", "workspace_roots": []string{repoDir},
		"file_path": brokenGo, "edits": []map[string]any{{"old_string": "package pkg\n", "new_string": "package pkg\nfunc Bad( {\n"}},
	})
	if adv, _ := unmarshalJSONObject(t, afterOut)["additional_context"].(string); !strings.Contains(adv, "broken.go") {
		t.Fatalf("expected Cursor afterFileEdit AST syntax advisory for broken.go, got %s", afterOut)
	}

	stopBlock := executeHookCmd(t, []string{"stop"}, map[string]any{
		"conversation_id": "cur-conv-001", "hook_event_name": "stop", "workspace_roots": []string{wtDir}, "status": "completed", "loop_count": 0,
	})
	if fm, _ := unmarshalJSONObject(t, stopBlock)["followup_message"].(string); !strings.Contains(fm, "nomos verify") {
		t.Fatalf("expected Cursor stop followup_message requiring nomos verify, got %s", stopBlock)
	}

	stopAborted := executeHookCmd(t, []string{"stop"}, map[string]any{
		"conversation_id": "cur-conv-001", "hook_event_name": "stop", "workspace_roots": []string{wtDir}, "status": "aborted", "loop_count": 0,
	})
	if stopAborted != "{}" {
		t.Fatalf("expected aborted Cursor stop to return '{}', got %s", stopAborted)
	}
}
