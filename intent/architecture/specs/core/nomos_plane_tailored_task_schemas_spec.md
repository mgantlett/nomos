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
title: Nomos Task Contract & Plane-Tailored Schemas Specification
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos Task Contract & Plane-Tailored Schemas Specification

## 1. Purpose

This specification defines the formal contract for Living Ledger tasks. The authoritative Single Source of Truth (SSoT) is the compiled CUE contract [`intent/contracts/cue/task.cue`](../../../contracts/cue/task.cue) (`#TaskRecord`), embedded and enforced deterministically by the Substrate engine (`substrate/src/nomos/modules/task/task.go`) under Law 4 (The Single Source of Truth Law) and Law 1 (Dual-Core Inversion).

Twin semantics (pure Intent tasks, bilateral pointers, ARC grouping) are codified in
[`task_lifecycle_cli_spec.md`](task_lifecycle_cli_spec.md) and [`task.cue`](../../../contracts/cue/task.cue).

## 2. Principle: reject, never default

Task inception is fail-closed (Law 7). The substrate never fills in a missing value on the
caller's behalf. Specifically, it does not:

- default the **type** (previously `Task`), or default the **plane** independently of the type;
- inject a schema **template** as the body when none is supplied;
- invent **burden** from body length or fall back to `1/1/1`;
- scaffold a **twin** with a template body.

Each of these defaults produced the weak tasks observed in October 2026 (for example SUB-469:
type `Intent`, plane `substrate`, template body, fabricated burden 8, twin in the wrong direction).

## 3. Field contract

| Field | Rule |
|---|---|
| `type` | Required. Strictly one of the 7 canonical schemas: `intent`, `substrate`, `membrane`, `infra`, `cycle`, `debt`, `spike`. Zero aliases tolerated under Law 6 (The Canonical Singularity Law). |
| `plane` | Deterministically derived from `type` under Law 4: `intent` maps to `plane: intent`; `substrate`, `membrane`, `infra`, `cycle`, `debt` map to `plane: substrate`. `--plane` is accepted and required solely for `spike`. |
| `key` | `INT-<N>` / `SUB-<N>`, derived from plane (Law 6). |
| `title` | Required, ≤ 100 chars, no placeholders. |
| `body` | Required: `-F <file>`, `-F -` (stdin), or `-d <markdown>`. |
| `context_burden`, `logic_depth` | Required, 1–5 each. |
| `burden` | Derived: context + logic (2–10). |
| `tier` | Derived: T1 when burden ≥ 6, else T2. |
| `arch_layer` | **Removed.** Two conflicting taxonomies existed and the value duplicated plane + type. The column stays read-only for historical rows. |

## 4. Section and content rules

Each schema has required sections (see `sections:` in the contract). A section passes only if,
after stripping HTML comments, it has at least one real content line, and list sections have at
least one non-empty item. Bodies are rejected if they contain:

- angle-bracket placeholders such as `<path-to-spec.md>`;
- empty checkboxes, `TODO`, `TBD`;
- the template warning banner;
- literal `\n` sequences (a sign of an unexpanded shell string; use `-F -` instead).

The Gen 2 four-section fallback is accepted when reading existing rows and rejected for new tasks.

## 5. Twins

- `--twin-of <KEY>` links a new task to an existing task on the other plane. Both tasks must
  already have real bodies.
- Template twin scaffolding (`--twin`) is removed.
- `intent`-type tasks require a twin, or `--no-twin "<reason>"`, which is journaled.
  This keeps the pure Intent task case from the pairing spec §3.2 explicit rather than silent.

## 6. Enforcement-gap analysis: why the parity engine passed broken task inception

All 23 DoD gates passed on SUB-467 while task inception was producing invalid tasks. The gates
were not lying, but none of them measured what we assumed:

1. **Intent Substrate Parity Gate** only byte-compares `AGENTS.md` and workflow files against their
   embedded copies. It checks that assets are in sync. It does not check that the code implements a spec.
2. **Intent Traceability Gate** checks that each `@intent INT-N` tag (a) exists in the DB on the intent
   plane and (b) appears somewhere in a vault Markdown file. The task module is tagged `INT-337`, which
   satisfies both, but:
   - INT-337 is a stub: status `IN_PROGRESS`, with a one-line body;
   - its only vault reference is `architecture/engine/missing_traceability.md`, a bare list of 14 IDs
     with no frontmatter or content, which exists only to satisfy check (b). The list includes
     `INT-324`, the traceability gate's own tag.

   The 97% "traceability density" is therefore a measure of how many functions carry a tag, not of
   whether the code conforms to any spec.
3. **This spec was stale.** It named paths (`src/nomos/modules/schema/`, `src/nomos/cmd/task_create_guided.go`)
   and a `--schema` flag that do not exist, and it claimed fail-closed behaviour that the code did not have.
   No gate compares a spec's claims with the code.
4. **The rules were duplicated.** Required sections existed both in Go (`modules/task/schema.go`) and in
   prose, so drift between them was invisible.

### Remedies

| Gap | Remedy | Owner |
|---|---|---|
| Rules duplicated in Go and prose | `task_contract.yaml` is the single source; the substrate embeds it and loads sections and content rules from it | SUB twin |
| Spec and code can drift | Add `task_contract.yaml` to the Intent Substrate Parity Gate byte-parity set, so editing the contract without rebuilding the substrate fails verification | SUB twin |
| Traceability anchors can be faked | A valid anchor must be a vault document with frontmatter that lists the ID in `tags`; bare ID-list files are rejected; anchors whose intent task is a stub (single-line body or contract-invalid) fail | SUB (gate hardening) |
| `missing_traceability.md` | Delete it, then re-anchor each listed ID to a real spec or mark it as debt | SUB (gate hardening) |

What the gates can prove after these remedies: the contract the CLI enforces is byte-identical to
the reviewed intent contract, and every `@intent` tag points to a non-stub, frontmatter-anchored spec.
What they still cannot prove: that a spec's prose is *correct*. That remains a Tier 1 review
responsibility during the `REVIEW` phase.

## 7. Table-Driven Verification Matrix

Under Law 2 (**The Non-Verbal Proof Law**), the Plane-Tailored Task Schemas are validated against the following deterministic test vectors:

| Test ID | Scenario | Input Conditions | Pre-Conditions | Expected Outcome | Exit Code |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **TV-SCH-01** | Canonical Type Acceptance | `type = "intent"`, valid body | Valid intent sections | Accepted: key `INT-<N>`, plane `intent` | 0 |
| **TV-SCH-02** | Type Alias Rejection | `type = "spec"` or `type = "refactor"` | Alias attempted | Rejected: `ERR_INVALID_SCHEMA_TYPE` (Law 6) | 1 |
| **TV-SCH-03** | Deterministic Plane Derivation | `type = "substrate"` without `--plane` | Valid substrate body | Accepted: plane `substrate`, key `SUB-<N>` | 0 |
| **TV-SCH-04** | Conflicting Plane Mismatch | `type = "intent"`, `--plane substrate` | Intent schema used | Rejected: `ERR_PLANE_TYPE_MISMATCH` (fail-closed) | 1 |
| **TV-SCH-05** | Spike Missing Plane | `type = "spike"` without `--plane` | Spike schema used | Rejected: `ERR_SPIKE_REQUIRES_PLANE` (Law 7) | 1 |
| **TV-SCH-06** | Placeholder Content Rejection | Body contains `<path-to-spec.md>` | Placeholder present | Rejected: `ERR_PLACEHOLDER_FORBIDDEN` | 1 |
| **TV-SCH-07** | Burden Boundary Validation | `context_burden = 6` or `logic_depth = 0` | Out of range 1–5 | Rejected: `ERR_BURDEN_OUT_OF_BOUNDS` | 1 |
| **TV-SCH-08** | CUE SSoT Unification | Serialized task record | Unification check | Unifies against `#TaskRecord` in `task.cue` | 0 |

All task schema validation executes synchronously in Go substrate and satisfies all Definition of Done (DoD) quality gates via `nomos verify`.
