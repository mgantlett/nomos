---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-09-29"
last_updated: "2026-11-01"
lineage:
  - INT-371
  - INT-864
  - INT-867
  - SUB-1549
  - SUB-1558
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Task Contract & Universal 3-Section Backlog Card Specification"
type: spec
version: "4.1.1"
---

# Nomos Task Contract & Universal 3-Section Backlog Card Specification

---

## 1. Executive Summary & Problem Statement

Nomos v3 suffered from over-ceremonialized task inception across 7 section taxonomies, 35+ H2 headers, regex bans on `TODO`, and 4 sizing flags. Early Nomos v4 swung to the opposite extreme with unvalidated `-d` prose walls and `burden: 0`.

This specification defines the formal contract for Living Ledger tasks across both `INT-*` and `SUB-*` planes. `substrate/src/nomos/modules/task/` compiles the authoritative Go SSoT (`TaskRecord` in `task.go` and `TaskCardInput` / `ValidateTaskCardBody` in `card.go`). [`task_lifecycle_cli_spec.md`](task_lifecycle_cli_spec.md) codifies twin semantics and lifecycle state transitions.

---

## 2. Axiomatic Invariants & Governing Laws

- **Law 1 (Dual-Core Inversion):** Rejecting unstructured backlog prose emits a deterministic 3-line HATEOAS remedy (`TaskCardRemedy`) guiding Tier 1 cognitive agents on turn 1.
- **Law 2 (Non-Verbal Proof):** Deterministic verification vectors (`TC-SCH-01`..`TC-SCH-08`) verify card synthesis, burden sizing, and key singularity.
- **Law 4 (Single Source of Truth):** `task.TaskRecord` and `task.TaskCardInput` form the sole compiled Go SSoT for task persistence and validation.
- **Law 6 (Canonical Singularity):** Task keys adhere strictly to `INT-<N>` or `SUB-<N>`, and durable backlog tasks share one canonical 3-section card schema.
- **Law 7 (Fail-Closed Pre-Flight):** `nomos task create` and `nomos task pair` reject empty descriptions, unstructured `-d` prose, and out-of-range `--burden` values.

Nomos resolves ceremony friction by splitting inception strictly by **task durability**:

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

Callers satisfy `#TaskCardInput` on turn 1 via either structured synthesis flags or Markdown body input:

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

## 5. Compiled Go SSoT Contracts

The authoritative Go SSoT declarations reside in `substrate/src/nomos/modules/task/task.go` and `substrate/src/nomos/modules/task/card.go`:

```go
package task

// SchemaType defines the plane-tailored task classification.
type SchemaType string

const (
	SchemaIntent    SchemaType = "intent"
	SchemaSubstrate SchemaType = "substrate"
	SchemaMembrane  SchemaType = "membrane"
	SchemaInfra     SchemaType = "infra"
	SchemaCycle     SchemaType = "cycle"
	SchemaDebt      SchemaType = "debt"
	SchemaSpike     SchemaType = "spike"
)

// TaskRecord defines the formal specification for Living Ledger task records (#TaskRecord).
type TaskRecord struct {
	ID            int        `json:"id,omitempty"`
	Key           string     `json:"key"`
	Plane         string     `json:"plane"`
	Project       string     `json:"project,omitempty"`
	SchemaType    SchemaType `json:"schema_type"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Status        Status     `json:"status"`
	Tier          Tier       `json:"tier"`
	Priority      int        `json:"priority"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
	CompletedAt   *string    `json:"completed_at,omitempty"`
	Dependencies  []string   `json:"dependencies,omitempty"`
	TwinKey       string     `json:"twin_key,omitempty"`
	Assignee      string     `json:"assignee,omitempty"`
	ArcID         string     `json:"arc_id,omitempty"`
	IntentDelta   string     `json:"intent_delta,omitempty"`
	Burden        int        `json:"burden,omitempty"`
	ContextBurden int        `json:"context_burden,omitempty"`
	ArchLayer     int        `json:"arch_layer,omitempty"`
}

// TaskCardInput models the fail-closed 3-section backlog card contract (#TaskCardInput).
type TaskCardInput struct {
	Scope              string   `json:"scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Verification       string   `json:"verification"`
	Burden             int      `json:"burden,omitempty"`
}

func ValidateTaskKey(key string) error
func ValidateTaskRecord(r *TaskRecord) error
func RenderTaskCardBody(in TaskCardInput) (string, error)
func ValidateTaskCardBody(body string) error
func TaskCardRemedy(subcommand, title, plane string) string
```

---

## 6. Table-Driven Verification Vectors

Under Law 2 (**The Non-Verbal Proof Law**), deterministic verification vectors validate the Universal 3-Section Task Card Contract:

| Test ID | Scenario | Input Conditions | Expected Outcome | Exit Code |
| :--- | :--- | :--- | :--- | :---: |
| `TC-SCH-01` | Structured Flag Synthesis | `--scope`, `--ac` (x2), `--verify` on `task create` | Renders canonical 3-section Markdown + derives non-zero `burden` | 0 |
| `TC-SCH-02` | Stdin Markdown Card (`-F -`) | Valid 3-section Markdown via stdin | Accepted and persisted verbatim in `SQLiteTaskStore` | 0 |
| `TC-SCH-03` | Plain-Text `-d` Fail-Closed | `nomos task create "Title" -d "flat paragraph"` | Rejected (`ERR_VALIDATION_FAILED`) with `--scope/--ac/--verify` HATEOAS remedy | 1 |
| `TC-SCH-04` | Empty Checkbox Rejection | `## Acceptance Criteria` contains only `- [ ] ` | Rejected (`ERR_VALIDATION_FAILED`) | 1 |
| `TC-SCH-05` | Domain Token Tolerance | Card mentions `StatusTodo` and `<task-key>` | Accepted without false-positive regex rejection | 0 |
| `TC-SCH-06` | Fast-Lane Chore Exemption | `nomos task chore "Quick typo fix"` | Provisions `EDIT` worktree immediately with zero template flags | 0 |
| `TC-SCH-07` | Explicit Burden Bounds | `--burden 11` or `--burden -1` | Rejected (`ERR_VALIDATION_FAILED`: must be 1-10) | 1 |
| `TC-SCH-08` | Go SSoT Validation | Serialized `TaskRecord` & `TaskCardInput` | Validates cleanly against `task.ValidateTaskKey` and `task.ValidateTaskCardBody` | 0 |

---

## 7. Cross-Plane Traceability & DoD Verification

1. **Compiled Go SSoT Paths:**
   - `substrate/src/nomos/modules/task/task.go` (`TaskRecord`, `SchemaType`, `ValidateTaskKey`, `ValidateTaskRecord`).
   - `substrate/src/nomos/modules/task/card.go` (`TaskCardInput`, `RenderTaskCardBody`, `ValidateTaskCardBody`, `TaskCardRemedy`).
   - `substrate/src/nomos/core/storage/db.go` (`SchemaDDL` relational `tasks` table parity).
2. **7x3 Invariant Matrix Coordinates:**
   - `Cell 6.3 (canonical_singularity)` enforces pure 2-plane `INT-<N>` / `SUB-<N>` task key singularity.
   - `Cell 4.1 (intent_traceability)` and `Cell 4.2 (contract_parity)` enforce Go SSoT contract and schema alignment.
   - `Cell 2.2 (go_toolchain / coverage)` executes `substrate/src/nomos/modules/task/` unit verification suites.
3. **Deterministic Verification:**
   - Execute `nomos verify` across all 42 Definition of Done quality gates and assert exit code `0`.
