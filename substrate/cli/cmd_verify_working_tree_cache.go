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

	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/autofix"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/prproof"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/ratchet"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/sarif"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"github.com/spf13/cobra"
)

type verifyPRFlags struct {
	prMode              bool
	intentBrief         bool
	updateBenchBaseline bool
	mutateDiff          bool
	sarifMode           bool
	fixesMode           bool
	fixMode             bool
	prOutput            string
	baseRef             string
}

func bindVerifyPRFlags(cmd *cobra.Command, prFlags *verifyPRFlags) {
	cmd.Flags().BoolVar(&prFlags.sarifMode, "sarif", false, "Emit OASIS SARIF 2.1.0 JSON report for GitHub Code Scanning")
	cmd.Flags().BoolVar(&prFlags.fixesMode, "fixes", false, "Emit deterministic machine-actionable repair recipes for failed gates (terminal or --json)")
	cmd.Flags().BoolVar(&prFlags.fixMode, "fix", false, "Apply lossless mechanical auto-fixes (gofmt, nomos.schema.json sync) inside worktrees and re-verify")
	cmd.Flags().BoolVar(&prFlags.prMode, "pr", false, "Emit tamper-evident Pull Request proof receipt (Markdown or --json)")
	cmd.Flags().BoolVar(&prFlags.intentBrief, "intent-brief", false, "Emit compact 15-second Architectural Intent Brief decision card (Markdown or --json)")
	cmd.Flags().BoolVar(&prFlags.updateBenchBaseline, "update-bench-baseline", false, "Execute benchmark_ratchet command and refresh baseline metric file")
	cmd.Flags().StringVar(&prFlags.prOutput, "pr-output", "", "Optional file path to write rendered Pull Request proof receipt")
	// @intent nomos_intent_traceability_spec.md
	// @intent tree_sitter_ast_gate_spec.md
	cmd.Flags().BoolVar(&prFlags.mutateDiff, "mutate-diff", false, "Probe modified Go, TypeScript/JS, and Python functions with zero-token AST micro-mutants")
	cmd.Flags().StringVar(&prFlags.baseRef, "base-ref", "", "Git base reference for PR receipt and diff micro-mutation comparison")
}

func (f verifyPRFlags) hasCustomProbe() bool {
	return f.prMode || f.intentBrief || f.mutateDiff || f.sarifMode || f.fixesMode || f.fixMode || strings.TrimSpace(f.prOutput) != ""
}

func runUpdateBenchBaseline(out io.Writer, targetDir string, jsonOutput bool) error {
	cfg, err := config.LoadConfig(targetDir)
	if err != nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "failed loading nomos.yaml: %v", err)
	}
	basePath, metrics, uErr := ratchet.UpdateBenchmarkBaseline(targetDir, cfg.Governance.QualityGates.BenchmarkRatchet)
	if uErr != nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "benchmark baseline update failed: %v", uErr)
	}
	if jsonOutput {
		if encoded, mErr := json.MarshalIndent(map[string]any{"baseline_file": basePath, "metrics": metrics}, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(encoded))
		}
		return nil
	}
	fmt.Fprintf(out, "✅ Updated benchmark ratchet baseline (%d metric(s)) at %s\n", len(metrics), filepath.Base(basePath))
	return nil
}

func finalizeVerificationReport(out io.Writer, ctx *vscope.VerifyContext, jsonOutput bool, report *verify.PipelineReport, prFlags verifyPRFlags) error {
	if prFlags.prMode || strings.TrimSpace(prFlags.prOutput) != "" {
		return executePRProofMode(out, ctx.WorktreePath, report, prFlags, jsonOutput)
	}
	if prFlags.intentBrief {
		return executeIntentBriefMode(out, ctx.WorktreePath, report, prFlags.baseRef, jsonOutput)
	}
	if prFlags.fixesMode || prFlags.fixMode {
		return executeVerifyFixesMode(out, ctx, report, prFlags, jsonOutput)
	}
	if prFlags.sarifMode {
		if err := writeVerifySarifOutput(out, report); err != nil {
			return err
		}
	} else {
		renderStandardVerifyReport(out, ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, jsonOutput, report)
	}
	if !report.Passed {
		return handleFailedVerificationReport(ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, ctx.AllFiles, ctx.IsBaseline, report)
	}
	if prFlags.mutateDiff {
		if err := executeStandaloneMutateDiff(out, ctx.WorktreePath, prFlags.baseRef); err != nil {
			return err
		}
	}
	sanitizeVerifiedWorkspaceFlowTaints(ctx.RepoRoot, ctx.WorktreePath)
	return nil
}

func sanitizeVerifiedWorkspaceFlowTaints(repoRoot, targetDir string) {
	if repoRoot != "" {
		if err := capbac.SanitizeWorkspaceExternalTaints(repoRoot); err != nil {
			return
		}
	}
	if targetDir != "" && filepath.Clean(targetDir) != filepath.Clean(repoRoot) {
		if err := capbac.SanitizeWorkspaceExternalTaints(targetDir); err != nil {
			return
		}
	}
}

func writeVerifySarifOutput(out io.Writer, report *verify.PipelineReport) error {
	raw, err := sarif.MarshalIndent(sarif.FromVerifyReport(report, RootCmdVersion))
	if err != nil {
		return types.NewNomosError(types.ErrCodeCore, "SARIF serialization failed: %v", err)
	}
	if _, wErr := fmt.Fprintln(out, string(raw)); wErr != nil {
		return types.NewNomosError(types.ErrCodeCore, "SARIF write failed: %v", wErr)
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
		md := prproof.FormatPRProofMarkdownForRepo(receipt, targetDir)
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

func executeIntentBriefMode(out io.Writer, targetDir string, report *verify.PipelineReport, baseRef string, jsonOutput bool) error {
	brief := prproof.BuildArchitecturalIntentBrief(targetDir, baseRef, report.Passed)
	if jsonOutput {
		if encoded, mErr := json.MarshalIndent(brief, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(encoded))
		}
	} else {
		fmt.Fprint(out, prproof.FormatArchitecturalIntentBrief(brief))
	}
	if !report.Passed {
		return types.NewNomosError(types.ErrCodeInvariant, "verification failed: %d/%d gates passed", report.GatesPassed, report.GatesTotal)
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

func executeVerifyFixesMode(out io.Writer, ctx *vscope.VerifyContext, report *verify.PipelineReport, prFlags verifyPRFlags, jsonOutput bool) error {
	plan := autofix.BuildVerifyFixPlan(report.Passed, report.GatesPassed, report.GatesTotal, report.GateResults)
	if prFlags.fixMode {
		applied, fixErr := autofix.ApplySafeAutoFixes(ctx.WorktreePath, ctx.IsBaseline, plan)
		if fixErr != nil {
			return fixErr
		}
		if len(applied) > 0 {
			pipeline := verify.NewPipeline(verify.DefaultBaselineGates())
			recheck, rErr := pipeline.Execute(ctx)
			if rErr != nil {
				return rErr
			}
			report = recheck
			plan = autofix.BuildVerifyFixPlan(report.Passed, report.GatesPassed, report.GatesTotal, report.GateResults)
			plan.AppliedFixes = applied
		}
	}
	if jsonOutput {
		encoded, mErr := json.MarshalIndent(plan, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling fix plan: %v", mErr)
		}
		if _, wErr := fmt.Fprintln(out, string(encoded)); wErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed writing fix plan: %v", wErr)
		}
	} else {
		if _, wErr := fmt.Fprint(out, autofix.FormatFixPlanTerminal(plan)); wErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed writing repair checklist: %v", wErr)
		}
	}
	if !report.Passed {
		return handleFailedVerificationReport(ctx.RepoRoot, ctx.WorktreePath, ctx.TaskID, ctx.AllFiles, ctx.IsBaseline, report)
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
	if cached.Passed {
		hookcmd.MarkWorktreeVerificationPassed(ctx.WorktreePath)
	}
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

func recordFailedReportShieldEvents(repoRoot, branch string, report *verify.PipelineReport) {
	if report == nil {
		return
	}
	for _, res := range report.GateResults {
		if res.Passed {
			continue
		}
		mem.RecordShieldEvent(repoRoot, classifyVerifyShieldCategory(res.GateName), res.GateName, branch, res.Message)
	}
}

func classifyVerifyShieldCategory(gateName string) string {
	lower := strings.ToLower(gateName)
	if strings.Contains(lower, "shim") || strings.Contains(lower, "test") || strings.Contains(lower, "tdd") || strings.Contains(lower, "mutation") {
		return mem.ShieldCategoryAntiCheat
	}
	if strings.Contains(lower, "security") || strings.Contains(lower, "secret") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "boundary") || strings.Contains(lower, "dependency") {
		return mem.ShieldCategorySecurityAirlock
	}
	return mem.ShieldCategoryStructuralRatchet
}
