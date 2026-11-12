// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"

	"github.com/spf13/cobra"
)

func renderCapabilitiesUI(cmd *cobra.Command, catalog CapabilitiesCatalog, items []CapabilityItem, filter string) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, tui.RenderBrandTitle("⚡ Nomos Substrate & Intent Capabilities Catalog"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))

	if filter != "" && filter != "all" {
		fmt.Fprintf(out, "  Filtered by type: %s %s\n",
			tui.UiBoldValueStyle.Render(fmt.Sprintf("%q", filter)),
			tui.UiMutedStyle.Render(fmt.Sprintf("(%d matched of %d total)", len(items), catalog.TotalCount)))
	} else {
		fmt.Fprintf(out, "  %s %d %s %s %d %s %s %d %s %s %d %s %s %d %s %s %d\n",
			tui.UiLabelStyle.Render("Total:"), catalog.TotalCount, tui.UiMutedStyle.Render("|"),
			tui.UiLabelStyle.Render("Hooks:"), catalog.HooksCount, tui.UiMutedStyle.Render("|"),
			tui.UiLabelStyle.Render("Directives:"), catalog.DirectivesCount, tui.UiMutedStyle.Render("|"),
			tui.UiLabelStyle.Render("Engines:"), catalog.EnginesCount, tui.UiMutedStyle.Render("|"),
			tui.UiLabelStyle.Render("Gates:"), catalog.GatesCount, tui.UiMutedStyle.Render("|"),
			tui.UiLabelStyle.Render("CLI:"), catalog.CLICount)
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
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	fmt.Fprintln(out, tui.UiFooterStyle.Render("  Tip: Run 'nomos capabilities <id>' for gate/probe remediation, or use --json / --type=<hook|directive|engine|gate|cli>."))
}

func renderSection(cmd *cobra.Command, title string, items []CapabilityItem) {
	if len(items) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s %s:\n", tui.UiHeaderStyle.Render(title), tui.UiMutedStyle.Render(fmt.Sprintf("(%d)", len(items))))
	for _, it := range items {
		statusStr := tui.UiSuccessStyle.Render("[" + it.Status + "]")
		if it.Status == "DISABLED" || it.Status == "UNGROUNDED" {
			statusStr = tui.UiWarnStyle.Render("[" + it.Status + "]")
		}
		inv := it.Invocation
		if inv == "" {
			inv = "-"
		}
		runsStr := ""
		if it.InvocationCount > 0 {
			runsStr = " " + tui.UiMutedStyle.Render(fmt.Sprintf("(%d runs)", it.InvocationCount))
		}
		fmt.Fprintf(out, "    %s %s %s %s%s\n",
			tui.UiSubstrateStyle.Render("•"),
			tui.PadStyled(it.Name, 32, tui.UiBoldValueStyle),
			tui.PadStyled(inv, 26, tui.UiCommandStyle),
			statusStr,
			runsStr)
		if it.Description != "" {
			fmt.Fprintf(out, "      %s\n", tui.UiMutedStyle.Render(it.Description))
		}
	}
}
