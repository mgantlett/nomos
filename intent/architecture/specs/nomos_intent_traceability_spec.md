---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-05"
last_updated: "2026-11-01"
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Intent Traceability Specification"
type: spec
version: "4.0.0"
---

# Nomos Intent Traceability Specification

---

## 1. Executive Summary & Problem Statement

The Nomos Open-Core protocol enforces Law 4 (Single Source of Truth) by separating intent specifications from substrate Go code. Without structural linkage between the Substrate AST and the Intent plane, zero deterministic proof links Go code to Intent tasks.

This specification defines the `// @intent <TARGET>` annotation syntax mapping Go substrate components to Intent specifications, heading anchors, or Go SSoT types.

---

## 2. Axiomatic Invariants & Governing Laws

- **Law 2: The Non-Verbal Proof Law**: Substrate code must compile and pass AST proofs. The Go compiler and AST parsers verify code tracing deterministically without human review.
- **Law 4: The Single Source of Truth Law**: Intent specifications reside solely in `intent/`, while Go implementations carry explicit pointers back to that SSoT.
- **Law 6: The Canonical Singularity Law**: All Intent task keys adhere strictly to `INT-[0-9]+` with zero aliases or non-canonical prefixes.
- **Law 7: The Fail-Closed Pre-Flight Law**: Unresolvable `#<TypeName>` AST anchors or missing `<name>_spec.md` specifications abort verification immediately.

---

## 3. The `@intent` Traceability Syntax

Significant packages, core structs, interfaces, and exported functions within the Substrate MUST include an `@intent` annotation in their Go docstrings.

### 3.1 Annotation Formats

The parser (`ParseIntentTag` / `verifyIntentASTAnchors`) supports three canonical binding forms:

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
   Binds the Go symbol directly to a compiled Go `type <TypeName>` AST declaration across `substrate/` or an Intent specification (`<name>_spec.md`) in `intent/`.

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

The Substrate implements intent binding resolution in `substrate/src/nomos/modules/verify/gates_intent.go` and `substrate/src/nomos/modules/verify/intent_schema_helpers.go`:

```go
package verify

// IntentTraceabilityGate verifies Go code links back to Intent specifications (Cell 4.1, Law 4).
type IntentTraceabilityGate struct{}

// Evaluate verifies exported functions possess valid @intent traceability tags.
func (g *IntentTraceabilityGate) Evaluate(ctx *VerifyContext) (*GateResult, error)

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

The `nomos verify` engine enforces intent traceability via **`IntentTraceabilityGate` (`Cell 4.1`, `intent_traceability`)**:

### 5.1 Gate Rules & Execution Stages
1. **AST Extraction**: `verifyIntentASTAnchors` parses package and exported function docstrings across the substrate using `go/parser` AST scanning.
2. **Density Enforcement**: Modified Go source files with exported functions must maintain `@intent` docstring coverage $\ge 50\%$.
3. **Go Type Anchor Resolution**: Every `#<TypeName>` token on an `@intent` line must resolve to a real `type <TypeName>` AST declaration across `substrate/`.
4. **Specification File Resolution**: Every `<name>_spec.md` token on an `@intent` line must resolve to an existing Markdown specification under `intent/`.
5. **Fail-Closed Rejection**: If an `@intent` annotation references an unresolvable `#<TypeName>` or missing `<name>_spec.md`, the gate rejects execution (exit code 1).

### 5.2 Scope of Tracing
`IntentTraceabilityGate` strictly requires traceability annotations for:
- Root-level package docstrings (`doc.go` or `main.go`).
- Public API surface (exported interfaces, structs, and functions).
- Critical invariant enforcement mechanisms (e.g., hooks, boundary guards).

`IntentTraceabilityGate` exempts the following symbols from traceability annotations:
- Unexported utility functions.
- Go verification suite files (`*_test.go`).
- Auto-generated or embedded files.

---

## 6. Table-Driven Verification Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-TRC-001` | `IntentTraceabilityGate` | `"// @intent INT-541"` | Accepted with valid docstring density | Law 4 & Law 6 (Canonical Singularity) |
| `TC-TRC-002` | `verifyIntentASTAnchors` | `"// @intent #HandshakeEvent (jit_handshake_and_session_journal_spec.md)"` | Resolves `type HandshakeEvent` in Go AST and spec file in `intent/` | Law 4 (SSoT AST & Spec Alignment) |
| `TC-TRC-003` | `verifyIntentASTAnchors` | `"// @intent #NonExistentPhantomType"` | `IntentTraceabilityGate` returns `Passed: false` identifying unresolved `#NonExistentPhantomType` | Law 2 & Law 7 (Fail-Closed AST Proof) |
| `TC-TRC-004` | `verifyIntentASTAnchors` | `"// @intent missing_ghost_spec.md"` | `IntentTraceabilityGate` returns `Passed: false` identifying missing spec | Law 4 & Law 7 (Fail-Closed Spec Proof) |

---

## 7. Cross-Plane Traceability & DoD Verification

1. **Compiled Go SSoT Paths:**
   - `substrate/src/nomos/modules/verify/gates_intent.go` (`IntentTraceabilityGate`, `countTraceabilityTags`).
   - `substrate/src/nomos/modules/verify/intent_schema_helpers.go` (`verifyIntentASTAnchors`, `collectDefinedGoTypeAnchors`, `collectIntentSpecBasenames`).
2. **7x3 Invariant Matrix Coordinates:**
   - `Cell 4.1 (intent_traceability)` enforces exported symbol `@intent` AST and specification anchor resolution.
   - `Cell 4.1 (broken_wire)` and `Cell 4.1 (intent_steelman)` enforce Markdown relative link integrity and specification structure.
3. **Verification Command:**
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
