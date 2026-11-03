---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Hermetic Git Hooks, Task Commit, and Staged Boundary Enforcement Specification"
type: spec
version: "4.1.0"
tags:
    - INT-734
    - SUB-1212
    - INT-874
    - SUB-1582
ssot_repository: nomos
---

# Hermetic Git Hooks, Task Commit, and Staged Boundary Enforcement Specification

## 1. Executive Summary & Problem Statement

Under **Law 1 (The Dual-Core Inversion Law)** and **Law 3 (The Hermetic Worktree Law)**, the substrate Go core governs state and execution while the Tier 1 coding agent operates strictly inside isolated task worktrees under indirect control.

In uncontained development environments, agents or developers may inadvertently:
1. Attempt raw `git commit` on protected trunk branches (`main`), corrupting canonical history.
2. Introduce un-sanitized `exec.Command("git", ...)` invocations that inherit host environment variables (`GIT_DIR`, `GIT_WORK_TREE`), causing cross-worktree state leakage.
3. Write transient files (e.g., walkthroughs or scratch logs) directly to the repository root instead of the repository-local `.tmp/` containment boundary.
4. Omit Dual-Layer commit metadata required by downstream cognitive consumers and release receipts.

This specification unifies the **Hermetic Git Hooks Lifecycle**, the canonical **`nomos task commit`** CLI command, and the activation of three statutory sensor probes: **`staged_boundaries`**, **`git_hygiene`**, and **`commit_schema`**.

---

## 2. Axiomatic Invariants & Laws Codified

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Substrate Go core governs execution; agents commit via canonical CLI rather than unmediated git. | `nomos task commit`, `git_hygiene` AST probe |
| **2** | **The Non-Verbal Proof Law** | Zero trust in conversational commit messages; Dual-Layer schema is verified before merge. | `commit_schema` gate, table-driven tests |
| **3** | **The Hermetic Worktree Law** | Mutations occur strictly in transient worktrees; repository root is immutable. | `pre-commit` hook, `staged_boundaries` gate |
| **4** | **The Single Source of Truth Law** | Commit schema format and hook scripts have one compiled Go SSoT. | `substrate/src/nomos/modules/verify/commit_schema_gate.go`, `substrate/src/nomos/core/workspace/hooks.go` |
| **5** | **The Zero-Hardcoded-Path Law** | Dynamic resolution of worktrees, hooks, and repository roots. | `config.ResolveWorktreeDir()`, `FindRepoRoot()` |
| **6** | **The Canonical Singularity Law** | Single canonical commit workflow: `nomos task commit`. Raw git commits are intercepted and blocked. | Zero-Alias pre-commit discipline |
| **7** | **The Fail-Closed Pre-Flight Law** | Boundary escapes, missing commit headers, or un-sanitized git calls fail closed. | Gate pipeline exit code 1 |

---

## 3. The Live Git Hooks Architecture

Nomos installs and maintains three hermetic client-side hooks within `.git/hooks/`:

### 3.1 `pre-commit`
- **Root Protection**: Rejects direct commits if the active branch is a protected trunk branch (`main`, `master`, `intent`, `substrate`).
- **Worktree Boundary Enforcement**: Verifies that the current working directory is inside `worktrees/nomos-<KEY>`.
- **Boundary Escape Audit**: Inspects `git diff --cached --name-only` to ensure no staged files escape to the repository root or parent directories (`..`).
- **Workflow Enforcement**: Fails closed unless the `NOMOS_COMMIT_IN_PROGRESS=1` environment variable is exported by `nomos task commit`.
- **Pre-Flight DoD Verification**: Executes `nomos verify` across active gates.

### 3.2 `commit-msg`
- **First-Line Format**: Validates that line 1 matches `^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]\s+(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+` and rejects duplicate trailing `[KEY]` or `(KEY)` tokens.
- **Dual-Layer Sections & Anti-Shim Guard**: Requires the markdown section headers `**Architectural Rationale:**`, `**Blast Radius & Contracts:**`, and `**Resolution Mechanics:**` with substantive content, explicitly rejecting canned boilerplate placeholder lines.
- **Rejection**: Fails with clear remediation instructions directing the agent to `nomos task commit` or `nomos task sync -F <walkthrough>`.

### 3.3 `pre-push`
- **Trunk Push Defense**: Prevents pushing incomplete task branches directly to remote trunk without prior 2PC atomic release (`nomos task sync`).

---

## 4. Canonical CLI Interface: `nomos task commit`

```text
Usage:
  nomos task commit [TASK_KEY] [flags]

Flags:
  -m, --message string        Commit summary line (normalized to Conventional Commit format)
      --rationale string      Architectural rationale (overrides Living Ledger Scope & Context)
      --blast-radius string   Blast radius & affected contracts (overrides staged file manifest)
      --mechanics string      Resolution mechanics (overrides walkthrough / Acceptance Criteria)
  -F, --file string           Optional walkthrough markdown file to hydrate commit sections
  -a, --all                   Automatically stage modified tracked files (default: true)
  -d, --dir string            Worktree working directory (default: current directory)
```

### 4.1 Deterministic Commit Context Hydration & Subject Normalization (`INT-874`)
Static canned placeholder bodies (such as `"satisfy <KEY> deterministic execution requirements"` or `"scoped strictly to <KEY> implementation files"`) are strictly prohibited under **Law 2 (The Non-Verbal Proof Law)**.

1. **Task Key Resolution**: If `TASK_KEY` is omitted, the command extracts the canonical `INT-<N>` or `SUB-<N>` key from the active worktree directory or `feature/<KEY>` branch.
2. **Subject Normalization**:
   - Strips redundant leading `[<KEY>]` or trailing `(<KEY>)` / `[<KEY>]` tokens from `--message` (or falls back to the Living Ledger task `Title`).
   - Preserves explicit `feat|fix|docs|chore|refactor|test` prefixes, or deterministically synthesizes `<type>(<scope>): <summary>` from plane/verb/staged package paths when a plain imperative title is provided.
3. **Three-Section Priority Cascade (Zero Shims)**:
   - **`**Architectural Rationale:**`**: Explicit `--rationale` $\rightarrow$ Living Ledger 3-section card `## Scope & Context` $\rightarrow$ walkthrough summary $\rightarrow$ concrete chore title and package context.
   - **`**Blast Radius & Contracts:**`**: Explicit `--blast-radius` $\rightarrow$ staged Git path manifest (`git diff --cached --name-status`) highlighting touched schemas, specs, and packages.
   - **`**Resolution Mechanics:**`**: Explicit `--mechanics` $\rightarrow$ walkthrough release notes (`-F` or `.tmp/walkthrough-<KEY>.md`) $\rightarrow$ Living Ledger card `## Acceptance Criteria` deliverables $\rightarrow$ staged per-file diffstat (`git diff --cached --numstat`).
4. **Dirty Worktree Sync Parity**: When `nomos task sync <KEY> -F <walkthrough>` auto-commits uncommitted worktree edits, `autoCommitDirtyWorktree` forwards both the `TaskRecord` and `walkthroughPath` into the hydration pipeline so trunk commits always carry real rationale, file blast radius, and walkthrough mechanics.
5. **Hermetic Execution**: Sets `NOMOS_COMMIT_IN_PROGRESS=1`, stages modified files if `--all` is set, and commits via `workspace.SafeGitCommand`.

---

## 5. Statutory Sensor Probes Activated

### 5.1 `staged_boundaries` (Cell 3.1 - Syntactic / Law 3)
- **Sensor**: `StagedBoundariesGate`
- **Target**: Staged files via `git status --porcelain`.
- **Invariant**: No files outside `worktrees/` may be staged on protected branches; scratch files and walkthroughs must reside strictly in repository-local `.tmp/`.

### 5.2 `git_hygiene` (Cell 3.2 - Dynamic / Law 3 & Law 1)
- **Sensor**: `GitSubprocessHygieneGate`
- **Target**: Go AST across all packages in `substrate/`.
- **Invariant**: All git executions must use `workspace.SafeGitCommand` or `workspace.RunSafeGit`. Any direct call to `exec.Command("git", ...)` or `exec.CommandContext(..., "git", ...)` fails closed.
- **Exemptions**: `substrate/src/nomos/core/workspace/git.go` (the canonical implementation itself).

### 5.3 `commit_schema` (Cell 6.1 - Syntactic / Law 6)
- **Sensor**: `CommitSchemaGate`
- **Target**: `HEAD` commit message via `git log -1 --format=%B`.
- **Invariant**: Validates canonical `[INT-N]` or `[SUB-N]` prefix and required Dual-Layer section headers.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Evaluation Subject | Condition | Expected Outcome |
| :--- | :--- | :--- | :--- |
| `TC-HOOK-01` | Raw `git commit` in worktree | `NOMOS_COMMIT_IN_PROGRESS` unset | Blocked with exit code 1; remediation: use `nomos task commit`. |
| `TC-HOOK-02` | `nomos task commit` | Valid message and headers | Success; pre-commit hook passes and commit is recorded. |
| `TC-GATE-01` | `staged_boundaries` | `walkthrough.md` staged at repo root | Fails closed: scratch files must reside in `.tmp/`. |
| `TC-GATE-02` | `git_hygiene` | AST contains `exec.Command("git", "status")` | Fails closed with file path and line number violation. |
| `TC-GATE-03` | `git_hygiene` | AST uses `workspace.SafeGitCommand(dir, "status")` | Passes with clean certification. |
| `TC-GATE-04` | `commit_schema` | Commit subject missing `[TASK_KEY]` | Fails closed: missing canonical prefix. |
| `TC-GATE-05` | `commit_schema` | Commit message with valid prefix and headers | Passes with clean certification. |
| `TC-HYD-01` | `nomos task commit` without body flags | Backlog task has 3-section card + staged diff | Hydrates Rationale from `Scope & Context`, Blast Radius from staged paths, and Mechanics from `Acceptance Criteria` / walkthrough (zero canned shims). |
| `TC-HYD-02` | `nomos task sync -F walkthrough.md` | Dirty worktree auto-commit | Hydrates Mechanics from `walkthrough.md`, Rationale from task card, and Blast Radius from staged files. |
| `TC-HYD-03` | Subject normalization | `-m "[SUB-9] docs(readme): update [SUB-9]"` | Deduplicates key tokens to `[SUB-9] docs(readme): update`. |
| `TC-HYD-04` | `commit_schema` & `commit-msg` | Commit contains canned boilerplate shim text | Fails closed (`Anti-Shim Commit Guard`). |
