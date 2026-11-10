---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-01"
last_updated: "2026-11-10"
lineage:
  - INT-1944
  - SUB-2157
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Controlled Technical English (NCTE-100) Architectural Specification"
type: spec
version: "4.8.0"
---

# Nomos Controlled Technical English (NCTE-100) Architectural Specification

---

## 1. Executive Summary & Problem Statement

Natural language serves as the compilation source for AI agent execution in the Nomos substrate.
Vague syntax and passive voice cause cognitive co-processors to hallucinate.
Unbounded sentences waste token budgets during context injection.
NCTE-100 defines a deterministic, machine-verifiable subset of English for Intent specifications and task contracts (`SPEC-NCTE-100`).
The substrate enforces NCTE-100 rules and the v4.8 Canonical Specification Blueprint through `ncte.Engine` and `IntentSteelmanGate` (`Cell 4.1`, `intent_steelman`).

---

## 2. Axiomatic Invariants & Governing Laws

NCTE-100 operationalizes **Laws 1, 2, 4, 5, 6, and 7** across five deterministic linguistic rules:

### 2.1 Rule 1: Length and Structural Bounds (Law 1)
- Descriptive sentences must not exceed 25 words.
- Procedural sentences must not exceed 20 words.
- Paragraphs must not exceed 6 sentences.

### 2.2 Rule 2: Imperative Mood and Active Voice (Law 1 & Law 7)
- Procedural instructions must use the imperative mood.
- Specifications must reject passive voice constructions.
- Specifications must prohibit ambiguous modal verbs: `should`, `would`, `could`, `might`, `ideally`, `as needed`, and `and/or`.

### 2.3 Rule 3: Canonical Term Singularity (Law 6)
- Every approved term must possess exactly one grammatical part of speech.
- Every approved term must possess exactly one architectural definition.
- Specifications must reject synonyms, convenience wrappers, and colloquial expressions.

### 2.4 Rule 4: Atomic Procedural Actions (Law 1)
- Procedural steps must contain exactly one discrete action per sentence.
- Specifications must reject compound instructions joined by conjunctions.

### 2.5 Rule 5: Non-Verbal Proof Assertions (Law 2 & Law 4)
- Acceptance criteria must declare measurable bounds, exit codes, or AST properties.
- Specifications must reject qualitative adjectives lacking measurable bounds.

---

## 3. Dual-Core Interface Contracts and SSoT Data Structures

NCTE-100 enforces a strict separation between Intent policy and Substrate execution.
The Intent plane defines linguistic bounds in Markdown specifications.
The Substrate plane compiles the AST analysis engine and policy thresholds in Go (`substrate/src/nomos/modules/ncte/`).

### 3.1 Compiled Policy Bounds Schema (`ncte/lexicon.go`)

The Substrate policy configuration defines numeric bounds, banned tokens, and vocabulary contracts.

```go
package ncte

// NCTEBounds declares numeric constraints for Markdown prose (#NCTEBounds).
type NCTEBounds struct {
	MaxDescriptiveWords   int `json:"max_descriptive_words"`
	MaxProceduralWords    int `json:"max_procedural_words"`
	MaxParagraphSentences int `json:"max_paragraph_sentences"`
	MaxTableCellWords     int `json:"max_table_cell_words"`
	MaxPassiveAdverbGap   int `json:"max_passive_adverb_gap"`
}

// NCTEPolicy defines the authoritative validation schema (#NCTEPolicy).
type NCTEPolicy struct {
	Bounds             NCTEBounds         `json:"bounds"`
	BannedModals       []string           `json:"banned_modals"`
	ExemptTableHeaders []string           `json:"exempt_table_headers"`
	Severity           DiagnosticSeverity `json:"severity"`
	Vocabulary         []VocabularyEntry  `json:"vocabulary"`
}
```

### 3.2 Compiled Go SSoT Execution Contracts (`ncte/lexicon.go` & `ncte/linter.go`)

The Substrate compiles interface definitions for high-speed AST traversal.

```go
package ncte

import (
	"time"
)

// DiagnosticSeverity defines fail-closed pre-flight levels (#NCTEPolicy).
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
	ErrSubjectiveAssertion  ErrorCode = "ERR_NCTE_SUBJECTIVE_ASSERTION"
)

// VocabularyEntry defines a canonical term in the compiled SSoT registry (#VocabularyEntry).
type VocabularyEntry struct {
	Domain          string   `json:"domain"`
	CanonicalTerm   string   `json:"canonical_term"`
	PartOfSpeech    string   `json:"part_of_speech"`
	ApprovedMeaning string   `json:"approved_meaning"`
	BannedSynonyms  []string `json:"banned_synonyms"`
	CLICommand      string   `json:"cli_command,omitempty"`
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
```

---

## 4. Markdown Tokenization and Isolation Mechanics

`ncte.Engine` in `substrate/src/nomos/modules/ncte/linter.go` tokenizes Markdown deterministically without external parser dependencies.
Token counting executes under deterministic line, fence, frontmatter, and inline-literal isolation rules:

### 4.1 Frontmatter Isolation
- `ncte.Engine.LintMarkdown` skips leading `---` YAML frontmatter blocks before prose evaluation.
- `IntentSteelmanGate` validates frontmatter keys against compiled Go rules.
- Frontmatter words never contribute to document sentence counts.

### 4.2 Fenced Code Block and Inline Literal Isolation
- `ncte.Engine` tracks ```` ``` ```` and `~~~` fence delimiters line by line and excludes fenced blocks from prose evaluation.
- `TokenizeSentence` collapses each inline backtick code span (`` `...` ``) into one atomic literal token.
- Literal tokens bypass banned-modal, passive-voice, and synonym matching.

### 4.3 Table Cell Word Counting and Exemption Markers
- `ncte.Engine` evaluates standard prose table cells as independent statements.
- Non-exempt prose table cells must not exceed 12 words.
- Column headers matching `exempt_table_headers` exempt their cells from word bounds.
- Backtick-wrapped or double-quoted table cells bypass prose length bounds as literal inputs.

### 4.4 Sentence Delimitation Algorithm
- `SplitSentences` splits prose on periods, question marks, and exclamation points outside inline backticks.
- Numeric decimals (`3.14`) and version strings (`v3.0.0`) do not trigger sentence splits.
- Hyphenated technical identifiers (`NCTE-100`, `INT-590`) count as single words.

### 4.5 Passive Voice Detection Algorithm
- The engine identifies forms of the auxiliary verb `be`: `am`, `is`, `are`, `was`, `were`, `been`, `being`, `be`.
- The engine scans forward for past participles within a maximum window of 2 intervening adverb tokens.
- Matching sequences emit `ErrPassiveVoiceDetected`.

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

## 6. Canonical Specification Structural Blueprint (`IntentSteelmanGate` v4.8)

`IntentSteelmanGate` (`Cell 4.1`, `intent_steelman`) in `substrate/src/nomos/modules/verify/intent_steelman.go` enforces the v4.8 Canonical Specification Structural Blueprint across `intent/`:

1. **Flat Intent Topology (Law 6):**
   - Every Intent Markdown file resides at `intent/intent.md` or flat `intent/architecture/specs/*_spec.md`.
   - The blueprint prohibits `handbook/` directories and nested subdirectories inside `intent/architecture/specs/`.
2. **YAML Frontmatter Allowlist (Law 4):**
   - Specifications declare 10 required keys (`authors`, `classification`, `date`, `last_updated`, `plane`, `ssot_repository`, `status`, `title`, `type: spec`, `version`).
   - Specifications accept one optional `lineage` key listing sorted `INT-<N>` and `SUB-<N>` task keys.
   - The gate rejects `trigger`, `tags`, `execution_frontier`, and all unregistered frontmatter keys.
3. **Clean Post-H1 Opening (Law 6):**
   - Documents open after YAML frontmatter with `# <Title>`, one `---` divider, and `## 1. Executive Summary & Problem Statement`.
   - The blueprint prohibits pre-section preambles, role headers, callouts, and metadata lists before Section 1.
4. **Contiguous 1-Indexed H2 Spine (Law 4 & Law 6):**
   - Specifications define contiguous `1..N` (`N >= 5`) `## <k>. <Title>` sections separated by `---` horizontal rules.
   - Documents prohibit extra `# H1` headings and unnumbered `##` headings outside fenced code blocks.
   - Sections `1` and `2` match `## 1. Executive Summary & Problem Statement` and `## 2. Axiomatic Invariants & Governing Laws`.
   - Sections `N-1` and `N` match `## <N-1>. Table-Driven Verification Vectors` and `## <N>. Cross-Plane Traceability & DoD Verification`.
5. **Compiled Go SSoT & Vector Parity (Law 2 & Law 4):**
   - Every specification includes at least one fenced `go` block declaring compiled `type`, `func`, `const`, or `var` contracts.
   - Verification tables define at least three `TC-<DOMAIN>-<NN>` vector rows with zero legacy `TV-*`, `VEC-*`, or `CDV-*` prefixes.
6. **Terminology, Dash & NCTE-100 Hygiene (Law 6 & Law 7):**
   - `IntentSteelmanGate` rejects Unicode em-dashes, Unicode en-dashes, legacy `Gate <N>` prose, and dangling `ADR-XXXX` shorthand.
   - Every specification passes `ncte.Engine.LintMarkdown` with zero `NCTE-100` diagnostics.

```go
package verify

// IntentSteelmanGate validates Intent specification YAML frontmatter, structure, and NCTE-100 prose (Cell 4.1, Law 4).
type IntentSteelmanGate struct{}

// Evaluate audits all Intent Markdown documents for valid YAML frontmatter, balanced fences, and strict NCTE-100 compliance.
func (g *IntentSteelmanGate) Evaluate(ctx *VerifyContext) (*GateResult, error)
```

```go
package handshake

// BrownfieldTopologyBrief summarizes active governance modes and discovered source topology (#BrownfieldTopologyBrief).
type BrownfieldTopologyBrief struct {
	GovernanceMode string              `json:"governance_mode"`
	VerifyScope    string              `json:"verify_scope,omitempty"`
	TDDMode        string              `json:"tdd_mode"`
	CoverageMode   string              `json:"coverage_mode"`
	Languages      []string            `json:"languages"`
	TestRunners    []string            `json:"test_runners"`
	SourceRoots    []string            `json:"source_roots"`
	TestRoots      []string            `json:"test_roots,omitempty"`
	SharedModules  []string            `json:"shared_modules,omitempty"`
	Hotspots       []BrownfieldHotspot `json:"hotspots,omitempty"`
}

// HandshakeEvent represents JIT session orientation telemetry (#HandshakeEvent).
type HandshakeEvent struct {
	Seat                 string                     `json:"seat"`
	Repository           string                     `json:"repository"`
	Plane                string                     `json:"plane,omitempty"`
	Phase                types.Phase                `json:"phase"`
	ActiveTask           string                     `json:"active_task,omitempty"`
	Assignee             string                     `json:"assignee,omitempty"`
	TokenID              string                     `json:"token_id,omitempty"`
	StepBudget           int                        `json:"step_budget,omitempty"`
	TokensUsed           int                        `json:"tokens_used,omitempty"`
	DatabasesHealthy     bool                       `json:"databases_healthy"`
	GateCount            int                        `json:"gate_count"`
	ReadyWavefront       []string                   `json:"ready_wavefront"`
	Directives           []string                   `json:"directives,omitempty"`
	LearnedPitfalls      []string                   `json:"learned_pitfalls,omitempty"`
	VerifiedPatterns     []string                   `json:"verified_patterns,omitempty"`
	GraduationCandidates []string                   `json:"graduation_candidates,omitempty"`
	ImpactSummary        []string                   `json:"impact_summary,omitempty"`
	StaleObligations     []task.StaleSpecObligation `json:"stale_obligations,omitempty"`
	StrictMode           bool                       `json:"strict_mode"`
	BrownfieldTopology   *BrownfieldTopologyBrief   `json:"brownfield_topology,omitempty"`
	NextActions          []types.ActionAffordance   `json:"next_actions,omitempty"`
	DeprecatedTerms      map[string]string          `json:"deprecated_terms,omitempty"`
	Timestamp            string                     `json:"timestamp,omitempty"`
}
```

---

## 7. Table-Driven Verification Vectors

The table below defines deterministic verification vectors for `ncte.Engine` and `IntentSteelmanGate`:

| Vector ID | Input Sentence | Target Classification | Expected Word Count | Expected Violation Code | Expected Pass |
| :--- | :--- | :--- | :---: | :--- | :---: |
| `TC-NCTE-01` | `"Execute baseline verification across all quality gates."` | Procedural | 7 | None | **true** |
| `TC-NCTE-02` | `"The database record was updated by the background agent worker."` | Descriptive | 10 | `ERR_NCTE_PASSIVE_VOICE` | **false** |
| `TC-NCTE-03` | `"Developers should ideally configure the cache properly before executing heavy queries to avoid degraded system response times."` | Descriptive | 17 | `ERR_NCTE_AMBIGUOUS_MODAL` | **false** |
| `TC-NCTE-04` | `"Start the transient worktree and compile the substrate binary and run all unit tests."` | Procedural | 14 | `ERR_NCTE_COMPOUND_ACTION` | **false** |
| `TC-NCTE-05` | `"The system initializes the telemetry collector across twelve distinct distributed network nodes during the primary boot sequence phase to ensure optimal log synchronization and cluster discovery."` | Descriptive | 26 | `ERR_NCTE_SENTENCE_TOO_LONG` | **false** |
| `TC-NCTE-06` | `"Please check the root branch sandbox before submitting your changes."` | Procedural | 10 | `ERR_NCTE_NON_SINGULAR_TERM` | **false** |
| `TC-NCTE-07` | `"Ensure the parser performance is very fast and extremely robust under heavy concurrency."` | Procedural | 12 | `ERR_NCTE_SUBJECTIVE_ASSERTION` | **false** |
| `TC-NCTE-08` | `"Specification missing Go SSoT block or contiguous H2 spine under strict_mode."` | Structural | 11 | `intent_steelman` rejection | **false** |
| `TC-JIT-01` | `"Execute nomos handshake on session boot."` | Procedural | 6 | None (`HandshakeEvent` emitted) | **true** |
| `TC-JIT-02` | `"Emit BrownfieldTopologyBrief with discovered source roots and test runners."` | Procedural | 9 | None (`BrownfieldTopology` populated) | **true** |
| `TC-JIT-03` | `"Surface active LearnedPitfalls and VerifiedPatterns in HandshakeEvent."` | Procedural | 7 | None (`LearnedPitfalls` attached) | **true** |
| `TC-JIT-04` | `"Surface StaleObligations when Intent specs change after SUB sync."` | Procedural | 9 | None (`StaleObligations` reported) | **true** |
| `TC-TIER-01` | `"Enforce Tier 1 indirect control via BoundaryConfig in nomos.yaml."` | Procedural | 9 | None (`Boundary.Enforce == true`) | **true** |
| `TC-TIER-02` | `"Reject prohibited sovereign packages in open-core substrate."` | Procedural | 7 | `boundary` gate rejection | **false** |
| `TC-TIER-03` | `"Reject prohibited sovereign CLI commands in open-core CommandRegistry."` | Procedural | 8 | `boundary` command rejection | **false** |
| `TC-TIER-04` | `"Emit HATEOAS NextActions on every HandshakeEvent."` | Procedural | 6 | None (`NextActions` populated) | **true** |

---

## 8. Cross-Plane Traceability & DoD Verification

1. **Compiled Go SSoT Paths (Law 4 & Law 5):**
   - `substrate/src/nomos/modules/ncte/lexicon.go` (`NCTEBounds`, `NCTEPolicy`, `VocabularyEntry`, `DefaultPolicy`, `CanonicalLexicon`).
   - `substrate/src/nomos/modules/ncte/linter.go` and `substrate/src/nomos/modules/ncte/linter_rules.go` (`Engine`, `LintMarkdown`, `SplitSentences`, `TokenizeSentence`).
   - `substrate/src/nomos/modules/verify/gates_intent.go` and `substrate/src/nomos/modules/verify/intent_steelman.go` (`IntentSteelmanGate`).
   - `IntentSteelmanGate` resolves repository and worktree roots dynamically via `VerifyContext` and `config.LoadConfig()`.
2. **7x3 Invariant Matrix Coordinates (Law 2 & Law 7):**
   - `Cell 4.1 (intent_steelman)` executes `validateIntentDocumentsWithOptions` and `ncte.Engine.LintMarkdown` across `intent/` during `nomos verify`.
   - `Cell 4.1 (broken_wire)` and `Cell 4.1 (intent_traceability)` verify cross-plane specification links and `@intent` AST bindings.
   - Operators audit individual specifications via `nomos directive --lint <file>` and full repository parity via `nomos verify` (exit code `0`).
