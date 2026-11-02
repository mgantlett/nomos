// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, uiTitleStyle.Render("📋 Nomos Just-In-Time (JIT) Directives"))
	fmt.Fprintln(out, uiRule(ruleNarrow))
	fmt.Fprintf(out, "  %s %s:\n\n",
		uiHeaderStyle.Render("Authoritative Directives"),
		uiMutedStyle.Render(fmt.Sprintf("(%d discovered)", len(directives))))

	for _, d := range directives {
		fmt.Fprintf(out, "  %s %s\n", uiSubstrateStyle.Render("•"), uiBoldValueStyle.Render(d.Name))
		if d.Title != "" {
			fmt.Fprintf(out, "    %s : %s\n", padStyled("Title", 7, uiLabelStyle), uiValueStyle.Render(d.Title))
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
		meta = append(meta, fmt.Sprintf("source: %s", uiSuccessStyle.Render(d.Source)))
		fmt.Fprintf(out, "    %s : %s\n", padStyled("Details", 7, uiLabelStyle), uiMutedStyle.Render(strings.Join(meta, " · ")))
		if d.Summary != "" {
			fmt.Fprintf(out, "    %s : %s\n", padStyled("Summary", 7, uiLabelStyle), uiValueStyle.Render(d.Summary))
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, uiRule(ruleNarrow))
	fmt.Fprintln(out, uiFooterStyle.Render("  Run 'nomos directive <name>' to inspect full directive content."))
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
