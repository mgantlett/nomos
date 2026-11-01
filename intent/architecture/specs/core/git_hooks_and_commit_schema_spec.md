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
version: "4.0.0"
tags:
    - INT-734
    - SUB-1212
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
| **4** | **The Single Source of Truth Law** | Commit schema format and hook scripts have one compiled Go SSoT with CUE contract parity. | `commit_rules.cue`, `substrate/src/nomos/core/workspace/hooks.go` |
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
- **First-Line Format**: Validates that line 1 matches `^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]\s+[a-z]+(\([a-z0-9_-]+\))?:`.
- **Dual-Layer Sections**: Requires at least one of the markdown section headers:
  - `**Architectural Rationale:**`
  - `**Blast Radius & Contracts:**`
  - `**Resolution Mechanics:**`
- **Rejection**: Fails with clear remediation instructions directing the agent to `nomos task commit`.

### 3.3 `pre-push`
- **Trunk Push Defense**: Prevents pushing incomplete task branches directly to remote trunk without prior 2PC atomic release (`nomos task sync`).

---

## 4. Canonical CLI Interface: `nomos task commit`

```text
Usage:
  nomos task commit [TASK_KEY] [flags]

Flags:
  -m, --message string        Commit summary line
      --rationale string      Architectural rationale
      --blast-radius string   Blast radius & affected contracts
      --mechanics string      Resolution mechanics and algorithmic steps
  -a, --all                   Automatically stage modified tracked files (default: true)
  -d, --dir string            Worktree working directory (default: current directory)
```

### 4.1 Resolution Flow
1. **Task Key Resolution**: If `TASK_KEY` is omitted, the command extracts the key from the current working directory name (`worktrees/nomos-<KEY>`).
2. **Metadata Lookup**: Queries `SQLiteTaskStore` to retrieve task title, plane, and twin key.
3. **Dual-Layer Formatting**: Assembles the standardized commit message:
   ```text
   [<KEY>] <summary>

   **Architectural Rationale:**
   <rationale>

   **Blast Radius & Contracts:**
   <blastRadius>

   **Resolution Mechanics:**
   <mechanics>
   ```
4. **Hermetic Execution**: Sets `NOMOS_COMMIT_IN_PROGRESS=1`, stages modified files if `--all` is set, and commits via `workspace.SafeGitCommand`.

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
