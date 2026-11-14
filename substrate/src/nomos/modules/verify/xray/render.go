// Package xray implements ANSI terminal and GitHub Markdown report formatters.
package xray

import (
	"fmt"
	"sort"
	"strings"
)

// FormatTerminal renders a deterministic human-readable XrayReport card for CLI output.
func FormatTerminal(rep *XrayReport) string {
	if rep == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔬 Nomos Brownfield X-Ray · %s (%dms)\n", rep.ProjectName, rep.DurationMillis))
	sb.WriteString(strings.Repeat("─", 76) + "\n")
	sb.WriteString(fmt.Sprintf("  AI Hangover Score : %d/100 (Grade %s)\n", rep.HangoverScore, rep.Grade))
	sb.WriteString(fmt.Sprintf("  Corpus Footprint  : %d source files · %d LOC · %s\n",
		rep.FilesScanned, rep.TotalLOC, FormatLanguageBreakdown(rep.Languages)))
	sb.WriteString(fmt.Sprintf("  Debt Signals      : %d clone group(s) · %d monolith(s) (>400 LOC) · %d oversized func(s)\n",
		rep.CloneGroups, rep.MonolithFiles, rep.OversizedFuncs))
	sb.WriteString(fmt.Sprintf("  Anti-Cheat Signals: %d empty/TODO stub(s) · %d skipped test(s) · %d CWE sink(s)\n",
		rep.EmptyStubs, rep.SkippedTests, rep.SecuritySinks))
	sb.WriteString(fmt.Sprintf("  Clean Boundaries  : %d zero-violation architecture boundary rule(s) ready\n", rep.InferredBoundaries))
	appendTerminalHotspots(&sb, rep.Hotspots)
	sb.WriteString(strings.Repeat("─", 76) + "\n")
	sb.WriteString(fmt.Sprintf("  ◈ Next Action: %s\n", rep.NextCommand))
	return sb.String()
}

func appendTerminalHotspots(sb *strings.Builder, hotspots []XrayHotspot) {
	if len(hotspots) == 0 {
		sb.WriteString("  Top Hotspots      : none (all scanned files within strict thresholds)\n")
		return
	}
	sb.WriteString("  Top Hotspots:\n")
	for i, h := range hotspots {
		reasonSummary := strings.Join(h.Reasons, ", ")
		if reasonSummary == "" {
			reasonSummary = fmt.Sprintf("cds=%d", h.CDS)
		}
		sb.WriteString(fmt.Sprintf("    %2d. %-42s [%s · %d LOC · CDS %d · V(G) %d] (%s)\n",
			i+1, h.File, h.Language, h.LOC, h.CDS, h.MaxComplexity, reasonSummary))
	}
}

// FormatMarkdown renders a shareable GitHub-flavored Markdown report for PRs, issues, or READMEs.
func FormatMarkdown(rep *XrayReport) string {
	if rep == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Nomos Brownfield X-Ray: `%s`\n\n", rep.ProjectName))
	sb.WriteString(fmt.Sprintf("**Hangover Score:** `%d/100` (Grade **%s**) · **Scanned:** `%d` files (`%d` LOC) in `%dms`\n\n",
		rep.HangoverScore, rep.Grade, rep.FilesScanned, rep.TotalLOC, rep.DurationMillis))
	sb.WriteString("| Signal | Count |\n| :--- | ---: |\n")
	sb.WriteString(fmt.Sprintf("| Structural Clone Groups | `%d` |\n", rep.CloneGroups))
	sb.WriteString(fmt.Sprintf("| Monolith Files (`>400` LOC) | `%d` |\n", rep.MonolithFiles))
	sb.WriteString(fmt.Sprintf("| Oversized / Complex Functions | `%d` |\n", rep.OversizedFuncs))
	sb.WriteString(fmt.Sprintf("| Empty / Stub Blocks | `%d` |\n", rep.EmptyStubs))
	sb.WriteString(fmt.Sprintf("| Skipped Tests | `%d` |\n", rep.SkippedTests))
	sb.WriteString(fmt.Sprintf("| Security Sinks (`CWE-89` / `CWE-22`) | `%d` |\n", rep.SecuritySinks))
	sb.WriteString(fmt.Sprintf("| Inferred Clean Architecture Rules | `%d` |\n\n", rep.InferredBoundaries))
	appendMarkdownHotspots(&sb, rep.Hotspots)
	sb.WriteString(fmt.Sprintf("Lock baseline debt with zero upfront cleanup:\n```sh\n%s\n```\n", rep.NextCommand))
	return sb.String()
}

func appendMarkdownHotspots(sb *strings.Builder, hotspots []XrayHotspot) {
	if len(hotspots) == 0 {
		return
	}
	sb.WriteString("### Top Structural Hotspots\n\n")
	sb.WriteString("| File | Language | LOC | CDS | Max Complexity | Triggers |\n")
	sb.WriteString("| :--- | :--- | ---: | ---: | ---: | :--- |\n")
	for _, h := range hotspots {
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%d` | `%d` | `%d` | `%s` |\n",
			h.File, h.Language, h.LOC, h.CDS, h.MaxComplexity, strings.Join(h.Reasons, ", ")))
	}
	sb.WriteString("\n")
}

// FormatLanguageBreakdown formats scanned file counts per language in deterministic descending order.
func FormatLanguageBreakdown(langs map[string]int) string {
	if len(langs) == 0 {
		return "no supported code files"
	}
	keys := make([]string, 0, len(langs))
	for k := range langs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if langs[keys[i]] != langs[keys[j]] {
			return langs[keys[i]] > langs[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, langs[k]))
	}
	return strings.Join(parts, ", ")
}
