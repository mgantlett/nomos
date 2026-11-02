package ncte_test

import (
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
)

func TestNCTE_SpecComplianceVectors_TV01_To_TV07(t *testing.T) {
	engine := ncte.NewEngine(nil)

	cases := []struct {
		id       string
		input    string
		wantCode ncte.ErrorCode
		wantPass bool
	}{
		{
			id:       "TV-01",
			input:    "Execute baseline verification across all quality gates.",
			wantPass: true,
		},
		{
			id:       "TV-02",
			input:    "The database record was updated by the background agent worker.",
			wantCode: ncte.ErrPassiveVoiceDetected,
			wantPass: false,
		},
		{
			id:       "TV-03",
			input:    "Developers should ideally configure the cache properly before executing heavy queries to avoid degraded system response times.",
			wantCode: ncte.ErrAmbiguousModal,
			wantPass: false,
		},
		{
			id:       "TV-04",
			input:    "Start the transient worktree and compile the substrate binary and run all unit tests.",
			wantCode: ncte.ErrCompoundAction,
			wantPass: false,
		},
		{
			id:       "TV-05",
			input:    "The system initializes the telemetry collector across twelve distinct distributed network nodes during the primary boot sequence phase to ensure optimal log synchronization and cluster discovery.",
			wantCode: ncte.ErrSentenceTooLong,
			wantPass: false,
		},
		{
			id:       "TV-06",
			input:    "Please check the root branch sandbox before submitting your changes.",
			wantCode: ncte.ErrNonSingularTerm,
			wantPass: false,
		},
		{
			id:       "TV-07",
			input:    "Ensure the parser performance is very fast and extremely robust under heavy concurrency.",
			wantCode: ncte.ErrSubjectiveAssertion,
			wantPass: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			rep, err := engine.LintMarkdown(tc.id+".md", []byte(tc.input))
			if err != nil {
				t.Fatalf("unexpected lint error: %v", err)
			}
			if rep.Passed != tc.wantPass {
				t.Fatalf("%s: expected Passed=%v, got Passed=%v (diags=%+v)", tc.id, tc.wantPass, rep.Passed, rep.Diagnostics)
			}
			if !tc.wantPass {
				found := false
				for _, d := range rep.Diagnostics {
					if d.Code == tc.wantCode {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%s: expected violation code %s, got %+v", tc.id, tc.wantCode, rep.Diagnostics)
				}
			}
		})
	}
}

func TestCanonicalLexicon_SingularityAndLookup(t *testing.T) {
	lex := ncte.CanonicalLexicon()
	if len(lex) < 36 {
		t.Fatalf("expected at least 36 canonical terms, got %d", len(lex))
	}
	seenTerms := make(map[string]bool, len(lex))
	for _, entry := range lex {
		if entry.CanonicalTerm == "" || entry.PartOfSpeech == "" || entry.ApprovedMeaning == "" {
			t.Fatalf("incomplete lexicon entry: %+v", entry)
		}
		if seenTerms[entry.CanonicalTerm] {
			t.Fatalf("duplicate canonical term %q violates Law 6", entry.CanonicalTerm)
		}
		seenTerms[entry.CanonicalTerm] = true
	}

	canon, isSyn := ncte.LookupTerm("Verify")
	if canon == nil || isSyn || canon.CanonicalTerm != "Verify" {
		t.Fatalf("expected direct lookup of Verify, got entry=%+v syn=%v", canon, isSyn)
	}
	fromSyn, isSyn := ncte.LookupTerm("sandbox")
	if fromSyn == nil || !isSyn || fromSyn.CanonicalTerm != "Worktree" {
		t.Fatalf("expected synonym 'sandbox' to resolve to Worktree, got entry=%+v syn=%v", fromSyn, isSyn)
	}
}

func TestLintMarkdown_IsolatesFrontmatterCodeFencesAndInlineLiterals(t *testing.T) {
	engine := ncte.NewEngine(nil)
	doc := `---
title: "Should ideally check sandbox in frontmatter without triggering prose rules"
plane: intent
---

# Clean Specification

Execute ` + "`nomos verify --all`" + ` inside the isolated worktree.

` + "```bash\n# Fenced code should ideally check sandbox without failing\nnomos verify\n```" + `
`
	rep, err := engine.LintMarkdown("spec.md", []byte(doc))
	if err != nil {
		t.Fatalf("LintMarkdown failed: %v", err)
	}
	if !rep.Passed {
		t.Fatalf("expected clean spec with frontmatter/fences/inline-code to pass, got %+v", rep.Diagnostics)
	}
}
