---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-03"
plane: intent
status: active
title: "Hermetic Git Hooks, Task Commit, and Staged Boundary Enforcement Specification"
type: spec
version: "4.4.0"
tags:
    - INT-734
    - SUB-1212
    - INT-874
    - INT-903
    - SUB-1700
ssot_repository: nomos
---

# Hermetic Git Hooks, Task Commit, and Staged Boundary Enforcement Specification

## 1. Executive Summary & Problem Statement

Under **Law 1 (The Dual-Core Inversion Law)** and **Law 3 (The Hermetic Worktree Law)**, the Go substrate governs execution while Tier 1 coding agents edit inside isolated worktrees.

In uncontained development environments, agents or developers may inadvertently:
1. Attempt raw `git commit` on protected trunk branches (`main`), corrupting canonical history.
2. Invoke unsanitized `exec.Command("git", ...)` calls that inherit host `GIT_DIR` or `GIT_WORK_TREE` variables across worktrees.
3. Write scratch walkthroughs directly to the repository root instead of the `.tmp/` containment boundary.
4. Pollute Git history with redundant file-list dumps or omit canonical `Task:` Git trailers needed by release tooling.

This specification defines the **Hermetic Git Hooks Lifecycle**, the **`nomos task commit`** CLI contract, and three statutory probes: **`staged_boundaries`**, **`git_hygiene`**, and **`commit_schema`**.

---

## 2. Axiomatic Invariants & Laws Codified

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Substrate Go core governs execution; agents commit through canonical CLI commands. | `nomos task commit`, `git_hygiene` AST probe |
| **2** | **The Non-Verbal Proof Law** | Commit rationale prose and anti-shim rules are machine-verified before merge. | `commit_schema` gate, table-driven tests |
| **3** | **The Hermetic Worktree Law** | Mutations execute strictly inside transient worktrees while trunk stays read-only. | `pre-commit` hook, `staged_boundaries` gate |
| **4** | **The Single Source of Truth Law** | Git's tree diff owns file manifests; hook scripts and commit gates share Go SSoT. | `commit_schema_gate.go`, `workspace/hooks.go` |
| **5** | **The Zero-Hardcoded-Path Law** | Worktrees, hooks, and repository roots resolve dynamically at runtime. | `config.ResolveWorktreeDir()`, `FindRepoRoot()` |
| **6** | **The Canonical Singularity Law** | One commit CLI (`nomos task commit`) emits Conventional Commits with `Task:` trailers. | Zero-Alias CLI gate, `commit-msg` hook |
| **7** | **The Fail-Closed Pre-Flight Law** | Boundary escapes, missing `Task:` trailers, or unsanitized git calls fail closed. | Gate pipeline exit code 1 |

---

## 3. The Live Git Hooks Architecture

Nomos installs and self-heals three hermetic client-side hooks inside `.git/hooks/`:

### 3.1 `pre-commit`
- **Root Protection**: Rejects direct commits on protected trunk branches (`main`, `master`, `intent`, `substrate`, `develop`).
- **Worktree Boundary Enforcement**: Verifies that the working directory resides inside `worktrees/<repo>-<KEY>`.
- **Boundary Escape Audit**: Inspects `git diff --cached --name-only` so staged paths never escape via `..` or absolute paths.
- **Workflow Enforcement**: Fails closed unless `nomos task commit` exports `NOMOS_COMMIT_IN_PROGRESS=1`.
- **Pre-Flight DoD Verification**: Executes `nomos verify` across all active quality gates.

### 3.2 `commit-msg`
- **Conventional Subject**: Enforces `^(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+` (`<= 100` chars) and blocks `[KEY]` prefixes or suffixes.
- **Anti-Dump & Anti-Shim Guard**: Rejects legacy Dual-Layer headers (`**Blast Radius & Contracts:**`), raw `- M file` diff dumps, and canned boilerplate shims.
- **Rationale & Trailer Check**: Requires substantive plain-text rationale prose and a canonical `Task: <INT|SUB-N>` RFC 822 Git trailer.

### 3.3 `pre-push`
- **Trunk Push Defense**: Blocks pushing non-trunk branches directly to remote trunk and runs `nomos verify --range` before trunk pushes.

---

## 4. Canonical CLI Interface: `nomos task commit`

```text
Usage:
  nomos task commit [TASK_KEY] [flags]

Flags:
  -m, --message string     Commit summary line (normalized to <=72-rune Conventional Commit subject)
      --rationale string   Problem context and architectural rationale (defaults to Scope & Context)
      --mechanics string   Optional key design mechanics (defaults to deduplicated walkthrough bullets)
  -F, --file string        Optional walkthrough markdown file to hydrate commit prose
  -a, --all                Automatically stage all modified files (default: true)
  -d, --dir string         Working directory (default: current directory or resolved task worktree)
```

### 4.1 Deterministic Golden Commit Hydration (`INT-874`, `INT-903`)
1. **Task Key Resolution**: Resolves `INT-<N>` or `SUB-<N>` from `TASK_KEY`, the active worktree directory name, or `feature/<KEY>`.
2. **72-Rune Subject Normalization**: Strips redundant `[KEY]` tokens, infers `<type>(<scope>):` from staged paths when needed, and lowercases plain English leading verbs while preserving acronyms (`AST`, `CapBAC`, `MCP`, `CLI`, `Go`).
3. **Deduplicated 72-Column Body**:
   - **Paragraph 1 (Rationale)**: Hydrates from `--rationale`, Living Ledger `## Scope & Context`, walkthrough lead prose, or concise module intent wrapped at `72` columns.
   - **Optional Mechanics Bullets**: Hydrates from `--mechanics`, deduplicated walkthrough bullets, or `## Acceptance Criteria` without repeating Paragraph 1 or dumping staged file lists.
4. **RFC 822 Git Trailers**: Appends `Task: <KEY>` and automatically appends `Twin: <TWIN_KEY>` whenever `TaskRecord.TwinKey` is bound.
5. **Dirty Worktree Sync Parity**: `nomos task sync <KEY> -F <walkthrough>` routes uncommitted edits through the identical Golden Commit pipeline.

---

## 5. Statutory Sensor Probes Activated

### 5.1 `staged_boundaries` (Cell 3.1 - Syntactic / Law 3)
- **Sensor**: `StagedBoundariesGate`
- **Invariant**: Blocks staged root scratch files outside `.tmp/` and enforces transient worktree containment.

### 5.2 `git_hygiene` (Cell 3.2 - Dynamic / Law 3 & Law 1)
- **Sensor**: `GitSubprocessHygieneGate`
- **Invariant**: Requires `workspace.SafeGitCommand` or `workspace.RunSafeGit` for every Go `git` subprocess outside `core/workspace/git.go`.

### 5.3 `commit_schema` (Cell 6.1 - Syntactic / Law 6)
- **Sensor**: `CommitSchemaGate`
- **Invariant**: Validates Conventional Commit subjects, plain-text rationale prose, zero diff dumps, and canonical `Task:` / `Twin:` Git trailers across `trunk..HEAD`.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Evaluation Subject | Condition | Expected Outcome |
| :--- | :--- | :--- | :--- |
| `TC-HOOK-01` | Raw `git commit` in worktree | `NOMOS_COMMIT_IN_PROGRESS` unset | Blocked (exit 1); directs caller to `nomos task commit`. |
| `TC-HOOK-02` | `nomos task commit` | Valid rationale and `Task:` trailer | Passes `pre-commit` and `commit-msg`; records Golden Commit. |
| `TC-GATE-01` | `staged_boundaries` | `walkthrough.md` staged at repo root | Fails closed; scratch files belong inside `.tmp/`. |
| `TC-GATE-02` | `git_hygiene` | AST contains `exec.Command("git", "status")` | Fails closed with exact file path and line number. |
| `TC-GATE-03` | `commit_schema` | Commit missing `Task:` trailer or carrying `[KEY]` subject | Fails closed with `nomos task commit` remedy. |
| `TC-HYD-01` | `nomos task commit` without flags | Task has 3-section card, twin, and walkthrough | Emits `<=72`-rune subject, deduplicated 72-col prose, `Task:`, and `Twin:` trailers. |
| `TC-HYD-02` | `commit_schema` & `commit-msg` | Commit contains `- M file.go` dump or canned shim | Fails closed under Law 2 and Law 4. |
