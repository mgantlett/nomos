package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHookPreTool_BinaryContextAndGeminiInlineMediaGuards(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	for _, binPath := range []string{"/app/video.mp4", "/app/model.safetensors", "/app/data.sqlite", "/app/libfoo.so", "/app/archive.tar.gz"} {
		outClaude := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "c-bin",
			"hook_event_name": "PreToolUse",
			"tool_name":       "Read",
			"tool_input":      map[string]any{"file_path": binPath},
		})
		if !strings.Contains(outClaude, `"permissionDecision":"deny"`) || !strings.Contains(outClaude, "Binary Context Guard") {
			t.Fatalf("expected Claude binary Read(%s) deny, got %q", binPath, outClaude)
		}

		outGemini := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "g-bin",
			"hook_event_name": "BeforeTool",
			"tool_name":       "read_file",
			"tool_input":      map[string]any{"file_path": binPath},
		})
		if dec := parseDecision(t, outGemini); dec.Decision != "deny" || !strings.Contains(dec.Reason, "Binary Context Guard") {
			t.Fatalf("expected Gemini binary read_file(%s) deny, got %+v", binPath, dec)
		}
	}

	for _, mediaPath := range []string{"/app/code.png", "/app/frame_000.jpg", "/app/invoice.pdf"} {
		gemMedia := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "g-img",
			"hook_event_name": "BeforeTool",
			"tool_name":       "read_file",
			"tool_input":      map[string]any{"file_path": mediaPath},
		})
		if dec := parseDecision(t, gemMedia); dec.Decision != "deny" || !strings.Contains(dec.Reason, "Gemini Inline-Media Guard") {
			t.Fatalf("expected Gemini container read_file(%s) deny, got %+v", mediaPath, dec)
		}

		claudeImg := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "c-img",
			"hook_event_name": "PreToolUse",
			"tool_name":       "Read",
			"tool_input":      map[string]any{"file_path": mediaPath},
		})
		if claudeImg != "{}" {
			t.Fatalf("expected Claude vision Read(%s) to remain allowed ({}), got %q", mediaPath, claudeImg)
		}
	}
}

func TestHookPreTool_HeadlessInteractiveHangGuard(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	deniedCmds := []string{
		"tail -f /var/log/nginx/access.log",
		"tail -n 20 -F /app/server.log",
		"vim /app/config.yaml",
		"sudo nano /etc/nginx/nginx.conf",
		"less /app/README.md",
		"top",
		"htop",
		"watch -n 1 ps aux",
	}
	for _, cmdStr := range deniedCmds {
		out := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "hang-1",
			"hook_event_name": "BeforeTool",
			"tool_name":       "run_shell_command",
			"tool_input":      map[string]any{"command": cmdStr},
		})
		if dec := parseDecision(t, out); dec.Decision != "deny" || !strings.Contains(dec.Reason, "Headless Hang Guard") {
			t.Fatalf("expected command %q to be denied by Headless Hang Guard, got %+v", cmdStr, dec)
		}
	}

	allowedCmds := []string{
		"tail -n 100 /var/log/nginx/access.log",
		"tail -f /var/log/nginx/access.log | head -n 5",
		"timeout 5 tail -f /var/log/nginx/access.log",
		"top -b -n 1",
		"vim --version",
		"grep -rn 'tail -f' /app",
		"python3 -c \"print('vim nano less')\"",
	}
	for _, cmdStr := range allowedCmds {
		out := executeHookCmd(t, []string{"pre-tool", "--container-sandbox"}, map[string]any{
			"cwd":             "/app",
			"session_id":      "hang-ok",
			"hook_event_name": "BeforeTool",
			"tool_name":       "run_shell_command",
			"tool_input":      map[string]any{"command": cmdStr},
		})
		if dec := parseDecision(t, out); dec.Decision != "allow" {
			t.Fatalf("expected bounded/non-interactive command %q to be allowed, got %+v", cmdStr, dec)
		}
	}
}

func TestHookPostTool_SessionBudgetCheckpointRadar(t *testing.T) {
	cacheRoot := t.TempDir()
	sandbox := t.TempDir()
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	payload := map[string]any{
		"cwd":             sandbox,
		"session_id":      "budget-sess-01",
		"hook_event_name": "AfterTool",
		"tool_name":       "run_shell_command",
		"tool_input":      map[string]any{"command": "pytest -q"},
	}

	outInit := executeHookCmd(t, []string{"post-tool", "--container-sandbox", "--budget-sec", "1000"}, payload)
	if outInit != "{}" {
		t.Fatalf("expected initial 0%% budget call to remain silent {}, got %q", outInit)
	}

	matches, err := filepath.Glob(filepath.Join(cacheRoot, "nomos", "hooks", "nomos-hook-budget-*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly 1 budget state file in XDG_CACHE_HOME, got %v (err=%v)", matches, err)
	}
	stampFile := matches[0]

	writeStamp := func(startedAgoSec int64, w75, w90 bool) {
		raw, mErr := json.Marshal(map[string]any{
			"started_unix": time.Now().Unix() - startedAgoSec,
			"warned_75":    w75,
			"warned_90":    w90,
		})
		if mErr != nil {
			t.Fatal(mErr)
		}
		if wErr := os.WriteFile(stampFile, raw, 0600); wErr != nil {
			t.Fatal(wErr)
		}
	}

	writeStamp(760, false, false)
	out75 := executeHookCmd(t, []string{"post-tool", "--container-sandbox", "--budget-sec", "1000"}, payload)
	if !strings.Contains(out75, "Nomos Budget Radar") || !strings.Contains(out75, "76%") {
		t.Fatalf("expected 75%%+ budget checkpoint advisory, got %q", out75)
	}

	out75Repeat := executeHookCmd(t, []string{"post-tool", "--container-sandbox", "--budget-sec", "1000"}, payload)
	if out75Repeat != "{}" {
		t.Fatalf("expected deduplicated silent {} after 75%% warning already fired, got %q", out75Repeat)
	}

	writeStamp(920, true, false)
	t.Setenv("NOMOS_BUDGET_SECONDS", "1000")
	out90 := executeHookCmd(t, []string{"post-tool", "--container-sandbox"}, payload)
	if !strings.Contains(out90, "Nomos Budget Radar") || !strings.Contains(out90, "92%") {
		t.Fatalf("expected 90%%+ urgent budget advisory via NOMOS_BUDGET_SECONDS, got %q", out90)
	}

	out90Repeat := executeHookCmd(t, []string{"post-tool", "--container-sandbox"}, payload)
	if out90Repeat != "{}" {
		t.Fatalf("expected deduplicated silent {} after 90%% warning already fired, got %q", out90Repeat)
	}

	entries, rErr := os.ReadDir(sandbox)
	if rErr != nil || len(entries) != 0 {
		t.Fatalf("expected zero filesystem footprint inside sandbox dir, got %v", entries)
	}
}

