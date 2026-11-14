package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookContainerSandbox_AntiRewardHackingAndSysadminParity(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	allowCases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "sysadmin nginx config write allowed",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Write",
					"args": map[string]any{"file_path": "/etc/nginx/nginx.conf"},
				},
			},
		},
		{
			name: "etc os-release read shell allowed",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "cat /etc/os-release"},
				},
			},
		},
		{
			name: "task sqlite3 database query allowed",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "sqlite3 /app/warehouse.db 'SELECT count(*) FROM orders;'"},
				},
			},
		},
		{
			name: "relative workspace tests directory allowed",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "pytest ./tests/test_api.py"},
				},
			},
		},
	}

	for _, tc := range allowCases {
		out := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, tc.payload)
		if dec := parseDecision(t, out); dec.Decision != "allow" {
			t.Fatalf("%s: expected allow, got %+v", tc.name, dec)
		}
	}

	denyCases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name: "verifier reward forgery file write denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Write",
					"args": map[string]any{"file_path": "/logs/verifier/reward.txt"},
				},
			},
		},
		{
			name: "oracle solution file read denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Read",
					"args": map[string]any{"file_path": "/solution/solve.sh"},
				},
			},
		},
		{
			name: "shell reward redirect denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "echo 1.0 > /logs/verifier/reward.txt"},
				},
			},
		},
		{
			name: "shell root tests inspection denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "cat /tests/test.sh"},
				},
			},
		},
		{
			name: "wildcard rm denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "rm -rf /*"},
				},
			},
		},
		{
			name: "TC-SHIM-02 grading harness file read denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Read",
					"args": map[string]any{"file_path": "/grading/score.py"},
				},
			},
		},
		{
			name: "verifier script shell snoop denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "cat /verifier/run.sh"},
				},
			},
		},
		{
			name: "pid1 environ verifier env snoop denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Bash",
					"args": map[string]any{"command": "tr '\\0' '\\n' < /proc/1/environ"},
				},
			},
		},
		{
			name: "multi-file payload hiding protected oracle path denied",
			payload: map[string]any{
				"cwd": "/app",
				"toolCall": map[string]any{
					"name": "Read",
					"args": map[string]any{"paths": []string{"/app/main.go", "/oracle/expected.json"}},
				},
			},
		},
	}

	for _, tc := range denyCases {
		out := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, tc.payload)
		if dec := parseDecision(t, out); dec.Decision != "deny" {
			t.Fatalf("%s: expected deny, got %+v", tc.name, dec)
		}
	}
}

func TestHookContainerSandbox_PostToolSyntaxWithoutLOCNoise(t *testing.T) {
	sandbox := t.TempDir()
	brokenGo := filepath.Join(sandbox, "broken.go")
	if err := os.WriteFile(brokenGo, []byte("package main\nfunc broken( {\n"), 0600); err != nil {
		t.Fatal(err)
	}
	outBroken := executeHookCmd(t, []string{"post-tool", "--container-sandbox"}, map[string]any{
		"cwd":        sandbox,
		"session_id": "s1",
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": brokenGo},
	})
	if !strings.Contains(outBroken, "Nomos AST Radar") {
		t.Fatalf("expected Go syntax advisory on broken.go, got %q", outBroken)
	}

	var bigValid strings.Builder
	bigValid.WriteString("package main\n\n")
	for i := 0; i < 450; i++ {
		bigValid.WriteString(fmt.Sprintf("var lineVar%d = %d\n", i, i))
	}
	legacyGo := filepath.Join(sandbox, "legacy_large.go")
	if err := os.WriteFile(legacyGo, []byte(bigValid.String()), 0600); err != nil {
		t.Fatal(err)
	}
	outLegacy := executeHookCmd(t, []string{"post-tool", "--container-sandbox"}, map[string]any{
		"cwd":        sandbox,
		"session_id": "s1",
		"tool_name":  "Edit",
		"tool_input": map[string]any{"file_path": legacyGo},
	})
	if outLegacy != "{}" {
		t.Fatalf("expected 100%% silent {} on valid 450-LOC legacy file without nomos.yaml, got %q", outLegacy)
	}
}

func TestHookContainerSandbox_GeminiCliBeforeAndAfterToolParity(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	denyDir := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
		"cwd":             "/app",
		"session_id":      "gem-1",
		"hook_event_name": "BeforeTool",
		"tool_name":       "list_directory",
		"tool_input":      map[string]any{"dir_path": "/tests"},
	})
	if dec := parseDecision(t, denyDir); dec.Decision != "deny" {
		t.Fatalf("expected Gemini BeforeTool dir_path=/tests deny JSON, got %q (%+v)", denyDir, dec)
	}

	denyShell := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
		"cwd":             "/app",
		"session_id":      "gem-1",
		"hook_event_name": "BeforeTool",
		"tool_name":       "run_shell_command",
		"tool_input":      map[string]any{"command": "echo 1 > /logs/verifier/reward.txt"},
	})
	if dec := parseDecision(t, denyShell); dec.Decision != "deny" {
		t.Fatalf("expected Gemini BeforeTool reward tamper deny JSON, got %q (%+v)", denyShell, dec)
	}

	allowShell := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
		"cwd":             "/app",
		"session_id":      "gem-1",
		"hook_event_name": "BeforeTool",
		"tool_name":       "run_shell_command",
		"tool_input":      map[string]any{"command": "ls -la /app"},
	})
	if dec := parseDecision(t, allowShell); dec.Decision != "allow" {
		t.Fatalf("expected Gemini BeforeTool allow JSON, got %q (%+v)", allowShell, dec)
	}

	sandbox := t.TempDir()
	brokenPy := filepath.Join(sandbox, "broken.py")
	if err := os.WriteFile(brokenPy, []byte("def broken(:\n    pass\n"), 0600); err != nil {
		t.Fatal(err)
	}
	afterOut := executeHookCmd(t, []string{"post-tool", "--container-sandbox"}, map[string]any{
		"cwd":             sandbox,
		"session_id":      "gem-1",
		"hook_event_name": "AfterTool",
		"tool_name":       "write_file",
		"tool_input":      map[string]any{"file_path": brokenPy},
	})
	if !strings.Contains(afterOut, "additionalContext") || strings.Contains(afterOut, "PostToolUse") {
		t.Fatalf("expected Gemini AfterTool additionalContext without Claude PostToolUse tag, got %q", afterOut)
	}
}
