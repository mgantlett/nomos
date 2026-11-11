package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// @verifies TC-HOOK-001
func TestHookAntigravity_PostToolBufferingAndGoSyntaxFlush(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "repo-SUB-2178")
	badGo := filepath.Join(wtDir, "pkg", "broken.go")
	if err := os.MkdirAll(filepath.Dir(badGo), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badGo, []byte("package pkg\nfunc Broken( {\n"), 0600); err != nil {
		t.Fatal(err)
	}

	postToolOut := executeHookCmd(t, []string{"post-tool"}, map[string]any{
		"conversationId": "ec33ebf9-0cba-4100-8142-c61503f6c587",
		"workspacePaths": []string{wtDir},
		"stepIdx":        5,
		"toolCall": map[string]any{
			"name": "write_file",
			"args": map[string]any{"Path": badGo},
		},
	})
	if postToolOut != "{}" {
		t.Fatalf("expected Antigravity PostToolUse to return '{}', got %s", postToolOut)
	}

	postInvokeOut := executeHookCmd(t, []string{"post-tool"}, map[string]any{
		"conversationId": "ec33ebf9-0cba-4100-8142-c61503f6c587",
		"workspacePaths": []string{wtDir},
		"invocationNum":  2,
	})
	if !strings.Contains(postInvokeOut, `"systemMessage"`) || !strings.Contains(postInvokeOut, "broken.go") {
		t.Fatalf("expected PostInvocation to flush buffered Go syntax radar, got %s", postInvokeOut)
	}

	secondInvoke := executeHookCmd(t, []string{"post-tool"}, map[string]any{
		"conversationId": "ec33ebf9-0cba-4100-8142-c61503f6c587",
		"workspacePaths": []string{wtDir},
		"invocationNum":  3,
	})
	if secondInvoke != "{}" {
		t.Fatalf("expected drained PostInvocation queue to return '{}', got %s", secondInvoke)
	}
}

func TestHookAntigravity_StopDecisionContinueAndTerminationReason(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "repo-SUB-2178")
	wtFile := filepath.Join(wtDir, "substrate", "app.go")
	if err := os.MkdirAll(filepath.Dir(wtFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wtFile, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}

	executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"conversationId": "ec33ebf9-0cba-4100-8142-c61503f6c587",
		"workspacePaths": []string{wtDir},
		"toolCall":       map[string]any{"name": "write_file", "args": map[string]any{"Path": wtFile}},
	})

	normalStop := executeHookCmd(t, []string{"stop"}, map[string]any{
		"conversationId":    "ec33ebf9-0cba-4100-8142-c61503f6c587",
		"workspacePaths":    []string{wtDir},
		"terminationReason": "NO_TOOL_CALL",
	})
	if dec := parseDecision(t, normalStop); dec.Decision != "continue" || !strings.Contains(dec.Reason, "nomos verify") {
		t.Fatalf("expected Antigravity NO_TOOL_CALL stop to emit decision='continue', got %+v", dec)
	}

	for _, reason := range []string{"USER_CANCELED", "ERROR", "MAX_INVOCATIONS"} {
		nonNormal := executeHookCmd(t, []string{"stop"}, map[string]any{
			"conversationId":    "ec33ebf9-0cba-4100-8142-c61503f6c587",
			"workspacePaths":    []string{wtDir},
			"terminationReason": reason,
		})
		if nonNormal != "{}" {
			t.Fatalf("expected terminationReason=%s to allow stop ('{}'), got %s", reason, nonNormal)
		}
	}
}

func TestHookPreTool_MultiFileApplyPatchAndCarbonProfile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("NOMOS_ADVISORY_PROFILE", "carbon")

	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "repo-SUB-2178")
	wtFile := filepath.Join(wtDir, "ok.go")
	trunkFile := filepath.Join(repoDir, "substrate", "sneaky.go")
	patch := fmt.Sprintf("*** Begin Patch\n*** Update File: %s\n@@\n*** Update File: %s\n@@\n*** End Patch\n", wtFile, trunkFile)

	outPatch := executeHookCmd(t, []string{"pre-tool"}, map[string]any{
		"tool_name":  "apply_patch",
		"tool_input": map[string]any{"patchText": patch},
	})
	if dec := parseDecision(t, outPatch); dec.Decision != "deny" || !strings.Contains(dec.Reason, "sneaky.go") {
		t.Fatalf("expected multi-file apply_patch targeting trunk in file #2 to be denied, got %+v", dec)
	}

	// Carbon profile trips loop breaker on strike 2 and read spiral on read 7.
	loopDir := workspace.InitHermeticGitRepo(t)
	for i := 0; i < 270; i++ {
		f := filepath.Join(loopDir, fmt.Sprintf("f_%03d.txt", i))
		if err := os.WriteFile(f, []byte("seed\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmdPayload := map[string]any{"cwd": loopDir, "tool_name": "Bash", "tool_input": map[string]any{"command": "false"}}
	if dec := parseDecision(t, executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", loopDir}, cmdPayload)); dec.Decision != "allow" {
		t.Fatalf("expected carbon strike 1 allow, got %+v", dec)
	}
	if dec := parseDecision(t, executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", loopDir}, cmdPayload)); dec.Decision != "deny" {
		t.Fatalf("expected carbon strike 2 deny, got %+v", dec)
	}
	lateFile := filepath.Join(loopDir, "zzz_late.go")
	if err := os.WriteFile(lateFile, []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if dec := parseDecision(t, executeHookCmd(t, []string{"pre-tool", "--hermetic-dir", loopDir}, cmdPayload)); dec.Decision != "allow" {
		t.Fatalf("expected git status digest to detect file #271 change and reset loop breaker, got %+v", dec)
	}
}
