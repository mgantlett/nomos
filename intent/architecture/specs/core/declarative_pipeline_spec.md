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
**Objective:** Decouple Definition of Done (DoD) quality gate registration and execution sequencing from ad-hoc switch blocks into a structured declarative `PipelineDeclaration` loader (`substrate/src/nomos/modules/verify/pipeline_loader.go`), enabling JSON/YAML pipeline manifests and zero-fork downstream adoption.

---

## 1. Problem Statement & Architectural Motivation
In early Nomos builds, Definition of Done (DoD) quality gate registration, sequencing, and configuration were tightly coupled to imperative constructor chains.

Tightly coupled registration presents two architectural drawbacks:
1. **Adoption Friction (Violation of Law 6)**: Downstream workspaces cannot inspect, toggle, or compose sensor pipelines declaratively without modifying Go constructors.
2. **Substrate Decoupling (The Game Engine Invariant)**: The Go substrate functions strictly as the deterministic *physics and sensor engine* (running Tree-sitter AST, git subcommands, coverage test execution, and sandboxing), while pipeline declarations bind sensor IDs to canonical 7x3 Law and Ring coordinates via `GateSpec` and `PipelineDeclaration`.

---

## 2. Core Axioms & Law Enforcements

| Law # | Law Name | Declarative Pipeline Enforcement Mechanism |
| :---: | :--- | :--- |
| **Law 2** | **The Non-Verbal Proof Law** | Every gate definition validates against `GateSpec` and evaluates with exit code 0 on verification. |
| **Law 4** | **The Single Source of Truth Law** | Compiled `CanonicalGateOrder` (`matrix.go`) and `PipelineDeclaration` (`pipeline_loader.go`) define the active 42-gate sequence. |
| **Law 6** | **The Canonical Singularity Law** | Zero aliases or duplicate registration channels; all gates execute through the declarative loader. |
| **Law 7** | **The Fail-Closed Pre-Flight Law** | If a custom gate definition is invalid or missing its sensor, verification fails immediately with a descriptive error. |

---

## 3. Declarative Pipeline Schema Architecture & Interface Contracts

Declarative pipeline manifests serialize cleanly as JSON or YAML matching `PipelineDeclaration`:

```json
{
  "version": "1.0.0",
  "gates": [
    {
      "id": "hermetic_worktree",
      "name": "Hermetic Worktree Invariant",
      "sensor": "git",
      "phase": "REVIEW",
      "enabled": true,
      "remedy": "Run `nomos task chore \"<title>\"` to enter an isolated worktree.",
      "rule_key": ""
    }
  ]
}
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

// PipelineDeclaration models the declarative pipeline manifest.
type PipelineDeclaration struct {
	Version string     `json:"version"`
	Gates   []GateSpec `json:"gates"`
}
```

---

## 4. Substrate Dynamic Pipeline Loader

The Go substrate compiles and dispatches gates deterministically:
1. **`pipeline_loader.go`**:
   - Validates `PipelineDeclaration` (`ValidatePipelineDeclaration`) and maps each `GateSpec.Sensor` and `GateSpec.ID` to its corresponding Go sensor executor in the internal `gateRegistry`.
2. **Custom Gate Composition**:
   - Workspaces and test harnesses can compose `PipelineDeclaration` manifests binding registered sensors without recompiling core executors.
3. **Fail-Closed Canonical Baseline**:
   - By default, `StandardPipeline` loads `DefaultBaselineGates()` bound 1:1 to `CanonicalGateOrder` (all 42 statutory probes).

---

## 5. Table-Driven Test Matrix & Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-PIP-001` | `BuildPipelineFromDeclaration` | Canonical 42-gate `PipelineDeclaration` | Successfully returns 42 enabled `Gate` instances matching canonical sequence | Law 4 (Single Source of Truth) |
| `TC-PIP-002` | `BuildPipelineFromDeclaration` | `GateSpec{ID: "custom_gate", Sensor: "unregistered_sensor", Remedy: "fix"}` | Rejection with `ErrCodeModules`: "unrecognized sensor unregistered_sensor" | Law 7 (Fail-Closed Pre-Flight) |
| `TC-PIP-003` | `StandardPipeline` baseline | Unconfigured workspace | Loads `DefaultBaselineGates()` containing all 42 built-in gates | Law 1 (Substrate Autonomy & Liveness) |

---

## 6. Verification Boundaries & DoD Evidence

1. **Manifest Validation**: `ValidatePipelineDeclaration` rejects empty IDs, unknown sensors, or missing remedies fail-closed.
2. **Canonical Sequence Execution**: All 42 DoD gates execute in their exact canonical sequence.
3. **Downstream Customization**: Workspaces configure gate thresholds and exclusions declaratively via `nomos.yaml` (`nomos.schema.json`).
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
