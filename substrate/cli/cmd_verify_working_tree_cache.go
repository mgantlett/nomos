// Package cli implements temporary-index working-tree SHA hashing and receipt reuse
// across unstaged 'nomos verify' and pre-commit hook invocations.
// @intent intent.md task_lifecycle_cli_spec.md
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

// loadValidWorkingTreeVerifyReceipt returns a cached PipelineReport when the current
// working tree (clean, staged, or unstaged) and trunk HEAD SHA match a fresh receipt.
func loadValidWorkingTreeVerifyReceipt(repoRoot, worktreePath, taskKey string, strict bool, now time.Time) *verify.PipelineReport {
	if os.Getenv(envDisableVerifyReceiptCache) == "1" {
		return nil
	}
	canonRepo, canonWT, ok := normalizeReceiptScopePaths(repoRoot, worktreePath)
	if !ok || hasPartiallyStagedIndex(canonWT) {
		return nil
	}
	liveTreeSHA := resolveStagedOrCleanWorktreeTreeSHA(canonWT)
	trunkSHA := resolveTrunkHeadSHA(canonRepo)
	if liveTreeSHA == "" || trunkSHA == "" {
		return nil
	}
	rec := readStoredWorktreeVerifyReceipt(verifyReceiptCachePath(canonRepo, canonWT))
	if strict && (rec == nil || !rec.Strict) {
		return nil
	}
	if !matchesReceiptInvariants(rec, canonRepo, canonWT, taskKey, liveTreeSHA, trunkSHA, now) {
		return nil
	}
	return rec.Report
}

func tryRenderCachedVerifyReceipt(out io.Writer, ctx *vscope.VerifyContext, jsonOutput bool) bool {
	if ctx == nil || ctx.IsBaseline || ctx.AllFiles || strings.TrimSpace(ctx.CommitRange) != "" {
		return false
	}
	cached := loadValidWorkingTreeVerifyReceipt(ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, ctx.Strict, time.Now())
	if cached == nil {
		return false
	}
	if jsonOutput {
		if data, mErr := json.MarshalIndent(cached, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
		}
		return true
	}
	renderVerificationSummary(cached)
	fmt.Printf("  %s\n", tui.UiMutedStyle.Render("⚡ Reused verified working-tree receipt cache"))
	if cached.Passed && ctx.TaskID != "" {
		tui.RenderPhaseTransitionNextActions(out, types.PhaseReview, ctx.TaskID, ctx.WorktreePath, ctx.RepoRoot)
	}
	return true
}

func resolveUnstagedWorkingTreeSHA(worktreeDir string) string {
	tmpDir, err := os.MkdirTemp("", "nomos-wt-idx-*")
	if err != nil {
		return ""
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpDir); rmErr != nil && !os.IsNotExist(rmErr) {
			return
		}
	}()
	idxEnv := "GIT_INDEX_FILE=" + filepath.Join(tmpDir, "index")

	readCmd := workspace.SafeGitCommand(worktreeDir, "read-tree", constants.ValHead)
	readCmd.Env = append(os.Environ(), idxEnv)
	if _, rErr := readCmd.CombinedOutput(); rErr != nil {
		// Repository without HEAD falls through to empty temporary index.
	}

	addCmd := workspace.SafeGitCommand(worktreeDir, "add", "-A")
	addCmd.Env = append(os.Environ(), idxEnv)
	if _, aErr := addCmd.CombinedOutput(); aErr != nil {
		return ""
	}

	writeCmd := workspace.SafeGitCommand(worktreeDir, "write-tree")
	writeCmd.Env = append(os.Environ(), idxEnv)
	treeOut, wErr := writeCmd.Output()
	if wErr != nil {
		return ""
	}
	return normalizeGitOID(string(treeOut))
}

func hasPartiallyStagedIndex(worktreeDir string) bool {
	out, err := workspace.SafeGitCommand(worktreeDir, "status", "--porcelain", "-z", "-uall").Output()
	if err != nil || len(out) == 0 {
		return false
	}
	sawStaged, sawUnstaged := false, false
	tokens := strings.Split(string(out), "\x00")
	for i := 0; i < len(tokens); i++ {
		st, un, extra := classifyPorcelainEntry(tokens[i])
		sawStaged = sawStaged || st
		sawUnstaged = sawUnstaged || un
		if extra {
			i++
		}
	}
	return sawStaged && sawUnstaged
}

func classifyPorcelainEntry(tok string) (staged, unstaged, extraPath bool) {
	if len(tok) < 4 {
		return false, false, false
	}
	x, y := tok[0], tok[1]
	staged = x != ' ' && x != '?' && x != '!'
	unstaged = (y != ' ' && y != '!') || x == '?'
	extraPath = x == 'R' || x == 'C'
	return staged, unstaged, extraPath
}
