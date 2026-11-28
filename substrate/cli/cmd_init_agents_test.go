package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/tui"
)

func TestParseSelectedAgents_ValidationAndAllExpansion(t *testing.T) {
	all, err := parseSelectedAgents("all")
	if err != nil {
		t.Fatalf("parseSelectedAgents(all) failed: %v", err)
	}
	if !reflect.DeepEqual(all, canonicalInitAgents) {
		t.Fatalf("expected all canonical agents %v, got %v", canonicalInitAgents, all)
	}

	sub, sErr := parseSelectedAgents("Claude, cursor, claude")
	if sErr != nil {
		t.Fatalf("parseSelectedAgents(claude,cursor) failed: %v", sErr)
	}
	if !reflect.DeepEqual(sub, []string{agentClaude, agentCursor}) {
		t.Fatalf("expected deduplicated [claude cursor], got %v", sub)
	}

	if _, bErr := parseSelectedAgents("claude,bogus"); bErr == nil {
		t.Fatalf("expected unknown agent token to fail closed")
	}
	if _, eErr := parseSelectedAgents(" , "); eErr == nil {
		t.Fatalf("expected empty comma list to fail closed")
	}
}

// @verifies TC-INIT-09
func TestDetectInstalledAgents_RepoSignalsHomeAndZeroDumpFallback(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	emptyDir := t.TempDir()
	fallback, fErr := resolveInitAgents(emptyDir, "")
	if fErr != nil || len(fallback) != 0 {
		t.Fatalf("TC-INIT-09: expected empty agent slice when no repo/PATH/HOME signals exist, got %v (err=%v)", fallback, fErr)
	}

	// Verify $HOME/.cursor auto-detection works without cursor on PATH.
	homeWithCursor := t.TempDir()
	if err := os.MkdirAll(filepath.Join(homeWithCursor, hookcmd.CursorDirToken), 0755); err != nil {
		t.Fatalf("mkdir ~/.cursor: %v", err)
	}
	t.Setenv("HOME", homeWithCursor)
	homeDetected, hErr := resolveInitAgents(emptyDir, "")
	if hErr != nil || !reflect.DeepEqual(homeDetected, []string{agentCursor}) {
		t.Fatalf("TC-INIT-09: expected [cursor] from $HOME/.cursor, got %v (err=%v)", homeDetected, hErr)
	}

	t.Setenv("HOME", t.TempDir())
	claudeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(claudeDir, claudeDirToken), 0755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	detected, dErr := resolveInitAgents(claudeDir, "")
	if dErr != nil {
		t.Fatalf("resolveInitAgents failed: %v", dErr)
	}
	if !reflect.DeepEqual(detected, []string{agentClaude}) {
		t.Fatalf("expected detected [claude] only, got %v", detected)
	}

	if err := scaffoldAmbientHooksForSelection(claudeDir, []string{agentClaude}); err != nil {
		t.Fatalf("scaffoldAmbientHooksForSelection failed: %v", err)
	}
	if !fileExists(filepath.Join(claudeDir, claudeDirToken, settingsFileName)) {
		t.Fatalf("expected claude settings manifest scaffolded")
	}
	if fileExists(filepath.Join(claudeDir, hookcmd.CursorDirToken, hookcmd.HooksFileName)) {
		t.Fatalf("did not expect cursor hooks scaffolded when only claude selected")
	}
	if !reflect.DeepEqual(detectExistingHookAgents(claudeDir), []string{agentClaude}) {
		t.Fatalf("expected detectExistingHookAgents=[claude], got %v", detectExistingHookAgents(claudeDir))
	}
}

// @verifies TC-INIT-11
func TestVSCodeAgentHooks_InitStealthInstallAndToolArgs(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("PATH", t.TempDir())

	if err := os.MkdirAll(filepath.Join(homeDir, tui.CopilotDirToken), 0755); err != nil {
		t.Fatalf("mkdir ~/.copilot: %v", err)
	}
	repoDir := t.TempDir()
	detected, err := resolveInitAgents(repoDir, "")
	if err != nil || !reflect.DeepEqual(detected, []string{agentVSCode}) {
		t.Fatalf("TC-INIT-11: expected [vscode] from ~/.copilot, got %v (err=%v)", detected, err)
	}
	if err := scaffoldAmbientHooksForSelection(repoDir, []string{agentVSCode}); err != nil {
		t.Fatalf("scaffold vscode hooks: %v", err)
	}
	assertVSCodeHookManifestAndDrift(t, repoDir)
	assertVSCodeStealthAndToolArgsPayload(t)
}

func assertVSCodeHookManifestAndDrift(t *testing.T, repoDir string) {
	t.Helper()
	manifestPath := tui.ResolveVSCodeManifestPath(false, repoDir)
	raw, rErr := os.ReadFile(manifestPath)
	if rErr != nil {
		t.Fatalf("read .github/hooks/nomos.json: %v", rErr)
	}
	var parsed map[string]any
	if uErr := json.Unmarshal(raw, &parsed); uErr != nil || parsed["version"] != float64(1) {
		t.Fatalf("expected valid version=1 VS Code hook manifest, err=%v raw=%s", uErr, string(raw))
	}
	if !strings.Contains(string(raw), "timeoutSec") || !strings.Contains(string(raw), "PreToolUse") {
		t.Fatalf("expected timeoutSec and PreToolUse in .github/hooks/nomos.json:\n%s", string(raw))
	}
	if !reflect.DeepEqual(detectExistingHookAgents(repoDir), []string{agentVSCode}) {
		t.Fatalf("expected detectExistingHookAgents=[vscode], got %v", detectExistingHookAgents(repoDir))
	}
	if err := os.WriteFile(manifestPath, []byte(`{"version":1,"hooks":{}}`+"\n"), 0644); err != nil {
		t.Fatalf("tamper vscode hooks: %v", err)
	}
	drifted := detectAdapterAndStealthDrift(repoDir)
	if len(drifted) == 0 || drifted[0] != ".github/hooks/nomos.json" {
		t.Fatalf("expected drift on .github/hooks/nomos.json, got %v", drifted)
	}
	userReceipt, uInstErr := executeHookInstall(hookInstallFlags{vscode: true, userScope: true})
	if uInstErr != nil || len(userReceipt.Targets) != 1 || !strings.HasSuffix(userReceipt.Targets[0], filepath.Join(".copilot", "hooks", "nomos.json")) {
		t.Fatalf("unexpected user --vscode install receipt: %+v err=%v", userReceipt, uInstErr)
	}
}

func assertVSCodeStealthAndToolArgsPayload(t *testing.T) {
	t.Helper()
	patterns := localStealthPatterns(false)
	foundVSCodeExclude := false
	for _, pat := range patterns {
		if pat == "/.github/hooks/nomos.json" {
			foundVSCodeExclude = true
		}
		if pat == "/.github/" {
			t.Fatalf("stealth exclude must not hide entire .github/ directory")
		}
	}
	if !foundVSCodeExclude {
		t.Fatalf("expected /.github/hooks/nomos.json in localStealthPatterns: %v", patterns)
	}
	pObj := hookcmd.ParseHookPayload([]byte(`{"hook_event_name":"PreToolUse","toolName":"bash","toolArgs":{"command":"git push --force origin main","filePath":"src/app.ts"}}`))
	if !pObj.IsClaudeEvent || !pObj.HasToolInput || pObj.ToolName != "bash" || pObj.CommandLine != "git push --force origin main" || pObj.TargetFile != "src/app.ts" {
		t.Fatalf("unexpected object toolArgs parse result: %+v", pObj)
	}
	pStr := hookcmd.ParseHookPayload([]byte(`{"hook_event_name":"PreToolUse","toolName":"editFile","toolArgs":"{\"file_path\":\"pkg/main.go\",\"content\":\"package main\"}"}`))
	if !pStr.HasToolInput || pStr.TargetFile != "pkg/main.go" || pStr.WrittenContent != "package main" {
		t.Fatalf("unexpected string toolArgs parse result: %+v", pStr)
	}
}
