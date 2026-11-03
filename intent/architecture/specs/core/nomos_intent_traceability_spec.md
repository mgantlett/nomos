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

This specification defines the exact code-level tagging syntax (`// @intent <TARGET>`) required to map Go substrate components to their originating Intent specifications, semantic heading anchors, or compiled Go SSoT type anchors.

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

3. **Compiled Go Type & Specification Anchor**:
   ```go
   // @intent #PhysicsFrame (software_physics_engine_spec.md)
   // @intent #PipelineDeclaration
   ```
   Binds the Go symbol directly to a compiled Go `type <TypeName>` AST declaration across `substrate/` and/or an Intent Markdown specification (`<name>_spec.md`) in `intent/`.

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

The Substrate implements intent binding resolution in `substrate/src/nomos/modules/verify/gates_intent.go` and `intent_schema_helpers.go`:

```go
// IntentTagBinding encapsulates a parsed @intent annotation.
type IntentTagBinding struct {
	Raw      string `json:"raw"`
	TaskKey  string `json:"task_key,omitempty"`
	Anchor   string `json:"anchor,omitempty"`
	SpecFile string `json:"spec_file,omitempty"`
	TypeName string `json:"type_name,omitempty"`
}
```

---

## 5. Substrate Verification (DoD Cell 4.1)

The `nomos verify` engine enforces intent traceability via **`IntentTraceabilityGate` (`Cell 4.1`)**:

### 5.1 Gate Rules & Execution Stages
1. **AST Extraction**: `verifyIntentASTAnchors` parses package and exported function docstrings across the substrate using `go/parser` AST scanning.
2. **Density Enforcement**: Modified Go source files with exported functions must maintain `@intent` docstring coverage $\ge 50\%$.
3. **Go Type Anchor Resolution**: Every `#<TypeName>` token on an `@intent` line must resolve to a real `type <TypeName>` AST declaration across `substrate/`.
4. **Specification File Resolution**: Every `<name>_spec.md` token on an `@intent` line must resolve to an existing Markdown specification under `intent/`.
5. **Fail-Closed Rejection**: If an `@intent` tag references an unresolvable `#<TypeName>` or non-existent `<name>_spec.md`, the gate immediately fails closed (exit code 1).

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
| `TC-TRC-001` | `IntentTraceabilityGate` | `"// @intent INT-541"` | Accepted with valid docstring density | Law 4 & Law 6 (Canonical Singularity) |
| `TC-TRC-002` | `verifyIntentASTAnchors` | `"// @intent #HandshakeEvent (jit_handshake_and_session_journal_spec.md)"` | Resolves `type HandshakeEvent` in Go AST and spec file in `intent/` | Law 4 (SSoT AST & Spec Alignment) |
| `TC-TRC-003` | `verifyIntentASTAnchors` | `"// @intent #NonExistentPhantomType"` | `IntentTraceabilityGate` returns `Passed: false` identifying unresolved `#NonExistentPhantomType` | Law 2 & Law 7 (Fail-Closed AST Proof) |
| `TC-TRC-004` | `verifyIntentASTAnchors` | `"// @intent missing_ghost_spec.md"` | `IntentTraceabilityGate` returns `Passed: false` identifying missing spec | Law 4 & Law 7 (Fail-Closed Spec Proof) |

---

## 7. Verification Boundaries & DoD Evidence

1. **AST Proof**: All exported symbols adhere to valid, resolving `@intent` annotations.
2. **Markdown Integrity**: All markdown anchors resolve deterministically without broken wires.
3. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
