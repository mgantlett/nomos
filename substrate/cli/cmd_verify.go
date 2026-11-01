// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
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
	var baseline bool
	var allFiles bool
	var strict bool
	var impactOnly bool
	var worktreeDir string
	var fastFile string
	var commitRange string

	cmd := &cobra.Command{
		Use:   "verify [path]",
		Short: "Execute automated Definition of Done (DoD) quality gates",
		Long: `Evaluate the 7 statutory Nomos Laws and 21 Invariant Cells across active worktrees.
Zero trust in conversational claims; non-verbal proof via AST and compilation exit code 0.`,
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
			return runVerification(targetDir, commitRange, baseline, allFiles, strict)
		},
	}

	cmd.Flags().BoolVar(&baseline, "baseline", false, "Execute baseline verification directly on repository trunk")
	cmd.Flags().BoolVarP(&allFiles, "all", "A", false, "Scan full repository instead of git diff only")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail closed on cognitive debt or minor warnings")
	cmd.Flags().BoolVar(&impactOnly, "impact", false, "Display deterministic 1-hop AST blast-radius & impact map")
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
		RepoRoot:     repoRoot,
		WorktreePath: targetDir,
		TaskID:       activeTask,
		CommitRange:  strings.TrimSpace(commitRange),
		IsBaseline:   baseline,
		AllFiles:     allFiles,
		Strict:       strict,
		Phase:        "REVIEW",
	}

	pipeline := verify.NewPipeline(verify.DefaultBaselineGates())
	report, err := pipeline.Execute(ctx)
	if err != nil {
		return err
	}

	renderVerificationSummary(report)
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
	fmt.Println("🛡️  Nomos Deterministic Engineering Substrate · Definition of Done")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	for _, law := range report.LawReports {
		status := "PASS"
		if !law.Passed {
			status = "FAIL"
		}
		fmt.Printf("  Law %d: %-30s [%d/%d Cells]     %s   (%s)\n",
			law.Law, law.Name, law.PassedCells, law.TotalCells, status, law.Domain)
	}
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	if report.Passed {
		fmt.Printf("  ✅ All Laws Enforced · %d/%d Gates Satisfied\n", report.GatesPassed, report.GatesTotal)
	} else {
		fmt.Printf("  ❌ Verification Failed · %d/%d Gates Passed\n", report.GatesPassed, report.GatesTotal)
		for _, res := range report.GateResults {
			if !res.Passed {
				fmt.Printf("    ⚠️  [%s] %s\n", res.GateName, res.Message)
				if res.Remedy != "" {
					fmt.Printf("       Remedy: %s\n", res.Remedy)
				}
				if res.ProbeID != "" {
					fmt.Printf("       Inspect: nomos capabilities %s\n", res.ProbeID)
				}
			}
		}
	}
}

// newVerifyProofCmd constructs the 'nomos verify proof' command evaluating the Three Invariant Attestation Vectors.
func newVerifyProofCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "proof [path]",
		Short: "Evaluate the Three Gate Integrity Invariant Vectors across local or external codebases",
		Long: `Execute deterministic gate integrity and invariant attestation across local or external repositories:
1. Mutation Testing Sensitivity (MS = 100%): Injects failure mutants directly into codebase ASTs to prove non-vacuity.
2. Metamorphic Reference Invariance (FPR = 0%): Evaluates clean codebase constructs to prove zero false alarms.
3. AST Structural Anti-Vacuity Audit: Audits all gate Evaluate implementations to eliminate constant-return shims.`,
		Args: cobra.MaximumNArgs(1),
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
			renderGateIntegrityAttestationSummary(report)
			if !report.Passed {
				return types.NewNomosError(types.ErrCodeInvariant, "gate integrity invariant attestation failed")
			}
			return nil
		},
	}
}

func renderGateIntegrityAttestationSummary(report *verify.ProofReport) {
	fmt.Println("🛡️  Nomos Gate Integrity & Invariant Attestation (Cell 2.3)")
	if report.IsCodebaseTargeted {
		fmt.Printf("  Target Codebase: %s (%d Go files analyzed)\n", report.TargetDir, report.CodebaseFilesScanned)
	}
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	p1Status := "PASS"
	if report.MutationScore < 100.0 {
		p1Status = "FAIL"
	}
	fmt.Printf("  Vector 1: Mutation Testing Sensitivity (MS)       [%.1f%% · %d/%d Killed]   %s   (%.2f ms)\n",
		report.MutationScore, report.MutationKilled, report.MutationTotal, p1Status, float64(report.DurationVector1Micros)/1000.0)

	p2Status := "PASS"
	if report.FalsePositiveRate > 0.0 {
		p2Status = "FAIL"
	}
	fmt.Printf("  Vector 2: Metamorphic Reference Invariance (FPR)  [%.1f%% · %d/%d Passed]   %s   (%.2f ms)\n",
		report.FalsePositiveRate, report.ReferenceGatesPass, report.ReferenceGatesTotal, p2Status, float64(report.DurationVector2Micros)/1000.0)

	p3Status := "PASS"
	if report.NonVacuousGates < report.TotalGatesEvaluated || report.TotalGatesEvaluated == 0 {
		p3Status = "FAIL"
	}
	fmt.Printf("  Vector 3: AST Structural Anti-Vacuity Audit        [%d/%d Non-Trivial]     %s   (%.2f ms)\n",
		report.NonVacuousGates, report.TotalGatesEvaluated, p3Status, float64(report.DurationVector3Micros)/1000.0)
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	if report.Passed {
		fmt.Printf("  ✅ Deterministic Gate Integrity Invariants Satisfied · Exit Code 0 (Total: %.2f ms)\n", float64(report.DurationTotalMicros)/1000.0)
	} else {
		fmt.Printf("  ❌ Gate Integrity Invariant Attestation Failed (Total: %.2f ms)\n", float64(report.DurationTotalMicros)/1000.0)
	}
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
		fmt.Printf("🚨 [Nomos Radar Breach] %s: %s\n", res.File, b)
	}
	for _, w := range res.Warnings {
		fmt.Printf("⚠️  [Nomos Radar >=80%% Budget] %s: %s\n", res.File, w)
	}
	if strict && len(res.Breaches) > 0 {
		return types.NewNomosError(types.ErrCodeInvariant, "fast-file radar detected %d breach(es) in %s", len(res.Breaches), res.File)
	}
	return nil
}

func runImpactMap(targetDir string) error {
	items := verify.ComputeWorkspaceImpact(targetDir, nil)
	fmt.Println("🧭 Nomos 1-Hop AST Blast-Radius & Impact Map")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	if len(items) == 0 {
		fmt.Println("  No modified source files detected in workspace.")
		return nil
	}
	for _, item := range items {
		fmt.Printf("  • %s\n", item)
	}
	return nil
}

