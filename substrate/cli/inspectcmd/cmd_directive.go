// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/spf13/cobra"
)

// NewDirectiveCmd constructs the canonical 'nomos directive' command.
func NewDirectiveCmd() *cobra.Command {
	var jsonOutput, showLexicon bool
	var domainFilter, termQuery, lintFile string

	cmd := &cobra.Command{
		Use:   "directive [name]",
		Short: "Inspect JIT phase directives and the NCTE-100 Succinct Action Lexicon",
		Long: `Inspect embedded and local Just-In-Time (JIT) phase directives governing Tier 1 agent operations,
query the 36-term NCTE-100 Succinct Action Lexicon (--lexicon / --term), or audit Markdown specifications (--lint).`,
		Example: tui.HelpDirectiveExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if lintFile != "" {
				return executeDirectiveLint(cmd, lintFile, jsonOutput)
			}
			if showLexicon || termQuery != "" || domainFilter != "" {
				return executeDirectiveLexicon(cmd, domainFilter, termQuery, jsonOutput)
			}
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

	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Output directive or lexicon in machine-readable JSON format")
	cmd.Flags().BoolVar(&showLexicon, "lexicon", false, "Display the 36-term NCTE-100 Succinct Action Lexicon")
	cmd.Flags().StringVar(&domainFilter, "domain", "", "Filter lexicon by steering domain (lifecycle|verification|isolation|memory|steering)")
	cmd.Flags().StringVar(&termQuery, "term", "", "Resolve a canonical term or banned synonym in the NCTE-100 lexicon")
	cmd.Flags().StringVar(&lintFile, "lint", "", "Audit a Markdown file against NCTE-100 sentence, modal, and lexicon rules")
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
	fmt.Fprintln(out, tui.RenderBrandTitle("📋 Nomos Just-In-Time (JIT) Directives"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	fmt.Fprintf(out, "  %s %s:\n\n",
		tui.UiHeaderStyle.Render("Authoritative Directives"),
		tui.UiMutedStyle.Render(fmt.Sprintf("(%d discovered)", len(directives))))

	for _, d := range directives {
		fmt.Fprintf(out, "  %s %s\n", tui.UiSubstrateStyle.Render("•"), tui.UiBoldValueStyle.Render(d.Name))
		if d.Title != "" {
			fmt.Fprintf(out, "    %s : %s\n", tui.PadStyled("Title", 7, tui.UiLabelStyle), tui.UiValueStyle.Render(d.Title))
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
		meta = append(meta, fmt.Sprintf("source: %s", tui.UiSuccessStyle.Render(d.Source)))
		fmt.Fprintf(out, "    %s : %s\n", tui.PadStyled("Details", 7, tui.UiLabelStyle), tui.UiMutedStyle.Render(strings.Join(meta, " · ")))
		if d.Summary != "" {
			fmt.Fprintf(out, "    %s : %s\n", tui.PadStyled("Summary", 7, tui.UiLabelStyle), tui.UiValueStyle.Render(d.Summary))
		}
		fmt.Fprintln(out)
	}

	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	fmt.Fprintln(out, tui.UiFooterStyle.Render("  Run 'nomos directive <name>' to inspect full directive content."))
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
