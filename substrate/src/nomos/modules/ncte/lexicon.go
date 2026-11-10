// Package ncte implements the Nomos Controlled Technical English (NCTE-100) lexicon and AST linter.
package ncte

import (
	"strings"
	"time"
)

// DiagnosticSeverity defines fail-closed pre-flight levels (#NCTEPolicy).
type DiagnosticSeverity string

const (
	// SeverityError halts the verification pipeline immediately with exit code 1.
	SeverityError DiagnosticSeverity = "ERROR"

	posVerb = "verb"
	posNoun = "noun"

	// DomainLifecycle groups task and phase transition action terms.
	DomainLifecycle = "lifecycle"
	// DomainVerification groups DoD gate, proof, and receipt terms.
	DomainVerification = "verification"
	// DomainIsolation groups worktree, plane, and physics boundary terms.
	DomainIsolation = "isolation"
	// DomainMemory groups Nomos Mneme recall and distillation terms.
	DomainMemory = "memory"
	// DomainSteering groups cybernetic indirect-control agent steering terms.
	DomainSteering = "steering"
)

// ErrorCode identifies the violated NCTE-100 invariant.
type ErrorCode string

const (
	// ErrSentenceTooLong signals a sentence exceeding descriptive (25) or procedural (20) word bounds.
	ErrSentenceTooLong ErrorCode = "ERR_NCTE_SENTENCE_TOO_LONG"
	// ErrParagraphTooLong signals a paragraph exceeding 6 sentences.
	ErrParagraphTooLong ErrorCode = "ERR_NCTE_PARAGRAPH_TOO_LONG"
	// ErrTableCellTooLong signals a standard table cell exceeding 12 words.
	ErrTableCellTooLong ErrorCode = "ERR_NCTE_TABLE_CELL_TOO_LONG"
	// ErrPassiveVoiceDetected signals auxiliary-be plus past-participle passive syntax.
	ErrPassiveVoiceDetected ErrorCode = "ERR_NCTE_PASSIVE_VOICE"
	// ErrAmbiguousModal signals prohibited modal verbs or equivocal connectors.
	ErrAmbiguousModal ErrorCode = "ERR_NCTE_AMBIGUOUS_MODAL"
	// ErrCompoundAction signals multiple imperative clauses joined by conjunctions.
	ErrCompoundAction ErrorCode = "ERR_NCTE_COMPOUND_ACTION"
	// ErrNonSingularTerm signals a banned synonym replacing a canonical lexicon term.
	ErrNonSingularTerm ErrorCode = "ERR_NCTE_NON_SINGULAR_TERM"
	// ErrSubjectiveAssertion signals unquantified qualitative intensifiers.
	ErrSubjectiveAssertion ErrorCode = "ERR_NCTE_SUBJECTIVE_ASSERTION"
)

// NCTEBounds declares numeric constraints for Markdown prose (#NCTEBounds).
// @intent nomos_controlled_technical_english_spec.md #NCTEBounds
type NCTEBounds struct {
	MaxDescriptiveWords   int `json:"max_descriptive_words"`
	MaxProceduralWords    int `json:"max_procedural_words"`
	MaxParagraphSentences int `json:"max_paragraph_sentences"`
	MaxTableCellWords     int `json:"max_table_cell_words"`
	MaxPassiveAdverbGap   int `json:"max_passive_adverb_gap"`
}

// VocabularyEntry defines a canonical term in the compiled SSoT registry (#VocabularyEntry).
// @intent nomos_controlled_technical_english_spec.md #VocabularyEntry
type VocabularyEntry struct {
	Domain          string   `json:"domain"`
	CanonicalTerm   string   `json:"canonical_term"`
	PartOfSpeech    string   `json:"part_of_speech"`
	ApprovedMeaning string   `json:"approved_meaning"`
	BannedSynonyms  []string `json:"banned_synonyms"`
	CLICommand      string   `json:"cli_command,omitempty"`
}

// NCTEPolicy defines the authoritative validation schema (#NCTEPolicy).
// @intent nomos_controlled_technical_english_spec.md #NCTEPolicy
type NCTEPolicy struct {
	Bounds             NCTEBounds         `json:"bounds"`
	BannedModals       []string           `json:"banned_modals"`
	ExemptTableHeaders []string           `json:"exempt_table_headers"`
	Severity           DiagnosticSeverity `json:"severity"`
	Vocabulary         []VocabularyEntry  `json:"vocabulary"`
}

// NCTEToken represents a parsed lexical token with source coordinates.
type NCTEToken struct {
	Text         string `json:"text"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	IsLiteral    bool   `json:"is_literal"`
	PartOfSpeech string `json:"part_of_speech"`
}

// NCTESentence represents an analyzed sentence unit.
type NCTESentence struct {
	RawText      string      `json:"raw_text"`
	WordCount    int         `json:"word_count"`
	IsProcedural bool        `json:"is_procedural"`
	HasPassive   bool        `json:"has_passive"`
	LineNumber   int         `json:"line_number"`
	Tokens       []NCTEToken `json:"tokens"`
}

// NCTEDiagnostic describes a deterministic NCTE-100 rule violation.
type NCTEDiagnostic struct {
	Code     ErrorCode          `json:"code"`
	Severity DiagnosticSeverity `json:"severity"`
	Line     int                `json:"line"`
	Column   int                `json:"column"`
	Message  string             `json:"message"`
	Sentence string             `json:"sentence"`
	Remedy   string             `json:"remedy"`
}

// NCTESpecReport contains the complete Markdown verification outcome.
type NCTESpecReport struct {
	FilePath        string           `json:"file_path"`
	TotalSentences  int              `json:"total_sentences"`
	TotalWords      int              `json:"total_words"`
	ViolationsCount int              `json:"violations_count"`
	Diagnostics     []NCTEDiagnostic `json:"diagnostics"`
	Passed          bool             `json:"passed"`
	VerifiedAt      time.Time        `json:"verified_at"`
}

// NCTELinter defines the mechanical Markdown verification contract.
type NCTELinter interface {
	LintMarkdown(filePath string, content []byte) (*NCTESpecReport, error)
	ValidateSentence(sentence NCTESentence) []NCTEDiagnostic
}

// DefaultPolicy returns the canonical NCTE-100 validation policy.
func DefaultPolicy() NCTEPolicy {
	return NCTEPolicy{
		Bounds: NCTEBounds{
			MaxDescriptiveWords:   25,
			MaxProceduralWords:    20,
			MaxParagraphSentences: 6,
			MaxTableCellWords:     12,
			MaxPassiveAdverbGap:   2,
		},
		BannedModals: []string{
			"should", "would", "could", "might", "ideally", "as needed", "and/or",
		},
		ExemptTableHeaders: []string{
			"Input Sentence", "Approved Semantic Meaning", "Banned Synonyms",
		},
		Severity:   SeverityError,
		Vocabulary: CanonicalLexicon(),
	}
}

// CanonicalLexicon returns the 36-term Succinct Action Lexicon across all 5 steering domains.
func CanonicalLexicon() []VocabularyEntry {
	entries := make([]VocabularyEntry, 0, 36)
	entries = append(entries, lifecycleLexicon()...)
	entries = append(entries, verificationLexicon()...)
	entries = append(entries, isolationLexicon()...)
	entries = append(entries, memoryAndSteeringLexicon()...)
	return entries
}

func lifecycleLexicon() []VocabularyEntry {
	return []VocabularyEntry{
		{Domain: DomainLifecycle, CanonicalTerm: "Pair", PartOfSpeech: posVerb, ApprovedMeaning: "Allocate linked INT and SUB twin task records atomically.", BannedSynonyms: []string{"bundle", "couple"}, CLICommand: "nomos task pair"},
		{Domain: DomainLifecycle, CanonicalTerm: "Chore", PartOfSpeech: posNoun, ApprovedMeaning: "A single-step burden-1 task provisioned directly into EDIT.", BannedSynonyms: []string{"quickfix", "hotfix"}, CLICommand: "nomos task chore"},
		{Domain: DomainLifecycle, CanonicalTerm: "Start", PartOfSpeech: posVerb, ApprovedMeaning: "Provision an isolated task worktree and mint a CapBAC lease.", BannedSynonyms: []string{"spin up", "kick off"}, CLICommand: "nomos task start"},
		{Domain: DomainLifecycle, CanonicalTerm: "Commit", PartOfSpeech: posVerb, ApprovedMeaning: "Record a Golden Conventional Commit with Task trailer on the feature branch.", BannedSynonyms: []string{"checkpoint", "snapshot"}, CLICommand: "nomos task commit"},
		{Domain: DomainLifecycle, CanonicalTerm: "Review", PartOfSpeech: posVerb, ApprovedMeaning: "Lock edits and evaluate all 42 Definition of Done gates.", BannedSynonyms: []string{"eyeball", "double-check"}, CLICommand: "nomos task review"},
		{Domain: DomainLifecycle, CanonicalTerm: "Sync", PartOfSpeech: posVerb, ApprovedMeaning: "Execute atomic 2PC fast-forward merge to trunk with walkthrough receipt.", BannedSynonyms: []string{"ship", "land", "push to prod"}, CLICommand: "nomos task sync"},
		{Domain: DomainLifecycle, CanonicalTerm: "Abort", PartOfSpeech: posVerb, ApprovedMeaning: "Revoke active CapBAC lease and remove unmerged task worktree.", BannedSynonyms: []string{"abandon", "scrap"}, CLICommand: "nomos task abort"},
		{Domain: DomainLifecycle, CanonicalTerm: "Prune", PartOfSpeech: posVerb, ApprovedMeaning: "Remove orphaned worktrees and merged feature branches safely.", BannedSynonyms: []string{"delete", "clean up", "wipe"}, CLICommand: "nomos task prune"},
	}
}

func verificationLexicon() []VocabularyEntry {
	return []VocabularyEntry{
		{Domain: DomainVerification, CanonicalTerm: "Verify", PartOfSpeech: posVerb, ApprovedMeaning: "Execute automated validation checks and assert exit code 0.", BannedSynonyms: []string{"check", "test", "inspect"}, CLICommand: "nomos verify"},
		{Domain: DomainVerification, CanonicalTerm: "Attest", PartOfSpeech: posVerb, ApprovedMeaning: "Prove gate mutation sensitivity, specificity, and anti-vacuity.", BannedSynonyms: []string{"certify", "vouch"}, CLICommand: "nomos verify proof"},
		{Domain: DomainVerification, CanonicalTerm: "Reject", PartOfSpeech: posVerb, ApprovedMeaning: "Abort execution and return a structured non-zero exit code.", BannedSynonyms: []string{"drop", "ignore", "fail"}},
		{Domain: DomainVerification, CanonicalTerm: "Seal", PartOfSpeech: posVerb, ApprovedMeaning: "Commit an immutable cryptographic Git Note onto release refs.", BannedSynonyms: []string{"stamp", "finalize", "tag"}},
		{Domain: DomainVerification, CanonicalTerm: "Receipt", PartOfSpeech: posNoun, ApprovedMeaning: "A cryptographic review or release proof stored in Git Notes or SQLite.", BannedSynonyms: []string{"note", "log", "certificate"}},
		{Domain: DomainVerification, CanonicalTerm: "Gate", PartOfSpeech: posNoun, ApprovedMeaning: "A deterministic AST or runtime invariant evaluator in the DoD pipeline.", BannedSynonyms: []string{"guardrail", "rule check"}},
		{Domain: DomainVerification, CanonicalTerm: "Probe", PartOfSpeech: posNoun, ApprovedMeaning: "A uniquely addressable diagnostic sensor registered under a Law cell.", BannedSynonyms: []string{"detector", "sniff"}},
		{Domain: DomainVerification, CanonicalTerm: "Mutant", PartOfSpeech: posNoun, ApprovedMeaning: "A synthetic AST defect injected to verify gate kill sensitivity.", BannedSynonyms: []string{"dummy bug", "fake error"}},
	}
}

func isolationLexicon() []VocabularyEntry {
	return []VocabularyEntry{
		{Domain: DomainIsolation, CanonicalTerm: "Trunk", PartOfSpeech: posNoun, ApprovedMeaning: "The primary protected read-only git branch (main).", BannedSynonyms: []string{"master", "root branch", "upstream"}},
		{Domain: DomainIsolation, CanonicalTerm: "Worktree", PartOfSpeech: posNoun, ApprovedMeaning: "An isolated transient git worktree directory under worktrees/.", BannedSynonyms: []string{"sandbox", "folder", "temp branch"}},
		{Domain: DomainIsolation, CanonicalTerm: "Plane", PartOfSpeech: posNoun, ApprovedMeaning: "One of the two hemispherically separated domains: intent or substrate.", BannedSynonyms: []string{"side", "zone"}},
		{Domain: DomainIsolation, CanonicalTerm: "Lease", PartOfSpeech: posNoun, ApprovedMeaning: "A step-bounded CapBAC capability token authorizing phase actions.", BannedSynonyms: []string{"permit", "ticket", "lock"}},
		{Domain: DomainIsolation, CanonicalTerm: "Mutate", PartOfSpeech: posVerb, ApprovedMeaning: "Modify or write persistent repository state or source files.", BannedSynonyms: []string{"change", "tweak", "patch"}},
		{Domain: DomainIsolation, CanonicalTerm: "Wavefront", PartOfSpeech: posNoun, ApprovedMeaning: "The set of causal DAG tasks with zero unresolved blockers.", BannedSynonyms: []string{"queue", "ready list"}, CLICommand: "nomos task dag"},
		{Domain: DomainIsolation, CanonicalTerm: "Twin", PartOfSpeech: posNoun, ApprovedMeaning: "A bilateral counterpart task linking INT-<N> and SUB-<N>.", BannedSynonyms: []string{"mirror task", "companion"}},
		{Domain: DomainIsolation, CanonicalTerm: "Hull", PartOfSpeech: posNoun, ApprovedMeaning: "Active worktree AST exports, imports, and touched files for collision checks.", BannedSynonyms: []string{"footprint", "blast zone"}, CLICommand: "nomos physics"},
	}
}

func memoryAndSteeringLexicon() []VocabularyEntry {
	return []VocabularyEntry{
		{Domain: DomainMemory, CanonicalTerm: "Save", PartOfSpeech: posVerb, ApprovedMeaning: "Persist an advisory reflection, decision, or axiom in Nomos Mneme.", BannedSynonyms: []string{"stash", "jot", "remember"}, CLICommand: "nomos mem save"},
		{Domain: DomainMemory, CanonicalTerm: "Recall", PartOfSpeech: posVerb, ApprovedMeaning: "Query repository-scoped or cross-repo Mneme memories by keyword.", BannedSynonyms: []string{"fetch memory", "lookup"}, CLICommand: "nomos mem recall"},
		{Domain: DomainMemory, CanonicalTerm: "Distill", PartOfSpeech: posVerb, ApprovedMeaning: "Extract verified trap-to-fix pitfalls and structural patterns during sync.", BannedSynonyms: []string{"summarize", "condense"}},
		{Domain: DomainMemory, CanonicalTerm: "Pitfall", PartOfSpeech: posNoun, ApprovedMeaning: "A machine-recorded gate failure paired with its verified task fix.", BannedSynonyms: []string{"gotcha", "footgun"}},
		{Domain: DomainMemory, CanonicalTerm: "Pattern", PartOfSpeech: posNoun, ApprovedMeaning: "A verified tuple of files, symbols, intents, and tests from a synced task.", BannedSynonyms: []string{"recipe", "boilerplate"}},
		{Domain: DomainMemory, CanonicalTerm: "Axiom", PartOfSpeech: posNoun, ApprovedMeaning: "A non-negotiable architectural principle persisted in Nomos Mneme.", BannedSynonyms: []string{"tip", "guideline"}},
		{Domain: DomainSteering, CanonicalTerm: "Handshake", PartOfSpeech: posVerb, ApprovedMeaning: "Initialize session state, verify CapBAC leases, and emit JIT directives.", BannedSynonyms: []string{"boot", "connect", "login"}, CLICommand: "nomos handshake"},
		{Domain: DomainSteering, CanonicalTerm: "Affordance", PartOfSpeech: posNoun, ApprovedMeaning: "A deterministic HATEOAS Next Actions CLI command emitted on stdout.", BannedSynonyms: []string{"suggestion", "next tip"}},
		{Domain: DomainSteering, CanonicalTerm: "Remedy", PartOfSpeech: posNoun, ApprovedMeaning: "An exact self-healing command attached to a fail-closed NomosError.", BannedSynonyms: []string{"workaround", "hint"}},
		{Domain: DomainSteering, CanonicalTerm: "Directive", PartOfSpeech: posNoun, ApprovedMeaning: "A phase-scoped JIT protocol document injected during workspace transitions.", BannedSynonyms: []string{"prompt", "playbook"}, CLICommand: "nomos directive"},
		{Domain: DomainSteering, CanonicalTerm: "Intercept", PartOfSpeech: posVerb, ApprovedMeaning: "Block an unsafe pre-tool mutation or 3-strike loop via nomos hook.", BannedSynonyms: []string{"catch", "trap"}, CLICommand: "nomos hook"},
		{Domain: DomainSteering, CanonicalTerm: "Lexicon", PartOfSpeech: posNoun, ApprovedMeaning: "The compiled NCTE-100 canonical vocabulary of singular verbs and nouns.", BannedSynonyms: []string{"glossary", "word list"}, CLICommand: "nomos directive --lexicon"},
	}
}

// LookupTerm resolves either a canonical term or a banned synonym to its authoritative VocabularyEntry.
func LookupTerm(query string) (entry *VocabularyEntry, matchedSynonym bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, false
	}
	lex := CanonicalLexicon()
	for i := range lex {
		if strings.ToLower(lex[i].CanonicalTerm) == q {
			return &lex[i], false
		}
	}
	for i := range lex {
		for _, syn := range lex[i].BannedSynonyms {
			if strings.ToLower(syn) == q {
				return &lex[i], true
			}
		}
	}
	return nil, false
}
