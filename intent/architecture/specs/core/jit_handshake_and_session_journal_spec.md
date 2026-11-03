---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-03"
plane: intent
status: active
title: "Just-In-Time (JIT) Handshake Dynamic Injection and Release Receipt Distillation Specification"
type: spec
version: "4.2.0"
tags:
    - INT-736
    - SUB-1214
    - INT-896
ssot_repository: nomos
---

# Just-In-Time (JIT) Handshake Dynamic Injection and Release Receipt Distillation Specification

## 1. Executive Summary & Problem Statement

Under **Law 1 (The Dual-Core Inversion Law)**, the Substrate Go core governs state and execution, while the Tier 1 coding agent operates strictly as a bounded cognitive co-processor under indirect control.

In stateless or poorly bounded agentic workflows, cognitive models frequently suffer from:
1. **Context Amnesia & Hallucination:** Agents lack visibility into current workspace state, active phase, remaining step budgets, and database health.
2. **Phase Boundary Drift:** Agents attempt file mutations while the workspace is conceptually in `PLAN` or `REVIEW` phase, violating phase discipline.
3. **Unchecked Capability Escalation:** Operations occur without verifiable Object Capability (CapBAC) authorization tokens or bounded lease lifetimes.
4. **Release Provenance Decay:** Epistemic progress made during engineering sessions is lost unless release receipts and structural patterns are deterministically distilled into Git Notes and Nomos Mneme.

This specification codifies the **JIT Handshake Dynamic Invariant Injection Subsystem**, the **Release Receipt & Mneme Distillation Pipeline** (superseding legacy v3 `nomos journal` tables; see [`session_management_cli_spec.md`](session_management_cli_spec.md) for live `nomos session` inspection), and two statutory Definition of Done (DoD) sensor probes: **`capbac_lease` (Cell 1.3)** and **`release_receipt` (Cell 3.3)** across the 42-gate pipeline.

---

## 2. Axiomatic Invariants & Statutory Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | The Go core synthesizes and injects bounded cognitive envelopes into Tier 1 agents. | `nomos handshake`, `capbac_lease` gate |
| **2** | **The Non-Verbal Proof Law** | Zero trust in verbal session reports; session outcomes require verifiable release receipts. | `release_receipt` probe (`refs/notes/nomos/releases`) |
| **3** | **The Hermetic Worktree Law** | Execution occurs strictly within leased worktrees bound to cryptographic token lifetimes. | CapBAC token leases in SQLite, worktree boundary checks |
| **4** | **The Single Source of Truth Law** | Handshake envelopes and release receipts adhere strictly to compiled Go SSoT structs. | `handshake.HandshakeEvent`, `task.ReleaseNotePayload` |
| **5** | **The Zero-Hardcoded-Path Law** | Dynamic resolution of database paths and git note namespaces. | `config.ResolveGlobalDBPath()`, `config.ResolveMnemeDBPath()` |
| **6** | **The Canonical Singularity Law** | `nomos handshake` serves as the sole canonical session initialization entrypoint. | Zero-alias CLI dispatch |
| **7** | **The Fail-Closed Pre-Flight Law** | Expired leases, unhealthy databases, or missing release receipts fail pre-flight verification closed. | DoD gate pipeline exit code 1 |

---

## 3. Just-In-Time (JIT) Handshake Dynamic Injection

### 3.1 Handshake Invariant Envelope
The JIT Handshake synthesizes a real-time snapshot of the engineering substrate, providing Tier 1 agents with immediate orientation automatically upon session boot via ambient `SessionStart` hooks or canonical CLI execution (`nomos handshake`), requiring zero slash-command ceremony from the developer.

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
| **`SYNC`**  | Atomic 2PC release, Git Notes receipts, Dual-Layer commit formatting. | Commit & fast-forward merge execution. |
| **`LEARN`** | Nomos Mneme episodic memory reflection, session distillation. | Release receipt & reflection persistence. |

---

## 4. Release Receipt & Episodic Mneme Distillation Pipeline

### 4.1 Purpose & Structure
Instead of maintaining a mutable SQLite `journal` table, Nomos v4 distills completed task walkthroughs directly into immutable cryptographic Git Notes (`refs/notes/nomos/releases`) during `nomos task sync -F <walkthrough>` and indexes verified structural patterns into local `mneme.db` storage (`Nomos Learn`).

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

### 5.1 `capbac_lease` Probe (Law 1, Cryptographic Ring, Cell 1.3)
- **Invariant:** Tasks operating in transient worktrees must possess an active, non-expired CapBAC capability lease registered in the `leases` table of `nomos_global.db`.
- **Failure Condition:** Missing token, expired lease timestamp, or mismatched task assignment.

### 5.2 `release_receipt` Probe (Law 3, Cryptographic Ring, Cell 3.3)
- **Invariant:** Synchronized release commits must be cryptographically attested by a Git Note in `refs/notes/nomos/releases` containing the signed release receipt JSON payload.
- **Failure Condition:** Release commit lacks associated release receipt note or payload fails schema validation.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Lifecycle Condition | Governing Law | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-JIT-01` | `nomos handshake` invoked on clean trunk baseline | Law 1 (Dual-Core Inversion) | `PASS` (Emits `IDLE` envelope & verifies Git hooks) |
| `TC-JIT-02` | Worktree mutation attempted with expired or missing CapBAC lease | Law 1 & Law 7 (`Cell 1.3`) | `FAIL` (`ERR_CAPBAC_LEASE_INVALID`) |
| `TC-JIT-03` | Release commit missing `refs/notes/nomos/releases` attestation | Law 3 (`Cell 3.3`) | `FAIL` (`ERR_MISSING_RELEASE_RECEIPT`) |
| `TC-JIT-04` | Release walkthrough synced and validated against `task.ReleaseNotePayload` | Law 4 (SSoT) | `PASS` (Recorded in `refs/notes/nomos/releases` & indexed into `mneme.db`) |
