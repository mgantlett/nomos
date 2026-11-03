---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
category: planning
classification: public
date: "2026-10-14"
flow:
    - PLAN
    - EDIT
items:
    - "Length Invariant: Descriptive sentences must not exceed 25 words; procedural sentences must not exceed 20 words"
    - "Mood Invariant: Procedural instructions must use imperative mood; reject passive voice constructions"
    - "Modal Prohibition: Specifications must prohibit ambiguous modals (should, would, could, might, ideally, as needed, and/or)"
    - "Singularity Invariant: Every approved term possesses exactly one part of speech and one architectural definition"
    - "Succinct Steering Lexicon: Query all 36 canonical verbs and nouns via nomos directive --lexicon"
last_updated: "2026-11-02"
phases:
    - PLAN
    - EDIT
plane: intent
priority: high
status: active
summary: "Authoritative JIT directive for authoring Intent specifications and agent-steering prompts under NCTE-100."
title: "Nomos Controlled Technical English (NCTE-100) Authoring Directives"
type: architecture
---

# Nomos Controlled Technical English (NCTE-100) Authoring Directives

## 1. Executive Purpose

NCTE-100 defines a deterministic subset of English for Intent authoring and agent steering.
Natural language serves as compilation input for AI cognitive co-processors.
Ambiguous syntax causes Tier 1 agents to hallucinate unrequested abstractions.
NCTE-100 eliminates ambiguity through machine-enforced vocabulary and sentence bounds.

---

## 2. The 5 Core Invariants

### Invariant 1: Sentence and Paragraph Bounds
- Write descriptive sentences with 25 words or fewer.
- Write procedural sentences with 20 words or fewer.
- Limit paragraphs to 6 sentences or fewer.

### Invariant 2: Active Voice and Imperative Mood
- Use imperative verbs for all procedural instructions.
- Reject passive voice constructions across all specifications.
- Prohibit ambiguous modal verbs: `should`, `would`, `could`, `might`, `ideally`, `as needed`, and `and/or`.

### Invariant 3: Canonical Term Singularity (Law 6)
- Restrict each approved term to one grammatical part of speech.
- Restrict each approved term to one architectural definition.
- Reject convenience synonyms, aliases, and colloquial metaphors.

### Invariant 4: Atomic Procedural Actions
- Declare exactly one discrete action per procedural sentence.
- Do not combine procedural instructions with conjunctions.

### Invariant 5: Measurable Assertions (Law 2 & Law 7)
- Express acceptance criteria as measurable bounds, exit codes, or AST properties.
- Reject qualitative intensifiers lacking numerical thresholds.

---

## 3. Succinct Action Lexicon Across the 5 Steering Domains

Run `nomos directive --lexicon` to list all 36 compiled terms or `nomos directive --term <word>` to resolve synonyms.

| Canonical Term | Part of Speech | Approved Semantic Meaning | Banned Synonyms |
| :--- | :--- | :--- | :--- |
| **Pair** | Verb | Allocate linked INT and SUB twin task records atomically. | bundle, couple |
| **Chore** | Noun | A single-step burden-1 task provisioned directly into EDIT. | quickfix, hotfix |
| **Start** | Verb | Provision an isolated task worktree and mint a CapBAC lease. | spin up, kick off |
| **Sync** | Verb | Execute atomic 2PC fast-forward merge to trunk with walkthrough receipt. | ship, land, push to prod |
| **Prune** | Verb | Remove orphaned worktrees and merged feature branches safely. | delete, clean up, wipe |
| **Verify** | Verb | Execute automated validation checks and assert exit code 0. | check, test, inspect |
| **Attest** | Verb | Prove gate mutation sensitivity, specificity, and anti-vacuity. | certify, vouch |
| **Reject** | Verb | Abort execution and return a structured non-zero exit code. | drop, ignore, fail |
| **Seal** | Verb | Commit an immutable cryptographic Git Note onto release refs. | stamp, finalize, tag |
| **Receipt** | Noun | A cryptographic review or release proof stored in Git Notes or SQLite. | note, log, certificate |
| **Trunk** | Noun | The primary protected read-only git branch (`main`). | master, root branch, upstream |
| **Worktree** | Noun | An isolated transient git worktree directory under `worktrees/`. | sandbox, folder, temp branch |
| **Mutate** | Verb | Modify or write persistent repository state or source files. | change, tweak, patch |
| **Wavefront** | Noun | The set of causal DAG tasks with zero unresolved blockers. | queue, ready list |
| **Save** | Verb | Persist an advisory reflection, decision, or axiom in Nomos Mneme. | stash, jot, remember |
| **Recall** | Verb | Query repository-scoped or cross-repo Mneme memories by keyword. | fetch memory, lookup |
| **Distill** | Verb | Extract verified trap-to-fix pitfalls and structural patterns during sync. | summarize, condense |
| **Handshake** | Verb | Initialize session state, verify CapBAC leases, and emit JIT directives. | boot, connect, login |
| **Affordance** | Noun | A deterministic HATEOAS Next Actions CLI command emitted on stdout. | suggestion, next tip |
| **Remedy** | Noun | An exact self-healing command attached to a fail-closed NomosError. | workaround, hint |
| **Intercept** | Verb | Block an unsafe pre-tool mutation or 3-strike loop via `nomos hook`. | catch, trap |

---

## 4. Procedural Authoring Sequence

1. "Declare the single-sentence intent objective using 25 words or fewer."
2. "State all axiomatic invariants with exact numeric bounds."
3. "Define data structures in compiled Go contracts and JSON Schema."
4. "Declare procedural steps with imperative verbs and 20 words or fewer."
5. "Audit finished specifications via `nomos directive --lint <file>`."
