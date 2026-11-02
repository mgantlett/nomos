// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
	"github.com/spf13/cobra"
)

func executeDirectiveLexicon(cmd *cobra.Command, domainFilter, termQuery string, jsonOutput bool) error {
	out := cmd.OutOrStdout()
	if strings.TrimSpace(termQuery) != "" {
		return renderLexiconLookup(out, termQuery, jsonOutput)
	}

	entries := ncte.CanonicalLexicon()
	if d := strings.ToLower(strings.TrimSpace(domainFilter)); d != "" {
		filtered := make([]ncte.VocabularyEntry, 0, len(entries))
		for _, e := range entries {
			if strings.ToLower(e.Domain) == d {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			err := types.NewNomosError(types.ErrCodeValidationFailed,
				"unknown lexicon domain %q: must be lifecycle, verification, isolation, memory, or steering", domainFilter)
			err.Remedy = "Run 'nomos directive --lexicon' to inspect all 5 succinct steering domains"
			return err
		}
		entries = filtered
	}

	if jsonOutput {
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling lexicon JSON: %v", err)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	renderLexiconTable(out, entries)
	return nil
}

func renderLexiconLookup(out io.Writer, query string, jsonOutput bool) error {
	entry, isSyn := ncte.LookupTerm(query)
	if entry == nil {
		err := types.NewNomosError(types.ErrCodeValidationFailed, "term %q not registered in NCTE-100 lexicon", query)
		err.Remedy = "Run 'nomos directive --lexicon' to view approved canonical terms and banned synonyms"
		return err
	}
	if jsonOutput {
		payload := map[string]any{"query": query, "matched_banned_synonym": isSyn, "entry": entry}
		if data, mErr := json.MarshalIndent(payload, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
		}
		return nil
	}
	fmt.Fprintln(out, renderBrandTitle("📖 NCTE-100 Succinct Action Lexicon Lookup"))
	fmt.Fprintln(out, uiRule(ruleNarrow))
	if isSyn {
		fmt.Fprintln(out, kvLine("Synonym Trap", 15, uiDangerStyle.Render(fmt.Sprintf("%q is banned -> use %q", query, entry.CanonicalTerm))))
	}
	fmt.Fprintln(out, kvLine("Canonical Term", 15, uiBoldValueStyle.Render(entry.CanonicalTerm)))
	fmt.Fprintln(out, kvLine("Domain / POS", 15, fmt.Sprintf("%s · %s", entry.Domain, entry.PartOfSpeech)))
	fmt.Fprintln(out, kvLine("Meaning", 15, entry.ApprovedMeaning))
	fmt.Fprintln(out, kvLine("Banned Synonyms", 15, uiMutedStyle.Render(strings.Join(entry.BannedSynonyms, ", "))))
	if entry.CLICommand != "" {
		fmt.Fprintln(out, kvLine("CLI Binding", 15, uiCommandStyle.Render(entry.CLICommand)))
	}
	fmt.Fprintln(out, uiRule(ruleNarrow))
	return nil
}

func renderLexiconTable(out io.Writer, entries []ncte.VocabularyEntry) {
	fmt.Fprintln(out, renderBrandTitle(fmt.Sprintf("📖 NCTE-100 Succinct Action Lexicon (%d Canonical Terms)", len(entries))))
	fmt.Fprintln(out, uiRule(ruleWide))
	fmt.Fprintf(out, "  %s %s %s %s\n",
		padStyled("TERM", 12, uiHeaderStyle),
		padStyled("DOMAIN", 14, uiHeaderStyle),
		padStyled("POS", 6, uiHeaderStyle),
		uiHeaderStyle.Render("APPROVED MEANING & BANNED SYNONYMS"))
	fmt.Fprintln(out, uiRule(ruleWide))
	for _, e := range entries {
		syns := uiMutedStyle.Render("[avoid: " + strings.Join(e.BannedSynonyms, ", ") + "]")
		fmt.Fprintf(out, "  %s %s %s %s %s\n",
			padStyled(e.CanonicalTerm, 12, uiBoldValueStyle),
			padStyled(e.Domain, 14, uiSubstrateStyle),
			padStyled(e.PartOfSpeech, 6, uiIntentStyle),
			uiValueStyle.Render(e.ApprovedMeaning),
			syns)
	}
	fmt.Fprintln(out, uiRule(ruleWide))
	renderNextActions(out, []types.ActionAffordance{
		{Command: "nomos directive --term <word>", Summary: "Resolve a specific term or banned synonym to its canonical verb/noun"},
		{Command: "nomos directive --lint <markdown-file>", Summary: "Audit a Markdown specification against NCTE-100 bounds"},
	})
}

func executeDirectiveLint(cmd *cobra.Command, targetPath string, jsonOutput bool) error {
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed reading %s: %v", targetPath, err)
	}
	rep, lErr := ncte.NewEngine(nil).LintMarkdown(targetPath, raw)
	if lErr != nil {
		return lErr
	}
	out := cmd.OutOrStdout()
	if jsonOutput {
		if data, mErr := json.MarshalIndent(rep, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
		}
	} else {
		fmt.Fprintln(out, renderBrandTitle("🔍 NCTE-100 Controlled English Audit"))
		fmt.Fprintln(out, kvLine("File", 12, rep.FilePath))
		fmt.Fprintln(out, kvLine("Sentences", 12, fmt.Sprintf("%d (%d words)", rep.TotalSentences, rep.TotalWords)))
		for _, d := range rep.Diagnostics {
			fmt.Fprintf(out, "  %s line %d [%s]: %s\n    Remedy: %s\n",
				uiDangerStyle.Render("✗"), d.Line, d.Code, d.Message, d.Remedy)
		}
	}
	if !rep.Passed {
		return types.NewNomosError(types.ErrCodeInvariant, "NCTE-100 audit detected %d violation(s) in %s", rep.ViolationsCount, targetPath)
	}
	return nil
}
