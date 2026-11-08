package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookPostTool_ReadSpiralAdvisoryAndMutationReset(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	repoDir := t.TempDir()
	wtDir := filepath.Join(repoDir, "worktrees", "repo-SUB-1")
	srcFile := filepath.Join(wtDir, "pkg", "router.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package pkg\n\nfunc Route() int { return 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	sendRead := func() string {
		return runInternalHook(t, []string{"post-tool"}, map[string]any{
			"session_id":      "sess-pacing-1",
			"hook_event_name": "PostToolUse",
			"cwd":             wtDir,
			"tool_name":       "Read",
			"tool_input":      map[string]any{"file_path": srcFile},
		})
	}

	for i := 1; i <= 11; i++ {
		if out := sendRead(); out != "{}" {
			t.Fatalf("read %d expected silent '{}', got %s", i, out)
		}
	}
	out12 := sendRead()
	if !strings.Contains(out12, "12 consecutive read/search calls without a source edit") {
		t.Fatalf("read 12 expected pacing advisory, got %s", out12)
	}

	for i := 13; i <= 19; i++ {
		if out := sendRead(); out != "{}" {
			t.Fatalf("read %d expected deduplicated '{}', got %s", i, out)
		}
	}
	out20 := sendRead()
	if !strings.Contains(out20, "20 consecutive read/search calls without a source edit") {
		t.Fatalf("read 20 expected urgent pacing advisory, got %s", out20)
	}

	// Running tests does not trigger or reset read spiral on its own, while editing resets it.
	writeOut := runInternalHook(t, []string{"post-tool"}, map[string]any{
		"session_id":      "sess-pacing-1",
		"hook_event_name": "PostToolUse",
		"cwd":             wtDir,
		"tool_name":       "Write",
		"tool_input":      map[string]any{"file_path": srcFile, "content": "package pkg\n\nfunc Route() int { return 2 }\n"},
	})
	if strings.Contains(writeOut, "Nomos Pacing Advisory") {
		t.Fatalf("did not expect pacing advisory on Write call, got %s", writeOut)
	}

	for i := 1; i <= 11; i++ {
		if out := sendRead(); out != "{}" {
			t.Fatalf("post-reset read %d expected silent '{}', got %s", i, out)
		}
	}
	if out := sendRead(); !strings.Contains(out, "12 consecutive read/search calls") {
		t.Fatalf("post-reset read 12 expected fresh pacing advisory, got %s", out)
	}

	// Outside a task worktree (trunk read exploration), never emit pacing advisory.
	for i := 1; i <= 15; i++ {
		trunkOut := runInternalHook(t, []string{"post-tool"}, map[string]any{
			"session_id":      "sess-trunk-1",
			"hook_event_name": "PostToolUse",
			"cwd":             repoDir,
			"tool_name":       "Grep",
			"tool_input":      map[string]any{"pattern": "Route"},
		})
		if trunkOut != "{}" {
			t.Fatalf("trunk read %d expected silent '{}', got %s", i, trunkOut)
		}
	}
}
