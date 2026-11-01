// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/spf13/cobra"
)

// NewDirectiveCmd constructs the canonical 'nomos directive' command.
func NewDirectiveCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "directive [name]",
		Short: "Inspect authoritative Just-In-Time (JIT) phase and orientation directives",
		Long: `Inspect embedded and local Just-In-Time (JIT) phase directives governing Tier 1 agent operations.
If no directive name is specified, lists all available directives with metadata.
If a directive name or prefix is provided, displays its complete text content.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed resolving working directory: %v", err)
			}
			if rRoot, rErr := workspace.FindRepoRoot(repoRoot); rErr == nil && rRoot != "" {
				repoRoot = rRoot
			}

			if len(args) == 0 || args[0] == "list" {
				return executeDirectiveList(cmd, repoRoot, jsonOutput)
			}

			return executeDirectiveShow(cmd, repoRoot, args[0], jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output directive in machine-readable JSON format")
	return cmd
}

func executeDirectiveList(cmd *cobra.Command, repoRoot string, jsonOutput bool) error {
	directives, err := handshake.ListDirectives(repoRoot)
	if err != nil {
		return err
	}

	if jsonOutput {
		data, mErr := json.MarshalIndent(directives, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling directive JSON: %v", mErr)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38bdf8"))
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f1f5f9"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	footerStyle := lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#64748b"))

	fmt.Fprintln(cmd.OutOrStdout(), titleStyle.Render("📋 Nomos Just-In-Time (JIT) Directives"))
	fmt.Fprintln(cmd.OutOrStdout(), lineStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Fprintf(cmd.OutOrStdout(), "  Authoritative Directives (%d discovered):\n\n", len(directives))

	for _, d := range directives {
		fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", nameStyle.Render(d.Name))
		if d.Title != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    Title   : %s\n", d.Title)
		}
		var meta []string
		if d.Category != "" {
			meta = append(meta, fmt.Sprintf("category: %s", d.Category))
		}
		if len(d.Phases) > 0 {
			var phases []string
			for _, p := range d.Phases {
				phases = append(phases, string(p))
			}
			meta = append(meta, fmt.Sprintf("phases: [%s]", strings.Join(phases, ", ")))
		}
		meta = append(meta, fmt.Sprintf("source: %s", sourceStyle.Render(d.Source)))
		fmt.Fprintf(cmd.OutOrStdout(), "    Details : %s\n", tagStyle.Render(strings.Join(meta, " · ")))
		if d.Summary != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "    Summary : %s\n", d.Summary)
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintln(cmd.OutOrStdout(), lineStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Fprintln(cmd.OutOrStdout(), footerStyle.Render("  Run 'nomos directive <name>' to inspect full directive content."))
	return nil
}

func executeDirectiveShow(cmd *cobra.Command, repoRoot, name string, jsonOutput bool) error {
	content, source, err := handshake.GetDirective(repoRoot, name)
	if err != nil {
		return err
	}

	if jsonOutput {
		resp := map[string]string{
			"name":    name,
			"source":  source,
			"content": string(content),
		}
		data, mErr := json.MarshalIndent(resp, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling directive show JSON: %v", mErr)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	fmt.Fprint(cmd.OutOrStdout(), string(content))
	return nil
}
