package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func executeHookCmd(t *testing.T, args []string, payload map[string]any) string {
	t.Helper()
	root := cli.NewRootCmd()
	outBuf := new(bytes.Buffer)
	root.SetOut(outBuf)
	root.SetErr(new(bytes.Buffer))

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}
	root.SetIn(bytes.NewReader(raw))
	root.SetArgs(append([]string{"hook"}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("hook command %v failed: %v", args, err)
	}
	return strings.TrimSpace(outBuf.String())
}

func parseDecision(t *testing.T, raw string) cli.HookDecision {
	t.Helper()
	var dec cli.HookDecision
	if err := json.Unmarshal([]byte(raw), &dec); err != nil {
		t.Fatalf("invalid HookDecision JSON %q: %v", raw, err)
	}
	return dec
}

func TestHookPreTool_WorktreeAndUnexpandedVarGuards(t *testing.T) {
	repoDir := t.TempDir()
	wtFile := filepath.Join(repoDir, "worktrees", "repo-SUB-1", "substrate", "main.go")
	trunkFile := filepath.Join(repoDir, "substrate", "main.go")
	traversalFile := filepath.Join(repoDir, "worktrees", "repo-SUB-1") + "/../../substrate/main.go"

	// 1. Allowed worktree mutation
	outAllow := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": wtFile}},
	})
	if dec := parseDecision(t, outAllow); dec.Decision != "allow" {
		t.Fatalf("expected worktree file to be allowed, got %+v", dec)
	}

	// 2. Denied trunk mutation & traversal
	outTrunk := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": trunkFile}},
	})
	if dec := parseDecision(t, outTrunk); dec.Decision != "deny" {
		t.Fatalf("expected trunk write to be denied, got %+v", dec)
	}
	outTrav := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": traversalFile}},
	})
	if dec := parseDecision(t, outTrav); dec.Decision != "deny" {
		t.Fatalf("expected traversal write to be denied, got %+v", dec)
	}

	// 3. Denied unexpanded $VAR and ~ paths on native file tools
	for _, badPath := range []string{"$HOME/worktrees/repo-SUB-1/a.go", "${WORKSPACE}/a.go", "~/worktrees/repo-SUB-1/a.go"} {
		outVar := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"tool_name":  "Write",
			"tool_input": map[string]any{"file_path": badPath},
		})
		if dec := parseDecision(t, outVar); dec.Decision != "deny" {
			t.Fatalf("expected unexpanded path %q to be denied, got %+v", badPath, dec)
		}
	}
}

func TestHookPreTool_CommandSafetyAndHermeticContainerMode(t *testing.T) {
	// 1. Destructive command guards (Law 7 & Law 1/3/4)
	deniedCmds := []string{
		"rm -rf /",
		"rm -rf *",
		"rm -rf ~",
		"rm -rf ..",
		"go install github.com/mgantlett/nomos/substrate/cmd/nomos@latest",
		"sqlite3 .nomos/mneme.db 'DELETE FROM memories'",
		"git stash push",
		"rm -f .git/hooks/pre-commit",
	}
	for _, cmdStr := range deniedCmds {
		out := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
			"tool_name":  "Bash",
			"tool_input": map[string]any{"command": cmdStr},
		})
		if dec := parseDecision(t, out); dec.Decision != "deny" {
			t.Fatalf("expected command %q to be denied, got %+v", cmdStr, dec)
		}
	}

	// 2. Hermetic container mode (--hermetic-dir)
	sandboxDir := t.TempDir()
	insideFile := filepath.Join(sandboxDir, "app", "service.ts")
	outsideFile := filepath.Join(filepath.Dir(sandboxDir), "escape.ts")

	outIn := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": insideFile},
	})
	if dec := parseDecision(t, outIn); dec.Decision != "allow" {
		t.Fatalf("expected file inside --hermetic-dir to be allowed, got %+v", dec)
	}

	outOut := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, map[string]any{
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": outsideFile},
	})
	if dec := parseDecision(t, outOut); dec.Decision != "deny" {
		t.Fatalf("expected file outside --hermetic-dir to be denied, got %+v", dec)
	}
}

func TestHookPostTool_ChannelPurityAndInMemorySyntax(t *testing.T) {
	sandboxDir := t.TempDir()

	// 1. 100% silent on Bash, Read, MCP, and PostInvocation channels
	for _, tool := range []string{"Bash", "Read", "mcp_postgres_query"} {
		out := executeHookCmd(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, map[string]any{
			"tool_name":  tool,
			"tool_input": map[string]any{"file_path": filepath.Join(sandboxDir, "broken.json")},
		})
		if out != "{}" {
			t.Fatalf("expected silent '{}' on %s channel, got %s", tool, out)
		}
	}

	// 2. Detects invalid JSON in-memory on Write channel without creating .tmp files
	badJSON := filepath.Join(sandboxDir, "config.json")
	if err := os.WriteFile(badJSON, []byte(`{"broken": `), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	outBad := executeHookCmd(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": badJSON},
	})
	if !strings.Contains(outBad, "invalid JSON syntax") {
		t.Fatalf("expected invalid JSON syntax advisory, got %s", outBad)
	}
	if _, err := os.Stat(filepath.Join(sandboxDir, ".tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected zero .tmp footprint in sandboxDir")
	}

	// 3. Silent on valid JSON
	if err := os.WriteFile(badJSON, []byte(`{"valid": true}`), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	outGood := executeHookCmd(t, []string{"post-tool", "--hermetic-dir", sandboxDir}, map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": badJSON},
	})
	if outGood != "{}" {
		t.Fatalf("expected '{}' on valid JSON edit, got %s", outGood)
	}
}

func TestHookChaining_AdditivePreservation(t *testing.T) {
	existingClaude := []byte(`{
  "permissions": {"allow": ["Bash(npm test)"]},
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{"type": "command", "command": "./custom-audit.sh", "timeout": 5}]
      }
    ]
  }
}`)
	mergedOnce, err := cli.MergeHookManifest(existingClaude, cli.DefaultClaudeSettingsHooksJSON())
	if err != nil {
		t.Fatalf("MergeHookManifest failed: %v", err)
	}
	mergedTwice, err := cli.MergeHookManifest(mergedOnce, cli.DefaultClaudeSettingsHooksJSON())
	if err != nil {
		t.Fatalf("second MergeHookManifest failed: %v", err)
	}
	if string(mergedOnce) != string(mergedTwice) {
		t.Fatalf("expected MergeHookManifest to be idempotent on repeat runs:\nonce:\n%s\ntwice:\n%s", string(mergedOnce), string(mergedTwice))
	}
	outStr := string(mergedTwice)
	for _, token := range []string{"Bash(npm test)", "./custom-audit.sh", "nomos hook pre-tool", "nomos hook post-tool", "nomos hook session-start"} {
		if !strings.Contains(outStr, token) {
			t.Fatalf("expected merged manifest to preserve/include %q, got:\n%s", token, outStr)
		}
	}
}

func TestHookPreTool_ThreeStrikeLoopCircuitBreaker(t *testing.T) {
	sandboxDir := t.TempDir()
	targetFile := filepath.Join(sandboxDir, "app.go")
	if err := os.WriteFile(targetFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	payload := map[string]any{
		"cwd":        sandboxDir,
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "pytest tests/test_missing.py"},
	}

	// Strike 1 & Strike 2 allowed
	for strike := 1; strike <= 2; strike++ {
		out := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, payload)
		if dec := parseDecision(t, out); dec.Decision != "allow" {
			t.Fatalf("expected strike %d to be allowed, got %+v", strike, dec)
		}
	}

	// Strike 3 without workspace progress is denied by Loop Circuit-Breaker
	outStrike3 := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, payload)
	dec3 := parseDecision(t, outStrike3)
	if dec3.Decision != "deny" || !strings.Contains(dec3.Reason, "Loop Circuit-Breaker (Law 1)") {
		t.Fatalf("expected strike 3 to trigger Loop Circuit-Breaker deny, got %+v", dec3)
	}

	// Workspace state mutation resets the circuit-breaker
	if err := os.WriteFile(targetFile, []byte("package main\n// updated\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	outAfterEdit := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, payload)
	if dec := parseDecision(t, outAfterEdit); dec.Decision != "allow" {
		t.Fatalf("expected circuit-breaker reset after workspace file update, got %+v", dec)
	}

	// 3 distinct edits on the same file with different Replacement args are allowed
	for i := 1; i <= 3; i++ {
		editOut := executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", sandboxDir}, map[string]any{
			"toolCall": map[string]any{
				"name": "edit_file",
				"args": map[string]any{
					"Path":        targetFile,
					"Target":      "old",
					"Replacement": strings.Repeat("x", i),
				},
			},
		})
		if dec := parseDecision(t, editOut); dec.Decision != "allow" {
			t.Fatalf("expected distinct edit %d on same file to be allowed, got %+v", i, dec)
		}
	}
}


