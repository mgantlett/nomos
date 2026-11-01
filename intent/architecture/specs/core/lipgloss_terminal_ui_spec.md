---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Open-Core Terminal UI Architecture & Totem CLI Ergonomics"
type: spec
version: "4.0.0"
execution_frontier:
  tasks:
    - key: "INT-557"
      plane: "intent"
      summary: "Codify Lipgloss terminal UI architecture, token taxonomy, and boundary invariants"
    - key: "SUB-654"
      plane: "substrate"
      summary: "Implement charmbracelet/lipgloss dependency, UI style tokens, and formatted CLI formatters"
ssot_repository: nomos
---

# Open-Core Terminal UI Architecture & Totem CLI Ergonomics

> [!IMPORTANT]
> This specification codifies the modern terminal interface architecture for the Nomos open-core developer harness.
> Per ADR-0031 (*Agent Tiers, Indirect Control and Adoption Profiles*) and the Open-Core Boundary Charter, rich terminal UI polish via `charmbracelet/lipgloss` is strictly exclusive to the Go-native Open-Core Terminal Totem CLI. The Svelte 5 / Runes Web Application Cockpit remains strictly Sovereign.

---

## 1. Architectural Intent & Scope

Currently, the open-core Nomos CLI renders terminal output across verification pipelines (`nomos verify`), task ledgers (`nomos task list`, `nomos task show`), workspace state (`nomos status`), and causal DAGs (`nomos task dag`) using basic standard library `fmt.Printf`, basic line strings, and ad-hoc ANSI escape codes. While functional, it lacks dynamic terminal-width awareness, responsive styling, structured status pill badges, and accessible color degradation in headless or constrained CI/CD environments.

This specification introduces a unified, zero-daemon, Go-native terminal UI system built on `charmbracelet/lipgloss` to elevate the developer and Tier 1 agent telemetry experience while upholding strict architectural boundaries.

---

## 2. Open-Core vs. Sovereign Boundary Invariant (ADR-0031)

To protect the sovereign moat and ensure open-core remains a pure single-binary developer harness:

| Layer | Technology | Distribution | Purpose |
| :--- | :--- | :--- | :--- |
| **Open-Core Layer (Terminal Totem CLI)** | Go + `charmbracelet/lipgloss` | Single compiled binary (`nomos`) | Single-developer control harness, DoD gatekeeper dashboard, local task lifecycle management. Zero web daemons, zero browser dependency, zero Node.js. |
| **Sovereign Layer (Command Cockpit & Flight Deck)** | Svelte 5 + Runes + Three.js + Canvas | Web Application (`substrate/membrane`) | Multi-seat enterprise mesh orchestration, Gemini Live bidirectional ATC voice streaming, interactive 3D DAG flight deck, continuous `sophia-soma` telemetry. |

Open-core CLI enhancements must never attempt to embed web servers, HTTP asset bundles, or browser shims into the open-core distribution.

---

## 3. UI Token Taxonomy & Semantic Styling

Styling tokens are centralized in the single source of truth package `substrate/cli/ui`:

### 3.1 Status Badge Palette (Canonical Task State Alignment)

In strict alignment with `intent/contracts/cue/task.cue` (`#TaskRecord.status` enum), status badges map deterministically to the semantic color palette:

| Status Code | Foreground | Background | Semantic Usage |
| :--- | :--- | :--- | :--- |
| `BACKLOG` / `TODO` | `#FFFFFF` | Slate (`#64748B`) / Muted Border (`#334155`) | Unstarted tasks in backlog or ready queue |
| `PLAN` | `#FFFFFF` | Purple (`#8B5CF6`) | Architectural formulation and causal planning |
| `EDIT` / `IN_PROGRESS` | `#0F172A` | Amber (`#F59E0B`) | Active worktree mutation and code authoring |
| `REVIEW` | `#0F172A` | Cyan (`#06B6D4`) | Definition of Done gatekeeper audit |
| `SYNC` | `#FFFFFF` | Sky (`#38BDF8`) | Two-Phase Commit atomic merge and Git Notes release |
| `DONE` / `PASS` | `#FFFFFF` | Emerald (`#10B981`) | Verified and green milestone |
| `BLOCKED` / `FAIL` | `#FFFFFF` | Crimson (`#EF4444`) | Gate failure, lease collision, or unmet causal prerequisite |
| `ABORTED` / `CANCELLED` | `#FFFFFF` | Slate (`#64748B`) | Safely rolled-back or tombstoned task |

### 3.2 Visual Components
- **Card Border**: Rounded borders (`lipgloss.RoundedBorder()`) styled with muted slate (`#334155`).
- **Dividers**: Clean horizontal and vertical glyphs with padding.
- **Remediation Callouts**: Indented blocks with bold red left-border pipes (`│`) and highlighted remedy instructions.
- **Workspace HUD**: Comprehensive card header rendering Phase, Worktree/Branch, and DB health.

---

## 4. In-Line Interface Contracts & Go DTO Structs

The Substrate implements responsive table and theme rendering in `substrate/cli/ui`:

```go
package ui

// TaskRow defines the tabular data for a task row in nomos task list.
type TaskRow struct {
	Key    string `json:"key"`
	Twin   string `json:"twin"`
	Arc    string `json:"arc"`
	Type   string `json:"type"`
	Plane  string `json:"plane"`
	Status string `json:"status"`
	Tier   string `json:"tier"`
	Title  string `json:"title"`
}

// CapabilityRow defines tabular data for a capability row across Substrate and Intent planes.
type CapabilityRow struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Plane           string `json:"plane"`
	Status          string `json:"status"`
	InvocationCount int    `json:"invocation_count"`
	LastAccessed    string `json:"last_accessed"`
}
```

Declarative CUE schema counterpart:

```cue
package schemas

// TaskRow models tabular task representations for CLI totems.
#TaskRow: close({
	key:    =~"^(INT|SUB)-[0-9]+$"
	twin?:  string
	arc?:   string
	type:   string
	plane:  "intent" | "substrate"
	status: "BACKLOG" | "TODO" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "DONE" | "BLOCKED" | "ABORTED"
	tier:   string
	title:  string
})
```

---

## 5. Deterministic Invariants & Graceful Degradation

1. **Law 1 (The Dual-Core Inversion Law)**: Lipgloss terminal UX serves as the high-clarity telemetry interface for human developers and Tier 1 cognitive agents under indirect control.
2. **Law 2 (The Non-Verbal Proof Law)**: Terminal formatting must strictly preserve deterministic command exit codes (`0` for success, non-zero for gate failures).
3. **Machine-Readable Flag Isolation**: When `--json`, `--quiet`, or batch flags are provided, all Lipgloss styling, ANSI codes, and ASCII art banners are completely bypassed. Output must emit raw, valid JSON or plaintext.
4. **Environment-Aware Color Stripping**: When `NO_COLOR` is present, `TERM=dumb`, or stdout is redirected to a non-TTY pipe/file, `IsColorEnabled()` returns `false`, and styling automatically degrades to clean plaintext without ANSI escape sequences.
5. **Sub-2ms Render Overhead**: Terminal layout and string styling computation must execute in $< 2$ms without blocking the CLI event loop.

---

## 6. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Environment / Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-UI-001` | `BadgeStatus` | `status: "EDIT"` with `NO_COLOR=""` | Rendered pill with amber background and dark text | Law 4 (SSoT State Alignment) |
| `TC-UI-002` | `IsColorEnabled` | `os.Setenv("NO_COLOR", "1")` | Returns `false`; `BadgeStatus("DONE")` returns `"[DONE]"` | Law 2 (Deterministic Plaintext Degradation) |
| `TC-UI-003` | `RenderTaskTable` | `[]TaskRow{...}` under `TERM=dumb` | Output contains ASCII separator lines and zero ANSI escape sequences | Invariant 4 (Environment-Aware Degradation) |
| `TC-UI-004` | AST Gate Check | Go source files outside `substrate/cli/ui` | Zero terminal escape literals detected by `ForbiddenLiteralsGate` | Law 5 & Law 6 (SSoT Package Isolation) |

---

## 7. Verification Boundaries & DoD Evidence

1. **DoD Compliance**: Passes 100% of Definition of Done quality gates (`nomos verify`) with exit code 0.
2. **Markdown Integrity**: All markdown relative links resolve deterministically via Gate 14 (`BrokenWireGate`).
3. **Clean AST Compliance**: Zero hardcoded terminal escape literals outside `substrate/cli/ui`.
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 29 Definition of Done quality gates with zero regressions.
