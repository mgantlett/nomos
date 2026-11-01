---
authors:
    - Mark Gantlett
    - Sophia
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
    - "Atomic Actions: Procedural steps must declare exactly one discrete action per sentence"
last_updated: "2026-10-14"
phases:
    - PLAN
    - EDIT
plane: intent
priority: high
status: active
summary: "Authoritative JIT directive for authoring Intent specifications and task contracts under Nomos Controlled Technical English."
title: "Nomos Controlled Technical English (NCTE-100) Authoring Directives"
type: architecture
---

# Nomos Controlled Technical English (NCTE-100) Authoring Directives

## 1. Executive Purpose

NCTE-100 defines a deterministic subset of English for Intent authoring and task contracts.
Natural language serves as compilation input for AI agents.
Ambiguous syntax causes cognitive co-processors to hallucinate.
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
- Reject subjective qualitative adjectives lacking numerical thresholds.

---

## 3. Approved Vocabulary Reference

| Canonical Term | Part of Speech | Approved Semantic Meaning | Banned Synonyms |
| :--- | :--- | :--- | :--- |
| **Verify** | Verb | Execute automated validation checks and assert exit code 0. | check, test, inspect |
| **Reject** | Verb | Abort execution and return a structured non-zero exit code. | drop, ignore, fail |
| **Trunk** | Noun | The primary protected git branch (`main`). | master, root, upstream |
| **Worktree** | Noun | An isolated transient git worktree directory. | sandbox, folder, temp branch |
| **Mutate** | Verb | Modify or write persistent state or files. | change, edit, tweak |
| **Prune** | Verb | Remove transient worktrees or merged branches safely. | delete, clean up, wipe |
| **Receipt** | Noun | A cryptographic review or release proof stored in the database. | note, log, certificate |
| **Seal** | Verb | Commit an immutable cryptographic Git Note onto release refs. | stamp, finalize, tag |

---

## 4. Procedural Authoring Sequence

1. "Declare the single-sentence intent objective using 25 words or fewer."
2. "State all axiomatic invariants with exact numeric bounds."
3. "Define data structures in CUE schemas and Go contracts."
4. "Declare procedural steps with imperative verbs and 20 words or fewer."
5. "Declare acceptance criteria as non-verbal proofs with exit codes."
