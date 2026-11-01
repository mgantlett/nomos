---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-01"
plane: intent
status: active
title: Nomos Declarative DoD Quality Gate Pipeline Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos Declarative DoD Quality Gate Pipeline Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Decouple Definition of Done (DoD) quality gate registration and execution sequencing from hardcoded Go source code into pure declarative CUE (`intent/contracts/cue/pipeline.cue`), enabling custom gates and zero-fork downstream adoption.

---

## 1. Problem Statement & Architectural Motivation
In Nomos Gen 3, the 29 Definition of Done (DoD) quality gates enforce the 7 Laws of the Nomos Open-Core Protocol.
While gate *evaluation rules* were successfully migrated to pure Cuelang in ADR-0056 (`gates.cue`), the *registration, sequence, and configuration* of active gates remained hardcoded in Go within `substrate/src/nomos/modules/verify/pipeline.go` (`StandardPipeline()`).

This hardcoded registration presents critical architectural drawbacks:
1. **Adoption Friction (Violation of Law 6)**: Downstream workspaces (such as `nomos-atc`, sovereign titles, or external companies) cannot reorder, toggle, or add custom gates without modifying or recompiling the Go binary.
2. **Substrate Decoupling (The Game Engine Invariant)**: The Go substrate should function strictly as the deterministic *physics and sensor engine* (running Tree-sitter AST, git subcommands, coverage test execution, and sandboxing), while the *rules of the game* (which gates execute and what violations they enforce) must reside in declarative CUE.

---

## 2. Core Axioms & Law Enforcements

| Law # | Law Name | Declarative Pipeline Enforcement Mechanism |
| :---: | :--- | :--- |
| **Law 2** | **The Non-Verbal Proof Law** | Every gate definition unifies against `#GateSpec` and must evaluate with exit code 0 on verification. |
| **Law 4** | **The Single Source of Truth Law** | Exactly one compiled CUE schema (`intent/contracts/cue/pipeline.cue`) defines the active quality gate sequence. |
| **Law 6** | **The Canonical Singularity Law** | Zero aliases or duplicate registration channels; all gates execute through the declarative loader. |
| **Law 7** | **The Fail-Closed Pre-Flight Law** | If a custom gate definition is invalid or missing its sensor, verification fails immediately with a descriptive error. |

---

## 3. Declarative Pipeline Schema Architecture & Interface Contracts

The pipeline schema is formalized in `intent/contracts/cue/pipeline.cue`:

```cue
package schemas

// SensorType defines the available telemetry drivers in the Go substrate.
#SensorType: "git" | "ast" | "test_coverage" | "secret" | "links" | "traceability" | "custom"

// GateSpec defines the declarative schema for an individual DoD quality gate.
#GateSpec: close({
	id:       string & != "" & =~"^[a-z0-9_-]+$"
	name:     string & != ""
	sensor:   #SensorType
	phase:    "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN" | *"REVIEW"
	enabled:  bool | *true
	remedy:   string & != ""
	rule_key: string | *""
})

// Pipeline defines the ordered collection of quality gates.
#Pipeline: close({
	version: string & =~"^\\d+\\.\\d+\\.\\d+$" | *"1.0.0"
	gates:   [#GateSpec, ...#GateSpec]
})
```

### 3.1 Substrate In-Line Interface Contracts & Go DTO Structs

The Go substrate unmarshals declarative pipeline definitions via DTO structs defined in `substrate/src/nomos/modules/verify/pipeline_loader.go`:

```go
// GateSpec defines the declarative schema for an individual quality gate.
type GateSpec struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Sensor  string `json:"sensor"`
	Phase   string `json:"phase"`
	Enabled bool   `json:"enabled"`
	Remedy  string `json:"remedy"`
	RuleKey string `json:"rule_key"`
}

// PipelineDeclaration models the compiled pipeline schema from CUE.
type PipelineDeclaration struct {
	Version string     `json:"version"`
	Gates   []GateSpec `json:"gates"`
}
```

---

## 4. Substrate Dynamic Pipeline Loader

The Go substrate compiles and dispatches gates dynamically:
1. **`pipeline_loader.go`**:
   - Queries `CUEEngine` to unmarshal `intent/contracts/cue/pipeline.cue`.
   - Maps each `#GateSpec.sensor` and `#GateSpec.id` to its corresponding Go sensor executor in the internal `gateRegistry`.
2. **Custom Gate Capability**:
   - Workspaces can add custom gates by appending a `#GateSpec` entry to `pipeline.cue` and defining the corresponding violation list in `gates.cue`.
3. **Fail-Closed Fallback**:
   - In unconfigured or transient environments where `pipeline.cue` is absent, the substrate defaults to the embedded 42-gate baseline sequence.

---

## 5. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-PIP-001` | `LoadPipelineFromCUE` | `intent/contracts/cue/pipeline.cue` | Successfully returns 42 enabled `Gate` instances matching canonical sequence | Law 4 (Single Source of Truth) |
| `TC-PIP-002` | `LoadPipelineFromCUE` | `#GateSpec{id: "custom_gate", sensor: "unregistered_sensor", remedy: "fix"}` | Rejection with `ErrCodeModules`: "unrecognized sensor unregistered_sensor" | Law 7 (Fail-Closed Pre-Flight) |
| `TC-PIP-003` | `StandardPipeline` fallback | Non-existent path `/tmp/missing_pipeline.cue` | Safely falls back to `DefaultBaselineGates()` containing all 42 built-in gates | Law 1 (Substrate Autonomy & Liveness) |

---

## 6. Verification Boundaries & DoD Evidence

1. **CUE AST Evaluation**: `pipeline.cue` compiles cleanly under CUE AST evaluation with zero incomplete instances.
2. **Canonical Sequence Execution**: All 42 DoD gates execute in their exact canonical sequence.
3. **Downstream Customization**: Workspaces can disable or add gates purely through declarative CUE with zero Go recompilation.
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
