package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScaffoldOpenCodePlugin_PreservesUserPluginsAndIdempotentMerge(t *testing.T) {
	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}

	customPluginPath := filepath.Join(pluginsDir, "custom_telemetry.ts")
	customContent := "export const CustomPlugin = async () => ({});\n"
	if err := os.WriteFile(customPluginPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("write custom plugin: %v", err)
	}

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("first executeInit failed: %v", err)
	}
	firstNomosBytes, err := os.ReadFile(filepath.Join(pluginsDir, "nomos.ts"))
	if err != nil {
		t.Fatalf("read scaffolded nomos.ts: %v", err)
	}
	if err := executeInit(tempDir); err != nil {
		t.Fatalf("second executeInit failed: %v", err)
	}
	secondNomosBytes, err := os.ReadFile(filepath.Join(pluginsDir, "nomos.ts"))
	if err != nil {
		t.Fatalf("read second nomos.ts: %v", err)
	}
	if string(firstNomosBytes) != string(secondNomosBytes) {
		t.Fatalf("expected idempotent nomos.ts across repeated nomos init runs")
	}

	gotCustom, err := os.ReadFile(customPluginPath)
	if err != nil || string(gotCustom) != customContent {
		t.Fatalf("expected sibling custom plugin preserved byte-for-byte, got %q (err=%v)", string(gotCustom), err)
	}
}

func TestMergeOpenCodePluginContent_MarkersAndCustomCode(t *testing.T) {
	tmpl, err := DefaultOpenCodePluginTS()
	if err != nil {
		t.Fatalf("DefaultOpenCodePluginTS: %v", err)
	}

	withCustomBanner := "// Team header\n" + openCodePluginStartMarker + "\n// old\n" + openCodePluginEndMarker + "\nexport const Extra = 1;\n"
	merged := string(mergeOpenCodePluginContent([]byte(withCustomBanner), tmpl))
	if !strings.Contains(merged, "// Team header") || !strings.Contains(merged, "export const Extra = 1;") {
		t.Fatalf("expected custom header and footer preserved, got:\n%s", merged)
	}
	if strings.Contains(merged, "// old") || !strings.Contains(merged, "NomosGovernancePlugin") {
		t.Fatalf("expected outdated marker block replaced by current template, got:\n%s", merged)
	}

	unmarkedUserFile := "export const TeamAudit = async () => ({});\n"
	appended := string(mergeOpenCodePluginContent([]byte(unmarkedUserFile), tmpl))
	if !strings.Contains(appended, "TeamAudit") || !strings.Contains(appended, "NomosGovernancePlugin") {
		t.Fatalf("expected unmarked file to retain user export alongside Nomos plugin, got:\n%s", appended)
	}
}

func TestOpenCodeHookPayloadParsingAndSub5msEvaluation(t *testing.T) {
	trunkDir := t.TempDir()
	rawBefore, err := json.Marshal(map[string]any{
		"hook_event_name": "tool.execute.before",
		"sessionID":       "ses_oc_1",
		"cwd":             trunkDir,
		"tool":            "edit",
		"tool_input": map[string]any{
			"filePath": filepath.Join(trunkDir, "app", "models", "post.rb"),
		},
	})
	if err != nil {
		t.Fatalf("marshal before payload: %v", err)
	}

	start := time.Now()
	var outBuf bytes.Buffer
	hookCmd := NewHookCmd()
	hookCmd.SetIn(bytes.NewReader(rawBefore))
	hookCmd.SetOut(&outBuf)
	hookCmd.SetArgs([]string{"pre-tool"})
	if cErr := hookCmd.Execute(); cErr != nil {
		t.Fatalf("pre-tool execute: %v", cErr)
	}
	elapsed := time.Since(start)
	if elapsed > 5*time.Millisecond {
		t.Logf("note: cold in-process pre-tool took %v", elapsed)
	}

	// Warm sub-5ms check on in-process pre-tool dispatch
	warmStart := time.Now()
	p := parseHookPayload(rawBefore)
	dec := EvaluatePreToolHook(p, "")
	warmElapsed := time.Since(warmStart)
	if warmElapsed >= 5*time.Millisecond {
		t.Fatalf("expected EvaluatePreToolHook < 5ms, took %v", warmElapsed)
	}
	if dec.Decision != "deny" || !strings.Contains(dec.Reason, "Hermetic Worktree Violation") {
		t.Fatalf("expected trunk edit via OpenCode filePath to deny, got %+v", dec)
	}

	// Verify apply_patch patchText target extraction
	patchPayload := parseHookPayload([]byte(`{
		"hook_event_name":"tool.execute.before",
		"tool":"apply_patch",
		"cwd":"` + filepath.ToSlash(trunkDir) + `",
		"tool_input":{"patchText":"*** Begin Patch\n*** Update File: app/models/user.rb\n@@\n*** End Patch"}
	}`))
	if patchPayload.TargetFile != "app/models/user.rb" {
		t.Fatalf("expected patchText target app/models/user.rb, got %q", patchPayload.TargetFile)
	}
	if patchDec := EvaluatePreToolHook(patchPayload, ""); patchDec.Decision != "deny" {
		t.Fatalf("expected apply_patch on trunk to deny, got %+v", patchDec)
	}
}

func TestOpenCodePluginNodeRuntimeEndToEnd(t *testing.T) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return
	}
	verOut, vErr := exec.Command(nodeBin, "-e", "process.stdout.write(process.versions.node.split('.')[0])").Output()
	if vErr != nil || strings.TrimSpace(string(verOut)) < "22" {
		return
	}

	env := append(ensureCompiledHookTestEnv(t), "XDG_CACHE_HOME="+t.TempDir())
	repoDir := t.TempDir()
	if iErr := executeInit(repoDir); iErr != nil {
		t.Fatalf("executeInit: %v", iErr)
	}

	driver := `
import { NomosGovernancePlugin } from "../.opencode/plugins/nomos.ts";
const sid = "ses_oc_e2e_" + process.pid;
const plugin = await NomosGovernancePlugin({ directory: process.cwd() });
let blocked = false;
try {
  await plugin["tool.execute.before"](
    { tool: "edit", sessionID: sid, callID: "c1" },
    { args: { filePath: "app/models/order.rb" } }
  );
} catch (err) {
  blocked = String(err?.message || err).includes("Hermetic Worktree Violation");
}
if (!blocked) {
  process.exit(2);
}
const afterOut = { output: "sh: bundle: command not found\nexit code 127" };
await plugin["tool.execute.before"](
  { tool: "bash", sessionID: sid, callID: "c2" },
  { args: { command: "bundle exec rails test" } }
);
await plugin["tool.execute.after"](
  { tool: "bash", sessionID: sid, callID: "c2" },
  afterOut
);
if (!afterOut.output.includes("[Nomos Verified Host State]")) {
  process.exit(3);
}
`
	driverDir := filepath.Join(repoDir, ".tmp")
	if mErr := os.MkdirAll(driverDir, 0755); mErr != nil {
		t.Fatalf("mkdir .tmp: %v", mErr)
	}
	driverPath := filepath.Join(driverDir, "oc_driver.mjs")
	if wErr := os.WriteFile(driverPath, []byte(driver), 0644); wErr != nil {
		t.Fatalf("write driver: %v", wErr)
	}
	cmd := exec.Command(nodeBin, driverPath)
	cmd.Dir = repoDir
	cmd.Env = env
	if out, rErr := cmd.CombinedOutput(); rErr != nil {
		t.Fatalf("OpenCode plugin runtime E2E failed (%v):\n%s", rErr, string(out))
	}

	// Verify GUI PATH fallback (~/.local/bin/nomos) when 'nomos' is absent from PATH.
	guiHome := t.TempDir()
	localBin := filepath.Join(guiHome, ".local", "bin")
	if mErr := os.MkdirAll(localBin, 0755); mErr != nil {
		t.Fatalf("mkdir .local/bin: %v", mErr)
	}
	binBytes, bErr := os.ReadFile(filepath.Join(compiledHookTestBinDir, "nomos"))
	if bErr != nil {
		t.Fatalf("read compiled nomos: %v", bErr)
	}
	if wErr := os.WriteFile(filepath.Join(localBin, "nomos"), binBytes, 0755); wErr != nil {
		t.Fatalf("stage ~/.local/bin/nomos: %v", wErr)
	}
	guiCmd := exec.Command(nodeBin, driverPath)
	guiCmd.Dir = repoDir
	guiCmd.Env = []string{"PATH=" + filepath.Dir(nodeBin), "HOME=" + guiHome, "XDG_CACHE_HOME=" + t.TempDir()}
	if out, gErr := guiCmd.CombinedOutput(); gErr != nil {
		t.Fatalf("OpenCode plugin GUI PATH fallback E2E failed (%v):\n%s", gErr, string(out))
	}
}
