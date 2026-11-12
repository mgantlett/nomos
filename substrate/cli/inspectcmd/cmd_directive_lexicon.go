// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
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
	fmt.Fprintln(out, tui.RenderBrandTitle("📖 NCTE-100 Succinct Action Lexicon Lookup"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	if isSyn {
		fmt.Fprintln(out, tui.KvLine("Synonym Trap", 15, tui.UiDangerStyle.Render(fmt.Sprintf("%q is banned -> use %q", query, entry.CanonicalTerm))))
	}
	fmt.Fprintln(out, tui.KvLine("Canonical Term", 15, tui.UiBoldValueStyle.Render(entry.CanonicalTerm)))
	fmt.Fprintln(out, tui.KvLine("Domain / POS", 15, fmt.Sprintf("%s · %s", entry.Domain, entry.PartOfSpeech)))
	fmt.Fprintln(out, tui.KvLine("Meaning", 15, entry.ApprovedMeaning))
	fmt.Fprintln(out, tui.KvLine("Banned Synonyms", 15, tui.UiMutedStyle.Render(strings.Join(entry.BannedSynonyms, ", "))))
	if entry.CLICommand != "" {
		fmt.Fprintln(out, tui.KvLine("CLI Binding", 15, tui.UiCommandStyle.Render(entry.CLICommand)))
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	return nil
}

func renderLexiconTable(out io.Writer, entries []ncte.VocabularyEntry) {
	fmt.Fprintln(out, tui.RenderBrandTitle(fmt.Sprintf("📖 NCTE-100 Succinct Action Lexicon (%d Canonical Terms)", len(entries))))
	fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
	fmt.Fprintf(out, "  %s %s %s %s\n",
		tui.PadStyled("TERM", 12, tui.UiHeaderStyle),
		tui.PadStyled("DOMAIN", 14, tui.UiHeaderStyle),
		tui.PadStyled("POS", 6, tui.UiHeaderStyle),
		tui.UiHeaderStyle.Render("APPROVED MEANING & BANNED SYNONYMS"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
	for _, e := range entries {
		syns := tui.UiMutedStyle.Render("[avoid: " + strings.Join(e.BannedSynonyms, ", ") + "]")
		fmt.Fprintf(out, "  %s %s %s %s %s\n",
			tui.PadStyled(e.CanonicalTerm, 12, tui.UiBoldValueStyle),
			tui.PadStyled(e.Domain, 14, tui.UiSubstrateStyle),
			tui.PadStyled(e.PartOfSpeech, 6, tui.UiIntentStyle),
			tui.UiValueStyle.Render(e.ApprovedMeaning),
			syns)
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
	tui.RenderNextActions(out, []types.ActionAffordance{
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
		fmt.Fprintln(out, tui.RenderBrandTitle("🔍 NCTE-100 Controlled English Audit"))
		fmt.Fprintln(out, tui.KvLine("File", 12, rep.FilePath))
		fmt.Fprintln(out, tui.KvLine("Sentences", 12, fmt.Sprintf("%d (%d words)", rep.TotalSentences, rep.TotalWords)))
		for _, d := range rep.Diagnostics {
			fmt.Fprintf(out, "  %s line %d [%s]: %s\n    Remedy: %s\n",
				tui.UiDangerStyle.Render("✗"), d.Line, d.Code, d.Message, d.Remedy)
		}
	}
	if !rep.Passed {
		return types.NewNomosError(types.ErrCodeInvariant, "NCTE-100 audit detected %d violation(s) in %s", rep.ViolationsCount, targetPath)
	}
	return nil
}
