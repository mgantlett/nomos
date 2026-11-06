---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-04"
lineage:
  - INT-922
  - SUB-1744
plane: intent
ssot_repository: nomos
status: active
title: "Open-Core Terminal UI Architecture & Totem CLI Ergonomics"
type: spec
version: "4.1.0"
---

# Open-Core Terminal UI Architecture & Totem CLI Ergonomics

---

## 1. Executive Summary & Problem Statement

Early Nomos CLI iterations rendered `nomos verify`, `nomos task`, and `nomos status` using raw `fmt.Printf` calls and ad-hoc ANSI strings. That baseline lacked terminal-width awareness, semantic status badges, and clean plaintext degradation in headless CI pipelines.

This specification codifies the zero-daemon, Go-native terminal UI built on `charmbracelet/lipgloss` for the Nomos open-core harness. Under the **Open-Core Indirect-Control Boundary** ([`agent_tiers_and_indirect_control_spec.md`](agent_tiers_and_indirect_control_spec.md)), `charmbracelet/lipgloss` rendering applies exclusively to the open-core CLI. Commercial web control-plane surfaces remain outside this repository.

---

## 2. Axiomatic Invariants & Governing Laws

1. **Law 1 (The Dual-Core Inversion Law)**: Lipgloss terminal UX delivers deterministic telemetry to developers and Tier 1 agents operating under indirect control.
2. **Law 2 (The Non-Verbal Proof Law)**: Terminal formatting preserves deterministic exit codes (`0` on pass, non-zero on rejection). `NO_COLOR`, `TERM=dumb`, and non-TTY stdout strip ANSI sequences cleanly.
3. **Law 4 (The Single Source of Truth Law)**: Semantic badges align with `task.TaskRecord.Status`, while `--json` bypasses Lipgloss styling to emit raw JSON.
4. **Law 6 (The Canonical Singularity Law)**: All CLI commands share one centralized token palette inside `substrate/cli/theme.go` with sub-`2ms` render overhead.
5. **Law 7 (The Fail-Closed Pre-Flight Law)**: Open-core CLI packages enforce strict boundary isolation (`Cell 1.1 (boundary)`) and never embed web servers, HTTP asset archives, or browser shims.

---

## 3. Open-Core Indirect-Control Boundary

To preserve a pure single-binary developer harness under the Open-Core Indirect-Control Boundary:

| Layer | Technology | Distribution | Purpose |
| :--- | :--- | :--- | :--- |
| **Open-Core Layer (Terminal CLI)** | Go + `charmbracelet/lipgloss` | Single binary (`nomos`) | Local governance harness, DoD dashboard, and zero-daemon task lifecycle. |
| **Enterprise Layer (Out of Scope)** | Commercial Extension | External distribution | Multi-seat enterprise orchestration and hosted observability outside open-core. |

Open-core CLI packages never embed web servers, HTTP asset archives, or browser shims.

---

## 4. UI Token Taxonomy & Semantic Styling

The Substrate centralizes all styling tokens inside `substrate/cli/theme.go` (`package cli`).

### 4.1 Status Badge Palette (Canonical Task State Alignment)

In alignment with `task.TaskRecord.Status`, adaptive status pills (`styleSemanticBadge`) map deterministically to semantic tokens:

| Status Code | Semantic Style | Usage |
| :--- | :--- | :--- |
| `IDLE` / `TODO` | Neutral Slate Pill (`pillNeutralStyle`) | Unstarted tasks in backlog or idle trunk baseline |
| `PLAN` / `LEARN` | Celestial Purple Pill (`pillPurpleStyle`) | Architectural formulation and episodic distillation |
| `EDIT` | Substrate Sky Cyan Pill (`pillCyanStyle`) | Active worktree mutation and code authoring |
| `REVIEW` / `WARN` | Solar Amber Pill (`pillWarnStyle`) | Definition of Done audit and warning diagnostics |
| `DONE` / `PASS` / `SYNC` / `ACTIVE` | Verification Mint Pill (`pillSuccessStyle`) | Verified green release, 2PC sync, or active session |
| `FAIL` / `ABORTED` / `DENY` | Fail-Closed Coral Pill (`pillDangerStyle`) | Gate rejection, lease collision, or tombstoned abort |

### 4.2 Visual Components
- **Adaptive Rules (`uiRule`)**: Responsive horizontal rules (`ruleNarrow`, `ruleStandard`, `ruleWide`, `ruleSession`).
- **Dual-Core Hemisphere Accents**: Intent Purple (`uiIntentStyle`) and Substrate Cyan (`uiSubstrateStyle`).
- **HATEOAS Affordances (`affordance_render.go`)**: Structured `Next Actions` commands rendered below operational outputs.

---

## 5. Compiled Go Styling Primitives (`substrate/cli/theme.go`)

The Substrate centralizes adaptive terminal rendering inside `substrate/cli/theme.go` and `substrate/cli/theme_gradient.go`:

```go
package cli

const (
	ruleNarrow   = 60
	ruleStandard = 76
	ruleWide     = 92
	ruleSession  = 108
)

// styleTaskKey colors INT-* keys in Celestial Purple and SUB-* keys in Substrate Sky Cyan.
func styleTaskKey(key string) string

// styleSemanticBadge renders a tinted glass pill in TTY mode and compact text in Ascii mode.
func styleSemanticBadge(token string) string

// styleSemanticBadgeWidth renders a tinted pill badge padded cleanly to exact visible column width.
func styleSemanticBadgeWidth(token string, width int) string

// kvLine formats an aligned "  Label : Value" telemetry line.
func kvLine(label string, labelWidth int, value string) string

// FormatCLIError renders top-level CLI errors with structured Lipgloss emphasis for human operators.
func FormatCLIError(err error) string
```

- `renderBrandTitle(title string)`: Adaptive gradient title header (`substrate/cli/theme_gradient.go`).
- `styleSemanticBadge(token string)` / `styleSemanticBadgeWidth(token string, width int)`: Semantic status pill formatting with `termenv.Ascii` fallback.
- `styleTaskKey(key string)`: Plane-colored task key formatting (`INT-*` purple, `SUB-*` cyan).
- `kvLine(label string, labelWidth int, value string)`: Aligned key-value telemetry rows.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Target Component | Input Environment | Expected Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-UI-001` | `styleSemanticBadge` | `token: "EDIT"` in color TTY | Tinted status pill | Law 4 (SSoT Alignment) |
| `TC-UI-002` | `NO_COLOR` Environment | `os.Setenv("NO_COLOR", "1")` | Plaintext without ANSI escapes | Law 2 (Deterministic Output) |
| `TC-UI-003` | `--json` Flag | `nomos task list --json` | Raw JSON array without ANSI | Machine-Readable Isolation |
| `TC-UI-004` | Theme Centralization | `substrate/cli/theme.go` | Shared styles across CLI commands | Law 6 (Canonical Singularity) |

---

## 7. Cross-Plane Traceability & DoD Verification

- **Intent Lineage**: `INT-922`.
- **Substrate SSoT**: `SUB-1744` (`substrate/cli/theme.go`, `substrate/cli/theme_gradient.go`, `substrate/cli/affordance_render.go`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 1.1 (boundary)`, `Cell 4.1 (broken_wire)`, `Cell 6.2 (zero_alias)`.
- **Deterministic Verification**: `nomos verify --strict` evaluates all `42` Definition of Done quality gates and verifies relative Markdown links with exit code `0`.
