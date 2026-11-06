---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-08"
last_updated: "2026-11-05"
plane: intent
ssot_repository: nomos
status: active
title: "Strongly-Typed Actionable Error Framework Specification"
type: spec
version: "4.0.0"
---

# Strongly-Typed Actionable Error Framework Specification

## 1. Executive Summary
The Nomos Substrate emits structured, actionable error messages whenever architectural invariants, execution boundaries, or Definition of Done (DoD) quality gates reject mutations. Unstructured `fmt.Errorf` strings obscure root causes, hinder programmatic parsing, and violate the Canonical Singularity Law (Law 6). This specification establishes the Strongly-Typed Actionable Error Framework for the `nomos/core/types` package, anchoring `NomosError` as the compiled Go Single Source of Truth (SSoT).

## 2. Axiomatic Invariants & Laws Codified
- **Law 2: The Non-Verbal Proof Law**: Error structures carry machine-parsable codes, contextual triage hashes, and verifiable root-cause unwrapping.
- **Law 4: The Single Source of Truth Law**: `NomosError` in `substrate/src/nomos/core/types/errors.go` defines the authoritative Go SSoT for structured error reporting and JSON serialization.
- **Law 6: The Canonical Singularity Law**: `nomos/core/types.NomosError` governs execution error reporting across all Substrate modules, eliminating ad-hoc formats and legacy wrappers.
- **Law 7: The Fail-Closed Pre-Flight Law**: Error construction requires non-empty codes and descriptions. Static AST analysis rejects naked `fmt.Errorf` or `errors.New` calls in verification gate logic.

## 3. Compiled Go SSoT Struct Contract
The compiled Go Single Source of Truth resides in `substrate/src/nomos/core/types/errors.go`. All Substrate modules and quality gates MUST emit or wrap failures using `NomosError`.

```go
package types

import (
	stderrors "errors"
	"fmt"
	"strings"
)

const (
	ErrCodeCore    = "ERR_CORE"
	ErrCodeModules = "ERR_MODULES"
)

// NomosError encapsulates agent-focused context for execution boundary constraint failures.
// @intent INT-369
// @intent SUB-562
type NomosError struct {
	Code              string   `json:"code"`               // Deterministic categorization (e.g., ERR_PHASE_VIOLATION)
	Description       string   `json:"description"`        // Contextual cause of the failure
	Remedy            string   `json:"remedy,omitempty"`   // Markdown-formatted instructions for agent recovery
	ContextFiles      []string `json:"context_files"`      // Absolute paths to the Intent SSoT the agent must read
	Fatal             bool     `json:"fatal"`              // If true, the agent must halt and request human intervention
	TriageHash        string   `json:"triage_hash,omitempty"` // Unique identifier for spawning a triage worktree (ERR-[a-f0-9]+)
	InteractivePrompt string   `json:"interactive_prompt,omitempty"` // System prompt injected into agent during triage
	Wrapped           error    `json:"-"`                  // Underlying error, if wrapped via %w
}

// NewNomosError creates a NomosError using a format string for the description.
func NewNomosError(code, format string, args ...any) *NomosError {
	err := fmt.Errorf(format, args...)
	return &NomosError{
		Code:        code,
		Description: err.Error(),
		Wrapped:     stderrors.Unwrap(err),
	}
}

// Unwrap returns the underlying wrapped error to support standard Go 1.13+ errors.Is and errors.As.
func (e *NomosError) Unwrap() error {
	return e.Wrapped
}
```

### 3.1 Standard Library Error Tree Compatibility
To guarantee full compatibility with the standard Go 1.13+ error handling ecosystem:
1. `NomosError` implements `Unwrap() error`, returning `e.Wrapped`.
2. Calls to `errors.Is(err, target)` evaluate transitively through `Unwrap()`.
3. Calls to `errors.As(err, &target)` evaluate transitively through `Unwrap()`.
4. Wrapping an existing error with `NewNomosError(code, "wrapped: %w", rootErr)` automatically extracts `rootErr` into `e.Wrapped`.

## 4. Markdown Blockquote Serialization Contract
The `NomosError.Error()` method produces deterministic, Markdown blockquote-formatted output optimized for direct cognitive ingestion by Tier 1 coding agents (AGY-IDE, Claude Code, OpenCode) in IDE terminals.

### 4.1 Serialization Rules
1. **Header**: The first line is strictly `>  Error: <Code>`.
2. **Description**: Prefixes each non-empty line of `Description` with `> `, followed by a blank `>` blockquote line.
3. **Remedy Protocol (Conditional)**: If `Remedy` is non-empty, emit `>  **Agent Remedy Protocol:**`, followed by each non-empty line of `Remedy` prefixed with `> `, followed by `>`.
4. **Required Reading (Conditional)**: If `ContextFiles` contains one or more items, emit `>  **Required Reading:**`, followed by each file path formatted as `> - <Path>`.
5. **Interactive Diagnostic Guidance (Conditional)**: If `TriageHash` is non-empty, emit a blank blockquote line `>` followed by:
   `>  **Diagnostic Context (`<TriageHash>`):** Run \`nomos directive\` or \`nomos verify\` to inspect structured remediation guidance.`
6. **Whitespace Normalization**: Passes the resulting string through `strings.TrimSpace` prior to return.

### 4.2 Canonical String Representation Example
```markdown
> ❌ Error: ERR_PHASE_VIOLATION
> Cannot modify substrate files during IDLE phase.
>
> 🤖 **Agent Remedy Protocol:**
> Run `nomos task chore "<title>"` (or `nomos task start <key> && nomos task edit <key>`) to transition to EDIT phase before modifying files.
>
> 📚 **Required Reading:**
> - intent/architecture/specs/core/task_lifecycle_cli_spec.md
>
> 💡 **Diagnostic Context (`ERR-9b2f4c`):** Run `nomos directive` or `nomos verify` to inspect structured remediation guidance.
```

## 5. Structured JSON Wire Representation
Under `--json` output mode, `NomosError` serializes deterministically with closed JSON field keys:

```json
{
  "code": "ERR_PHASE_VIOLATION",
  "description": "Cannot modify substrate files during IDLE phase.",
  "remedy": "Run `nomos task chore \"<title>\"` to enter EDIT phase.",
  "context_files": [
    "intent/architecture/specs/core/task_lifecycle_cli_spec.md"
  ],
  "fatal": false,
  "triage_hash": "ERR-9b2f4c"
}
```

### 5.1 Canonical Error Code Registry
The following error codes constitute the canonical, closed registry of Substrate execution failure categories:

| Error Code | Category | Associated Quality Gate / Invariant |
| :--- | :--- | :--- |
| `ERR_DOD_GATE_FAILED` | Gate Failure | General DoD Quality Gate failure (`nomos verify`) |
| `ERR_PHASE_VIOLATION` | Phase Discipline | Phase Discipline Invariant (`core/types/phase.go`) |
| `ERR_HERMETIC_BREACH` | Worktree Isolation | Hermetic Worktree Invariant (Law 3) |
| `ERR_SSOT_DRIFT` | Schema / Intent Parity | Intent SSoT Parity Audit (Law 4) |
| `ERR_PATH_HARDCODED` | Static Topology | Forbidden Literals Invariant (Law 5) |
| `ERR_CANONICAL_SINGULARITY` | Singularity | Canonical Singularity Invariant (Law 6) |
| `ERR_PREFLIGHT_FAILED` | Fail-Closed | Fail-Closed Pre-Flight Invariant (Law 7) |
| `ERR_COGNITIVE_DEBT` | Code Quality | Cognitive Complexity & Cognitive Debt Invariants |
| `ERR_COVERAGE_DEFICIT` | Verification | Statement Coverage & TDD Test Pair Invariants |
| `ERR_CORE` | Substrate Internal | Core runtime system error |
| `ERR_MODULES` | Substrate Extension | Module execution failure |

## 6. Fail-Closed Pre-Flight Gating & AST Enforcement
To prevent the recurrence of naked `fmt.Errorf` usage across the Substrate:

1. **Pre-Flight Validation Predicate**:
   ```go
   func ValidateNomosError(err *NomosError) error {
       if err == nil {
           return fmt.Errorf("NomosError cannot be nil")
       }
       if strings.TrimSpace(err.Code) == "" {
           return fmt.Errorf("NomosError.Code must not be empty")
       }
       if strings.TrimSpace(err.Description) == "" {
           return fmt.Errorf("NomosError.Description must not be empty")
       }
       return nil
   }
   ```
2. **Static AST Enforcement Gate (Law 2 / Gate 28)**:
   The Substrate gate pipeline audits exported functions in `substrate/src/nomos/modules/verify/` and `substrate/src/nomos/core/`. Returning unencapsulated `fmt.Errorf` or `errors.New` calls outside `NewNomosError` or `NomosError` aborts verification with exit code 1.

## 7. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Input Struct State | Expected Formatting / Unwrapping Behavior | Status |
| :--- | :--- | :--- | :--- |
| `TC-ERR-001` | `Code: "ERR_TEST_MIN"`, `Description: "Minimal error"` | Produces: `>  Error: ERR_TEST_MIN\n> Minimal error\n>`. Omits remedy, context, and triage blocks. | PASS |
| `TC-ERR-002` | `Code: "ERR_PHASE"`, multiline `Description`, multiline `Remedy`, 2 `ContextFiles`, `TriageHash: "ERR-1a2b3c"` | Produces complete blockquote output with remedy header, file list, and triage prompt. | PASS |
| `TC-ERR-003` | `root := errors.New("underlying")`, `nErr := NewNomosError("ERR_CORE", "wrapped: %w", root)` | `errors.Is(nErr, root) == true`, `errors.Unwrap(nErr) == root`. | PASS |
| `TC-ERR-004` | `Code: ""`, `Description: "invalid"` | `ValidateNomosError` fails closed returning error matching `"Code must not be empty"`. | PASS |

## 8. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-369` (Actionable Errors), `INT-529` (Error Remediation Protocol)
- **Substrate Plane**: `SUB-562` (`substrate/src/nomos/core/types/errors.go`)
- **DoD Gates**: Gate 14 (Broken Wire), Gate 16 (Cognitive Complexity), Gate 25 (Traceability), Gate 28 (Unchecked Error Hygiene)

## 9. Rigor & Verification Boundary
Continuous verification enforces `nomos_actionable_error_framework_spec.md` and related contracts through:
1. `nomos verify` evaluating Go SSoT contract schema parity and AST traceability.
2. `nomos verify` across all automated DoD quality gates.
