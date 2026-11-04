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
    - SUB-1700
ssot_repository: nomos
---

# Commit Schema Invariant Gate Specification

## CommitSchemaGate
The `CommitSchemaGate` enforces Golden Conventional Commits, anti-dump hygiene, and RFC 822 Git Trailers on branch commits before release.

## 1. Executive Summary & Problem Statement
Client-side Git hooks (`.git/hooks/commit-msg`) alone cannot guarantee commit quality because callers can bypass hooks with `--no-verify`. Earlier Nomos releases also prepended `[SUB-<N>]` to commit subjects and dumped `git diff --name-status` file lists into commit bodies. Those file-list dumps duplicated Git's native tree diff while crowding out high-signal architectural rationale.

To uphold **Law 4 (SSoT)**, **Law 6 (Canonical Singularity)**, and **Law 7 (Fail-Closed Pre-Flight)**, `CommitSchemaGate` audits every unmerged commit on `trunk..HEAD`. Each commit must pair a clean Conventional Commit subject with substantive rationale prose and a canonical `Task:` Git trailer.

## 2. Axiomatic Invariants & Laws Codified
- **Law 2 (The Non-Verbal Proof Law)**: Rejects canned placeholder shims and trivial one-word bodies through deterministic revision-range inspection.
- **Law 4 (The Single Source of Truth Law)**: Prohibits `git diff --name-status` file dumps inside commit prose because Git's tree object already owns file manifests.
- **Law 5 (The Zero-Hardcoded-Path Law)**: Resolves repository roots, worktree boundaries, and trunk divergence SHAs dynamically through `workspace.SafeGitCommand`.
- **Law 6 (The Canonical Singularity Law)**: Enforces pure Conventional Commit subjects without leading or trailing `[KEY]` tokens and binds lineage via `Task: <INT|SUB-N>` trailers.
- **Law 7 (The Fail-Closed Pre-Flight Law)**: Evaluates every commit across `trunk..HEAD` so intermediate `--no-verify` commits fail closed before `nomos task sync`.

## 3. Compiled Go Gate Contract
The Go Single Source of Truth resides in `substrate/src/nomos/modules/verify/commit_schema_gate.go`.

```go
package verify

// CommitSchemaGate enforces the Golden Conventional Commit schema, anti-dump hygiene,
// and canonical RFC 822 Git Trailers across all unmerged branch commits.
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
<feat|fix|docs|chore|refactor|test>(<scope>): <imperative summary>

<72-column plain-text rationale explaining the problem, constraint, or design>

- Optional concise bullets capturing key mechanisms when distinct from paragraph 1.

Task: <INT|SUB-N>
Twin: <INT|SUB-M>
```

### 4.1 Verification Protocol
1. **Pure Conventional Subject**: Matches `^(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+`, targets `<= 72` runes (`100`-rune hard ceiling), and forbids `[KEY]` prefixes or suffixes.
2. **Substantive Plain-Text Body**: Requires a blank line after the subject and at least `12` runes of plain-text rationale above the trailer block.
3. **Anti-Dump & Anti-Shim Guard**: Rejects legacy Dual-Layer bold headers (`**Blast Radius & Contracts:**`), raw `- M path/file.go` diff dumps, truncation filler, and canned shims.
4. **RFC 822 Git Trailers**: Requires an end-of-message trailer block containing exactly one `Task: <INT|SUB-N>` trailer (and validates optional `Twin:` keys).
5. **Branch Revision Range Audit**: Validates every unmerged commit on `trunk..HEAD` so compliant `HEAD` commits cannot mask earlier rogue commits.

## 5. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Commit Message Sample | Evaluated State | Expected Gate Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-CMT-001` | `feat(verify): enforce Task trailer\n\nKeep commit history concise.\n\nTask: SUB-1700` | Valid Golden Commit + trailer | Passes gate (`Passed: true`). | PASS |
| `TC-CMT-002` | `[SUB-1700] feat(verify): legacy prefix\n\nKeep history concise.\n\nTask: SUB-1700` | Leading `[KEY]` subject prefix | Fails closed (`Passed: false`). Rejects leading `[TaskID]` prefix. | PASS |
| `TC-CMT-003` | `feat(verify): missing trailer\n\nKeep commit history concise.` | Missing `Task:` trailer | Fails closed (`Passed: false`). Emits missing `Task:` trailer error. | PASS |
| `TC-CMT-004` | `feat(verify): dump files\n\nRationale.\n\n- M cli/root.go\n\nTask: SUB-1700` | Raw staged diff dump | Fails closed (`Passed: false`). Rejects raw file-list dump (Law 4). | PASS |
| `TC-CMT-005` | Commit A (`--no-verify` rogue) + Commit B (valid `HEAD`) | Multi-commit revision audit | Detects intermediate rogue Commit A and blocks release. | PASS |

## 6. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-380`, `INT-874`, `INT-903`
- **Substrate Plane**: `SUB-1700` (`substrate/src/nomos/modules/verify/commit_schema_gate.go`, `substrate/cli/commit_hydration.go`)
- **DoD Gates**: Cell 6.1 (`commit_schema`), Cell 6.3 (`canonical_singularity`)
