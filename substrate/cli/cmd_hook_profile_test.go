// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"strings"
	"testing"
	"time"
)

// TestResolveHookAdvisoryProfile_ClampingAndEnvFallbacks verifies TV-LRN-12 bounds and env overrides.
func TestResolveHookAdvisoryProfile_ClampingAndEnvFallbacks(t *testing.T) {
	def := resolveHookAdvisoryProfile(0, 0, 0, "")
	if def.WarnPct != 75 || def.CritPct != 90 || def.LoopStrikes != 3 || def.AdvisoryStyle != "standard" {
		t.Fatalf("unexpected default advisory profile: %+v", def)
	}

	low := resolveHookAdvisoryProfile(10, 30, 1, "unknown")
	if low.WarnPct != 25 || low.CritPct != 50 || low.LoopStrikes != 2 || low.AdvisoryStyle != "standard" {
		t.Fatalf("expected low-bound clamping (25, 50, 2, standard), got %+v", low)
	}

	high := resolveHookAdvisoryProfile(95, 92, 50, "TERSE")
	if high.WarnPct != 90 || high.CritPct != 95 || high.LoopStrikes != 10 || high.AdvisoryStyle != "terse" {
		t.Fatalf("expected high-bound clamping (90, 95, 10, terse), got %+v", high)
	}

	t.Setenv("NOMOS_BUDGET_WARN_PCT", "65")
	t.Setenv("NOMOS_BUDGET_CRIT_PCT", "85")
	t.Setenv("NOMOS_LOOP_STRIKES", "4")
	t.Setenv("NOMOS_ADVISORY_STYLE", "terse")
	envProf := resolveHookAdvisoryProfile(0, 0, 0, "")
	if envProf.WarnPct != 65 || envProf.CritPct != 85 || envProf.LoopStrikes != 4 || envProf.AdvisoryStyle != "terse" {
		t.Fatalf("expected env fallback profile (65, 85, 4, terse), got %+v", envProf)
	}
}

// TestHookAdvisoryProfile_TV_LRN_12_EndToEndAndHardGateInvariance verifies CLI flags and Law 3/7 invariance.
func TestHookAdvisoryProfile_TV_LRN_12_EndToEndAndHardGateInvariance(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	sandboxDir := t.TempDir()

	// 1. Custom 65% warn / 85% crit budget checkpoints + terse advisory style
	p := &HookPayload{SessionID: "sess-prof-budget"}
	statePath := resolveBudgetStateFilePath(p, sandboxDir)
	saveBudgetState(statePath, sessionBudgetState{
		StartedUnix: time.Now().Unix() - 68,
	})

	warnOut := runInternalHook(t, []string{
		"post-tool",
		"--hermetic-dir", sandboxDir,
		"--budget-sec", "100",
		"--budget-warn-pct", "65",
		"--budget-crit-pct", "85",
		"--advisory-style", "terse",
	}, map[string]any{
		"session_id":      "sess-prof-budget",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "echo step"},
		"tool_response":   map[string]any{"stdout": "step\n"},
	})
	if !strings.Contains(warnOut, "[Nomos Budget]") || !strings.Contains(warnOut, "checkpoint working baseline now") {
		t.Fatalf("TV-LRN-12: expected terse 65%% warn checkpoint at 68s/100s, got %s", warnOut)
	}

	st := loadBudgetState(statePath)
	st.StartedUnix = time.Now().Unix() - 86
	saveBudgetState(statePath, st)
	critOut := runInternalHook(t, []string{
		"post-tool",
		"--hermetic-dir", sandboxDir,
		"--budget-sec", "100",
		"--budget-warn-pct", "65",
		"--budget-crit-pct", "85",
		"--advisory-style", "terse",
	}, map[string]any{
		"session_id":      "sess-prof-budget",
		"hook_event_name": "PostToolUse",
		"tool_name":       "Bash",
		"tool_input":      map[string]any{"command": "curl https://example.invalid"},
		"tool_response":   map[string]any{"stderr": "Could not resolve host: example.invalid"},
	})
	if !strings.Contains(critOut, "finalize and run final verification now") || !strings.Contains(critOut, "[Nomos Host] Egress/DNS offline") {
		t.Fatalf("TV-LRN-12: expected terse 85%% crit + terse DNS host advisory, got %s", critOut)
	}

	// 2. Custom --loop-strikes 4 permits 3rd call and denies on 4th call
	loopDir := t.TempDir()
	loopPayload := map[string]any{
		"session_id": "sess-prof-loop",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "false"},
	}
	for attempt := 1; attempt <= 3; attempt++ {
		out := runInternalHook(t, []string{"pre-tool", "--hermetic-dir", loopDir, "--loop-strikes", "4"}, loopPayload)
		if out != "{}" && !strings.Contains(out, `"allow"`) {
			t.Fatalf("TV-LRN-12: expected allow ('{}') on attempt %d with --loop-strikes 4, got %s", attempt, out)
		}
	}
	fourth := runInternalHook(t, []string{"pre-tool", "--hermetic-dir", loopDir, "--loop-strikes", "4"}, loopPayload)
	if !strings.Contains(fourth, `"deny"`) || !strings.Contains(fourth, "repeated 4 consecutive times") {
		t.Fatalf("TV-LRN-12: expected deny on 4th attempt with --loop-strikes 4, got %s", fourth)
	}

	// 3. Hard safety gates remain strictly invariant regardless of advisory flags
	hardDenyRm := runInternalHook(t, []string{
		"pre-tool", "--container-sandbox", "--loop-strikes", "10", "--advisory-style", "terse",
	}, map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "rm -rf /"},
	})
	if !strings.Contains(hardDenyRm, `"deny"`) {
		t.Fatalf("TV-LRN-12: expected hard Law 7 deny on rm -rf / regardless of advisory flags, got %s", hardDenyRm)
	}

	hardDenyVerifier := runInternalHook(t, []string{
		"pre-tool", "--container-sandbox", "--loop-strikes", "10",
	}, map[string]any{
		"tool_name":  "Read",
		"tool_input": map[string]any{"file_path": "/tests/test_outputs.py"},
	})
	if !strings.Contains(hardDenyVerifier, `"deny"`) {
		t.Fatalf("TV-LRN-12: expected hard anti-reward-hacking deny on /tests regardless of advisory flags, got %s", hardDenyVerifier)
	}
}
