// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func renderCapabilitiesUI(cmd *cobra.Command, catalog CapabilitiesCatalog, items []CapabilityItem, filter string) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, renderBrandTitle("⚡ Nomos Substrate & Intent Capabilities Catalog"))
	fmt.Fprintln(out, uiRule(ruleNarrow))

	if filter != "" && filter != "all" {
		fmt.Fprintf(out, "  Filtered by type: %s %s\n",
			uiBoldValueStyle.Render(fmt.Sprintf("%q", filter)),
			uiMutedStyle.Render(fmt.Sprintf("(%d matched of %d total)", len(items), catalog.TotalCount)))
	} else {
		fmt.Fprintf(out, "  %s %d %s %s %d %s %s %d %s %s %d %s %s %d %s %s %d\n",
			uiLabelStyle.Render("Total:"), catalog.TotalCount, uiMutedStyle.Render("|"),
			uiLabelStyle.Render("Hooks:"), catalog.HooksCount, uiMutedStyle.Render("|"),
			uiLabelStyle.Render("Directives:"), catalog.DirectivesCount, uiMutedStyle.Render("|"),
			uiLabelStyle.Render("Engines:"), catalog.EnginesCount, uiMutedStyle.Render("|"),
			uiLabelStyle.Render("Gates:"), catalog.GatesCount, uiMutedStyle.Render("|"),
			uiLabelStyle.Render("CLI:"), catalog.CLICount)
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
	fmt.Fprintln(out, uiRule(ruleNarrow))
	fmt.Fprintln(out, uiFooterStyle.Render("  Tip: Run 'nomos capabilities <id>' for gate/probe remediation, or use --json / --type=<hook|directive|engine|gate|cli>."))
}

func renderSection(cmd *cobra.Command, title string, items []CapabilityItem) {
	if len(items) == 0 {
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s %s:\n", uiHeaderStyle.Render(title), uiMutedStyle.Render(fmt.Sprintf("(%d)", len(items))))
	for _, it := range items {
		statusStr := uiSuccessStyle.Render("[" + it.Status + "]")
		if it.Status == "DISABLED" || it.Status == "UNGROUNDED" {
			statusStr = uiWarnStyle.Render("[" + it.Status + "]")
		}
		inv := it.Invocation
		if inv == "" {
			inv = "-"
		}
		runsStr := ""
		if it.InvocationCount > 0 {
			runsStr = " " + uiMutedStyle.Render(fmt.Sprintf("(%d runs)", it.InvocationCount))
		}
		fmt.Fprintf(out, "    %s %s %s %s%s\n",
			uiSubstrateStyle.Render("•"),
			padStyled(it.Name, 32, uiBoldValueStyle),
			padStyled(inv, 26, uiCommandStyle),
			statusStr,
			runsStr)
		if it.Description != "" {
			fmt.Fprintf(out, "      %s\n", uiMutedStyle.Render(it.Description))
		}
	}
}
