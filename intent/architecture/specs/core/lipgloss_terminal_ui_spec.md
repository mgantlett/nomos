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

Styling tokens are centralized in the single source of truth `substrate/cli/theme.go` (`package cli`):

### 3.1 Status Badge Palette (Canonical Task State Alignment)

In strict alignment with `task.TaskRecord.Status`, adaptive status pills (`styleSemanticBadge`) map deterministically to the semantic palette:

| Status Code | Semantic Style | Usage |
| :--- | :--- | :--- |
| `BACKLOG` / `TODO` | Neutral Slate Pill (`pillNeutralStyle`) | Unstarted tasks in backlog or ready queue |
| `PLAN` | Celestial Purple Pill (`pillPurpleStyle`) | Architectural formulation and causal planning |
| `EDIT` / `IN_PROGRESS` | Solar Amber Pill (`pillWarnStyle`) | Active worktree mutation and code authoring |
| `REVIEW` / `SYNC` | Substrate Sky Cyan Pill (`pillCyanStyle`) | Definition of Done audit and 2PC merge |
| `DONE` / `PASS` / `ACTIVE` | Verification Mint Pill (`pillSuccessStyle`) | Verified green milestone or active session |
| `BLOCKED` / `FAIL` | Fail-Closed Coral Pill (`pillDangerStyle`) | Gate failure, lease collision, or unmet prerequisite |
| `ABORTED` | Neutral Slate Pill (`pillNeutralStyle`) | Safely rolled-back or tombstoned task |

### 3.2 Visual Components
- **Adaptive Rules (`uiRule`)**: Responsive horizontal rules (`ruleNarrow`, `ruleStandard`, `ruleWide`, `ruleSession`).
- **Dual-Core Hemisphere Accents**: Intent Purple (`uiIntentStyle`) and Substrate Cyan (`uiSubstrateStyle`).
- **HATEOAS Remediation Callouts (`affordance_render.go`)**: Structured `Next Actions` command hints rendered below operational outputs.

---

## 4. Substrate Styling Primitives (`substrate/cli/theme.go`)

The Substrate centralizes adaptive terminal rendering inside `substrate/cli/theme.go` and `substrate/cli/theme_gradient.go`:
- `renderBrandTitle(title string)` — Adaptive gradient title header.
- `styleSemanticBadge(status string)` / `styleSemanticBadgeWidth(status string, width int)` — Semantic status pill formatting.
- `styleTaskKey(key string)` — Plane-colored task key formatting (`INT-*` purple, `SUB-*` cyan).
- `kvLine(label string, width int, value string)` — Aligned key-value telemetry rows.

---

## 5. Deterministic Invariants & Graceful Degradation

1. **Law 1 (The Dual-Core Inversion Law)**: Lipgloss terminal UX serves as the high-clarity telemetry interface for human developers and Tier 1 cognitive agents under indirect control.
2. **Law 2 (The Non-Verbal Proof Law)**: Terminal formatting must strictly preserve deterministic command exit codes (`0` for success, non-zero for gate failures).
3. **Machine-Readable JSON Isolation**: When `--json` is passed on any inspection command, all Lipgloss styling, ANSI codes, and banners are bypassed in favor of raw, valid JSON.
4. **Environment-Aware Color Stripping**: When `NO_COLOR` is present, `TERM=dumb`, or stdout is redirected to a non-TTY pipe/file, styling automatically degrades to clean plaintext without ANSI escape sequences.
5. **Sub-2ms Render Overhead**: Terminal layout and string styling computation must execute in $< 2$ms without blocking CLI execution.

---

## 6. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Environment / Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-UI-001` | `styleSemanticBadge` | `status: "EDIT"` in color TTY | Rendered pill with amber styling | Law 4 (SSoT State Alignment) |
| `TC-UI-002` | `NO_COLOR` Environment | `os.Setenv("NO_COLOR", "1")` | Degrades cleanly without ANSI escape sequences | Law 2 (Deterministic Plaintext Degradation) |
| `TC-UI-003` | `--json` Flag | `nomos task list --json` | Emits valid raw JSON array with zero ANSI prefixes | Machine-Readable Isolation |
| `TC-UI-004` | SSoT Theme Centralization | `substrate/cli/theme.go` | Shared adaptive styles used across `status`, `task`, `verify`, `session`, and `capabilities` | Law 6 (Canonical Singularity) |

---

## 7. Verification Boundaries & DoD Evidence

1. **DoD Compliance**: Passes 100% of Definition of Done quality gates (`nomos verify`) with exit code 0.
2. **Markdown Integrity**: All markdown relative links resolve deterministically via `BrokenWireGate`.
3. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
