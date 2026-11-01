---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-08"
last_updated: "2026-11-01"
plane: intent
status: active
title: Actionable JIT Contextual Learning & Interactive Triage Worktrees
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Actionable JIT Contextual Learning & Interactive Triage Worktrees

## 1. Executive Summary & Problem Statement
In conventional developer tooling, a build failure, broken invariant, or failed quality gate is a terminal dead-end: a stack trace is dumped, and the session halts. In the Nomos autonomous pair-programming paradigm, a constraint violation is not a dead-end, but a **conversational and diagnostic pivot**. When an execution failure occurs, `NomosError` produces a deterministic triage identifier (`TriageHash`, formatted as `ERR-[a-f0-9]+`). 

This specification establishes the **Actionable JIT Contextual Learning and Interactive Triage Worktree** subsystem. By invoking `nomos task triage <triageHash>`, the Substrate dynamically provisions an isolated, transient diagnostic worktree pre-loaded with the exact Just-In-Time (JIT) semantic intent context (`ContextFiles`) and remediation instructions (`InteractivePrompt`), empowering the Tier 1 cognitive agent to debug and iterate without mutating the protected trunk or polluting task backlogs.

## 2. Axiomatic Invariants & Laws Codified
- **Law 1: The Dual-Core Inversion Law**: The Go Substrate engine retains absolute governance over worktree creation, branch provisioning, and context injection. The Tier 1 coding agent operates strictly as a bounded cognitive worker within the scaffolded triage sandbox.
- **Law 3: The Hermetic Worktree Law**: Protected trunk branches (`main`) are physically immutable. Interactive triage sessions execute strictly within isolated transient worktrees (`worktrees/nomos-triage-<hash>`) on transient branches (`triage/<hash>`).
- **Law 4: The Single Source of Truth Law**: Triage session metadata is governed by unified contracts across planes: the Go struct `TriageSession` in `substrate/src/nomos/core/workspace/` and the compiled CUE schema `#TriageSession` in `intent/contracts/cue/triage.cue`.
- **Law 5: The Zero-Hardcoded-Path Law**: Worktree destination directories and repository roots resolve dynamically via workspace discovery (`resolveWorkspaceRoot()`, `config.ResolveWorktreeDir()`). Path literals and hardcoded workstation directories are strictly prohibited.
- **Law 6: The Canonical Singularity Law**: Triage sessions are transient diagnostic sandboxes, NOT canonical tasks. Task keys strictly remain `^(INT|SUB)-[0-9]+$`. Direct synchronization to trunk (`nomos task sync`) from a `triage/*` branch is strictly prohibited. To merge fixes, the work must be explicitly promoted or bound to a canonical task (`SUB-<N>` or `INT-<N>`).
- **Law 7: The Fail-Closed Pre-Flight Law**: Quality gates are never bypassed, weakened, or silenced during triage verification. While transient work is in progress within the triage sandbox, attempting to sync without full 29-gate certification fails closed.

## 3. Compiled Go DTO Contract
The Go Single Source of Truth resides in `substrate/src/nomos/core/workspace/` and maps directly to CLI command handlers.

```go
package workspace

// TriageSession captures the runtime state of an isolated diagnostic worktree.
// @intent INT-531
// @intent SUB-563
type TriageSession struct {
	TriageHash   string   `json:"triage_hash"`             // Deterministic error hash (e.g., ERR-9b2f4c)
	BranchName   string   `json:"branch_name"`             // Transient git branch (e.g., triage/ERR-9b2f4c)
	WorktreePath string   `json:"worktree_path"`           // Dynamically resolved absolute path to worktree
	ContextFiles []string `json:"context_files"`           // Injected JIT intent documentation paths
	SystemPrompt string   `json:"system_prompt,omitempty"` // Contextual remediation prompt for Tier 1 agent
	CreatedAt    string   `json:"created_at"`              // RFC3339 timestamp of triage session creation
	PromotedTask string   `json:"promoted_task,omitempty"` // Canonical task key (INT-X or SUB-X) upon promotion
}

// TriageProvisionOptions provides input parameters for scaffolding a triage sandbox.
type TriageProvisionOptions struct {
	WorkspaceRoot string
	TriageHash    string
	ContextFiles  []string
	SystemPrompt  string
}
```

## 4. Companion CUE Schema Contract
The declarative companion schema resides in `intent/contracts/cue/triage.cue`.

```cue
package schemas

import "strings"

// TriageSession defines the declarative contract for an interactive triage worktree.
#TriageSession: close({
	triage_hash:    string & =~"^ERR-[a-f0-9]+$"
	branch_name:    string & =~"^triage/ERR-[a-f0-9]+$"
	worktree_path:  string & strings.MinRunes(1)
	context_files:  [...string]
	system_prompt:  string
	created_at:     string
	promoted_task?: string & =~"^(INT|SUB)-[0-9]+$"
})
```

## 5. Substrate CLI Command & Execution State Machine
The interactive triage engine is invoked via `nomos task triage [triageHash]`, implemented in `substrate/src/nomos/modules/handshake/triage.go`.

```
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                        INTERACTIVE TRIAGE STATE MACHINE                                │
├────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                        │
│   [Execution Failure / Gate Violation]                                                │
│              │                                                                         │
│              ▼                                                                         │
│   [NomosError Emits TriageHash: ERR-9b2f4c]                                            │
│              │                                                                         │
│              ▼                                                                         │
│   `nomos task triage ERR-9b2f4c`                                                       │
│              │                                                                         │
│              ├─► 1. Pre-Flight Validation: Verify hash matches `^ERR-[a-f0-9]+$`      │
│              │                                                                         │
│              ├─► 2. Dynamic Discovery: Resolve root via `resolveWorkspaceRoot()`       │
│              │                                                                         │
│              ├─► 3. Hermetic Provisioning: `workspace.RunSafeGit` creates               │
│              │      branch `triage/ERR-9b2f4c` & worktree `worktrees/nomos-triage-...` │
│              │                                                                         │
│              ├─► 4. JIT Context Injection: Injects `ContextFiles` into worktree        │
│              │                                                                         │
│              ▼                                                                         │
│   [Tier 1 Agent Diagnostic Sandbox (EDIT/DEBUG)]                                       │
│              │                                                                         │
│              ├─► `nomos verify` (Runs all 42 DoD gates in worktree)                    │
│              │                                                                         │
│              ├─► Direct `nomos task sync` ──►  REJECTED (Law 6 & Law 7 Fail-Closed)   │
│              │                                                                         │
│              ▼                                                                         │
│   (phantom: [Promotion to Canonical Task: `nomos task promote --task SUB-755`])                  │
│              │                                                                         │
│              ▼                                                                         │
│   [Canonical Sync: `nomos task sync SUB-755 -F .tmp/walkthrough.md`]                   │
│                                                                                        │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### 5.1 Step-by-Step Lifecycle Protocol
1. **Hash Validation (Fail-Closed Pre-Flight)**: The Substrate validates that `triageHash` conforms strictly to `^ERR-[a-f0-9]+$`. Invalid or malicious inputs are rejected before executing any git subprocess.
2. **Dynamic Root Resolution (Law 5)**: The CLI traverses upward from the current working directory to locate `.git`, determining `workspaceRoot` dynamically without hardcoded path literals.
3. **Idempotent Worktree Provisioning (Law 3)**:
   - Branch: `triage/<triageHash>`
   - Worktree Path: `filepath.Join(workspaceRoot, "worktrees", fmt.Sprintf("nomos-triage-%s", triageHash))`
   - Invokes `workspace.RunSafeGit(workspaceRoot, "worktree", "add", "-b", triageBranch, worktreePath)`. If the branch already exists, it checks out the existing branch cleanly.
4. **JIT Context Injection**: Context files specified in the triggering `NomosError.ContextFiles` are surfaced directly into the Tier 1 agent's active instruction set.
5. **Promotion Boundary Enforcement (Law 6 & Law 7)**:
   - Attempting to execute `nomos task sync` within a triage worktree or on a `triage/*` branch fails closed with `ERR_CANONICAL_SINGULARITY`.
   - To merge verified code changes into trunk, the developer or agent must bind the work to a canonical task key (`INT-<N>` or `SUB-<N>`) via (phantom: `nomos task promote --task <key>`), ensuring full traceability and 2-phase commit (2PC) verification.

## 6. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Input Triage Hash | Initial Repository State | Expected State Machine Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-TRG-001` | `ERR-1a2b3c` | Clean repo, branch does not exist | Provisions branch `triage/ERR-1a2b3c` and worktree `worktrees/nomos-triage-ERR-1a2b3c`. Injects context files. Returns exit code 0. | PASS |
| `TC-TRG-002` | `ERR-1a2b3c` | Branch `triage/ERR-1a2b3c` already exists | Idempotently attaches existing branch to worktree without error. Returns exit code 0. | PASS |
| `TC-TRG-003` | `invalid-hash-99` | Clean repo | Fails closed during pre-flight hash validation. Emits `ERR_PREFLIGHT_FAILED`. Worktree is not provisioned. Exit code 1. | PASS |
| `TC-TRG-004` | `ERR-1a2b3c` | Active worktree on `triage/ERR-1a2b3c` | Direct `nomos task sync` rejected with `ERR_CANONICAL_SINGULARITY`. After `nomos task promote --task SUB-755`, sync proceeds through DoD gates. | PASS |

## 7. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-531` (Contextual Learning), `INT-532` (Interactive Triage Worktrees)
- **Substrate Plane**: `SUB-563` (`substrate/src/nomos/modules/handshake/triage.go`)
- **DoD Gates**: Gate 2 (Hermetic Worktree), Gate 4 (Canonical Singularity), Gate 9 (Forbidden Literals), Gate 14 (Broken Wire), Gate 29 (Dead-Code Reachability)

## 8. Rigor & Verification Boundary
All specifications and state machines defined herein are continuously certified by:
1. `nomos verify` evaluating CUE contract schema integrity and AST parity.
2. `nomos verify` enforcing all automated DoD quality gates.
