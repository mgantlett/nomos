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
	var jsonOutput, showLexicon bool
	var domainFilter, termQuery, lintFile string

	cmd := &cobra.Command{
		Use:   "directive [name]",
		Short: "Inspect JIT phase directives and the NCTE-100 Succinct Action Lexicon",
		Long: `Inspect embedded and local Just-In-Time (JIT) phase directives governing Tier 1 agent operations,
query the 36-term NCTE-100 Succinct Action Lexicon (--lexicon / --term), or audit Markdown specifications (--lint).`,
		Example: helpDirectiveExample,
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

	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Output directive or lexicon in machine-readable JSON format")
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
	fmt.Fprintln(out, renderBrandTitle("📋 Nomos Just-In-Time (JIT) Directives"))
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
