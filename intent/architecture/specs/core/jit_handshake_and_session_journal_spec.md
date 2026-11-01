---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Just-In-Time (JIT) Handshake Dynamic Injection and Session Journal Distillation Specification"
type: spec
version: "4.0.0"
tags:
    - INT-736
    - SUB-1214
ssot_repository: nomos
---

# Just-In-Time (JIT) Handshake Dynamic Injection and Session Journal Distillation Specification

## 1. Executive Summary & Problem Statement

Under **Law 1 (The Dual-Core Inversion Law)**, the Substrate Go core governs state and execution, while the Tier 1 coding agent operates strictly as a bounded cognitive co-processor under indirect control.

In stateless or poorly bounded agentic workflows, cognitive models frequently suffer from:
1. **Context Amnesia & Hallucination:** Agents lack visibility into current workspace state, active phase, remaining step budgets, and database health.
2. **Phase Boundary Drift:** Agents attempt file mutations while the workspace is conceptually in `PLAN` or `REVIEW` phase, violating phase discipline.
3. **Unchecked Capability Escalation:** Operations occur without verifiable Object Capability (CapBAC) authorization tokens or bounded lease lifetimes.
4. **Session Journal Decay:** Epistemic progress made during engineering sessions is lost because no deterministic distillation pipeline records session outcomes to the Intent Single Source of Truth (SSoT).

This specification codifies the **JIT Handshake Dynamic Invariant Injection Subsystem**, the **Session Journal Distillation Engine**, and the activation of two statutory Definition of Done (DoD) sensor probes: **`capbac_lease` (Cell 1.3)** and **`release_receipt` (Cell 3.3)**, advancing active gate enforcement to 21 statutory gates.

---

## 2. Axiomatic Invariants & Statutory Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | The Go core synthesizes and injects bounded cognitive envelopes into Tier 1 agents. | `nomos handshake`, `capbac_lease` gate |
| **2** | **The Non-Verbal Proof Law** | Zero trust in verbal session reports; session outcomes require verifiable release receipts. | `release_receipt` probe (`refs/notes/nomos/releases`) |
| **3** | **The Hermetic Worktree Law** | Execution occurs strictly within leased worktrees bound to cryptographic token lifetimes. | CapBAC token leases in SQLite, worktree boundary checks |
| **4** | **The Single Source of Truth Law** | Handshake envelopes and session journals adhere strictly to compiled CUE contracts. | `handshake.cue`, `session_journal.cue` parity audits |
| **5** | **The Zero-Hardcoded-Path Law** | Dynamic resolution of session journal vaults, database paths, and git note namespaces. | `config.ResolveGlobalDBPath()`, `config.ResolveMnemeDBPath()` |
| **6** | **The Canonical Singularity Law** | `nomos handshake` serves as the sole canonical session initialization entrypoint. | Zero-alias CLI dispatch |
| **7** | **The Fail-Closed Pre-Flight Law** | Expired leases, unhealthy databases, or missing release receipts fail pre-flight verification closed. | DoD gate pipeline exit code 1 |

---

## 3. Just-In-Time (JIT) Handshake Dynamic Injection

### 3.1 Handshake Invariant Envelope
The JIT Handshake synthesizes a real-time snapshot of the engineering substrate, providing Tier 1 agents with immediate orientation automatically upon session boot via ambient `SessionStart` hooks or canonical CLI execution (`nomos handshake`), requiring zero slash-command ceremony from the developer.

The telemetry payload conforms to `#HandshakeEvent` in `intent/contracts/cue/handshake.cue`:

```cue
#HandshakeEvent: close({
	seat:              string & strings.MinRunes(1)
	repository:        string & strings.MinRunes(1)
	plane?:            "intent" | "substrate"
	phase:             "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"
	active_task?:      #TaskID
	assignee?:         string & strings.MinRunes(1)
	token_id?:         string & strings.MinRunes(1)
	step_budget?:      int & >= 0
	tokens_used?:      int & >= 0
	databases_healthy: bool
	gate_count:        int & >= 0
	ready_wavefront:   [...#TaskID]
	directives?:       [...string & strings.MinRunes(1)]
	deprecated_terms?: [string]: string
	timestamp?:        string
})
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

## 4. Session Journal Distillation Pipeline

### 4.1 Purpose & Structure
Session journals serve as the permanent, machine-auditable record of engineering iterations. Each journal is distilled into cryptographic Git Notes (`refs/notes/nomos/releases`) and indexed into persistent Nomos Mneme SQLite storage.

### 4.2 Authoritative Frontmatter Contract
All session journals must strictly satisfy `#SessionJournal` defined in `intent/contracts/cue/session_journal.cue`:

```yaml
---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-10-23"
hemisphere: substrate
status: active
title: "Milestone 5 - JIT Handshake and Session Journal Distillation"
type: session_journal
tasks:
    - INT-736
    - SUB-1214
invariants_verified:
    - Law 1: Dual-Core Inversion
    - Law 3: Hermetic Worktree
    - Law 4: Single Source of Truth
tags:
    - handshake
    - session_journal
    - capbac
version: "1.0.0"
---
```

### 4.3 Dual-Layer Markdown Structure
Each distilled session journal contains:
1. **Executive Summary:** High-level narrative of objectives and outcomes.
2. **Task Lifecycles Completed:** Key, Title, Plane, and Commit SHA for each synchronized task.
3. **Invariants & Gates Enforced:** Non-verbal proof of DoD gate evaluation results.
4. **Epistemic Reflections:** Architectural lessons persisted into Nomos Mneme.

---

## 5. Statutory Sensor Probes Activation

Milestone 5 activates two cryptographic ring probes, bringing the active gate suite from 19 to 21:

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
| `TC-JIT-04` | Session journal unified against `session_journal.cue` | Law 4 (SSoT) | `PASS` (Indexed cleanly into `mneme.db`) |
