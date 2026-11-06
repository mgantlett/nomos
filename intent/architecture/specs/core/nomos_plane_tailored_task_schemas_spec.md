---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-09-29"
last_updated: "2026-11-05"
plane: intent
ssot_repository: nomos
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
    - INT-867
    - SUB-1558
title: "Nomos Task Contract & Universal 3-Section Backlog Card Specification"
type: spec
version: "4.1.1"
---

# Nomos Task Contract & Universal 3-Section Backlog Card Specification

## 1. Purpose

This specification defines the formal contract for Living Ledger tasks. `substrate/src/nomos/modules/task/` compiles the authoritative SSoT (`TaskRecord` in `task.go` and `ValidateCardDescription` / `SynthesizeCardMarkdown` in `card.go`). Deterministic gates enforce Law 4 (Single Source of Truth), Law 6 (Canonical Singularity), and Law 7 (Fail-Closed Pre-Flight).

[`task_lifecycle_cli_spec.md`](task_lifecycle_cli_spec.md) codifies twin semantics and lifecycle state transitions.

---

## 2. Two-Lane Ceremony Architecture (`INT-864`)

Nomos v3 suffered from over-ceremonialized task inception across 7 section taxonomies, 35+ H2 headers, regex bans on `TODO`, and 4 sizing flags. Early Nomos v4 swung to the opposite extreme with unvalidated `-d` prose walls and `burden: 0`.

Nomos v4.1 resolves this tension by splitting ceremony strictly by **task durability**:

| Lane | CLI Surface | Ceremony Level | Body & Sizing Rule |
| :--- | :--- | :--- | :--- |
| **Fast Lane (Transient)** | `nomos task chore "<title>"` | **Zero template** | Provisions an `EDIT` worktree in 1 turn with `burden: 1`. |
| **Backlog Lane (Durable)** | `nomos task create`, `nomos task pair` | **Fail-closed 3-section card** | Rejects unstructured `-d` prose (`ERR_VALIDATION_FAILED`) and auto-derives `burden` (`1-8`). |

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
   - `--scope <text>` - populates `## Scope & Context` (>= 10 runes).
   - `--ac <item>` (repeatable) - populates `- [ ] <item>` checklist bullets under `## Acceptance Criteria` (>= 1 non-empty item).
   - `--verify <text>` - populates `## Verification` (>= 5 runes).
   - On `nomos task pair`, optional `--int-scope` / `--sub-scope` overrides allow distinct Intent vs. Substrate scope descriptions while sharing `--scope` as fallback.
2. **Markdown Body Input (`-F <file>`, `-F -` stdin heredoc, or `-d <markdown>`):**
   - Validated case-insensitively by fuzzy H2/H3 section matching:
     - **Scope section** (`Scope`, `Scope & Context`, `Context & Scope`, or `Problem & Scope`): >= 1 non-comment content line (>= 10 runes).
     - **Acceptance Criteria section** (`Acceptance Criteria`): >= 1 non-empty Markdown task-list item (`- [ ] <text>` or `- [x] <text>`).
     - **Verification section** (`Verification`): >= 1 non-comment content line (>= 5 runes).
   - The validator rejects literal unexpanded `\n` escape sequences outside fenced code blocks and directs callers to `--scope/--ac/--verify` or `-F -`.
   - Unlike v3, the validator never triggers false positives on legitimate domain tokens (`TODO`, `StatusTodo`, `<flag>`).

### 3.2 Fail-Closed HATEOAS Remedy (Law 1 & Law 7)

Invoking `nomos task create` or `nomos task pair` with an empty description or single-paragraph `-d` prose aborts (exit 1) and emits a 3-line remedy:

```text
Hint: Durable backlog tasks require the 3-section card contract (use 'nomos task chore "<title>"' for 1-turn fixes):
  nomos task create "<title>" --plane substrate --scope "<context & target files>" --ac "<deliverable>" --verify "<test command>"
  # Or pass structured Markdown via stdin: nomos task create "<title>" -F - <<'EOF' ... EOF
```

### 3.3 Deterministic Burden Auto-Sizing

- Optional `-b, --burden <1-10>` sets explicit `#TaskRecord.burden`.
- Omitting `--burden` (`0`) derives `burden` (`2-8`) from checklist count and body density. Backlog cards never display `0 pts`.

---

## 4. Field Contract (`#TaskRecord`)

| Field | Rule |
| :--- | :--- |
| `key` | `INT-<N>` / `SUB-<N>`, derived from `plane` unless explicit valid `--key` supplied (Law 6). |
| `plane` | Strictly `"intent"` or `"substrate"` (`INT-*` -> `intent`, `SUB-*` -> `substrate`). |
| `project` | Repository key in `nomos_global.db`, resolved from `nomos.yaml` or `--project` / `nomos task project`. |
| `schema_type` | One of `"intent"`, `"substrate"`, `"membrane"`, `"infra"`, `"cycle"`, `"debt"`, `"spike"`. |
| `title` | Required, >= 5 characters. |
| `description` | Required 3-section Markdown card on `create`/`pair`; historical SQLite rows remain readable. |
| `burden` | Integer `1-10` on newly created tasks (`--burden` override or deterministic density derivation). |
| `twin_key` | Optional cross-plane `INT-<N>` / `SUB-<N>` pointer (`--twin-of` or atomic `nomos task pair`). |

---

## 5. Table-Driven Verification Matrix

Under Law 2 (**The Non-Verbal Proof Law**), deterministic verification vectors validate the Universal 3-Section Task Card Contract:

| Test ID | Scenario | Input Conditions | Expected Outcome | Exit Code |
| :--- | :--- | :--- | :--- | :---: |
| **TV-SCH-01** | Structured Flag Synthesis | `--scope`, `--ac` (x2), `--verify` on `task create` | Renders canonical 3-section Markdown + derives non-zero `burden` | 0 |
| **TV-SCH-02** | Stdin Markdown Card (`-F -`) | Valid 3-section Markdown via stdin | Accepted and persisted verbatim in `SQLiteTaskStore` | 0 |
| **TV-SCH-03** | Plain-Text `-d` Fail-Closed | `nomos task create "Title" -d "flat paragraph"` | Rejected (`ERR_VALIDATION_FAILED`) with `--scope/--ac/--verify` HATEOAS hint | 1 |
| **TV-SCH-04** | Empty Checkbox Rejection | `## Acceptance Criteria` contains only `- [ ] ` | Rejected (`ERR_VALIDATION_FAILED`) | 1 |
| **TV-SCH-05** | Domain Token Tolerance | Card mentions `StatusTodo` and `<task-key>` | Accepted without false-positive regex rejection | 0 |
| **TV-SCH-06** | Fast-Lane Chore Exemption | `nomos task chore "Quick typo fix"` | Provisions `EDIT` worktree immediately with zero template flags | 0 |
| **TV-SCH-07** | Explicit Burden Bounds | `--burden 11` or `--burden -1` | Rejected (`ERR_VALIDATION_FAILED`: must be 1-10) | 1 |
| **TV-SCH-08** | Go SSoT Validation | Serialized `TaskRecord` & `CardSections` | Validates cleanly against `task.ValidateTaskKey` and `task.ValidateCardDescription` | 0 |
