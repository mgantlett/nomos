// Package cli verifies one-time JIT directory-scoped Mneme pitfall injection
// and session-keyed idle-trunk read-spiral pacing advisories (TC-MNM-030).
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

// seedScopedRepoPitfallForTest inserts an active directory-scoped pitfall record in Mneme.
func seedScopedRepoPitfallForTest(t *testing.T, repoID, scope, content, taskID string) int {
	t.Helper()
	store, err := mem.NewStore("")
	if err != nil {
		t.Fatalf("open mneme store: %v", err)
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			t.Errorf("close mneme store: %v", cErr)
		}
	}()
	rec := &mem.Memory{
		RepoID:  repoID,
		Kind:    mem.KindPitfall,
		Topic:   "pitfall:cognitive_complexity_invariant:" + scope,
		Content: content,
		TaskID:  taskID,
		Status:  mem.StatusActive,
	}
	if sErr := store.Save(rec); sErr != nil {
		t.Fatalf("save pitfall: %v", sErr)
	}
	return rec.ID
}

// writeValidGoWorktreeFile creates a minimal compilable Go source file inside a test worktree.
func writeValidGoWorktreeFile(t *testing.T, path, pkgName, fnName string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	src := "// Package " + pkgName + " provides test helpers.\npackage " + pkgName + "\n\n// " + fnName + " returns one.\nfunc " + fnName + "() int { return 1 }\n"
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// assertPitfallAccessCount verifies the persisted task-deduplicated access_count for memID.
func assertPitfallAccessCount(t *testing.T, memID, want int) {
	t.Helper()
	store, err := mem.NewStore("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			t.Errorf("close store: %v", cErr)
		}
	}()
	all, err := store.ListAll()
	if err != nil {
		t.Fatalf("list store: %v", err)
	}
	for _, m := range all {
		if m.ID == memID {
			if m.AccessCount != want {
				t.Fatalf("expected pitfall %d access_count=%d, got %d", memID, want, m.AccessCount)
			}
			return
		}
	}
	t.Fatalf("pitfall ID %d not found", memID)
}

// @verifies TC-MNM-030
func TestHookPostTool_JITDirectoryPitfallRecallOncePerScope(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoDir := filepath.Join(t.TempDir(), "jit-pitfall-repo")
	wtDir := filepath.Join(repoDir, "worktrees", "jit-pitfall-repo-SUB-2211")
	cliPitfallID := seedScopedRepoPitfallForTest(
		t, "jit-pitfall-repo", "substrate/cli",
		"[Cognitive Complexity Invariant in substrate/cli] Trap: arity 9 exceeds max 8 -> Verified Fix (SUB-2181): split options struct",
		"SUB-2181",
	)
	verifyPitfallID := seedScopedRepoPitfallForTest(
		t, "jit-pitfall-repo", "substrate/verify",
		"[Cognitive Complexity Invariant in substrate/verify] Trap: cyclomatic 17 -> Verified Fix (SUB-2180): extract gate helper",
		"SUB-2180",
	)

	otherFile := filepath.Join(wtDir, "substrate", "other", "clean.go")
	cliFileA := filepath.Join(wtDir, "substrate", "cli", "handler_a.go")
	cliFileB := filepath.Join(wtDir, "substrate", "cli", "handler_b.go")
	verifyFile := filepath.Join(wtDir, "substrate", "verify", "gate.go")
	writeValidGoWorktreeFile(t, otherFile, "other", "Clean")
	writeValidGoWorktreeFile(t, cliFileA, "cli", "HandleA")
	writeValidGoWorktreeFile(t, cliFileB, "cli", "HandleB")
	writeValidGoWorktreeFile(t, verifyFile, "verify", "RunGate")

	sendEdit := func(target string, extraArgs ...string) string {
		args := append([]string{"post-tool"}, extraArgs...)
		return runInternalHook(t, args, map[string]any{
			"session_id": "sess-jit-1", "hook_event_name": "PostToolUse",
			"cwd": wtDir, "tool_name": "Edit",
			"tool_input": map[string]any{"file_path": target},
		})
	}

	if out := sendEdit(otherFile); out != "{}" {
		t.Fatalf("expected silent '{}' on directory without scoped pitfall, got %s", out)
	}
	assertPitfallAccessCount(t, cliPitfallID, 0)
	if out := sendEdit(cliFileA, "--container-sandbox"); out != "{}" {
		t.Fatalf("expected silent '{}' in --container-sandbox mode, got %s", out)
	}

	assertScopedEditSequence(t, wtDir, cliFileA, cliFileB, verifyFile, cliPitfallID, verifyPitfallID, sendEdit)
}

func assertScopedEditSequence(
	t *testing.T,
	wtDir, cliFileA, cliFileB, verifyFile string,
	cliPitfallID, verifyPitfallID int,
	sendEdit func(string, ...string) string,
) {
	t.Helper()
	firstCli := sendEdit(cliFileA)
	if !strings.Contains(firstCli, "Nomos JIT Pitfall · substrate/cli") || !strings.Contains(firstCli, "arity 9 exceeds max 8") {
		t.Fatalf("expected JIT pitfall for substrate/cli on first edit, got %s", firstCli)
	}
	if strings.Contains(firstCli, "substrate/verify") {
		t.Fatalf("did not expect unrelated substrate/verify pitfall on substrate/cli edit: %s", firstCli)
	}
	assertPitfallAccessCount(t, cliPitfallID, 1)

	if secondCli := sendEdit(cliFileB); secondCli != "{}" {
		t.Fatalf("expected silent '{}' on second edit in substrate/cli, got %s", secondCli)
	}
	tr := loadHookSessionTrace(wtDir)
	if tr == nil || !containsTraceItem(tr.RecalledPitfallDirs, "substrate/cli") {
		t.Fatalf("expected substrate/cli recorded in RecalledPitfallDirs, got %+v", tr)
	}

	firstVerify := sendEdit(verifyFile)
	if !strings.Contains(firstVerify, "Nomos JIT Pitfall · substrate/verify") || !strings.Contains(firstVerify, "cyclomatic 17") {
		t.Fatalf("expected JIT pitfall for substrate/verify, got %s", firstVerify)
	}
	assertPitfallAccessCount(t, verifyPitfallID, 1)
	if repeatVerify := sendEdit(verifyFile); repeatVerify != "{}" {
		t.Fatalf("expected silent '{}' on repeat edit in substrate/verify, got %s", repeatVerify)
	}
}

// @verifies TC-MNM-030
func TestHookPostTool_IdleTrunkSessionReadSpiralStandardAndCarbon(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	trunkDir := filepath.Join(t.TempDir(), "governed-trunk")
	if err := os.MkdirAll(trunkDir, 0755); err != nil {
		t.Fatalf("mkdir trunk: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trunkDir, constants.FileNomosYAML), []byte("project: governed-trunk\ndefault_branch: main\n"), 0600); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}

	sendTrunkRead := func(sessID string, extraArgs ...string) string {
		args := append([]string{"post-tool"}, extraArgs...)
		return runInternalHook(t, args, map[string]any{
			"session_id": sessID, "hook_event_name": "PostToolUse",
			"cwd": trunkDir, "tool_name": "Grep",
			"tool_input": map[string]any{"pattern": "Router"},
		})
	}

	for i := 1; i <= 11; i++ {
		if out := sendTrunkRead("sess-trunk-std"); out != "{}" {
			t.Fatalf("standard trunk read %d expected '{}', got %s", i, out)
		}
	}
	warnStd := sendTrunkRead("sess-trunk-std")
	if !strings.Contains(warnStd, "12 consecutive exploratory reads on trunk") || !strings.Contains(warnStd, "nomos task chore") {
		t.Fatalf("expected trunk read-spiral warn at 12, got %s", warnStd)
	}
	if otherOut := sendTrunkRead("sess-trunk-isolated"); otherOut != "{}" {
		t.Fatalf("expected isolated session read 1 to stay '{}', got %s", otherOut)
	}
	assertStandardCritAndReset(t, trunkDir, sendTrunkRead)
	assertCarbonTrunkThresholds(t, sendTrunkRead)
}

func assertStandardCritAndReset(t *testing.T, trunkDir string, sendTrunkRead func(string, ...string) string) {
	t.Helper()
	for i := 13; i <= 19; i++ {
		if out := sendTrunkRead("sess-trunk-std"); out != "{}" {
			t.Fatalf("standard trunk read %d expected '{}', got %s", i, out)
		}
	}
	if critStd := sendTrunkRead("sess-trunk-std"); !strings.Contains(critStd, "20 consecutive exploratory reads on trunk") {
		t.Fatalf("expected urgent trunk read-spiral at 20, got %s", critStd)
	}
	tracesDir := filepath.Join(config.GetNomosHome(), constants.DirCache, pacingTracesSubdir)
	if entries, err := os.ReadDir(tracesDir); err != nil || len(entries) < 2 {
		t.Fatalf("expected session trace files in %s, got entries=%d err=%v", tracesDir, len(entries), err)
	}
	resetOut := runInternalHook(t, []string{"post-tool"}, map[string]any{
		"session_id": "sess-trunk-std", "hook_event_name": "PostToolUse",
		"cwd": trunkDir, "tool_name": "Bash",
		"tool_input": map[string]any{"command": `nomos task chore "fix router"`},
	})
	if resetOut != "{}" {
		t.Fatalf("expected silent '{}' on task chore command, got %s", resetOut)
	}
	if afterReset := sendTrunkRead("sess-trunk-std"); afterReset != "{}" {
		t.Fatalf("expected silent '{}' after trunk task transition reset, got %s", afterReset)
	}
}

func assertCarbonTrunkThresholds(t *testing.T, sendTrunkRead func(string, ...string) string) {
	t.Helper()
	for i := 1; i <= 6; i++ {
		if out := sendTrunkRead("sess-trunk-carbon", "--advisory-profile", "carbon"); out != "{}" {
			t.Fatalf("carbon trunk read %d expected '{}', got %s", i, out)
		}
	}
	if warnCarbon := sendTrunkRead("sess-trunk-carbon", "--advisory-profile", "carbon"); !strings.Contains(warnCarbon, "7 consecutive exploratory reads on trunk") {
		t.Fatalf("expected carbon trunk warn at 7, got %s", warnCarbon)
	}
	for i := 8; i <= 11; i++ {
		if out := sendTrunkRead("sess-trunk-carbon", "--advisory-profile", "carbon"); out != "{}" {
			t.Fatalf("carbon trunk read %d expected '{}', got %s", i, out)
		}
	}
	if critCarbon := sendTrunkRead("sess-trunk-carbon", "--advisory-profile", "carbon"); !strings.Contains(critCarbon, "12 consecutive exploratory reads on trunk") {
		t.Fatalf("expected carbon trunk urgent advisory at 12, got %s", critCarbon)
	}
}

// @verifies TC-MNM-030
func TestHookPostTool_AntigravityBatchJITPitfallAndTrunkPacing(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoDir := filepath.Join(t.TempDir(), "agy-batch-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, constants.FileNomosYAML), []byte("project: agy-batch-repo\ndefault_branch: main\n"), 0600); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	seedScopedRepoPitfallForTest(
		t, "agy-batch-repo", "pkg/engine",
		"[Unchecked Error Hygiene Invariant in pkg/engine] Trap: blank discard -> Verified Fix (SUB-900): check Close()",
		"SUB-900",
	)
	wtDir := filepath.Join(repoDir, "worktrees", "agy-batch-repo-SUB-901")
	engFile := filepath.Join(wtDir, "pkg", "engine", "worker.go")
	writeValidGoWorktreeFile(t, engFile, "engine", "Work")

	queued := runInternalHook(t, []string{"post-tool"}, map[string]any{
		"conversationId": "conv-agy-pitfall-1", "workspacePaths": []string{wtDir}, "stepIdx": 2,
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": engFile}},
	})
	if queued != "{}" {
		t.Fatalf("expected Antigravity PostToolUse to queue silently ('{}'), got %s", queued)
	}
	flushed := runInternalHook(t, []string{"post-tool"}, map[string]any{
		"conversationId": "conv-agy-pitfall-1", "workspacePaths": []string{wtDir}, "invocationNum": 1,
	})
	if !strings.Contains(flushed, `"systemMessage"`) || !strings.Contains(flushed, "Nomos JIT Pitfall · pkg/engine") {
		t.Fatalf("expected PostInvocation flush of JIT pitfall, got %s", flushed)
	}

	assertAntigravityTrunkPacingFlush(t, repoDir)
}

func assertAntigravityTrunkPacingFlush(t *testing.T, repoDir string) {
	t.Helper()
	for i := 1; i <= 7; i++ {
		out := runInternalHook(t, []string{"post-tool", "--advisory-profile", "carbon"}, map[string]any{
			"conversationId": "conv-agy-trunk-1", "workspacePaths": []string{repoDir}, "stepIdx": i,
			"toolCall": map[string]any{"name": "grep_search", "args": map[string]any{"Query": "Work"}},
		})
		if out != "{}" {
			t.Fatalf("expected Antigravity trunk read %d to return '{}', got %s", i, out)
		}
	}
	trunkFlush := runInternalHook(t, []string{"post-tool", "--advisory-profile", "carbon"}, map[string]any{
		"conversationId": "conv-agy-trunk-1", "workspacePaths": []string{repoDir}, "invocationNum": 2,
	})
	if !strings.Contains(trunkFlush, "7 consecutive exploratory reads on trunk") {
		t.Fatalf("expected Antigravity PostInvocation trunk pacing flush, got %s", trunkFlush)
	}
}
