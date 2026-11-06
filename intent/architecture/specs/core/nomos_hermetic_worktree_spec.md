---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-06"
last_updated: "2026-11-05"
plane: intent
ssot_repository: nomos
status: active
title: "Hermetic Worktree and Git Hygiene Specification"
type: spec
version: "4.0.0"
---

# Hermetic Worktree and Git Hygiene Specification

## 1. Executive Summary & Problem Statement
The Nomos Open-Core protocol enforces **Law 3 (The Hermetic Worktree Law)** across protected trunk branches (`main`, `master`, `intent`, `substrate`). Protected trunks remain physically immutable, and all mutations execute inside isolated task worktrees. Unrestricted trunk mutations introduce untracked state drift, broken CI pipelines, and multi-agent collisions.

This specification formalizes the **Hermetic Worktree and Git Hygiene Subsystem** in `substrate/src/nomos/core/workspace/`. It codifies the worktree lifecycle state machine, `SafeGitCommand` wrappers, dynamic root discovery, and 42-gate DoD verification.

## 2. Axiomatic Invariants & Laws Codified
- **Law 1: The Dual-Core Inversion Law**: The Go Substrate engine retains absolute governance over worktree creation, branch provisioning, and git executions. Tier 1 coding agents (AGY-IDE, Claude Code, OpenCode) execute within the hermetic boundary under indirect control. The substrate prohibits raw `git` shell invocations that bypass substrate envelopes.
- **Law 2: The Non-Verbal Proof Law**: Automated, deterministic table-driven verification vectors continuously prove worktree boundary integrity and subprocess sanitization.
- **Law 3: The Hermetic Worktree Law**: Git pre-commit hooks and CapBAC boundary checks guard protected trunk branches against direct mutation. Code modifications, dependency installs, and file generation must execute exclusively inside isolated task worktrees (`worktrees/nomos-<KEY>`).
- **Law 4: The Single Source of Truth Law**: Worktree configuration schemas bind directly to compiled Go structs in `nomos/core/workspace` and `nomos/core/config`, reflected into `nomos.schema.json`.
- **Law 5: The Zero-Hardcoded-Path Law**: Worktree directories and repository roots resolve dynamically via configuration discovery (`config.ResolveWorktreeDir()`, `FindRepoRoot()`). The substrate strictly prohibits hardcoded host home prefixes, container mount literals (`/app/...`), and literal workstation paths.
- **Law 6: The Canonical Singularity Law**: `workspace.SafeGitCommand` unifies all git subprocess executions under one canonical envelope. The substrate prohibits ad-hoc `exec.Command("git", ...)` calls across all Go packages.
- **Law 7: The Fail-Closed Pre-Flight Law**: Staging or committing root repository files on a protected branch immediately aborts with `ERR_HERMETIC_BREACH`.

## 3. Compiled Go SSoT Contracts
The Go Single Source of Truth resides in `substrate/src/nomos/core/workspace/`.

```go
package workspace

import (
	"os/exec"
)

// ProtectedTrunkBranches defines protected branches that cannot be modified directly in the root repository.
var ProtectedTrunkBranches = []string{"main", "master", "intent", "substrate"}

// Manager orchestrates git worktrees for tasks.
type Manager struct {
	RepoRoot string `json:"repo_root"`
}

// FindRepoRoot discovers the root git repository directory containing the primary .git directory,
// correctly resolving through transient worktrees and nested subdirectories.
func FindRepoRoot(startDir string) (string, error)

// SafeGitCommand constructs an exec.Cmd with sanitized environment variables, preventing
// command injection and ensuring strict hermetic execution across worktrees.
func SafeGitCommand(dir string, args ...string) *exec.Cmd

// RunSafeGit executes a git command via SafeGitCommand and returns trimmed stdout.
func RunSafeGit(dir string, args ...string) (string, error)

// ValidateWorktreeBoundary ensures that mutations during a task execute strictly within
// an isolated worktree when on a protected branch (Law 3).
func ValidateWorktreeBoundary(repoRoot, currentBranch, targetFile string) error

// ValidateUncommittedBoundaries checks that none of the mutated files violate worktree isolation boundaries.
func ValidateUncommittedBoundaries(repoRoot, currentBranch string, uncommittedFiles []string) error

// LockTrunk sets read-only filesystem permissions on the root repository trunk (Law 3).
func LockTrunk(repoRoot string) error

// UnlockTrunk temporarily enables write permissions on the root repository strictly for 2PC fast-forward merges.
func UnlockTrunk(repoRoot string) error
```

## 4. Worktree Configuration & Session DTOs
The configuration and session structs reside in `substrate/src/nomos/core/config/` and `substrate/src/nomos/core/workspace/`:

```go
package workspace

// WorktreeBoundaryConfig defines hermetic worktree boundaries and protected branches.
type WorktreeBoundaryConfig struct {
	ProtectedBranches []string `json:"protected_branches"`
	WorktreeBaseDir   string   `json:"worktree_base_dir"`
	ExemptPatterns    []string `json:"exempt_patterns,omitempty"`
}

// WorktreeSession captures the runtime state of an isolated task worktree.
type WorktreeSession struct {
	TaskKey      string `json:"task_key"`
	BranchName   string `json:"branch_name"`
	WorktreePath string `json:"worktree_path"`
	CreatedAt    string `json:"created_at"`
}
```

## 5. Substrate DoD Quality Gate Integration
During `nomos verify`, three dedicated Definition of Done quality gates enforce hermetic worktree and git hygiene invariants:

| Gate Name | Pipeline Key | Associated Invariant | Enforcement Logic |
| :--- | :--- | :--- | :--- |
| **Hermetic Worktree Invariant** | `hermetic_worktree` | Law 3 | Verifies execution inside an isolated `nomos-<KEY>` worktree rather than trunk. |
| **Worktree Boundaries Invariant** | `staged_boundaries` | Law 3 / Law 7 | Runs `ValidateUncommittedBoundaries` and rejects root file mutations on trunk. |
| **Git Subprocess Hygiene Invariant** | `git_hygiene` | Law 1 / Law 6 | Scans Go ASTs and rejects `exec.Command("git", ...)` bypassing `workspace.SafeGitCommand`. |

## 6. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Invariant / Boundary Evaluated | Input Scenario | Expected Deterministic Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-WT-001` | Direct mutation on protected branch | `ValidateWorktreeBoundary(root, "main", "root/file.go")` | Rejects root trunk mutation with `ERR_CORE` boundary error. | PASS |
| `TC-WT-002` | Mutation within isolated task worktree | `ValidateWorktreeBoundary(root, "main", "root/worktrees/nomos-SUB-747/file.go")` | Passes boundary validation (`err == nil`) for isolated execution. | PASS |
| `TC-WT-003` | Root clutter violation (`walkthrough.md`) | Staging `walkthrough.md` directly at `repoRoot` | `ValidateUncommittedBoundaries` rejects root scratch files outside `.tmp/`. | PASS |
| `TC-WT-004` | Git subprocess environment sanitization | `SafeGitCommand(dir, "status")` | Sanitizes environment and clears `GIT_DIR` and `GIT_WORK_TREE` overrides. | PASS |

## 7. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-327` (Hermetic Worktree Architecture)
- **Substrate Plane**: `SUB-327` (`substrate/src/nomos/core/workspace/`)
- **DoD Gates**: Gate 2 (`hermetic_worktree`), Gate 3 (`staged_boundaries`), Gate 11 (`git_hygiene`)

## 8. Rigor & Verification Boundary
The substrate continuously certifies every specification and state machine defined herein via:
1. `nomos verify` evaluating Go SSoT contract schema parity and AST traceability.
2. `nomos verify` enforcing all automated Definition of Done quality gates.
