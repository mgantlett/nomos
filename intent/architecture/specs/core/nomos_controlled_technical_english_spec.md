---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-01"
last_updated: "2026-11-02"
plane: intent
status: active
title: "Nomos Controlled Technical English (NCTE-100) Architectural Specification"
type: spec
version: "4.1.0"
ssot_repository: nomos
---

# Nomos Controlled Technical English (NCTE-100) Architectural Specification

- **Specification Key:** `SPEC-NCTE-100`.
- **Classification:** Public Open-Core Specification.
- **Governing Laws:** Laws 1, 2, 4, 5, 6, and 7.

---

## 1. Executive Summary

Natural language serves as the compilation source for AI agent execution in the Nomos substrate. 
Vague syntax and passive voice cause cognitive co-processors to hallucinate. 
Unbounded sentences waste token budgets during context injection. 
NCTE-100 defines a deterministic, machine-verifiable subset of English for Intent specifications and task contracts. 
The substrate enforces NCTE-100 rules deterministically using an AST markdown analysis engine.

---

## 2. Axiomatic Invariants (The 5 Rules of NCTE-100)

### Rule 1: Length and Structural Bounds
- Descriptive sentences must not exceed 25 words.
- Procedural sentences must not exceed 20 words.
- Paragraphs must not exceed 6 sentences.

### Rule 2: Imperative Mood and Active Voice
- Procedural instructions must use the imperative mood.
- Specifications must reject passive voice constructions.
- Specifications must prohibit ambiguous modal verbs: `should`, `would`, `could`, `might`, `ideally`, `as needed`, and `and/or`.

### Rule 3: Canonical Term Singularity (Law 6)
- Every approved term must possess exactly one grammatical part of speech.
- Every approved term must possess exactly one architectural definition.
- Specifications must reject synonyms, convenience wrappers, and colloquial expressions.

### Rule 4: Atomic Procedural Actions
- Procedural steps must contain exactly one discrete action per sentence.
- Specifications must reject compound instructions joined by conjunctions.

### Rule 5: Non-Verbal Proof Assertions (Law 2 & Law 7)
- Acceptance criteria must declare measurable bounds, exit codes, or AST properties.
- Specifications must reject qualitative adjectives lacking measurable bounds.

---

## 3. Dual-Core Interface Contracts and SSoT Data Structures

NCTE-100 enforces a strict separation between Intent policy and Substrate execution.
The Intent plane defines linguistic bounds in Markdown specifications.
The Substrate plane compiles the AST analysis engine and policy thresholds in Go.

### 3.1 Compiled Policy Bounds Schema (`ncte/linter.go`)

The Substrate policy configuration defines numeric bounds, banned tokens, and vocabulary contracts.

```go
package ncte

// NCTEBounds declares numeric constraints for markdown prose.
type NCTEBounds struct {
	MaxDescriptiveWords   int `json:"max_descriptive_words"`
	MaxProceduralWords    int `json:"max_procedural_words"`
	MaxParagraphSentences int `json:"max_paragraph_sentences"`
	MaxTableCellWords     int `json:"max_table_cell_words"`
	MaxPassiveAdverbGap   int `json:"max_passive_adverb_gap"`
}

// NCTEPolicy defines the authoritative validation policy in Go SSoT.
type NCTEPolicy struct {
	Bounds             NCTEBounds        `json:"bounds"`
	BannedModals       []string          `json:"banned_modals"`
	ExemptTableHeaders []string          `json:"exempt_table_headers"`
	Severity           DiagnosticSeverity `json:"severity"`
	Vocabulary         []VocabularyEntry `json:"vocabulary"`
}
```

### 3.2 Compiled Go SSoT Execution Contracts

The Substrate compiles interface definitions for high-speed AST traversal.

```go
package ncte

import (
	"time"
)

// DiagnosticSeverity defines fail-closed pre-flight levels.
type DiagnosticSeverity string

const (
	// SeverityError halts the verification pipeline immediately with exit code 1.
	SeverityError DiagnosticSeverity = "ERROR"
)

// ErrorCode identifies the violated NCTE-100 invariant.
type ErrorCode string

const (
	ErrSentenceTooLong      ErrorCode = "ERR_NCTE_SENTENCE_TOO_LONG"
	ErrParagraphTooLong     ErrorCode = "ERR_NCTE_PARAGRAPH_TOO_LONG"
	ErrTableCellTooLong     ErrorCode = "ERR_NCTE_TABLE_CELL_TOO_LONG"
	ErrPassiveVoiceDetected ErrorCode = "ERR_NCTE_PASSIVE_VOICE"
	ErrAmbiguousModal       ErrorCode = "ERR_NCTE_AMBIGUOUS_MODAL"
	ErrCompoundAction       ErrorCode = "ERR_NCTE_COMPOUND_ACTION"
	ErrNonSingularTerm      ErrorCode = "ERR_NCTE_NON_SINGULAR_TERM"
	ErrSubjectiveAssertion   ErrorCode = "ERR_NCTE_SUBJECTIVE_ASSERTION"
)

// VocabularyEntry defines a canonical term in the compiled SSoT registry.
type VocabularyEntry struct {
	CanonicalTerm   string   `json:"canonical_term"`
	PartOfSpeech    string   `json:"part_of_speech"`
	ApprovedMeaning string   `json:"approved_meaning"`
	BannedSynonyms  []string `json:"banned_synonyms"`
}

// NCTEToken represents a parsed token with AST metadata.
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

// NCTEDiagnostic describes a deterministic rule violation.
type NCTEDiagnostic struct {
	Code     ErrorCode          `json:"code"`
	Severity DiagnosticSeverity `json:"severity"`
	Line     int                `json:"line"`
	Column   int                `json:"column"`
	Message  string             `json:"message"`
	Sentence string             `json:"sentence"`
	Remedy   string             `json:"remedy"`
}

// NCTESpecReport contains the complete verification outcome.
type NCTESpecReport struct {
	FilePath        string           `json:"file_path"`
	TotalSentences  int              `json:"total_sentences"`
	TotalWords      int              `json:"total_words"`
	ViolationsCount int              `json:"violations_count"`
	Diagnostics     []NCTEDiagnostic `json:"diagnostics"`
	Passed          bool             `json:"passed"`
	VerifiedAt      time.Time        `json:"verified_at"`
}

// NCTELinter defines the mechanical AST verification contract.
type NCTELinter interface {
	LintMarkdown(filePath string, content []byte) (*NCTESpecReport, error)
	ValidateSentence(sentence NCTESentence) []NCTEDiagnostic
}
```

---

## 4. Markdown AST Parsing and Tokenization Mechanics

The NCTE linter converts raw markdown into an Abstract Syntax Tree using `yuin/goldmark`. 
Token counting executes under deterministic isolation rules:

### 4.1 Frontmatter Isolation
- The parser extracts YAML frontmatter before AST evaluation.
- `IntentSteelmanGate` validates frontmatter keys against compiled Go rules.
- Frontmatter words do not contribute to document sentence counts.

### 4.2 Fenced Code Block and Inline Literal Handling
- Fenced code blocks (` ``` `) represent executable artifacts.
- The parser excludes fenced code blocks from prose word counts.
- Inline code spans (`` ` ``) count as exactly one atomic technical token.

### 4.3 Table Cell Word Counting and Exemption Markers
- The parser evaluates standard prose table cells as independent statements.
- Standard prose table cells must not exceed 12 words.
- The parser exempts table cells from word bounds when the column header matches any entry in `exempt_table_headers`.
- The parser exempts code-wrapped or quoted table cells representing input literals.

### 4.4 Sentence Delimitation Algorithm
- The parser splits prose on periods, question marks, and exclamation points followed by whitespace.
- Numeric decimals (`3.14`) and version strings (`v3.0.0`) must not trigger sentence splits.
- Hyphenated technical identifiers (`NCTE-100`, `INT-590`) count as single words.

### 4.5 Passive Voice Detection Algorithm
- The engine identifies forms of the auxiliary verb `be`: `am`, `is`, `are`, `was`, `were`, `been`, `being`, `be`.
- The engine scans forward for past participles within a maximum window of 2 intervening adverb tokens.
- The engine flags matching sequences and outputs `ErrPassiveVoiceDetected`.

---

## 5. Controlled Vocabulary Registry (SSoT)

The substrate compiles a 36-term Succinct Action Lexicon across five domains (`lifecycle`, `verification`, `isolation`, `memory`, `steering`) in `substrate/src/nomos/modules/ncte/lexicon.go`.
Operators query the compiled registry via `nomos directive --lexicon`, `nomos directive --term <word>`, and `nomos directive --lint <file>`.
The system rejects unregistered synonyms.

| Canonical Term | Part of Speech | Approved Semantic Meaning | Banned Synonyms |
| :--- | :--- | :--- | :--- |
| **Pair** | Verb | Allocate linked INT and SUB twin task records atomically. | bundle, couple |
| **Chore** | Noun | A single-step burden-1 task provisioned directly into EDIT. | quickfix, hotfix |
| **Start** | Verb | Provision an isolated task worktree and mint a CapBAC lease. | spin up, kick off |
| **Sync** | Verb | Execute atomic 2PC fast-forward merge to trunk with walkthrough receipt. | ship, land, push to prod |
| **Verify** | Verb | Execute automated validation checks and assert exit code 0. | check, test, inspect |
| **Attest** | Verb | Prove gate mutation sensitivity, specificity, and anti-vacuity. | certify, vouch |
| **Reject** | Verb | Abort execution and return a structured non-zero exit code. | drop, ignore, fail |
| **Trunk** | Noun | The primary protected git branch (`main`). | master, root, upstream |
| **Worktree** | Noun | An isolated transient git worktree directory. | sandbox, folder, temp branch |
| **Mutate** | Verb | Modify or write persistent state or files. | change, edit, tweak |
| **Prune** | Verb | Remove transient worktrees or merged branches safely. | delete, clean up, wipe |
| **Receipt** | Noun | A cryptographic review or release proof stored in the database. | note, log, certificate |
| **Seal** | Verb | Commit an immutable cryptographic Git Note onto release refs. | stamp, finalize, tag |
| **Handshake** | Verb | Initialize session state, verify CapBAC leases, and emit JIT directives. | boot, connect, login |
| **Affordance** | Noun | A deterministic HATEOAS Next Actions CLI command emitted on stdout. | suggestion, next tip |
| **Remedy** | Noun | An exact self-healing command attached to a fail-closed NomosError. | workaround, hint |

---

## 6. Table-Driven Test Matrix (TV-01 through TV-07)

The table below defines deterministic verification vectors for the NCTE AST engine.

| Vector ID | Input Sentence | Target Classification | Expected Word Count | Expected Violation Code | Expected Pass |
| :--- | :--- | :--- | :---: | :--- | :---: |
| **TV-01** | "Execute baseline verification across all quality gates." | Procedural | 7 | None | **true** |
| **TV-02** | "The database record was updated by the background agent worker." | Descriptive | 10 | `ERR_NCTE_PASSIVE_VOICE` | **false** |
| **TV-03** | "Developers should ideally configure the cache properly before executing heavy queries to avoid degraded system response times." | Descriptive | 17 | `ERR_NCTE_AMBIGUOUS_MODAL` | **false** |
| **TV-04** | "Start the transient worktree and compile the substrate binary and run all unit tests." | Procedural | 14 | `ERR_NCTE_COMPOUND_ACTION` | **false** |
| **TV-05** | "The system initializes the telemetry collector across twelve distinct distributed network nodes during the primary boot sequence phase to ensure optimal log synchronization and cluster discovery." | Descriptive | 26 | `ERR_NCTE_SENTENCE_TOO_LONG` | **false** |
| **TV-06** | "Please check the root branch sandbox before submitting your changes." | Procedural | 10 | `ERR_NCTE_NON_SINGULAR_TERM` | **false** |
| **TV-07** | "Ensure the parser performance is very fast and extremely robust under heavy concurrency." | Procedural | 12 | `ERR_NCTE_SUBJECTIVE_ASSERTION` | **false** |

---

## 7. Dynamic Substrate Integration (Law 5 & Law 7)

1. **Dynamic Path Resolution (Law 5):**
   - The NCTE engine discovers Intent source roots using `config.LoadConfig()`.
   - The engine resolves directories dynamically via `cfg.Governance.QualityGates.IntentSourceRoots`.
   - The engine never embeds hardcoded repository paths.

2. **Fail-Closed Gatekeeper Enforcement (Law 7):**
   - The engine registers as Gate 29 in the Definition of Done pipeline.
   - Any diagnostic with `SeverityError` terminates verification with exit code 1.
   - Pre-commit hooks reject commits containing NCTE violations.
   - The engine permits zero warning overrides.
