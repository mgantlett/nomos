---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-08"
last_updated: "2026-11-03"
plane: intent
status: active
title: Commit Schema Invariant Gate Specification
type: spec
version: "4.4.0"
tags:
    - INT-380
    - INT-874
    - INT-903
    - INT-904
    - SUB-1703
ssot_repository: nomos
---

# Commit Schema Invariant Gate Specification

## CommitSchemaGate
The `CommitSchemaGate` enforces Golden Conventional Commits (`[<KEY>]` prefix), flowing rationale prose, anti-dump hygiene, and RFC 822 Git Trailers.

## 1. Executive Summary & Problem Statement
Client-side Git hooks (`.git/hooks/commit-msg`) alone cannot guarantee commit quality because callers can bypass hooks with `--no-verify`. Earlier Nomos releases also dumped `git diff --name-status` file lists into commit bodies and hard-wrapped prose at 72 columns, breaking IDE CommonMark hover cards.

To uphold **Law 4 (SSoT)**, **Law 6 (Canonical Singularity)**, and **Law 7 (Fail-Closed Pre-Flight)**, `CommitSchemaGate` audits every unmerged commit on `trunk..HEAD`. Each commit preserves `[<KEY>]` 2-plane visibility on line 1, flowing CommonMark paragraphs, a `___` divider, and `Task:` / `Twin:` Git trailers.

## 2. Axiomatic Invariants & Laws Codified
- **Law 2 (The Non-Verbal Proof Law)**: Rejects canned placeholder shims and trivial bodies through deterministic revision-range inspection.
- **Law 4 (The Single Source of Truth Law)**: Prohibits `git diff --name-status` file dumps inside commit prose because Git's tree object already owns file manifests.
- **Law 5 (The Zero-Hardcoded-Path Law)**: Resolves repository roots, worktree boundaries, and trunk divergence SHAs dynamically through `workspace.SafeGitCommand`.
- **Law 6 (The Canonical Singularity Law)**: Enforces `[<INT|SUB-N>] <type>(<scope>): <summary>` subjects with strict key parity against the `Task: <KEY>` trailer.
- **Law 7 (The Fail-Closed Pre-Flight Law)**: Evaluates every commit across `trunk..HEAD` so intermediate `--no-verify` commits fail closed before `nomos task sync`.

## 3. Compiled Go Gate Contract
The Go Single Source of Truth resides in `substrate/src/nomos/modules/verify/commit_schema_gate.go`.

```go
package verify

// CommitSchemaGate enforces the Golden Conventional Commit schema ([KEY] subject prefix,
// flowing rationale prose, anti-dump hygiene, and RFC 822 Git Trailers) across branch commits.
// @intent nomos_commit_schema_invariant_spec.md #CommitSchemaGate
type CommitSchemaGate struct{}

func (g *CommitSchemaGate) Name() string {
	return "Commit Schema Invariant Gate"
}

func (g *CommitSchemaGate) Evaluate(ctx *VerifyContext) (*GateResult, error)
```

## 4. Golden Conventional Commit & Git Trailer Specification
Every non-merge branch commit must satisfy the Golden Commit structure:

```text
[<TASK_KEY>] <feat|fix|docs|chore|refactor|test>(<scope>): <imperative summary>

<Flowing unwrapped plain-text rationale explaining the problem, constraint, or design>

- Optional flowing bullets capturing key mechanisms when distinct from paragraph 1.

___

Task: <INT|SUB-N>
Twin: <INT|SUB-M>
```

### 4.1 Verification Protocol
1. **Prefixed Conventional Subject**: Matches `^\[((?:INT|SUB)-[1-9][0-9]*(?:-[a-z0-9-]+)?)\]\s+(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+`, targets `<= 80` runes (`110`-rune ceiling), and forbids trailing `[KEY]` duplicates.
2. **Flowing CommonMark Body**: Requires a blank line after the subject and at least `12` runes of unwrapped rationale prose so IDE hover cards wrap naturally.
3. **Trailer-Safe Divider (`___`)**: Uses CommonMark `___` rather than `---` (`git-mailinfo` patch scissors) so `git interpret-trailers --parse` preserves all trailers.
4. **Anti-Dump & Anti-Shim Guard**: Rejects legacy Dual-Layer headers (`**Blast Radius & Contracts:**`), raw `- M path/file.go` diff dumps, truncation filler, and canned shims.
5. **RFC 822 Git Trailers**: Requires an end-of-message trailer block whose `Task: <KEY>` value matches the `[<KEY>]` subject prefix (plus optional `Twin:` keys).

## 5. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Commit Message Sample | Evaluated State | Expected Gate Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-CMT-001` | `[SUB-1703] feat(verify): enforce Task trailer\n\nKeep commit history concise.\n\n___\n\nTask: SUB-1703` | Valid Golden Commit + trailer | Passes gate (`Passed: true`). | PASS |
| `TC-CMT-002` | `feat(verify): missing prefix\n\nKeep history concise.\n\n___\n\nTask: SUB-1703` | Missing `[KEY]` subject prefix | Fails closed (`Passed: false`). Rejects missing `[INT-N]`/`[SUB-N]` prefix. | PASS |
| `TC-CMT-003` | `[SUB-1703] feat(verify): mismatch\n\nKeep history concise.\n\n___\n\nTask: SUB-999` | Subject/trailer key mismatch | Fails closed (`Passed: false`). Enforces `[KEY]` == `Task: <KEY>` parity. | PASS |
| `TC-CMT-004` | `[SUB-1703] feat(verify): scissors\n\nRationale prose.\n\n---\n\nTask: SUB-1703` | `---` patch-scissors line | Fails closed (`Passed: false`). Directs caller to use `___` divider. | PASS |
| `TC-CMT-005` | Commit A (`--no-verify` rogue) + Commit B (valid `HEAD`) | Multi-commit revision audit | Detects intermediate rogue Commit A and blocks release. | PASS |

## 6. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-380`, `INT-874`, `INT-903`, `INT-904`
- **Substrate Plane**: `SUB-1700`, `SUB-1703` (`substrate/src/nomos/modules/verify/commit_schema_gate.go`, `substrate/cli/commit_hydration.go`)
- **DoD Gates**: Cell 6.1 (`commit_schema`), Cell 6.3 (`canonical_singularity`)
