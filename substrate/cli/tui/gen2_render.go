// Package tui implements canonical Lipgloss terminal renderers for Gen-2 xray, prproof, and MTPO diagnostics.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/prproof"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

// RenderXrayTerminal formats an XrayReport using canonical SophiaLabs Lipgloss design tokens.
func RenderXrayTerminal(rep *xray.XrayReport) string {
	if rep == nil {
		return ""
	}
	var sb strings.Builder
	title := fmt.Sprintf("🔬 Nomos Brownfield X-Ray · %s (%dms)", rep.ProjectName, rep.DurationMillis)
	sb.WriteString(RenderBrandTitle(title) + "\n")
	sb.WriteString(UiRule(RuleStandard) + "\n")
	appendXraySummaryLines(&sb, rep)
	appendLipglossHotspots(&sb, rep.Hotspots)
	sb.WriteString(UiRule(RuleStandard) + "\n")
	nextSummary := "Freeze existing brownfield debt into a Day-0 ratchet baseline"
	if rep.Initialized {
		nextSummary = "Evaluate all 42 deterministic Definition of Done gates across the repository"
	}
	RenderNextActions(&sb, []types.ActionAffordance{
		{Command: rep.NextCommand, Summary: nextSummary},
	})
	return sb.String()
}

func appendXraySummaryLines(sb *strings.Builder, rep *xray.XrayReport) {
	scoreVal := fmt.Sprintf("%s %s",
		UiBoldValueStyle.Render(fmt.Sprintf("%d/100", rep.HangoverScore)),
		RenderXrayGradeBadge(rep.Grade))
	sb.WriteString(KvLine("AI Hangover Score", 18, scoreVal) + "\n")

	footprint := fmt.Sprintf("%s source files · %s LOC · %s",
		UiBoldValueStyle.Render(fmt.Sprintf("%d", rep.FilesScanned)),
		UiBoldValueStyle.Render(fmt.Sprintf("%d", rep.TotalLOC)),
		UiMutedStyle.Render(xray.FormatLanguageBreakdown(rep.Languages)))
	sb.WriteString(KvLine("Corpus Footprint", 18, footprint) + "\n")

	debt := fmt.Sprintf("%s clone group(s) · %s monolith(s) (>400 LOC) · %s oversized func(s)",
		styleNonZeroCount(rep.CloneGroups, UiWarnStyle),
		styleNonZeroCount(rep.MonolithFiles, UiWarnStyle),
		styleNonZeroCount(rep.OversizedFuncs, UiWarnStyle))
	sb.WriteString(KvLine("Debt Signals", 18, debt) + "\n")

	antiCheat := fmt.Sprintf("%s empty/TODO stub(s) · %s skipped test(s) · %s CWE sink(s)",
		styleNonZeroCount(rep.EmptyStubs, UiWarnStyle),
		styleNonZeroCount(rep.SkippedTests, UiWarnStyle),
		styleNonZeroCount(rep.SecuritySinks, UiDangerStyle))
	sb.WriteString(KvLine("Anti-Cheat Signals", 18, antiCheat) + "\n")

	bounds := fmt.Sprintf("%s zero-violation architecture boundary rule(s) ready",
		UiSuccessStyle.Render(fmt.Sprintf("%d", rep.InferredBoundaries)))
	sb.WriteString(KvLine("Clean Boundaries", 18, bounds) + "\n")
}

func appendLipglossHotspots(sb *strings.Builder, hotspots []xray.XrayHotspot) {
	if len(hotspots) == 0 {
		sb.WriteString(KvLine("Top Hotspots", 18, UiSuccessStyle.Render("none (all scanned files within strict thresholds)")) + "\n")
		return
	}
	sb.WriteString("  " + UiHeaderStyle.Render("Top Hotspots:") + "\n")
	for i, h := range hotspots {
		reasons := strings.Join(h.Reasons, ", ")
		if reasons == "" {
			reasons = fmt.Sprintf("cds=%d", h.CDS)
		}
		rank := UiMutedStyle.Render(fmt.Sprintf("%2d.", i+1))
		fileCol := PadStyled(h.File, 42, UiCommandStyle)
		meta := fmt.Sprintf("[%s · %d LOC · CDS %s · V(G) %d]",
			UiSubstrateStyle.Render(h.Language),
			h.LOC,
			styleHotspotCDS(h.CDS),
			h.MaxComplexity)
		sb.WriteString(fmt.Sprintf("    %s %s %s %s\n", rank, fileCol, meta, UiMutedStyle.Render("("+reasons+")")))
	}
}

// RenderPRProofTerminal renders a PRProofReceipt card using Lipgloss semantic styles.
func RenderPRProofTerminal(receipt *prproof.PRProofReceipt) string {
	if receipt == nil {
		return ""
	}
	var sb strings.Builder
	verdictToken := "PASS"
	if !receipt.Passed {
		verdictToken = "FAIL"
	}
	sb.WriteString(fmt.Sprintf("%s %s\n",
		RenderBrandTitle("🛡️  Nomos Zero-Trust PR Proof Receipt"),
		StyleSemanticBadge(verdictToken)))
	sb.WriteString(UiRule(RuleStandard) + "\n")
	appendPRProofCoreFields(&sb, receipt)
	appendPRProofMutationAndSeal(&sb, receipt)
	sb.WriteString(UiRule(RuleStandard) + "\n")
	return sb.String()
}

func appendPRProofCoreFields(sb *strings.Builder, receipt *prproof.PRProofReceipt) {
	projVal := fmt.Sprintf("%s @ %s %s",
		UiBoldValueStyle.Render(receipt.ProjectName),
		UiCommandStyle.Render(receipt.HeadCommit),
		UiMutedStyle.Render(fmt.Sprintf("(base %s)", receipt.BaseRef)))
	sb.WriteString(KvLine("Project / Head", 18, projVal) + "\n")

	gateStyle := UiSuccessStyle
	if receipt.GatesPassed < receipt.GatesTotal {
		gateStyle = UiDangerStyle
	}
	gateVal := fmt.Sprintf("%s deterministic gates satisfied",
		gateStyle.Render(fmt.Sprintf("%d/%d", receipt.GatesPassed, receipt.GatesTotal)))
	sb.WriteString(KvLine("DoD Gate Posture", 18, gateVal) + "\n")
	sb.WriteString(KvLine("Plane Separation", 18, StylePlane(receipt.PlaneSummary)) + "\n")

	sign := ""
	deltaStyle := UiSuccessStyle
	if receipt.DebtDelta.DeltaScore > 0 {
		sign = "+"
		deltaStyle = UiDangerStyle
	}
	debtVal := fmt.Sprintf("%d -> %d (%s, %s)",
		receipt.DebtDelta.BaseScore,
		receipt.DebtDelta.HeadScore,
		deltaStyle.Render(fmt.Sprintf("%s%d", sign, receipt.DebtDelta.DeltaScore)),
		RenderXrayGradeBadge(receipt.DebtDelta.HeadGrade))
	sb.WriteString(KvLine("X-Ray Debt Delta", 18, debtVal) + "\n")
}

func appendPRProofMutationAndSeal(sb *strings.Builder, receipt *prproof.PRProofReceipt) {
	if receipt.MutationReport != nil {
		rep := receipt.MutationReport
		mutStyle := UiSuccessStyle
		if !rep.Passed {
			mutStyle = UiDangerStyle
		}
		mutVal := fmt.Sprintf("%s killed (%d/%d mutants, %d survived)",
			mutStyle.Render(fmt.Sprintf("%.1f%%", rep.KillScorePct)),
			rep.MutantsKilled, rep.MutantsTotal, rep.MutantsSurvived)
		sb.WriteString(KvLine("Diff Micro-Mutation", 18, mutVal) + "\n")
	}
	sb.WriteString(KvLine("Receipt SHA-256", 18, UiMutedStyle.Render(receipt.ReceiptSHA256)) + "\n")
	if receipt.MutationReport != nil && len(receipt.MutationReport.Survivors) > 0 {
		appendLipglossMutantSurvivors(sb, receipt.MutationReport.Survivors)
	}
}

// RenderDiffMutationTerminal renders a standalone --mutate-diff report with Lipgloss emphasis.
func RenderDiffMutationTerminal(rep *prproof.DiffMutationReport) string {
	if rep == nil {
		return ""
	}
	var sb strings.Builder
	verdict := "PASS"
	scoreStyle := UiSuccessStyle
	if !rep.Passed {
		verdict = "FAIL"
		scoreStyle = UiDangerStyle
	}
	sb.WriteString(fmt.Sprintf("%s %s · %s (%d/%d mutants vs %s in %dms)\n",
		RenderBrandTitle("🧬 Nomos Diff Micro-Mutation Oracle"),
		StyleSemanticBadge(verdict),
		scoreStyle.Render(fmt.Sprintf("%.1f%% Killed", rep.KillScorePct)),
		rep.MutantsKilled, rep.MutantsTotal,
		UiCommandStyle.Render(rep.BaseRef),
		rep.DurationMillis))
	if len(rep.Survivors) > 0 {
		appendLipglossMutantSurvivors(&sb, rep.Survivors)
	}
	return sb.String()
}

// RenderMTPOAdvisories styles MTPO conflict advisories with Lipgloss hemisphere and hazard tokens.
func RenderMTPOAdvisories(conflicts []mtpo.MTPOConflict) string {
	if len(conflicts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		kindBadge := UiWarnStyle.Render("[" + c.HazardKind + "]")
		if strings.Contains(strings.ToUpper(c.HazardKind), "WRITE") {
			kindBadge = UiDangerStyle.Render("[" + c.HazardKind + "]")
		}
		line := fmt.Sprintf("⚠️  %s Trajectory %s (%s) overlaps predecessor %s (%s) %s files=%v symbols=%v",
			UiWarnStyle.Render("[Nomos MTPO Advisory]"),
			StyleTaskKey(c.LaterTaskKey),
			UiCommandStyle.Render(fmt.Sprintf("seq #%d", c.LaterSeq)),
			StyleTaskKey(c.EarlierTaskKey),
			UiCommandStyle.Render(fmt.Sprintf("seq #%d", c.EarlierSeq)),
			kindBadge,
			c.OverlappingFiles,
			c.OverlappingSymbols)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func appendLipglossMutantSurvivors(sb *strings.Builder, survivors []prproof.DiffMutant) {
	for _, s := range survivors {
		loc := UiCommandStyle.Render(fmt.Sprintf("%s:%d", s.File, s.Line))
		fn := UiBoldValueStyle.Render(s.Function)
		mut := UiDangerStyle.Render(fmt.Sprintf("%q -> %q", s.OriginalOp, s.MutatedOp))
		sb.WriteString(fmt.Sprintf("  ⚠️  [%s] %s (%s) operator %s survived package tests\n",
			UiWarnStyle.Render(s.ID), loc, fn, mut))
	}
}

// RenderXrayGradeBadge colors A/B/C/D-F grades using semantic Lipgloss tokens.
func RenderXrayGradeBadge(grade string) string {
	g := strings.ToUpper(strings.TrimSpace(grade))
	switch {
	case strings.HasPrefix(g, "A"):
		return UiSuccessStyle.Render("Grade " + grade)
	case strings.HasPrefix(g, "B"):
		return UiSubstrateStyle.Render("Grade " + grade)
	case strings.HasPrefix(g, "C"):
		return UiWarnStyle.Render("Grade " + grade)
	default:
		return UiDangerStyle.Render("Grade " + grade)
	}
}

func styleNonZeroCount(n int, nonZeroStyle lipgloss.Style) string {
	if n == 0 {
		return UiSuccessStyle.Render("0")
	}
	return nonZeroStyle.Render(fmt.Sprintf("%d", n))
}

func styleHotspotCDS(cds int) string {
	if cds >= 60 {
		return UiDangerStyle.Render(fmt.Sprintf("%d", cds))
	}
	if cds >= 30 {
		return UiWarnStyle.Render(fmt.Sprintf("%d", cds))
	}
	return UiBoldValueStyle.Render(fmt.Sprintf("%d", cds))
}
