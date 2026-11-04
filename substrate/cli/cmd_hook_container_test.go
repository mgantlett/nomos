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
