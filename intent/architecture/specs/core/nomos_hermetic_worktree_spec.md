---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-06"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Hermetic Worktree and Git Hygiene Specification"
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Hermetic Worktree and Git Hygiene Specification

## 1. Executive Summary & Problem Statement
The Nomos Open-Core protocol enforces **Law 3 (The Hermetic Worktree Law)** to ensure that protected trunk branches (`main`, `master`, `intent`, `substrate`) remain physically immutable, and that all modifications occur strictly within transient, isolated task worktrees. Unrestricted mutations to trunk branches introduce untracked state drift, broken CI pipelines, and multi-agent collisions.

This specification formalizes the **Hermetic Worktree and Git Hygiene Subsystem** implemented in `substrate/src/nomos/core/workspace/`. It codifies the state machine governing isolated worktree lifecycle, git subprocess security wrappers (`SafeGitCommand`), dynamic root discovery, and automated DoD quality gate enforcement across the 42-gate verification pipeline.

## 2. Axiomatic Invariants & Laws Codified
- **Law 1: The Dual-Core Inversion Law**: The Go Substrate engine retains absolute governance over worktree creation, branch provisioning, and git executions. Tier 1 coding agents (AGY-IDE, Claude Code, OpenCode) execute within the hermetic boundary under indirect control. Direct shell invocations of raw `git` commands that bypass substrate envelopes are prohibited.
- **Law 2: The Non-Verbal Proof Law**: Worktree boundary integrity and subprocess sanitization are continuously verified via automated, deterministic table-driven test vectors.
- **Law 3: The Hermetic Worktree Law**: Protected trunk branches are guarded against direct mutation via Git pre-commit hooks and CapBAC boundary enforcement. Code modifications, dependency installs, and file generation must execute exclusively inside isolated task worktrees (`worktrees/nomos-<KEY>`).
- **Law 4: The Single Source of Truth Law**: Worktree configuration schemas are unified across planes: the Go structs in `nomos/core/workspace` and the compiled CUE schema `#WorktreeBoundaryConfig` in `intent/contracts/cue/worktree.cue`.
- **Law 5: The Zero-Hardcoded-Path Law**: Worktree directories and repository roots resolve dynamically via configuration discovery (`config.ResolveWorktreeDir()`, `FindRepoRoot()`). Hardcoded host home directory prefixes, container mount literals (`/app/...`), and literal workstation paths are strictly prohibited.
- **Law 6: The Canonical Singularity Law**: Subprocess executions are unified under the singular envelope `workspace.SafeGitCommand`. Ad-hoc `exec.Command("git", ...)` invocations are prohibited across the Substrate codebase.
- **Law 7: The Fail-Closed Pre-Flight Law**: Attempting to stage or commit files to the root repository while on a protected branch fails closed immediately with `ERR_HERMETIC_BREACH`.

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

## 4. Companion CUE Schema Contract
The declarative companion schema resides in `intent/contracts/cue/worktree.cue`.

```cue
package schemas

import "strings"

// WorktreeBoundaryConfig defines the contract for hermetic worktree boundaries and protected branches.
#WorktreeBoundaryConfig: close({
	protected_branches:  [...string]
	worktree_base_dir:   string & strings.MinRunes(1)
	exempt_patterns?:    [...string]
})

// WorktreeSession captures the runtime state of an isolated task worktree.
#WorktreeSession: close({
	task_key:      =~"^(INT|SUB)-[0-9]+$"
	branch_name:   string & strings.MinRunes(1)
	worktree_path: string & strings.MinRunes(1)
	created_at:    string
})
```

## 5. Substrate DoD Quality Gate Integration
The hermetic worktree and git hygiene invariants are enforced automatically during `nomos verify` via three dedicated Definition of Done quality gates:

| Gate Name | Pipeline Key | Associated Invariant | Enforcement Logic |
| :--- | :--- | :--- | :--- |
| **Hermetic Worktree Invariant** | `hermetic_worktree` | Law 3 | Verifies active execution occurs within an isolated worktree (`nomos-<KEY>`) rather than root repository trunk. |
| **Worktree Boundaries Invariant** | `staged_boundaries` | Law 3 / Law 7 | Scans staged and uncommitted files via `ValidateUncommittedBoundaries`. Fails closed if any root file (e.g. `walkthrough.md`) is modified on trunk. |
| **Git Subprocess Hygiene Invariant** | `git_hygiene` | Law 1 / Law 6 | AST static analysis across all Go packages: flags any un-sanitized `exec.Command("git", ...)` that bypasses `workspace.SafeGitCommand`. |

## 6. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Invariant / Boundary Evaluated | Input Scenario | Expected Deterministic Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-WT-001` | Direct mutation on protected branch | `ValidateWorktreeBoundary(root, "main", "root/file.go")` | Fails closed with `ERR_CORE`: "direct mutation of root repository on protected trunk branch is strictly prohibited". | PASS |
| `TC-WT-002` | Mutation within isolated task worktree | `ValidateWorktreeBoundary(root, "main", "root/worktrees/nomos-SUB-747/file.go")` | Passes boundary check (`err == nil`). Isolated execution permitted. | PASS |
| `TC-WT-003` | Root clutter violation (`walkthrough.md`) | Staging `walkthrough.md` directly at `repoRoot` | `ValidateUncommittedBoundaries` fails closed: "scratch files and walkthroughs must go in .tmp/". | PASS |
| `TC-WT-004` | Git subprocess environment sanitization | `SafeGitCommand(dir, "status")` | Environment variables sanitized; `GIT_DIR` and `GIT_WORK_TREE` overrides cleared to prevent host traversal. | PASS |

## 7. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-327` (Hermetic Worktree Architecture)
- **Substrate Plane**: `SUB-327` (`substrate/src/nomos/core/workspace/`)
- **DoD Gates**: Gate 2 (`hermetic_worktree`), Gate 3 (`staged_boundaries`), Gate 11 (`git_hygiene`)

## 8. Rigor & Verification Boundary
All specifications and state machines defined herein are continuously certified by:
1. `nomos verify` evaluating CUE contract schema integrity and AST parity.
2. `nomos verify` enforcing all automated Definition of Done quality gates.
