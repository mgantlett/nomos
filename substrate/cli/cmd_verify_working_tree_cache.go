// Package cli implements temporary-index working-tree SHA hashing, receipt reuse,
// and Pull Request proof / diff micro-mutation verification dispatch.
// @intent intent.md task_lifecycle_cli_spec.md nomos_intent_traceability_spec.md tree_sitter_ast_gate_spec.md
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
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/prproof"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

type verifyPRFlags struct {
	prMode     bool
	mutateDiff bool
	prOutput   string
	baseRef    string
}

func (f verifyPRFlags) hasCustomProbe() bool {
	return f.prMode || f.mutateDiff || strings.TrimSpace(f.prOutput) != ""
}

func finalizeVerificationReport(out io.Writer, ctx *vscope.VerifyContext, jsonOutput bool, report *verify.PipelineReport, prFlags verifyPRFlags) error {
	if prFlags.prMode || strings.TrimSpace(prFlags.prOutput) != "" {
		return executePRProofMode(out, ctx.WorktreePath, report, prFlags, jsonOutput)
	}
	renderStandardVerifyReport(out, ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, jsonOutput, report)
	if !report.Passed {
		return handleFailedVerificationReport(ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, ctx.AllFiles, ctx.IsBaseline, report)
	}
	if prFlags.mutateDiff {
		return executeStandaloneMutateDiff(out, ctx.WorktreePath, prFlags.baseRef)
	}
	return nil
}

func renderStandardVerifyReport(out io.Writer, repoRoot, targetDir, activeTask string, jsonOutput bool, report *verify.PipelineReport) {
	if jsonOutput {
		if data, mErr := json.MarshalIndent(report, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
		}
		return
	}
	renderVerificationSummary(report)
	if report.Passed && activeTask != "" {
		tui.RenderPhaseTransitionNextActions(out, types.PhaseReview, activeTask, targetDir, repoRoot)
	}
}

func executePRProofMode(out io.Writer, targetDir string, report *verify.PipelineReport, prFlags verifyPRFlags, jsonOutput bool) error {
	posture := prproof.GatePostureSummary{
		GatesPassed: report.GatesPassed, GatesTotal: report.GatesTotal, PipelinePass: report.Passed,
	}
	receipt, err := prproof.BuildPRProofReceipt(targetDir, posture, prproof.PRProofOptions{
		BaseRef: prFlags.baseRef, MutateDiff: prFlags.mutateDiff, OutputPath: prFlags.prOutput,
	})
	if err != nil {
		return types.NewNomosError(types.ErrCodeCore, "PR proof generation failed: %v", err)
	}
	if strings.TrimSpace(prFlags.prOutput) != "" {
		md := prproof.FormatPRProofMarkdown(receipt)
		if wErr := os.WriteFile(prFlags.prOutput, []byte(md), 0644); wErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed writing PR proof receipt %s: %v", prFlags.prOutput, wErr)
		}
	}
	if jsonOutput {
		if encoded, mErr := json.MarshalIndent(receipt, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(encoded))
		}
	} else {
		fmt.Fprint(out, tui.RenderPRProofTerminal(receipt))
	}
	if !receipt.Passed {
		return types.NewNomosError(types.ErrCodeInvariant, "PR proof verification failed")
	}
	return nil
}

func executeStandaloneMutateDiff(out io.Writer, targetDir, baseRef string) error {
	mutRep, err := prproof.EvaluateDiffMutations(targetDir, baseRef)
	if err != nil {
		return types.NewNomosError(types.ErrCodeCore, "diff micro-mutation probe failed: %v", err)
	}
	fmt.Fprint(out, tui.RenderDiffMutationTerminal(mutRep))
	if !mutRep.Passed {
		return types.NewNomosError(types.ErrCodeInvariant, "diff micro-mutation oracle detected %d surviving mutant(s)", mutRep.MutantsSurvived)
	}
	return nil
}

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
