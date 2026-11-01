---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-09-29"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - nomos
    - architecture
    - schemas
    - task
    - living-ledger
    - INT-371
    - INT-864
    - SUB-1549
title: Nomos Task Contract & Universal 3-Section Backlog Card Specification
type: spec
version: "4.1.0"
ssot_repository: nomos
---

# Nomos Task Contract & Universal 3-Section Backlog Card Specification

## 1. Purpose

This specification defines the formal contract for Living Ledger tasks. The authoritative Single Source of Truth (SSoT) is the compiled CUE contract [`intent/contracts/cue/task.cue`](../../../contracts/cue/task.cue) (`#TaskRecord` and `#TaskCardInput`), enforced deterministically by the Go substrate (`substrate/src/nomos/modules/task/`) under Law 4 (The Single Source of Truth Law), Law 6 (The Canonical Singularity Law), and Law 7 (The Fail-Closed Pre-Flight Law).

Twin semantics and lifecycle state transitions are codified in [`task_lifecycle_cli_spec.md`](task_lifecycle_cli_spec.md) and [`task.cue`](../../../contracts/cue/task.cue).

---

## 2. Two-Lane Ceremony Architecture (`INT-864`)

Nomos v3 suffered from over-ceremonialized task inception (7 distinct section taxonomies, 35+ memorized H2 headers, brittle regex bans on `TODO`, and 4 manual sizing flags), whereas early Nomos v4 swung to the opposite extreme (unvalidated `-d` prose walls and `burden: 0`).

Nomos v4.1 resolves this tension by splitting ceremony strictly by **task durability**:

| Lane | CLI Surface | Ceremony Level | Body & Sizing Rule |
| :--- | :--- | :--- | :--- |
| **Fast Lane (Transient)** | `nomos task chore "<title>"` | **Zero template** | Immediate 1-turn worktree inception in `EDIT` phase; auto-populates minimal chore description and `burden: 1`. |
| **Backlog Lane (Durable)** | `nomos task create`, `nomos task pair` | **Fail-closed 3-section card** | Rejects empty or unstructured `-d` prose (`ERR_VALIDATION_FAILED`) with a 3-line HATEOAS remedy; auto-derives `burden` (`1-8`) when `--burden` is omitted. |

---

## 3. Universal 3-Section Backlog Card Contract (`#TaskCardInput`)

Every durable backlog task across **both** `INT-*` and `SUB-*` planes shares a single canonical 3-section Markdown structure:

```markdown
## Scope & Context
<Target files/specs/twin key and concise statement of the problem being solved>

## Acceptance Criteria
- [ ] <Concrete deliverable 1>
- [ ] <Concrete deliverable 2>

## Verification
- <Concrete test command, cleanroom vector, or gate proof>
```

### 3.1 Turn-1 Authoring Modes (Zero Multi-Turn Friction)

Callers may satisfy `#TaskCardInput` on turn 1 via either structured synthesis flags or Markdown body input:

1. **Structured CLI Synthesis Flags (Recommended for 1-liners & agents):**
   - `--scope <text>` — populates `## Scope & Context` (>= 10 runes).
   - `--ac <item>` (repeatable) — populates `- [ ] <item>` checklist bullets under `## Acceptance Criteria` (>= 1 non-empty item).
   - `--verify <text>` — populates `## Verification` (>= 5 runes).
   - On `nomos task pair`, optional `--int-scope` / `--sub-scope` overrides allow distinct Intent vs. Substrate scope descriptions when needed while sharing `--scope` as fallback.
2. **Markdown Body Input (`-F <file>`, `-F -` stdin heredoc, or `-d <markdown>`):**
   - Validated case-insensitively by fuzzy H2/H3 section matching:
     - **Scope section** (`Scope`, `Scope & Context`, `Context & Scope`, or `Problem & Scope`): >= 1 non-comment content line (>= 10 runes).
     - **Acceptance Criteria section** (`Acceptance Criteria`): >= 1 non-empty Markdown task-list item (`- [ ] <text>` or `- [x] <text>`).
     - **Verification section** (`Verification`): >= 1 non-comment content line (>= 5 runes).
   - Literal unexpanded `\n` escape sequences outside fenced code blocks are rejected with guidance to use `--scope/--ac/--verify` or `-F -`.
   - Unlike v3, legitimate domain tokens (`TODO`, `StatusTodo`, `<flag>`) are **never** false-positived.

### 3.2 Fail-Closed HATEOAS Remedy (Law 1 & Law 7)

Invoking `nomos task create` or `nomos task pair` with an empty description or single-paragraph `-d` prose fails closed (exit 1) and emits a copy-pasteable 3-line remedy:

```text
Hint: Durable backlog tasks require the 3-section card contract (use 'nomos task chore "<title>"' for 1-turn fixes):
  nomos task create "<title>" --plane substrate --scope "<context & target files>" --ac "<deliverable>" --verify "<test command>"
  # Or pass structured Markdown via stdin: nomos task create "<title>" -F - <<'EOF' ... EOF
```

### 3.3 Deterministic Burden Auto-Sizing

- Optional `-b, --burden <1-10>` sets explicit `#TaskRecord.burden`.
- When `--burden` is omitted (`0`), the substrate deterministically derives `burden` (`2-8`) from checklist item count and scope/body density so Glass Cockpit and Living Ledger cards never display `0 pts` on backlog items.

---

## 4. Field Contract (`#TaskRecord`)

| Field | Rule |
| :--- | :--- |
| `key` | `INT-<N>` / `SUB-<N>`, derived from `plane` unless explicit valid `--key` supplied (Law 6). |
| `plane` | Strictly `"intent"` or `"substrate"` (`INT-*` -> `intent`, `SUB-*` -> `substrate`). |
| `schema_type` | One of `"intent"`, `"substrate"`, `"membrane"`, `"infra"`, `"cycle"`, `"debt"`, `"spike"`. |
| `title` | Required, >= 5 characters. |
| `description` | Required 3-section Markdown card on `create`/`pair`; historical SQLite rows remain readable without retroactive breakage. |
| `burden` | Integer `1-10` on newly created tasks (`--burden` override or deterministic density derivation). |
| `twin_key` | Optional cross-plane `INT-<N>` / `SUB-<N>` pointer (`--twin-of` or atomic `nomos task pair`). |

---

## 5. Table-Driven Verification Matrix

Under Law 2 (**The Non-Verbal Proof Law**), the Universal 3-Section Task Card Contract is validated against the following deterministic test vectors:

| Test ID | Scenario | Input Conditions | Expected Outcome | Exit Code |
| :--- | :--- | :--- | :--- | :---: |
| **TV-SCH-01** | Structured Flag Synthesis | `--scope`, `--ac` (x2), `--verify` on `task create` | Renders canonical 3-section Markdown + derives non-zero `burden` | 0 |
| **TV-SCH-02** | Stdin Markdown Card (`-F -`) | Valid 3-section Markdown via stdin | Accepted and persisted verbatim in `SQLiteTaskStore` | 0 |
| **TV-SCH-03** | Plain-Text `-d` Fail-Closed | `nomos task create "Title" -d "flat paragraph"` | Rejected (`ERR_VALIDATION_FAILED`) with `--scope/--ac/--verify` HATEOAS hint | 1 |
| **TV-SCH-04** | Empty Checkbox Rejection | `## Acceptance Criteria` contains only `- [ ] ` | Rejected (`ERR_VALIDATION_FAILED`) | 1 |
| **TV-SCH-05** | Domain Token Tolerance | Card mentions `StatusTodo` and `<task-key>` | Accepted without false-positive regex rejection | 0 |
| **TV-SCH-06** | Fast-Lane Chore Exemption | `nomos task chore "Quick typo fix"` | Provisions `EDIT` worktree immediately with zero template flags | 0 |
| **TV-SCH-07** | Explicit Burden Bounds | `--burden 11` or `--burden -1` | Rejected (`ERR_VALIDATION_FAILED`: must be 1-10) | 1 |
| **TV-SCH-08** | CUE SSoT Unification | Serialized `#TaskRecord` & `#TaskCardInput` | Unifies cleanly against `intent/contracts/cue/task.cue` | 0 |
