---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-08"
last_updated: "2026-11-01"
lineage:
  - INT-369
  - INT-529
  - INT-842
  - SUB-562
plane: intent
ssot_repository: nomos
status: active
title: "Strongly-Typed Actionable Error Framework Specification"
type: spec
version: "4.0.0"
---

# Strongly-Typed Actionable Error Framework Specification

---

## 1. Executive Summary & Problem Statement

The Nomos Substrate emits structured, actionable error messages whenever architectural invariants, execution boundaries, or Definition of Done (DoD) quality gates reject mutations. Unstructured `fmt.Errorf` strings obscure root causes, hinder programmatic parsing, and violate the Canonical Singularity Law (Law 6).

This specification establishes the Strongly-Typed Actionable Error Framework for the `nomos/core/types` package, anchoring `NomosError` as the compiled Go Single Source of Truth (SSoT).

---

## 2. Axiomatic Invariants & Governing Laws

- **Law 2: The Non-Verbal Proof Law**: Error structures carry machine-parsable codes, contextual triage hashes, and verifiable root-cause inspection commands.
- **Law 4: The Single Source of Truth Law**: `NomosError` in `substrate/src/nomos/core/types/errors.go` defines the authoritative Go SSoT for structured error reporting and JSON serialization.
- **Law 6: The Canonical Singularity Law**: `nomos/core/types.NomosError` governs execution error reporting across all Substrate modules, eliminating ad-hoc formats and legacy wrappers.
- **Law 7: The Fail-Closed Pre-Flight Law**: Error construction requires valid `^ERR_[A-Z0-9_]+$` codes and non-empty descriptions enforced by `NomosError.Validate()` and static AST error hygiene gates.

---

## 3. Compiled Go SSoT Struct Contract

The compiled Go Single Source of Truth resides in `substrate/src/nomos/core/types/errors.go`. All Substrate modules and quality gates MUST emit or wrap failures using `NomosError`:

```go
package types

const (
	ErrCodeCore               = "ERR_CORE"
	ErrCodeIntent             = "ERR_INTENT"
	ErrCodeSubstrate          = "ERR_SUBSTRATE"
	ErrCodeBoundary           = "ERR_BOUNDARY"
	ErrCodeInvariant          = "ERR_INVARIANT"
	ErrCodeCapBAC             = "ERR_CAPBAC"
	ErrCodePhaseViolation     = "ERR_PHASE_VIOLATION"
	ErrCodeValidationFailed   = "ERR_VALIDATION_FAILED"
	ErrCodeCrossPlaneMutation = "ERR_CROSS_PLANE_MUTATION"
)

// NomosError encapsulates structured agent-focused context for execution boundary
// and invariant constraint failures.
type NomosError struct {
	Code              string   `json:"code"`
	Description       string   `json:"description"`
	Remedy            string   `json:"remedy,omitempty"`
	ExplainCommand    string   `json:"explain_command,omitempty"`
	ContextFiles      []string `json:"context_files,omitempty"`
	Fatal             bool     `json:"fatal"`
	TriageHash        string   `json:"triage_hash,omitempty"`
	InteractivePrompt string   `json:"interactive_prompt,omitempty"`
}

// NewNomosError instantiates a basic NomosError with code and formatted description.
func NewNomosError(code, format string, args ...any) *NomosError

// WithRemedy attaches an actionable agent remedy to the error.
func (e *NomosError) WithRemedy(remedy string) *NomosError

// WithExplainCommand attaches a canonical inspection CLI command (#NomosError).
func (e *NomosError) WithExplainCommand(cmd string) *NomosError

// Validate verifies that the NomosError satisfies all structural constraints.
func (e *NomosError) Validate() error
```

### 3.1 Standard Library Error Tree Compatibility
To guarantee full compatibility with standard Go `errors.As` and structured CLI error inspection:
1. `NomosError` implements `Error() string`, returning `[<Code>] <Description>` with optional `(Remedy: ...)` and `(Inspect: ...)` suffixes.
2. Calls to `errors.As(err, &nErr)` extract `*types.NomosError` across top-level CLI dispatchers (`cli.FormatCLIError`).
3. Builder methods (`WithRemedy`, `WithExplainCommand`, `WithContextFiles`, `WithFatal`, `WithTriageHash`, `WithInteractivePrompt`) preserve nil-receiver safety.

---

## 4. Markdown Blockquote & CLI Serialization Contract

Terminal and hook error surfaces format `NomosError` deterministically for direct cognitive ingestion by Tier 1 coding agents (AGY-IDE, Claude Code, OpenCode) in IDE terminals.

### 4.1 Serialization Rules
1. **Header**: Emits the error code and primary description (`[<Code>] <Description>` in `NomosError.Error()`, or `> Error: <Code>` in hook blockquote guidance).
2. **Description**: Presents the contextual cause of the boundary or invariant violation.
3. **Remedy Protocol (Conditional)**: When `Remedy` is non-empty, renders actionable recovery commands (`Remedy : <Remedy>` or `> **Agent Remedy Protocol:**`).
4. **Inspection & Required Reading (Conditional)**: Non-empty `ExplainCommand` or `ContextFiles` fields render exact CLI inspection commands (`Inspect: <ExplainCommand>`) and SSoT specification paths.
5. **Interactive Diagnostic Guidance (Conditional)**: When `TriageHash` (`^ERR-[a-f0-9]+$`) is non-empty, renders structured diagnostic context referencing `nomos directive` or `nomos verify`.

### 4.2 Canonical String Representation Example
```markdown
> ❌ Error: ERR_PHASE_VIOLATION
> Cannot modify substrate files during IDLE phase.
>
> 🤖 **Agent Remedy Protocol:**
> Run `nomos task chore "<title>"` (or `nomos task start <key> && nomos task edit <key>`) to transition to EDIT phase before modifying files.
>
> 📚 **Required Reading:**
> - intent/architecture/specs/task_lifecycle_cli_spec.md
>
> 💡 **Diagnostic Context (`ERR-9b2f4c`):** Run `nomos directive` or `nomos verify` to inspect structured remediation guidance.
```

---

## 5. Structured JSON Wire Representation

Under `--json` output mode, `NomosError` serializes deterministically with closed JSON field keys:

```json
{
  "code": "ERR_PHASE_VIOLATION",
  "description": "Cannot modify substrate files during IDLE phase.",
  "remedy": "Run `nomos task chore \"<title>\"` to enter EDIT phase.",
  "explain_command": "nomos status",
  "context_files": [
    "intent/architecture/specs/task_lifecycle_cli_spec.md"
  ],
  "fatal": false,
  "triage_hash": "ERR-9b2f4c"
}
```

### 5.1 Canonical Error Code Registry
The following error codes constitute the canonical registry of Substrate execution failure categories:

| Error Code | Category | Associated Quality Gate / Invariant |
| :--- | :--- | :--- |
| `ERR_DOD_GATE_FAILED` | Gate Failure | General DoD Quality Gate failure (`nomos verify`) |
| `ERR_PHASE_VIOLATION` | Phase Discipline | Phase Discipline Invariant (`Cell 1.2 (phase_discipline)`) |
| `ERR_BOUNDARY` | Worktree / Package Boundary | Hermetic Worktree & Package Boundary Invariants (`Cell 1.1` / `Cell 3.2`) |
| `ERR_INVARIANT` | Schema / Intent Parity | Intent SSoT Parity Audit (`Cell 4.2` / `Cell 4.3`) |
| `ERR_VALIDATION_FAILED` | Pre-Flight Validation | Input and schema validation failure (Law 4 / Law 7) |
| `ERR_CROSS_PLANE_MUTATION` | Plane Separation | Pure 2-Plane Singularity (`Cell 7.1 (plane_separation)`) |
| `ERR_CAPBAC` | Capability Lease | CapBAC Object Capability lease violation (`Cell 1.3 (capbac_lease)`) |
| `ERR_CORE` | Substrate Internal | Core runtime system error |
| `ERR_SUBSTRATE` | Substrate Module | Module execution or persistence failure |
| `ERR_INTENT` | Intent Plane | Intent specification or directive violation |

---

## 6. Fail-Closed Pre-Flight Gating & AST Enforcement

To enforce structured error hygiene across the Substrate:

1. **Pre-Flight Validation Predicate (`NomosError.Validate`)**:
   ```go
   func (e *NomosError) Validate() error {
       if e == nil {
           return fmt.Errorf("nomos error cannot be nil")
       }
       if !regexNomosErrorCode.MatchString(e.Code) {
           return fmt.Errorf("invalid error code %q: must match regex ^ERR_[A-Z0-9_]+$", e.Code)
       }
       if strings.TrimSpace(e.Description) == "" {
           return fmt.Errorf("error description cannot be empty")
       }
       if e.TriageHash != "" && !regexTriageHash.MatchString(e.TriageHash) {
           return fmt.Errorf("invalid triage hash %q: must match regex ^ERR-[a-f0-9]+$", e.TriageHash)
       }
       return nil
   }
   ```
2. **Static AST Enforcement Gate (Law 7 / `Cell 7.2 (unchecked_errors)`)**:
   The Substrate gate pipeline audits functions across `substrate/` to reject unchecked error discards (`_ = ...`) and enforce fail-closed error propagation with exit code `1`.

---

## 7. Table-Driven Verification Vectors

| Test Vector ID | Input Struct State | Expected Formatting / Validation Behavior | Status |
| :--- | :--- | :--- | :--- |
| `TC-ERR-001` | `Code: "ERR_CORE"`, `Description: "Minimal error"` | `Error()` produces `[ERR_CORE] Minimal error` without optional suffixes; `Validate()` returns `nil`. | PASS |
| `TC-ERR-002` | `Code: "ERR_PHASE_VIOLATION"`, `Remedy`, `ExplainCommand`, `ContextFiles`, `TriageHash: "ERR-1a2b3c"` | `FormatCLIError` and JSON output include code, remedy, inspect, and triage hash. | PASS |
| `TC-ERR-003` | `nErr := NewNomosError("ERR_CORE", "cause").WithRemedy("fix")` | `errors.As(nErr, &target) == true` and nil-safe builder chaining preserves `Remedy`. | PASS |
| `TC-ERR-004` | `Code: ""` or `Code: "bad"`, `Description: "invalid"` | `NomosError.Validate()` fails closed matching `^ERR_[A-Z0-9_]+$`. | PASS |

---

## 8. Cross-Plane Traceability & DoD Verification

- **Intent Lineage**: `INT-369` (Actionable Errors), `INT-529` (Error Remediation Protocol), `INT-842` (Explain Command Affordance).
- **Substrate SSoT**: `SUB-562` (`substrate/src/nomos/core/types/errors.go`, `substrate/cli/theme.go`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 1.1 (complexity)`, `Cell 4.1 (broken_wire)`, `Cell 4.1 (intent_traceability)`, `Cell 4.2 (contract_parity)`, `Cell 7.2 (unchecked_errors)`.
- **Deterministic Verification**: `nomos verify` validates Go SSoT contract parity, AST `@intent #NomosError` traceability, and fail-closed error hygiene across all 42 DoD gates with exit code `0`.
