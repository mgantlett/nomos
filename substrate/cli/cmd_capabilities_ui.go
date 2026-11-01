// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	capTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38bdf8"))
	capLineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
	capHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f1f5f9"))
	capNameStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#e2e8f0"))
	capSubStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	capGreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	capYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#eab308"))
)

func renderCapabilitiesUI(cmd *cobra.Command, catalog CapabilitiesCatalog, items []CapabilityItem, filter string) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, capTitleStyle.Render("⚡ Nomos Substrate & Intent Capabilities Catalog"))
	fmt.Fprintln(out, capLineStyle.Render("────────────────────────────────────────────────────────────"))

	if filter != "" && filter != "all" {
		fmt.Fprintf(out, "  Filtered by type: %q (%d matched of %d total)\n", filter, len(items), catalog.TotalCount)
	} else {
		fmt.Fprintf(out, "  Total: %d | Hooks: %d | Directives: %d | Engines: %d | Gates: %d | CLI: %d\n",
			catalog.TotalCount, catalog.HooksCount, catalog.DirectivesCount,
			catalog.EnginesCount, catalog.GatesCount, catalog.CLICount)
	}

	grouped := make(map[string][]CapabilityItem)
	for _, it := range items {
		grouped[it.Type] = append(grouped[it.Type], it)
	}

	renderSection(cmd, "🪝 Ambient Lifecycle Hooks", grouped["hook"])
	renderSection(cmd, "📋 JIT Phase Directives", grouped["directive"])
	renderSection(cmd, "🏛️ Architectural Engines", grouped["engine"])
	renderSection(cmd, "🛡️ Invariant Quality Gates (7x3 DoD)", grouped["gate"])
	renderSection(cmd, "⌨️ Core CLI Commands", grouped["cli"])

	fmt.Fprintln(out)
	fmt.Fprintln(out, capLineStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Fprintln(out, capSubStyle.Render("  Tip: Run 'nomos capabilities <id>' for gate/probe remediation, or use --json / --type=<hook|directive|engine|gate|cli>."))
}

func renderSection(cmd *cobra.Command, title string, items []CapabilityItem) {
	if len(items) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s (%d):\n", capHeaderStyle.Render(title), len(items))
	for _, it := range items {
		statusStr := capGreenStyle.Render("[" + it.Status + "]")
		if it.Status == "DISABLED" || it.Status == "UNGROUNDED" {
			statusStr = capYellowStyle.Render("[" + it.Status + "]")
		}
		inv := it.Invocation
		if inv == "" {
			inv = "-"
		}
		runsStr := ""
		if it.InvocationCount > 0 {
			runsStr = " " + capSubStyle.Render(fmt.Sprintf("(%d runs)", it.InvocationCount))
		}
		fmt.Fprintf(out, "    • %-32s %-26s %s%s\n", capNameStyle.Render(it.Name), capSubStyle.Render(inv), statusStr, runsStr)
		if it.Description != "" {
			fmt.Fprintf(out, "      %s\n", capSubStyle.Render(it.Description))
		}
	}
}
