// Package prproof builds and renders SHA-256 Pull Request proof receipts.
package prproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

// BuildPRProofReceipt constructs and hashes a portable PR proof receipt for CI and code review.
// @intent nomos_intent_traceability_spec.md #PRProofReceipt
func BuildPRProofReceipt(targetDir string, posture GatePostureSummary, opts PRProofOptions) (*PRProofReceipt, error) {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed resolving repository directory: %w", err)
	}
	baseRef := resolveEffectiveBaseRef(absTarget, opts.BaseRef)
	projectName, intentRoots, substrateRoots := resolveProjectGovernance(absTarget)
	headCommit := resolveShortHeadCommit(absTarget)
	planeSummary := classifyDiffPlaneSummary(absTarget, baseRef, intentRoots, substrateRoots)
	debtDelta, dErr := computePRDebtDelta(absTarget, baseRef)
	if dErr != nil {
		return nil, dErr
	}

	var mutRep *DiffMutationReport
	mutPassed := true
	if opts.MutateDiff {
		var mErr error
		mutRep, mErr = EvaluateDiffMutations(absTarget, baseRef)
		if mErr != nil {
			return nil, mErr
		}
		mutPassed = mutRep.Passed
	}

	receipt := &PRProofReceipt{
		SchemaVersion:  PRProofSchemaVersion,
		ProjectName:    projectName,
		BaseRef:        baseRef,
		HeadCommit:     headCommit,
		GatesPassed:    posture.GatesPassed,
		GatesTotal:     posture.GatesTotal,
		PlaneSummary:   planeSummary,
		DebtDelta:      debtDelta,
		MutationReport: mutRep,
		Passed:         posture.PipelinePass && mutPassed && planeSummary != "cross-plane",
	}
	receipt.ReceiptSHA256 = computeReceiptSHA256(receipt)
	return receipt, nil
}

func resolveProjectGovernance(absTarget string) (string, []string, []string) {
	intentRoots := []string{"intent"}
	substrateRoots := []string{"substrate"}
	projectName := filepath.Base(absTarget)
	cfg, err := config.LoadConfig(filepath.Join(absTarget, constants.FileNomosYAML))
	if err == nil && cfg != nil {
		if strings.TrimSpace(cfg.Project) != "" {
			projectName = strings.TrimSpace(cfg.Project)
		}
		if len(cfg.Governance.QualityGates.IntentSourceRoots) > 0 {
			intentRoots = cfg.Governance.QualityGates.IntentSourceRoots
		}
		if len(cfg.Governance.QualityGates.SubstrateSourceRoots) > 0 {
			substrateRoots = cfg.Governance.QualityGates.SubstrateSourceRoots
		}
	}
	return projectName, intentRoots, substrateRoots
}

func resolveShortHeadCommit(absTarget string) string {
	out, err := workspace.RunSafeGit(absTarget, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "uncommitted"
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "uncommitted"
	}
	return trimmed
}

func classifyDiffPlaneSummary(absTarget, baseRef string, intentRoots, substrateRoots []string) string {
	out, err := workspace.RunSafeGit(absTarget, "diff", "--name-only", baseRef, "--")
	if err != nil {
		return "clean"
	}
	hasIntent, hasSubstrate, hasOther := tallyDiffPlanes(string(out), intentRoots, substrateRoots)
	return summarizePlaneFlags(hasIntent, hasSubstrate, hasOther)
}

func tallyDiffPlanes(rawDiff string, intentRoots, substrateRoots []string) (bool, bool, bool) {
	var hasIntent, hasSubstrate, hasOther bool
	for _, line := range strings.Split(strings.TrimSpace(rawDiff), "\n") {
		f := filepath.ToSlash(strings.TrimSpace(line))
		if f == "" {
			continue
		}
		if matchesAnyRootPrefix(f, intentRoots) || f == constants.FileNomosYAML {
			hasIntent = true
		} else if matchesAnyRootPrefix(f, substrateRoots) {
			hasSubstrate = true
		} else {
			hasOther = true
		}
	}
	return hasIntent, hasSubstrate, hasOther
}

func summarizePlaneFlags(hasIntent, hasSubstrate, hasOther bool) string {
	switch {
	case hasIntent && hasSubstrate:
		return "cross-plane"
	case hasIntent:
		return "intent-only"
	case hasSubstrate:
		return "substrate-only"
	case hasOther:
		return "config-or-ci"
	default:
		return "clean"
	}
}

func matchesAnyRootPrefix(relSlash string, roots []string) bool {
	for _, r := range roots {
		clean := strings.Trim(filepath.ToSlash(strings.TrimSpace(r)), "/")
		if clean == "" {
			continue
		}
		if relSlash == clean || strings.HasPrefix(relSlash, clean+"/") {
			return true
		}
	}
	return false
}

func computePRDebtDelta(absTarget, baseRef string) (PRDebtDelta, error) {
	headRep, err := xray.ScanRepository(absTarget, xray.XrayScanOptions{TopHotspots: 5})
	if err != nil {
		return PRDebtDelta{}, err
	}
	delta := PRDebtDelta{
		BaseRef:   baseRef,
		BaseScore: headRep.HangoverScore,
		HeadScore: headRep.HangoverScore,
		HeadGrade: headRep.Grade,
	}
	if manifest, mErr := xray.LoadBaselineManifest(absTarget); mErr == nil && manifest != nil {
		delta.BaseScore = manifest.HangoverScore
		delta.BaselineManifest = true
	} else if baseScore, ok := scanArchivedBaseScore(absTarget, baseRef); ok {
		delta.BaseScore = baseScore
	}
	delta.DeltaScore = delta.HeadScore - delta.BaseScore
	return delta, nil
}

func scanArchivedBaseScore(absTarget, baseRef string) (int, bool) {
	if !gitRefExists(absTarget, baseRef) {
		return 0, false
	}
	tmpBase, err := os.MkdirTemp("", "nomos-pr-base-*")
	if err != nil {
		return 0, false
	}
	defer func() {
		if rmErr := os.RemoveAll(tmpBase); rmErr != nil && !os.IsNotExist(rmErr) {
			return
		}
	}()
	archiveBytes, aErr := workspace.RunSafeGit(absTarget, "archive", "--format=tar", baseRef)
	if aErr != nil || len(archiveBytes) == 0 {
		return 0, false
	}
	if uErr := untarBytesToDir(archiveBytes, tmpBase); uErr != nil {
		return 0, false
	}
	rep, sErr := xray.ScanRepository(tmpBase, xray.XrayScanOptions{TopHotspots: 1})
	if sErr != nil || rep == nil {
		return 0, false
	}
	return rep.HangoverScore, true
}

func computeReceiptSHA256(receipt *PRProofReceipt) string {
	cp := *receipt
	cp.ReceiptSHA256 = ""
	raw, err := json.Marshal(cp)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// FormatPRProofMarkdown renders a deterministic Markdown PR proof summary table.
// @intent nomos_intent_traceability_spec.md #PRProofReceipt
func FormatPRProofMarkdown(receipt *PRProofReceipt) string {
	if receipt == nil {
		return ""
	}
	var b bytes.Buffer
	verdict := "PASS"
	if !receipt.Passed {
		verdict = "FAIL"
	}
	fmt.Fprintf(&b, "## Nomos Zero-Trust PR Proof Receipt (`%s`)\n\n", verdict)
	fmt.Fprintf(&b, "| Dimension | Result |\n")
	fmt.Fprintf(&b, "| :--- | :--- |\n")
	fmt.Fprintf(&b, "| **Project / Head** | `%s` @ `%s` (base `%s`) |\n", receipt.ProjectName, receipt.HeadCommit, receipt.BaseRef)
	fmt.Fprintf(&b, "| **DoD Gate Posture** | `%d/%d` deterministic gates satisfied |\n", receipt.GatesPassed, receipt.GatesTotal)
	fmt.Fprintf(&b, "| **Plane Separation** | `%s` |\n", receipt.PlaneSummary)
	sign := ""
	if receipt.DebtDelta.DeltaScore > 0 {
		sign = "+"
	}
	fmt.Fprintf(&b, "| **X-Ray Debt Delta** | `%d -> %d` (`%s%d`, Grade `%s`) |\n",
		receipt.DebtDelta.BaseScore, receipt.DebtDelta.HeadScore, sign, receipt.DebtDelta.DeltaScore, receipt.DebtDelta.HeadGrade)
	if receipt.MutationReport != nil {
		fmt.Fprintf(&b, "| **Diff Micro-Mutation** | `%.1f%%` killed (`%d/%d` mutants, `%d` survived) |\n",
			receipt.MutationReport.KillScorePct, receipt.MutationReport.MutantsKilled,
			receipt.MutationReport.MutantsTotal, receipt.MutationReport.MutantsSurvived)
	}
	fmt.Fprintf(&b, "| **Receipt SHA-256** | `%s` |\n", receipt.ReceiptSHA256)
	if receipt.MutationReport != nil && len(receipt.MutationReport.Survivors) > 0 {
		fmt.Fprintf(&b, "\n### Surviving Diff Mutants (Anti-Tautology Failures)\n\n")
		for _, s := range receipt.MutationReport.Survivors {
			fmt.Fprintf(&b, "- `%s` `%s:%d` (`%s`): `%s` -> `%s` survived test suite\n",
				s.ID, s.File, s.Line, s.Function, s.OriginalOp, s.MutatedOp)
		}
	}
	return b.String()
}

// FormatDiffMutationSummary renders a concise CLI summary for standalone --mutate-diff runs.
func FormatDiffMutationSummary(rep *DiffMutationReport) string {
	if rep == nil {
		return ""
	}
	var b strings.Builder
	status := "PASS"
	if !rep.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&b, "🧬 Nomos Diff Micro-Mutation Oracle [%s] · %.1f%% Killed (%d/%d mutants vs %s in %dms)\n",
		status, rep.KillScorePct, rep.MutantsKilled, rep.MutantsTotal, rep.BaseRef, rep.DurationMillis)
	for _, s := range rep.Survivors {
		fmt.Fprintf(&b, "  ⚠️  [%s] %s:%d (%s) operator %q -> %q survived package tests\n",
			s.ID, s.File, s.Line, s.Function, s.OriginalOp, s.MutatedOp)
	}
	return b.String()
}

// RenderVerifyPRAndMutation handles --pr, --pr-output, and standalone --mutate-diff output and verdicts.
func RenderVerifyPRAndMutation(out io.Writer, targetDir string, posture GatePostureSummary, opts PRProofOptions, jsonOutput bool) (bool, error) {
	receipt, err := BuildPRProofReceipt(targetDir, posture, opts)
	if err != nil {
		return false, err
	}
	md := FormatPRProofMarkdown(receipt)
	if strings.TrimSpace(opts.OutputPath) != "" {
		if wErr := os.WriteFile(opts.OutputPath, []byte(md), 0644); wErr != nil {
			return false, fmt.Errorf("failed writing PR proof receipt %s: %w", opts.OutputPath, wErr)
		}
	}
	if jsonOutput {
		encoded, mErr := json.MarshalIndent(receipt, "", "  ")
		if mErr != nil {
			return false, mErr
		}
		fmt.Fprintln(out, string(encoded))
	} else {
		fmt.Fprint(out, md)
	}
	return receipt.Passed, nil
}

