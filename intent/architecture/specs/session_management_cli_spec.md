---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-11-03"
last_updated: "2026-11-03"
lineage:
  - INT-896
  - SUB-1668
plane: intent
ssot_repository: nomos
status: active
title: "Tier 1 Agent Session Discovery, Telemetry & Sentinel CLI Specification"
type: spec
version: "4.2.0"
---

# Tier 1 Agent Session Discovery, Telemetry & Sentinel CLI Specification

---

## 1. Executive Summary & Problem Statement

In Nomos v4, Tier 1 coding agents operate across multiple developer harnesses while sharing the Living Ledger and hermetic worktrees. Legacy v3 session journaling (`nomos journal`, `nomos task journal`, `nomos task note`) required manual SQLite `journal` writes and drifted easily.

Nomos v4 replaces manual journaling with two non-verbal mechanisms:
1. **Immutable Release Receipts (`refs/notes/nomos/releases`):** Minted atomically at merge time by `nomos task sync`.
2. **Live Transcript Introspection (`nomos session`):** Discovers, labels, visualizes, and audits live Tier 1 conversation transcripts directly from local harness storage without agent cooperation.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Substrate Go core examines Tier 1 transcripts externally under indirect control. | `session.DiscoverSessions`, `session.GetSessionDetail` |
| **2** | **The Non-Verbal Proof Law** | Trajectory stages and Sentinel scores derive strictly from observed tool calls. | `InferTrajectory`, `EvaluateSentinel` |
| **3** | **The Hermetic Worktree Law** | Sentinel correlates sessions to `worktrees/<repo>-<KEY>` and flags direct trunk mutations. | `evalHermeticWorktree` Sentinel invariant |
| **4** | **The Single Source of Truth Law** | JSON payloads bind 1:1 to `session.AgentSession`, `session.SessionVisualGraph`, `session.SessionDetail`, and `session.SessionTrajectory`. | Go struct JSON tags & `multi_agent_session_test.go` |
| **5** | **The Zero-Hardcoded-Path Law** | Transcript directories resolve dynamically from configuration roots or `--brain` / `--root`. | `session.DefaultProviders` |
| **6** | **The Canonical Singularity Law** | `nomos session` (`name`, `list`, `visual`, `show`) replaces excised `nomos journal`. | `Cell 6.2 (zero_alias)`, `Cell 2.1 (dead_code)` |
| **7** | **The Fail-Closed Pre-Flight Law** | Missing active sessions or empty `session name` arguments reject with `ERR_VALIDATION_FAILED` / `ERR_CORE`. | `NewSessionCmd` argument & discovery guards |

---

## 3. Polymorphic Tier 1 Provider Architecture & Go SSoT Contracts

`substrate/src/nomos/modules/session/` implements polymorphic discovery across the four supported Tier 1 coding agent harnesses:

1. **Antigravity IDE (`AGY-IDE`):** Parses `.system_generated/logs/transcript.jsonl` and `session_meta.json` under the active Gemini IDE brain directory.
2. **Antigravity CLI (`AGY-CLI`):** Parses headless CLI conversation brain directories and transcript records.
3. **Claude Code (`Claude Code`):** Discovers project JSONL transcripts and tool-use blocks from local Claude session storage.
4. **OpenCode (`OpenCode`):** Discovers local OpenCode session metadata and message/part storage trees.

### 3.1 7-Stage Causal Trajectory (`#SessionTrajectory`)
`InferTrajectory` reconstructs seven ordered milestones from transcript tool calls: `BOOT`, `ALLOCATION`, `BASELINE`, `EXPLORATION`, `ORACLE`, `IMPLEMENTATION`, and `RELEASE`.

### 3.2 Zero-Trust Sentinel Scorecard
`EvaluateSentinel` audits every tool call in the transcript across five statutory probes (20 points each, `0..100`):
1. Hermetic worktree isolation (`evalHermeticWorktree`)
2. Repository boundary containment (`evalRepoBoundary`)
3. Non-verbal DoD verification (`evalDoDVerification`)
4. Database encapsulation under the Database Encapsulation Invariant (`evalDBEncapsulation`)
5. AST / file budget discipline (`evalASTBudget`)

### 3.3 Compiled Go SSoT Contracts (`substrate/src/nomos/modules/session/`)
`session.go`, `session_detail.go`, and `session_trajectory.go` define the authoritative session telemetry structs:

```go
package session

import "time"

// AgentSession captures the live state, telemetry, and coordinates of an agent conversation arc (#AgentSession).
type AgentSession struct {
	ID            string    `json:"id"`
	Name          string    `json:"name,omitempty"`
	AgentType     string    `json:"agent_type"`
	TaskKey       string    `json:"task_key,omitempty"`
	TaskTitle     string    `json:"task_title,omitempty"`
	WorktreePath  string    `json:"worktree_path,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	Phase         string    `json:"phase"`
	TurnCount     int       `json:"turn_count"`
	ToolCallCount int       `json:"tool_call_count"`
	LastActiveAt  time.Time `json:"last_active_at"`
	Status        string    `json:"status"`
	RecentSummary string    `json:"recent_summary,omitempty"`
}

// SessionVisualGraph represents the topological view of active agent sessions (#SessionVisualGraph).
type SessionVisualGraph struct {
	Mermaid  string         `json:"mermaid"`
	Sessions []AgentSession `json:"sessions"`
}

// TrajectoryStage models an individual causal milestone stage in an agent trajectory (#TrajectoryStage).
type TrajectoryStage struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

// SessionTrajectory models the complete 7-stage causal arc of a Tier 1 conversation session (#SessionTrajectory).
type SessionTrajectory struct {
	CurrentStage   string            `json:"current_stage"`
	CompletedCount int               `json:"completed_count"`
	TotalCount     int               `json:"total_count"`
	Stages         []TrajectoryStage `json:"stages"`
}

// SessionDetail captures the deep causal inspection telemetry of an agent session (#SessionDetail).
type SessionDetail struct {
	AgentSession
	ToolsSummary  map[string]int     `json:"tools_summary"`
	Events        []SessionEvent     `json:"events"`
	RecentPrompts []string           `json:"recent_prompts,omitempty"`
	Sentinel      *SentinelScorecard `json:"sentinel,omitempty"`
	Trajectory    *SessionTrajectory `json:"trajectory,omitempty"`
	Persona       string             `json:"persona,omitempty"`
}

func DiscoverSessions(rootOverride, repoRoot string) ([]AgentSession, error)
func GetSessionDetail(rootOverride, repoRoot, sessionID string) (*SessionDetail, error)
func InferTrajectory(detail *SessionDetail, rawLines []string) *SessionTrajectory
```

---

## 4. CLI Command Taxonomy & Flags (`substrate/cli/cmd_session.go`)

### 4.1 `nomos session name [NAME]`
- **Flags:** `--id` / `-i <SESSION_ID>`, `--brain` / `-b <DIR>`, `--json`
- **Semantics:** Persists an operator title in `session_meta.json` for `--id` or the latest active session. Rejects empty `NAME` arguments.

### 4.2 `nomos session list`
- **Flags:** `--brain` / `-b <DIR>`, `--json`
- **Semantics:** Discovers sessions across all supported Tier 1 providers sorted by `last_active_at` descending, rendering a Lipgloss table or `#AgentSession` JSON array.

### 4.3 `nomos session visual`
- **Flags:** `--brain` / `-b <DIR>`, `--root` / `-r <REPO_ROOT>`, `--json`
- **Semantics:** Renders a Mermaid topology mapping active Tier 1 sessions to `INT-*` / `SUB-*` tasks and worktrees, or `#SessionVisualGraph` JSON with `--json`.

### 4.4 `nomos session show [SESSION_ID]`
- **Flags:** `--brain` / `-b <DIR>`, `--root` / `-r <REPO_ROOT>`, `--json`
- **Semantics:** Examines `SESSION_ID` or the latest active session, rendering turn/tool counts, phase, worktree, Sentinel score, and 7-stage trajectory (`#SessionDetail` JSON).

---

## 5. Table-Driven Verification Vectors

| Vector ID | Target Component | Input Vector | Expected Verdict | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-SES-01` | `DiscoverSessions` | Clean-room mock transcripts for `AGY-IDE`, `AGY-CLI`, `Claude Code`, `OpenCode` | Discovers all 4 sessions with accurate `TaskKey`, `TurnCount`, and `Status` | Law 1 & Law 5 |
| `TC-SES-02` | `nomos session name` | `nomos session name ""` (empty name) | Fails closed with `ERR_VALIDATION_FAILED` | Law 7 (Fail-Closed) |
| `TC-SES-03` | `InferTrajectory` | Transcript containing `nomos handshake`, `nomos task start`, `nomos verify`, and file edits | Reconstructs 7-stage `#SessionTrajectory` with `ORACLE` and `IMPLEMENTATION` marked `COMPLETED` | Law 2 (Non-Verbal Proof) |
| `TC-SES-04` | `GenerateMermaidVisual` | `nomos session visual --json` | Emits valid `SessionVisualGraph` JSON containing Mermaid graph and `AgentSession` slice | Law 4 (Compiled Go SSoT) |

---

## 6. Cross-Plane Traceability & DoD Verification

| Intent Contract | Compiled Go SSoT Path | Enforcing 7x3 Matrix Cell |
| :--- | :--- | :--- |
| Polymorphic discovery (`AgentSession`, `DiscoverSessions`) | `substrate/src/nomos/modules/session/session.go`, `provider.go` | `Cell 1.1 (boundary)`, `Cell 5.1 (forbidden_literals)` |
| Causal trajectory (`SessionTrajectory`, `InferTrajectory`) | `substrate/src/nomos/modules/session/session_trajectory.go` | `Cell 2.1 (tdd)`, `Cell 2.2 (go_toolchain)` |
| Zero-trust Sentinel scorecard (`EvaluateSentinel`) | `substrate/src/nomos/modules/session/session_sentinel.go` | `Cell 3.2 (hermetic_worktree)`, `Cell 7.2 (unchecked_errors)` |
| CLI command surface (`nomos session`) | `substrate/cli/cmd_session.go` | `Cell 6.2 (zero_alias)`, `Cell 4.1 (intent_traceability)` |

Run `nomos verify` to execute all 42 Definition of Done quality gates and confirm exit code `0`.
