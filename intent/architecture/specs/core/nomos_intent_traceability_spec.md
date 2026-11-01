---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-05"
last_updated: "2026-11-01"
plane: intent
status: active
title: Nomos Intent Traceability Specification
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos Intent Traceability Specification

## 1. Executive Summary
The Nomos Open-Core protocol enforces Law 4 (Single Source of Truth) by maintaining intent specifications separately from the substrate Go code. However, without a cryptographic or structural linkage between the Substrate AST and the Intent plane, there is no deterministic proof that a given block of code implements a specific Intent task.

This specification defines the exact code-level tagging syntax (`// @intent <TARGET>`) required to map Go substrate components to their originating Intent specifications, semantic heading anchors, or declarative CUE schema contracts.

---

## 2. Axiomatic Invariants & Laws Codified
- **Law 2: The Non-Verbal Proof Law**: Substrate code must compile and pass AST proofs. Code tracing must be verified by the Go compiler and AST parsers, not by human review.
- **Law 4: The Single Source of Truth Law**: Intent specifications reside solely in the `intent/` plane, but the code implementation must carry a pointer back to that SSoT.
- **Law 6: The Canonical Singularity Law**: All Intent task keys adhere strictly to `INT-[0-9]+` with zero aliases or non-canonical prefixes.

---

## 3. The `@intent` Traceability Syntax
To enforce traceability, all significant packages, core structs, interfaces, and public functions within the Substrate MUST include an `@intent` tag in their Go docstrings.

### 3.1 Tag Formats
The tag parser (`ParseIntentTag`) supports three canonical binding forms:

1. **Canonical Intent Task Key**:
   ```go
   // @intent INT-541
   ```
   Binds the annotated Go symbol directly to the SSoT task record in the Living Ledger.

2. **Semantic Heading Anchor**:
   ```go
   // @intent INT-541#continuous-collision-detection
   ```
   Binds the symbol to a specific slugified Markdown section heading within the Intent specification.

3. **Declarative CUE Schema Contract**:
   ```go
   // @intent CUE:physics#PhysicsFrame
   // @intent CUE:pipeline
   ```
   Binds the Go DTO or sensor implementation directly to a compiled schema in `intent/contracts/cue/<name>.cue`.

### 3.2 Example Implementation
```go
// @intent INT-324#validate-staged-boundaries
// ValidateStagedBoundaries checks that none of the staged files violate worktree isolation.
func ValidateStagedBoundaries(repoRoot, currentBranch string, stagedFiles []string) error {
    // ...
}
```

---

## 4. In-Line Interface Contracts & Go DTO Structs

The Substrate implements intent binding resolution in `substrate/src/nomos/modules/verify/gates_intent.go`:

```go
// IntentTagBinding encapsulates a parsed @intent annotation.
type IntentTagBinding struct {
	Raw        string `json:"raw"`
	TaskKey    string `json:"task_key,omitempty"`
	Anchor     string `json:"anchor,omitempty"`
	IsCUE      bool   `json:"is_cue"`
	SchemaName string `json:"schema_name,omitempty"`
	TypeName   string `json:"type_name,omitempty"`
}
```

Declarative CUE schema counterpart:

```cue
package schemas

// IntentTagBinding defines the contract for parsed AST traceability annotations.
#IntentTagBinding: close({
	raw:          string & != ""
	task_key?:    =~"^INT-[0-9]+$"
	anchor?:      string
	is_cue:       bool | *false
	schema_name?: string
	type_name?:   string
})
```

---

## 5. Substrate Verification (DoD Gate 25)

The `nomos verify` engine enforces intent traceability via **Gate 25 (`IntentTraceabilityGate`)**:

### 5.1 Gate Rules & Execution Stages
1. **AST Extraction**: `ExtractIntentTagBindings` parses all package and symbol docstrings across the substrate using Tree-sitter AST scanning.
2. **Task Ledger Validation**: For `INT-` tasks, the parsed task key must exist in `nomos_global.db` under the `tasks` table with `plane="intent"`.
3. **Semantic Anchor Verification**: When `#anchor` is present, the parser asserts that the targeted markdown heading exists within the corresponding Intent document.
4. **CUE Contract Resolution**: For `CUE:` bindings, `ValidateCUEContract` verifies that `intent/contracts/cue/<name>.cue` exists and defines `#<TypeName>`.
5. **Fail-Closed Rejection**: If an `@intent` tag references a non-existent task, missing anchor, or unresolvable CUE definition, the gate immediately fails closed (exit code 1).

### 5.2 Scope of Tracing
Traceability tags are strictly required for:
- Root-level package docstrings (`doc.go` or `main.go`).
- Public API surface (exported interfaces, structs, and functions).
- Critical invariant enforcement mechanisms (e.g., hooks, boundary guards).

Traceability tags are NOT required for:
- Unexported utility functions.
- Unit test files (`*_test.go`).
- Auto-generated or embedded files.

---

## 6. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-TRC-001` | `ParseIntentTag` | `"INT-541"` | `IntentTagBinding{Raw: "INT-541", TaskKey: "INT-541"}` | Law 4 & Law 6 (Canonical Singularity) |
| `TC-TRC-002` | `ParseIntentTag` | `"INT-541#physics-hull"` | `IntentTagBinding{Raw: "INT-541#physics-hull", TaskKey: "INT-541", Anchor: "physics-hull"}` | Law 4 (SSoT Semantic Alignment) |
| `TC-TRC-003` | `ParseIntentTag` | `"CUE:physics#PhysicsFrame"` | `IntentTagBinding{Raw: "CUE:physics#PhysicsFrame", IsCUE: true, SchemaName: "physics", TypeName: "PhysicsFrame"}` | Law 4 (SSoT CUE Contract Alignment) |
| `TC-TRC-004` | `IntentTraceabilityGate` | Tag pointing to non-existent `INT-99999` | Gate 25 returns `Passed: false` with descriptive error and line number | Law 2 & Law 7 (Fail-Closed AST Proof) |

---

## 7. Verification Boundaries & DoD Evidence

1. **AST Proof**: All exported symbols adhere to valid, resolving `@intent` annotations.
2. **Markdown Integrity**: All markdown anchors resolve deterministically without broken wires.
3. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 29 Definition of Done quality gates with zero regressions.
