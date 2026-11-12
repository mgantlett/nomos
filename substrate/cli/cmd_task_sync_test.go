package cli

import (
	"encoding/json"
	"github.com/mgantlett/nomos/substrate/cli/diag"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWalkthroughContentAndScrubPipeline(t *testing.T) {
	dir := t.TempDir()
	wtFile := filepath.Join(dir, "walkthrough.md")
	if err := os.WriteFile(wtFile, []byte("Updated file://"+filepath.ToSlash(dir)+"/README.md"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := diag.NormalizeAndScrubWalkthrough(dir, dir, readWalkthroughContent(wtFile))
	if strings.Contains(got, "file://") || !strings.Contains(got, "README.md") {
		t.Fatalf("unexpected scrubbed walkthrough: %s", got)
	}
}

// @verifies TC-SYNC-025
func TestTaskSync_InlineMessageOneTurnChoreLifecycle(t *testing.T) {
	useCompiledNomosInPath(t)
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("NOMOS_IN_TOOLCHAIN_GATE", "")

	repoDir := filepath.Join(t.TempDir(), "aurora-microchore")
	seedZeroSourceBrownfieldRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if chErr := os.Chdir(origDir); chErr != nil {
			t.Logf("restore dir: %v", chErr)
		}
	}()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := runTaskChoreWithPlane("Clamp negative item amounts in checkout", "substrate"); err != nil {
		t.Fatalf("runTaskChoreWithPlane: %v", err)
	}
	assertSyncRejectsConflictingFlags(t, repoDir)
	executeInlineFastSyncAndVerifyReceipt(t, repoDir)
}

func assertSyncRejectsConflictingFlags(t *testing.T, repoDir string) {
	t.Helper()
	walkFile := filepath.Join(repoDir, ".tmp", "conflict.md")
	writeRawFile(t, walkFile, "file summary")
	bothCmd := newTaskSyncCmd()
	bothCmd.SetArgs([]string{"SUB-1", "-F", walkFile, "-m", "inline summary"})
	if err := bothCmd.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected simultaneous -F and -m to fail closed, got: %v", err)
	}
}

func executeInlineFastSyncAndVerifyReceipt(t *testing.T, repoDir string) {
	t.Helper()
	wtDir := filepath.Join(repoDir, "worktrees", "aurora-microchore-SUB-1")
	pyImpl := "\"\"\"Checkout totals.\"\"\"\n\n\ndef total_cents(items: list[int]) -> int:\n    \"\"\"Sum non-negative item cents.\"\"\"\n    return sum(v for v in items if v > 0)\n"
	pyTest := "\"\"\"Tests for checkout totals.\"\"\"\nfrom substrate.src.checkout import total_cents\n\n\ndef test_total_cents_clamps_negatives() -> None:\n    \"\"\"Ignore negative adjustments.\"\"\"\n    assert total_cents([150, -25, 50]) == 200\n"
	writeRawFile(t, filepath.Join(wtDir, "substrate", "src", "checkout.py"), pyImpl)
	writeRawFile(t, filepath.Join(wtDir, "substrate", "src", "test_checkout.py"), pyTest)

	inlineMsg := "Clamp negative item amounts in total_cents and add regression test"
	syncCmd := newTaskSyncCmd()
	syncCmd.SetArgs([]string{"SUB-1", "-m", inlineMsg})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("1-turn sync -m failed: %v", err)
	}

	if _, stErr := os.Stat(wtDir); !os.IsNotExist(stErr) {
		t.Fatalf("expected worktree %s pruned after 1-turn sync", wtDir)
	}
	noteRaw, nErr := workspace.SafeGitCommand(repoDir, "notes", "--ref", "refs/notes/nomos/releases", "show", "HEAD").Output()
	if nErr != nil {
		t.Fatalf("expected Git Note release receipt on main, got err=%v", nErr)
	}
	var note task.ReleaseNotePayload
	if uErr := json.Unmarshal(noteRaw, &note); uErr != nil {
		t.Fatalf("unmarshal release note: %v", uErr)
	}
	if !strings.Contains(note.WalkthroughContent, inlineMsg) || note.DoDGatesPassed < 42 {
		t.Fatalf("unexpected release receipt payload: %+v", note)
	}
}
