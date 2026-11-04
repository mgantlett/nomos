---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-04"
plane: intent
status: active
title: "Open-Core Terminal UI Architecture & Totem CLI Ergonomics"
type: spec
version: "4.1.0"
execution_frontier:
  tasks:
    - key: "INT-922"
      plane: "intent"
      summary: "Purge Sovereign stack implementation bleed and enforce NCTE-100 compliance"
    - key: "SUB-1744"
      plane: "substrate"
      summary: "Harden release pipeline, uninstall state resolution, and CLI scope boundaries"
ssot_repository: nomos
---

# Open-Core Terminal UI Architecture & Totem CLI Ergonomics

> [!IMPORTANT]
> This specification codifies the terminal interface architecture for the Nomos open-core harness.
> Per ADR-0031, `charmbracelet/lipgloss` terminal rendering applies exclusively to the Go-native open-core CLI. Commercial web control-plane surfaces remain outside this repository.

---

## 1. Architectural Intent & Scope

Early Nomos CLI iterations rendered `nomos verify`, `nomos task`, and `nomos status` using raw `fmt.Printf` calls and ad-hoc ANSI strings. That baseline lacked terminal-width awareness, semantic status badges, and clean plaintext degradation in headless CI pipelines.

This specification defines a zero-daemon, Go-native terminal UI built on `charmbracelet/lipgloss` for clear developer and Tier 1 agent telemetry.

---

## 2. Open-Core vs. Commercial Boundary Invariant (ADR-0031)

To preserve a pure single-binary developer harness:

| Layer | Technology | Distribution | Purpose |
| :--- | :--- | :--- | :--- |
| **Open-Core Layer (Terminal CLI)** | Go + `charmbracelet/lipgloss` | Single binary (`nomos`) | Local governance harness, DoD dashboard, and zero-daemon task lifecycle. |
| **Enterprise Layer (Out of Scope)** | Commercial Extension | External distribution | Multi-seat enterprise orchestration and hosted observability per ADR-0031. |

Open-core CLI packages never embed web servers, HTTP asset archives, or browser shims.

---

## 3. UI Token Taxonomy & Semantic Styling

The Substrate centralizes all styling tokens inside `substrate/cli/theme.go` (`package cli`).

### 3.1 Status Badge Palette (Canonical Task State Alignment)

In alignment with `task.TaskRecord.Status`, adaptive status pills (`styleSemanticBadge`) map deterministically to semantic tokens:

| Status Code | Semantic Style | Usage |
| :--- | :--- | :--- |
| `BACKLOG` / `TODO` | Neutral Slate Pill (`pillNeutralStyle`) | Unstarted tasks in backlog or ready wavefront |
| `PLAN` | Celestial Purple Pill (`pillPurpleStyle`) | Architectural formulation and causal planning |
| `EDIT` / `IN_PROGRESS` | Solar Amber Pill (`pillWarnStyle`) | Active worktree mutation and code authoring |
| `REVIEW` / `SYNC` | Substrate Sky Cyan Pill (`pillCyanStyle`) | Definition of Done audit and 2PC merge |
| `DONE` / `PASS` / `ACTIVE` | Verification Mint Pill (`pillSuccessStyle`) | Verified green release or active session |
| `BLOCKED` / `FAIL` | Fail-Closed Coral Pill (`pillDangerStyle`) | Gate rejection, lease collision, or blocker |
| `ABORTED` | Neutral Slate Pill (`pillNeutralStyle`) | Rolled-back or tombstoned task |

### 3.2 Visual Components
- **Adaptive Rules (`uiRule`)**: Responsive horizontal rules (`ruleNarrow`, `ruleStandard`, `ruleWide`, `ruleSession`).
- **Dual-Core Hemisphere Accents**: Intent Purple (`uiIntentStyle`) and Substrate Cyan (`uiSubstrateStyle`).
- **HATEOAS Affordances (`affordance_render.go`)**: Structured `Next Actions` commands rendered below operational outputs.

---

## 4. Substrate Styling Primitives (`substrate/cli/theme.go`)

The Substrate centralizes adaptive terminal rendering inside `substrate/cli/theme.go` and `substrate/cli/theme_gradient.go`:
- `renderBrandTitle(title string)` — Adaptive gradient title header.
- `styleSemanticBadge(status string)` / `styleSemanticBadgeWidth(status string, width int)` — Semantic status pill formatting.
- `styleTaskKey(key string)` — Plane-colored task key formatting (`INT-*` purple, `SUB-*` cyan).
- `kvLine(label string, width int, value string)` — Aligned key-value telemetry rows.

---

## 5. Deterministic Invariants & Graceful Degradation

1. **Law 1 (Dual-Core Inversion)**: Lipgloss terminal UX delivers clear telemetry to developers and Tier 1 agents under indirect control.
2. **Law 2 (Non-Verbal Proof)**: Terminal formatting preserves deterministic command exit codes (`0` on pass, non-zero on rejection).
3. **Machine-Readable JSON Isolation**: Passing `--json` bypasses all Lipgloss styling, ANSI codes, and banners to emit raw JSON.
4. **Environment-Aware Color Stripping**: Setting `NO_COLOR`, `TERM=dumb`, or non-TTY stdout degrades output to clean plaintext without ANSI sequences.
5. **Sub-2ms Render Overhead**: Terminal layout and styling computations execute in under `2ms` without blocking CLI dispatch.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Target Component | Input Environment | Expected Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-UI-001` | `styleSemanticBadge` | `status: "EDIT"` in color TTY | Amber status pill | Law 4 (SSoT Alignment) |
| `TC-UI-002` | `NO_COLOR` Environment | `os.Setenv("NO_COLOR", "1")` | Plaintext without ANSI escapes | Law 2 (Deterministic Output) |
| `TC-UI-003` | `--json` Flag | `nomos task list --json` | Raw JSON array without ANSI | Machine-Readable Isolation |
| `TC-UI-004` | Theme Centralization | `substrate/cli/theme.go` | Shared styles across CLI commands | Law 6 (Canonical Singularity) |

---

## 7. Verification Boundaries & DoD Evidence

1. **DoD Compliance**: Passes all `42` Definition of Done quality gates (`nomos verify`) with exit code `0`.
2. **Markdown Integrity**: `BrokenWireGate` verifies all relative Markdown links on disk.
3. **Verification Command**:
   ```bash
   nomos verify --strict
   ```
