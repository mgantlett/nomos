---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-16"
last_updated: "2026-11-05"
plane: intent
ssot_repository: nomos
status: active
title: "Zero-Source Downstream Developer Experience & Agentic HATEOAS Specification"
trigger: always_on
type: spec
version: "4.0.0"
execution_frontier:
  tasks:
    - key: "INT-840"
      plane: "intent"
      summary: "Agentic HATEOAS Transition Envelopes & Frontmatter Directive Matching"
    - key: "INT-841"
      plane: "intent"
      summary: "Downstream-Neutral Protocol Template, Scrubbed Directives & Marker-Scoped Byte Parity"
    - key: "INT-842"
      plane: "intent"
      summary: "Downstream-Safe Gate Remedies, Capabilities Drill-Down & CDV HATEOAS Trajectory Suite"
---

# Zero-Source Downstream Developer Experience & Agentic HATEOAS Specification

**Document Key:** `SPEC-ZERO-SOURCE-HATEOAS`.  
**Classification:** Public Open-Core Specification.  
**Version:** `4.0.0`.  
**Governing Laws:** Law 1 (Dual-Core Inversion), Law 2 (Non-Verbal Proof), Law 3 (Hermetic Worktrees), and Law 4 (SSoT). Also enforces Law 5 (Zero-Hardcoded-Path), Law 6 (Canonical Singularity), and Law 7 (Fail-Closed Pre-Flight).

---

## 1. Problem Statement & Architectural Motivation

Downstream developers install only the precompiled `nomos` binary inside external polyglot repositories.
Downstream repositories contain zero Nomos Go source code and zero Nomos internal architecture specifications.
Unconstrained Tier 1 coding agents waste context tokens hunting for missing source files when CLI responses omit next-step affordances.

This specification codifies **Hypermedia as the Engine of Agentic State (Agentic HATEOAS)** for zero-source downstream onboarding.
Every CLI state query and phase transition emits pre-interpolated next-step commands directly in stdout and structured JSON.
Downstream Tier 1 coding agents achieve full phase discipline in under 60 seconds using only embedded operator directives and CLI envelopes.

---

## 2. Axiomatic Invariants

| Law # | Law Name | Zero-Source Downstream Invariant |
| :---: | :--- | :--- |
| **1** | **Dual-Core Inversion** | The `nomos` binary governs phase transitions via deterministic HATEOAS command affordances. |
| **2** | **Non-Verbal Proof** | Clean-room downstream vector (`CDV`) suites verify zero-source trajectories with exit code 0. |
| **3** | **Hermetic Worktrees** | Protected trunk branches remain read-only; mutations execute strictly in isolated worktrees. |
| **4** | **Single Source of Truth** | Embedded directives match workspace state through parsed `#DirectiveFrontmatter.phases` metadata. |
| **5** | **Zero-Hardcoded-Path** | Envelopes interpolate dynamic repository and worktree paths without host path literals. |
| **6** | **Canonical Singularity** | Affordances emit exact canonical CLI commands with zero aliases or duplicate subcommands. |
| **7** | **Fail-Closed Pre-Flight** | Missing prerequisites terminate execution with deterministic remediation commands. |

---

## 3. Agentic HATEOAS Transition Envelopes (`INT-840`)

Every lifecycle phase transition and orientation command emits valid next-step hypermedia commands (`#ActionAffordance`).

### 3.1 Compiled Go SSoT Contract (`core/types/affordance.go` & `phase.go`)

```go
package types

// ActionAffordance represents a deterministic HATEOAS next-step command.
type ActionAffordance struct {
	Command string `json:"command"`
	Summary string `json:"summary"`
	Phase   Phase  `json:"phase,omitempty"`
}
```

`#HandshakeEvent` and `#PhaseState` expose `next_actions?: [...#ActionAffordance]` for machine-readable `--json` consumers and terminal banners.

### 3.2 Phase-Driven Affordance Matrix

| Active Phase | Emitted `next_actions` Canonical Commands |
| :--- | :--- |
| **`IDLE`** | `nomos task chore "<title>"` · `nomos task start <KEY> --edit` · `nomos directive <active>` |
| **`PLAN`** | `cd <worktree>` · `nomos task edit <KEY>` · `nomos directive <active>` |
| **`EDIT`** | `cd <worktree>` · `nomos task commit <KEY> -m "<summary>"` · `nomos verify` · `nomos task sync <KEY> -F .tmp/walkthrough-<KEY>.md` |
| **`REVIEW`** | `nomos task sync <KEY> -F .tmp/walkthrough-<KEY>.md` · `nomos task edit <KEY>` |
| **`SYNC`** | `nomos handshake` · `nomos status` |

### 3.3 Frontmatter-Driven Directive Resolution

`LoadPhaseDirectives` parses YAML frontmatter (`#DirectiveFrontmatter.phases`) across local overrides and embedded substrate directives.
Every directive name emitted in `HandshakeEvent.directives` resolves with exit code 0 when passed to `nomos directive <name>`.
The substrate prohibits synthetic fallback identifiers that lack corresponding embedded directive payloads.

### 3.4 Downstream-Neutral Protocol Block & Marker-Scoped Parity (`INT-841`)

Foundry self-hosting architecture specifications remain strictly inside `intent/architecture/specs/` and never enter `embed.FS`.
Embedded directives and `templates/AGENTS.md` contain strictly repository-agnostic polyglot guidance (`worktrees/<repo>-<KEY>`) and advertise `nomos directive`, `nomos capabilities`, and `nomos task chore`.
`IntentParityGate` evaluates SHA-256 byte parity strictly over the `<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->` ... `<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->` managed region (`#ProtocolMarkerBounds`).
Downstream repositories append project-specific instructions outside the protocol markers without triggering Law 4 parity errors.
Ambient `worktree-guard.sh` permits `.tmp/` scratch and walkthrough file writes alongside `/worktrees/` mutations.

### 3.5 Downstream-Safe Gate Remedies & Capability Drill-Down (`INT-842`)

Gate failure remedies and defense hook rejections emit contradiction-free, repository-agnostic recovery commands.
Remedies never recommend commands prohibited by ambient guards (such as `git stash`) and never reference internal Foundry milestones.
`nomos capabilities [id]` accepts a capability ID, cell ID (`cell_7_1`), or probe ID (`plane_separation`).
It emits `#CapabilityDetail` with Law, Ring, invariant rules, and canonical remediation steps under Law 6.

---

## 4. The 60-Second Zero-Source Lifecycle

```text
T=0s   $ nomos init
         └── Scaffolds AGENTS.md, CLAUDE.md, nomos.yaml, .agents/hooks.json, .agents/scripts/
T=5s   $ nomos handshake
         └── Emits IDLE envelope, valid embedded directives, and Next Actions
T=15s  $ nomos task chore "Add payment validator"
         └── Mints SUB-1, creates worktree, enters EDIT, prints Next Actions with worktree path
T=35s  [Tier 1 agent edits files inside isolated task worktree]
T=45s  $ nomos verify
         └── Evaluates 42 DoD gates against polyglot workspace (exit code 0)
T=55s  $ nomos task sync SUB-1 -F .tmp/walkthrough-SUB-1.md
         └── Executes 2PC merge, writes Git Notes receipt, prunes worktree, emits IDLE Next Actions
```

---

## 5. Table-Driven Verification Vectors (`CDV-HATEOAS`)

| Vector ID | Clean-Room Step | Evaluated Invariant | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `CDV-HAT-01` | `nomos init` in external repo | Scaffolds downstream-clean protocol & hooks | **PASS** |
| `CDV-HAT-02` | `nomos handshake` in `IDLE` | Every emitted directive resolves via `nomos directive` | **PASS** |
| `CDV-HAT-03` | `nomos task start` / `chore` / `edit` | Emits pre-interpolated `Next:` commands & `next_actions` JSON | **PASS** |
| `CDV-HAT-04` | `nomos task commit` / `review` / `sync` | Completes 2PC lifecycle with valid next-step guidance | **PASS** |
| `CDV-HAT-05` | `nomos capabilities <id>` on cell or probe | Resolves `#CapabilityDetail` with Law, Ring, and Remedy | **PASS** |
| `CDV-HAT-06` | Audit all 42 gate remedies & hook scripts | Zero `git stash` contradictions or internal Foundry leaks | **PASS** |

---

## 6. Verification Gate

```bash
nomos verify
```
Must pass all 42 Definition of Done quality gates with exit code 0.
