package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func runGitOrFail(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}

func TestStageAndCommitGovernanceOnly_PreservesPreStagedUserIndex(t *testing.T) {
	for _, unbornHead := range []bool{true, false} {
		label := "existing_head"
		if unbornHead {
			label = "unborn_head"
		}
		t.Run(label, func(t *testing.T) {
			repoDir := t.TempDir()
			runGitOrFail(t, repoDir, "init", "-b", "main")
			runGitOrFail(t, repoDir, "config", "user.email", "dev@example.com")
			runGitOrFail(t, repoDir, "config", "user.name", "First Timer")

			if !unbornHead {
				if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# Existing App\n"), 0644); err != nil {
					t.Fatalf("write README: %v", err)
				}
				runGitOrFail(t, repoDir, "add", "README.md")
				runGitOrFail(t, repoDir, "commit", "-m", "chore: initial app commit")
			}

			// Simulate developer having pre-staged WIP work before running 'nomos init'.
			wipPath := filepath.Join(repoDir, "wip_feature.go")
			if err := os.WriteFile(wipPath, []byte("package main\n\nfunc Wip() int { return 42 }\n"), 0644); err != nil {
				t.Fatalf("write wip_feature.go: %v", err)
			}
			runGitOrFail(t, repoDir, "add", "wip_feature.go")

			if err := executeInit(repoDir); err != nil {
				t.Fatalf("executeInit failed: %v", err)
			}

			headStat := runGitOrFail(t, repoDir, "show", "--name-only", "--pretty=format:", "HEAD")
			if strings.Contains(headStat, "wip_feature.go") {
				t.Fatalf("expected pre-staged wip_feature.go NOT to be swept into baseline init commit, got HEAD files:\n%s", headStat)
			}
			for _, gov := range []string{"nomos.yaml", "AGENTS.md", "CLAUDE.md", filepath.Join(claudeDirToken, settingsFileName), filepath.Join(geminiDirToken, settingsFileName), filepath.Join(cursorDirToken, hooksFileName), filepath.Join(openCodeDirToken, "plugins", "nomos.ts")} {
				if !strings.Contains(headStat, gov) {
					t.Errorf("expected %s committed in baseline governance commit, got:\n%s", gov, headStat)
				}
			}

			statusShort := runGitOrFail(t, repoDir, "status", "--short")
			if strings.TrimSpace(statusShort) != "A  wip_feature.go" {
				t.Fatalf("expected ONLY pre-staged wip_feature.go to remain staged in git status, got:\n%s", statusShort)
			}
		})
	}
}

func extractFirstShellHookCommand(t *testing.T, rawJSON []byte, eventKey string) string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		t.Fatalf("unmarshal hook manifest: %v", err)
	}
	hooksObj, ok := parsed["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("missing hooks map")
	}
	arr, ok := hooksObj[eventKey].([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("missing %s entries", eventKey)
	}
	entry, ok := arr[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid %s entry", eventKey)
	}
	inner, ok := entry["hooks"].([]any)
	if !ok || len(inner) == 0 {
		t.Fatalf("missing inner hooks array on %s", eventKey)
	}
	h0, ok := inner[0].(map[string]any)
	if !ok {
		t.Fatalf("invalid inner hook object on %s", eventKey)
	}
	cmdStr, ok := h0["command"].(string)
	if !ok || cmdStr == "" {
		t.Fatalf("missing command string on %s", eventKey)
	}
	return cmdStr
}

func TestPosixHookShellCommand_GUIFallbackPathAndMissingBinaryGrace(t *testing.T) {
	if env := ensureCompiledHookTestEnv(t); len(env) == 0 {
		t.Fatalf("expected non-empty compiled hook test env")
	}
	compiledBin := filepath.Join(compiledHookTestBinDir, "nomos")
	binBytes, err := os.ReadFile(compiledBin)
	if err != nil {
		t.Fatalf("read compiled nomos binary: %v", err)
	}

	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("LookPath(sh): %v", err)
	}
	minSysBin := filepath.Dir(shBin)

	claudePreCmd := extractFirstShellHookCommand(t, DefaultClaudeSettingsHooksJSON(), "PreToolUse")
	geminiPreCmd := extractFirstShellHookCommand(t, DefaultGeminiSettingsHooksJSON(), "BeforeTool")
	if claudePreCmd != geminiPreCmd {
		t.Fatalf("expected identical POSIX pre-tool command across Claude and Gemini manifests")
	}

	// 1. Teammate machine without nomos installed exits 0 with '{}' instead of exit 127.
	emptyHome := t.TempDir()
	missingCmd := exec.Command(shBin, "-c", claudePreCmd)
	missingCmd.Env = []string{"PATH=" + minSysBin, "HOME=" + emptyHome}
	missingCmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	missingOut, mErr := missingCmd.CombinedOutput()
	if mErr != nil || strings.TrimSpace(string(missingOut)) != "{}" {
		t.Fatalf("expected graceful '{}' when nomos is not installed, err=%v out=%q", mErr, string(missingOut))
	}

	// 2. GUI IDE launch without ~/.local/bin in PATH resolves binary via $HOME/.local/bin fallback.
	guiHome := t.TempDir()
	localBin := filepath.Join(guiHome, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatalf("mkdir .local/bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localBin, "nomos"), binBytes, 0755); err != nil {
		t.Fatalf("stage ~/.local/bin/nomos: %v", err)
	}
	repoDir := t.TempDir()
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit: %v", err)
	}
	payload := `{"hook_event_name":"PreToolUse","session_id":"gui-1","cwd":"` + filepath.ToSlash(repoDir) + `","tool_name":"Write","tool_input":{"file_path":"` + filepath.ToSlash(filepath.Join(repoDir, "app.go")) + `"}}`
	guiCmd := exec.Command(shBin, "-c", claudePreCmd)
	guiCmd.Dir = repoDir
	guiCmd.Env = []string{"PATH=" + minSysBin, "HOME=" + guiHome}
	guiCmd.Stdin = strings.NewReader(payload)
	guiOut, gErr := guiCmd.CombinedOutput()
	if gErr != nil {
		t.Fatalf("expected POSIX fallback hook execution to succeed, err=%v out=%s", gErr, string(guiOut))
	}
	if !strings.Contains(string(guiOut), `"permissionDecision":"deny"`) {
		t.Fatalf("expected GUI fallback nomos binary to enforce trunk Write block, got:\n%s", string(guiOut))
	}
}
