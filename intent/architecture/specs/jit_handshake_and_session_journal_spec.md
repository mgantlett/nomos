---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-03"
lineage:
  - INT-736
  - INT-896
  - SUB-1214
plane: intent
ssot_repository: nomos
status: active
title: "Just-In-Time (JIT) Handshake Dynamic Injection and Release Receipt Distillation Specification"
type: spec
version: "4.2.0"
---

# Just-In-Time (JIT) Handshake Dynamic Injection and Release Receipt Distillation Specification

---

## 1. Executive Summary & Problem Statement

Under **Law 1 (Dual-Core Inversion)**, the Go core governs state and execution while Tier 1 coding agents operate under bounded indirect control.

In stateless or poorly bounded agentic workflows, cognitive models frequently suffer from:
1. **Context Amnesia & Hallucination:** Agents lack visibility into current workspace state, active phase, remaining step budgets, and database health.
2. **Phase Boundary Drift:** Agents attempt file mutations while the workspace is conceptually in `PLAN` or `REVIEW` phase, violating phase discipline.
3. **Unchecked Capability Escalation:** Operations occur without verifiable Object Capability (CapBAC) authorization tokens or bounded lease lifetimes.
4. **Release Provenance Decay:** Engineering sessions lose epistemic provenance unless the substrate distills release receipts and structural patterns into Git Notes and Nomos Mneme.

This specification codifies the **JIT Handshake Subsystem** and the **Release Receipt & Mneme Distillation Pipeline** (superseding v3 `nomos journal`; see [`session_management_cli_spec.md`](session_management_cli_spec.md)). It also defines two statutory 42-gate DoD probes: `Cell 1.3 (capbac_lease)` and `Cell 3.3 (release_receipt)`.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Go core injects bounded cognitive envelopes into Tier 1 agents. | `nomos handshake`, `Cell 1.3 (capbac_lease)` gate |
| **2** | **The Non-Verbal Proof Law** | Zero trust in verbal session reports; session outcomes require verifiable release receipts. | `Cell 3.3 (release_receipt)` probe (`refs/notes/nomos/releases`) |
| **3** | **The Hermetic Worktree Law** | Execution occurs strictly within leased worktrees bound to cryptographic token lifetimes. | CapBAC token leases in SQLite, worktree boundary checks |
| **4** | **The Single Source of Truth Law** | Handshake envelopes and release receipts adhere strictly to compiled Go SSoT structs. | `handshake.HandshakeEvent`, `task.ReleaseNotePayload` |
| **5** | **The Zero-Hardcoded-Path Law** | Dynamic resolution of database paths and git note namespaces. | `config.ResolveGlobalDBPath()`, `config.ResolveMnemeDBPath()` |
| **6** | **The Canonical Singularity Law** | `nomos handshake` serves as the sole canonical session initialization entrypoint. | `Cell 6.2 (zero_alias)` CLI dispatch |
| **7** | **The Fail-Closed Pre-Flight Law** | Expired leases, unhealthy databases, or missing release receipts fail pre-flight verification closed. | DoD gate pipeline exit code 1 |

---

## 3. Just-In-Time (JIT) Handshake Dynamic Injection

### 3.1 Handshake Invariant Envelope
The JIT Handshake synthesizes real-time workspace state to orient Tier 1 agents at session initialization. Ambient `SessionStart` hooks or `nomos handshake` deliver this orientation with zero slash-command ceremony.

The telemetry payload conforms to `handshake.HandshakeEvent` in `substrate/src/nomos/modules/handshake/handshake.go`:

```go
package handshake

// HandshakeEvent represents the dynamic orientation payload injected at session boot.
type HandshakeEvent struct {
	Seat                 string            `json:"seat"`
	Repository           string            `json:"repository"`
	Plane                string            `json:"plane,omitempty"`
	Phase                string            `json:"phase"`
	ActiveTask           string            `json:"active_task,omitempty"`
	Assignee             string            `json:"assignee,omitempty"`
	TokenID              string            `json:"token_id,omitempty"`
	StepBudget           int               `json:"step_budget,omitempty"`
	TokensUsed           int               `json:"tokens_used,omitempty"`
	DatabasesHealthy     bool              `json:"databases_healthy"`
	GateCount            int               `json:"gate_count"`
	ReadyWavefront       []string          `json:"ready_wavefront"`
	Directives           []string          `json:"directives,omitempty"`
	DeprecatedTerms      map[string]string `json:"deprecated_terms,omitempty"`
	LearnedPitfalls      []string          `json:"learned_pitfalls,omitempty"`
	VerifiedPatterns     []string          `json:"verified_patterns,omitempty"`
	GraduationCandidates []string          `json:"graduation_candidates,omitempty"`
	ImpactSummary        []string          `json:"impact_summary,omitempty"`
	Timestamp            string            `json:"timestamp,omitempty"`
}
```

### 3.2 Dynamic Phase Directives
The handshake dynamically filters and loads statutory directives from compiled substrate assets (embedded via `//go:embed`) matching the current workspace phase:

| Phase | Injected Directive Context | Mutability Rules |
| :---: | :--- | :--- |
| **`IDLE`** | Ecosystem quiescence, task discovery, ready wavefront inspection. | Workspace is immutable. Trunk locked. |
| **`PLAN`** | NCTE technical English, sizing & DAG planning, architectural synthesis. | Plan files only. Code files immutable. |
| **`EDIT`** | Traceability anchors, SSOT alignment, schema synthesis, refactoring rules. | Mutations strictly in `worktrees/<repo>-<task>`. |
| **`REVIEW`**| Definition of Done (DoD) quality gates, adversarial verification, anti-shim. | Mutations locked pending audit. |
| **`SYNC`**  | Atomic 2PC release, Git Notes receipts, Golden Commit + `Task:` trailer hygiene. | Commit & fast-forward merge execution. |
| **`LEARN`** | Nomos Mneme episodic memory reflection, pitfall dismissal, and axiom persistence. | Advisory reflection & axiom persistence in `mneme.db`. |

---

## 4. Release Receipt & Episodic Mneme Distillation Pipeline

### 4.1 Purpose & Structure
Nomos v4 eliminates mutable SQLite `journal` tables in favor of immutable `refs/notes/nomos/releases` records during `nomos task sync -F <walkthrough>`. Sync simultaneously indexes verified structural patterns into local `mneme.db` storage (`Nomos Learn`).

### 4.2 Authoritative Git Notes Receipt Contract
All synchronized release receipts conform strictly to `task.ReleaseNotePayload` defined in `substrate/src/nomos/modules/task/git_notes.go`:

```go
package task

// ReleaseNotePayload represents the immutable cryptographic receipt stored in refs/notes/nomos/releases.
type ReleaseNotePayload struct {
	TaskKey            string `json:"task_key"`
	CommitSHA          string `json:"commit_sha"`
	Timestamp          string `json:"timestamp"`
	Phase              string `json:"phase"`
	DoDGatesPassed     int    `json:"dod_gates_passed"`
	CapBACTokenID      string `json:"capbac_token_id"`
	WalkthroughContent string `json:"walkthrough_content"`
	SHA256Receipt      string `json:"sha256_receipt"`
}
```

### 4.3 Walkthrough & Structural Pattern Distillation
Each synchronized release walkthrough (`-F <walkthrough>`) captures:
1. **Executive Summary:** High-level narrative of objectives and outcomes.
2. **Key Changes:** Concrete files, symbols, and invariants modified inside the isolated worktree.
3. **Verification Proof:** Non-verbal DoD gate evaluation results (`42/42` gates satisfied), automatically distilled into `mneme.db` structural patterns and recalled by subsequent `nomos handshake` invocations.

---

## 5. Statutory Sensor Probes Activation

Two cryptographic ring probes enforce capability lease validity and release attestation within the 42-gate pipeline:

### 5.1 `Cell 1.3 (capbac_lease)`
- **Invariant:** Tasks operating in transient worktrees must possess an active, non-expired CapBAC capability lease registered in the `leases` table of `nomos_global.db`.
- **Failure Condition:** Missing token, expired lease timestamp, or mismatched task assignment.

### 5.2 `Cell 3.3 (release_receipt)`
- **Invariant:** Every synchronized release commit requires an immutable cryptographic receipt under `refs/notes/nomos/releases` containing the signed JSON payload.
- **Failure Condition:** Release commit lacks an associated `refs/notes/nomos/releases` receipt or the payload violates schema validation.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Lifecycle Condition | Governing Law | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-JIT-01` | `nomos handshake` invoked on clean trunk baseline | Law 1 (Dual-Core Inversion) | `PASS` (Emits `IDLE` envelope & verifies Git hooks) |
| `TC-JIT-02` | Worktree mutation attempted with expired or missing CapBAC lease | Law 1 & Law 7 (`Cell 1.3 (capbac_lease)`) | `FAIL` (`ERR_CAPBAC_LEASE_INVALID`) |
| `TC-JIT-03` | Release commit missing `refs/notes/nomos/releases` attestation | Law 3 (`Cell 3.3 (release_receipt)`) | `FAIL` (`ERR_MISSING_RELEASE_RECEIPT`) |
| `TC-JIT-04` | Release walkthrough synced and validated against `task.ReleaseNotePayload` | Law 4 (SSoT) | `PASS` (Recorded in `refs/notes/nomos/releases` & indexed into `mneme.db`) |

---

## 7. Cross-Plane Traceability & DoD Verification

- **Intent Lineage**: `INT-736`, `INT-896`.
- **Substrate SSoT**: `SUB-1214` (`substrate/src/nomos/modules/handshake/handshake.go`, `substrate/src/nomos/modules/task/git_notes.go`, `substrate/src/nomos/modules/verify/gates_capbac_release.go`, `substrate/src/nomos/modules/mem/learn.go`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 1.2 (phase_discipline)`, `Cell 1.3 (capbac_lease)`, `Cell 3.3 (release_receipt)`, `Cell 4.3 (intent_parity)`.
- **Deterministic Verification**: `nomos verify` validates CapBAC lease integrity, Git Notes release receipts, and embedded phase directive parity with exit code `0`.
