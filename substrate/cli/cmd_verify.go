// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

// NewVerifyCmd constructs the canonical 'nomos verify' command.
func NewVerifyCmd() *cobra.Command {
	var baseline, allFiles, strict, impactOnly, jsonOutput bool
	var worktreeDir, fastFile, commitRange string

	cmd := &cobra.Command{
		Use:   "verify [path]",
		Short: "Execute automated Definition of Done (DoD) quality gates",
		Long: `Evaluate the 7 statutory Nomos Laws and 21 Invariant Cells across active worktrees.
Zero trust in conversational claims; non-verbal proof via AST and compilation exit code 0.`,
		Example: helpVerifyExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetDir, err := resolveVerifyTargetDir(worktreeDir, args)
			if err != nil {
				return err
			}
			if fastFile != "" {
				return runFastFileRadar(targetDir, fastFile, strict)
			}
			if impactOnly {
				return runImpactMap(targetDir)
			}
			return runVerificationOutput(cmd.OutOrStdout(), targetDir, commitRange, baseline, allFiles, strict, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&baseline, "baseline", false, "Execute baseline verification directly on repository trunk")
	cmd.Flags().BoolVarP(&allFiles, "all", "A", false, "Scan full repository instead of git diff only")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail closed on cognitive debt or minor warnings")
	cmd.Flags().BoolVar(&impactOnly, "impact", false, "Display deterministic 1-hop AST blast-radius & impact map")
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit verification report as structured JSON")
	cmd.Flags().StringVarP(&worktreeDir, "worktree", "w", "", "Target worktree path to evaluate")
	cmd.Flags().StringVar(&fastFile, "fast-file", "", "Evaluate sub-15ms single-file AST & security early-warning radar")
	cmd.Flags().StringVar(&commitRange, "range", "", "Evaluate unattested commits in range (e.g. origin/main..HEAD) for pre-push verification")

	cmd.AddCommand(newVerifyProofCmd())

	return cmd
}

func resolveVerifyTargetDir(worktreeDir string, args []string) (string, error) {
	targetDir, err := os.Getwd()
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed resolving current directory: %v", err)
	}
	if worktreeDir != "" {
		targetDir = worktreeDir
	} else if len(args) > 0 {
		targetDir = args[0]
	} else if treeRoot, tErr := workspace.FindWorkingTreeRoot(targetDir); tErr == nil {
		targetDir = treeRoot
	}
	if absDir, aErr := filepath.Abs(targetDir); aErr == nil {
		targetDir = absDir
	}
	return targetDir, nil
}

func runVerification(targetDir, commitRange string, baseline, allFiles, strict bool) error {
	return runVerificationOutput(os.Stdout, targetDir, commitRange, baseline, allFiles, strict, false)
}

func runVerificationOutput(out io.Writer, targetDir, commitRange string, baseline, allFiles, strict, jsonOutput bool) error {
	repoRoot, err := workspace.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	cleanTarget := filepath.ToSlash(targetDir)
	if abs, err := filepath.Abs(targetDir); err == nil {
		cleanTarget = filepath.ToSlash(abs)
	}
	if !baseline && !strings.Contains(cleanTarget, "/worktrees/") {
		baseline = true
	}
	activeTask := ""
	if !baseline {
		activeTask = resolveWorktreeTaskKey(targetDir)
	}
	ctx := &verify.VerifyContext{
		RepoRoot: repoRoot, WorktreePath: targetDir, TaskID: activeTask,
		CommitRange: strings.TrimSpace(commitRange), IsBaseline: baseline,
		AllFiles: allFiles, Strict: strict, Phase: "REVIEW",
	}

	pipeline := verify.NewPipeline(verify.DefaultBaselineGates())
	report, err := pipeline.Execute(ctx)
	recordVerificationAnomalies(repoRoot, activeTask, "verify", report, err)
	if err != nil {
		return err
	}

	if jsonOutput {
		if data, mErr := json.MarshalIndent(report, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
		}
	} else {
		renderVerificationSummary(report)
		if report.Passed && activeTask != "" {
			renderPhaseTransitionNextActions(out, types.PhaseReview, activeTask, targetDir, repoRoot)
		}
	}
	if !report.Passed {
		recordFailedReportTraps(repoRoot, targetDir, activeTask, report)
		for _, res := range report.GateResults {
			if !res.Passed && res.GateName == "Plane Separation Invariant" {
				nErr := types.NewNomosError(types.ErrCodeCrossPlaneMutation, "%s", res.Message)
				nErr.Remedy = res.Remedy
				return nErr
			}
		}
		return types.NewNomosError(types.ErrCodeInvariant, "verification failed: %d/%d gates passed", report.GatesPassed, report.GatesTotal)
	}
	return nil
}

func renderVerificationSummary(report *verify.PipelineReport) {
	fmt.Println(renderBrandTitle("🛡️  Nomos Deterministic Engineering Substrate · Definition of Done"))
	fmt.Println(uiRule(ruleStandard))
	for _, law := range report.LawReports {
		status := "PASS"
		if !law.Passed {
			status = "FAIL"
		}
		cells := fmt.Sprintf("[%d/%d Cells]", law.PassedCells, law.TotalCells)
		fmt.Printf("  %s %s %s     %s   %s\n",
			uiLabelStyle.Render(fmt.Sprintf("Law %d:", law.Law)),
			padStyled(law.Name, 30, uiBoldValueStyle),
			padStyled(cells, 11, uiMutedStyle),
			styleSemanticBadge(status),
			uiMutedStyle.Render("("+law.Domain+")"))
	}
	fmt.Println(uiRule(ruleStandard))
	if report.Passed {
		fmt.Printf("  %s\n", uiSuccessStyle.Render(
			fmt.Sprintf("✅ All Laws Enforced · %d/%d Gates Satisfied", report.GatesPassed, report.GatesTotal)))
	} else {
		renderFailedGatesBlock(report)
	}
}

func renderFailedGatesBlock(report *verify.PipelineReport) {
	fmt.Printf("  %s\n", uiDangerStyle.Render(
		fmt.Sprintf("❌ Verification Failed · %d/%d Gates Passed", report.GatesPassed, report.GatesTotal)))
	for _, res := range report.GateResults {
		if !res.Passed {
			fmt.Printf("    ⚠️  %s %s\n",
				uiDangerStyle.Render("["+res.GateName+"]"),
				uiValueStyle.Render(res.Message))
			if res.Remedy != "" {
				fmt.Printf("       %s %s\n", uiWarnStyle.Render("Remedy:"), uiValueStyle.Render(res.Remedy))
			}
			if res.ProbeID != "" {
				fmt.Printf("       %s %s\n", uiLabelStyle.Render("Inspect:"), uiCommandStyle.Render("nomos capabilities "+res.ProbeID))
			}
		}
	}
}

// newVerifyProofCmd constructs the 'nomos verify proof' command evaluating the Three Invariant Attestation Vectors.
func newVerifyProofCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "proof [path]",
		Short: "Evaluate the Three Gate Integrity Invariant Vectors across local or external codebases",
		Long: `Execute deterministic gate integrity and invariant attestation across local or external repositories:
1. Mutation Testing Sensitivity (MS = 100%): Injects failure mutants directly into codebase ASTs to prove non-vacuity.
2. Metamorphic Reference Invariance (FPR = 0%): Evaluates clean codebase constructs to prove zero false alarms.
3. AST Structural Anti-Vacuity Audit: Audits all gate Evaluate implementations to eliminate constant-return shims.`,
		Example: helpVerifyProofExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) > 0 {
				target = args[0]
			}
			absTarget, err := filepath.Abs(target)
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed resolving target directory: %v", err)
			}
			report, pErr := verify.EvaluateGateIntegrityAttestation(absTarget)
			if pErr != nil {
				return pErr
			}
			if jsonOutput {
				if data, mErr := json.MarshalIndent(report, "", "  "); mErr == nil {
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				}
			} else {
				renderGateIntegrityAttestationSummary(report)
			}
			if !report.Passed {
				return types.NewNomosError(types.ErrCodeInvariant, "gate integrity invariant attestation failed")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit gate integrity proof report as structured JSON")
	return cmd
}

func renderGateIntegrityAttestationSummary(report *verify.ProofReport) {
	fmt.Println(renderBrandTitle("🛡️  Nomos Gate Integrity & Invariant Attestation (Cell 2.3)"))
	if report.IsCodebaseTargeted {
		fmt.Printf("  %s %s %s\n",
			uiLabelStyle.Render("Target Codebase:"),
			uiBoldValueStyle.Render(report.TargetDir),
			uiMutedStyle.Render(fmt.Sprintf("(%d Go files analyzed)", report.CodebaseFilesScanned)))
	}
	fmt.Println(uiRule(ruleStandard))
	p1Status := "PASS"
	if report.MutationScore < 100.0 {
		p1Status = "FAIL"
	}
	renderProofVectorRow("Vector 1: Mutation Testing Sensitivity (MS)",
		fmt.Sprintf("[%.1f%% · %d/%d Killed]", report.MutationScore, report.MutationKilled, report.MutationTotal),
		p1Status, report.DurationVector1Micros)

	p2Status := "PASS"
	if report.FalsePositiveRate > 0.0 {
		p2Status = "FAIL"
	}
	renderProofVectorRow("Vector 2: Metamorphic Reference Invariance (FPR)",
		fmt.Sprintf("[%.1f%% · %d/%d Passed]", report.FalsePositiveRate, report.ReferenceGatesPass, report.ReferenceGatesTotal),
		p2Status, report.DurationVector2Micros)

	p3Status := "PASS"
	if report.NonVacuousGates < report.TotalGatesEvaluated || report.TotalGatesEvaluated == 0 {
		p3Status = "FAIL"
	}
	renderProofVectorRow("Vector 3: AST Structural Anti-Vacuity Audit",
		fmt.Sprintf("[%d/%d Non-Trivial]", report.NonVacuousGates, report.TotalGatesEvaluated),
		p3Status, report.DurationVector3Micros)
	fmt.Println(uiRule(ruleStandard))
	totalMs := float64(report.DurationTotalMicros) / 1000.0
	if report.Passed {
		fmt.Printf("  %s %s\n",
			uiSuccessStyle.Render("✅ Deterministic Gate Integrity Invariants Satisfied · Exit Code 0"),
			uiMutedStyle.Render(fmt.Sprintf("(Total: %.2f ms)", totalMs)))
	} else {
		fmt.Printf("  %s %s\n",
			uiDangerStyle.Render("❌ Gate Integrity Invariant Attestation Failed"),
			uiMutedStyle.Render(fmt.Sprintf("(Total: %.2f ms)", totalMs)))
	}
}

func renderProofVectorRow(label, metric, status string, durationMicros int64) {
	fmt.Printf("  %s %s %s   %s\n",
		padStyled(label, 50, uiBoldValueStyle),
		padStyled(metric, 22, uiMutedStyle),
		styleSemanticBadge(status),
		uiMutedStyle.Render(fmt.Sprintf("(%.2f ms)", float64(durationMicros)/1000.0)))
}

func resolveWorktreeTaskKey(worktreeDir string) string {
	out, err := workspace.SafeGitCommand(worktreeDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	branch := strings.TrimSpace(string(out))
	if strings.HasPrefix(branch, "feature/") {
		return strings.TrimPrefix(branch, "feature/")
	}
	return ""
}

func runFastFileRadar(targetDir, fastFile string, strict bool) error {
	repoRoot, err := workspace.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	res, err := verify.EvaluateFastFileRadar(repoRoot, fastFile)
	if err != nil || res == nil {
		return err
	}
	for _, b := range res.Breaches {
		fmt.Printf("🚨 %s %s: %s\n",
			uiDangerStyle.Render("[Nomos Radar Breach]"),
			uiBoldValueStyle.Render(res.File),
			uiValueStyle.Render(b))
	}
	for _, w := range res.Warnings {
		fmt.Printf("⚠️  %s %s: %s\n",
			uiWarnStyle.Render("[Nomos Radar >=80% Budget]"),
			uiBoldValueStyle.Render(res.File),
			uiValueStyle.Render(w))
	}
	if strict && len(res.Breaches) > 0 {
		return types.NewNomosError(types.ErrCodeInvariant, "fast-file radar detected %d breach(es) in %s", len(res.Breaches), res.File)
	}
	return nil
}

func runImpactMap(targetDir string) error {
	items := verify.ComputeWorkspaceImpact(targetDir, nil)
	fmt.Println(renderBrandTitle("🧭 Nomos 1-Hop AST Blast-Radius & Impact Map"))
	fmt.Println(uiRule(ruleStandard))
	if len(items) == 0 {
		fmt.Printf("  %s\n", uiMutedStyle.Render("No modified source files detected in workspace."))
		return nil
	}
	for _, item := range items {
		fmt.Printf("  %s %s\n", uiSubstrateStyle.Render("•"), uiValueStyle.Render(item))
	}
	return nil
}

