---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-01"
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Declarative DoD Quality Gate Pipeline Specification"
type: spec
version: "4.0.0"
---

# Nomos Declarative DoD Quality Gate Pipeline Specification

---

## 1. Executive Summary & Problem Statement

Early Nomos builds coupled Definition of Done (DoD) quality gate registration, sequencing, and configuration to imperative constructor chains.
This specification decouples DoD gate registration into a declarative `PipelineDeclaration` loader (`substrate/src/nomos/modules/verify/pipeline_loader.go`) supporting JSON and YAML manifests with zero-fork adoption.

Tightly coupled registration presents two architectural drawbacks:
1. **Adoption Friction (Violation of Law 6)**: Downstream workspaces cannot examine, toggle, or compose sensor pipelines declaratively without modifying Go constructors.
2. **Substrate Decoupling (The Game Engine Invariant)**: The Go substrate operates strictly as the deterministic *physics and sensor engine* for AST, Git, coverage, and worktree containment. Declarative manifests bind sensor IDs to canonical 7x3 Law and Ring coordinates via `GateSpec` and `PipelineDeclaration`.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Declarative Pipeline Enforcement Mechanism |
| :---: | :--- | :--- |
| **Law 2** | **The Non-Verbal Proof Law** | Every gate definition matches `GateSpec` and exits `0` on verification. |
| **Law 4** | **The Single Source of Truth Law** | Compiled `CanonicalGateOrder` (`matrix.go`) and `PipelineDeclaration` (`pipeline_loader.go`) define all 42 gates. |
| **Law 6** | **The Canonical Singularity Law** | All gates register singularly through the declarative pipeline loader. |
| **Law 7** | **The Fail-Closed Pre-Flight Law** | Invalid gate definitions or unknown sensors trigger immediate verification rejection. |

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
package verify

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
   - Workspaces and verification harnesses compose `PipelineDeclaration` manifests binding registered sensors without recompiling core executors.
3. **Fail-Closed Canonical Baseline**:
   - By default, `StandardPipeline` loads `DefaultBaselineGates()` bound 1:1 to `CanonicalGateOrder` (all 42 statutory probes across the 21 cells in `DefaultBaselineCells()`).

---

## 5. Table-Driven Verification Vectors

| Test Case ID | Target Component | Input Vector | Expected State / Output | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-PIP-001` | `BuildPipelineFromDeclaration` | Canonical 42-gate `PipelineDeclaration` | Successfully returns 42 enabled `Gate` instances matching canonical sequence | Law 4 (Single Source of Truth) |
| `TC-PIP-002` | `BuildPipelineFromDeclaration` | `GateSpec{ID: "custom_gate", Sensor: "unregistered_sensor", Remedy: "fix"}` | Rejection with `ErrCodeModules`: `"unrecognized sensor unregistered_sensor"` | Law 7 (Fail-Closed Pre-Flight) |
| `TC-PIP-003` | `StandardPipeline` baseline | Unconfigured workspace | Loads `DefaultBaselineGates()` containing all 42 built-in gates | Law 1 (Substrate Autonomy & Liveness) |

---

## 6. Cross-Plane Traceability & DoD Verification

1. **Manifest Loader (`substrate/src/nomos/modules/verify/pipeline_loader.go`)**: `ValidatePipelineDeclaration` rejects empty IDs, unknown sensors, or missing remedies fail-closed.
2. **7x3 Canonical Matrix (`substrate/src/nomos/modules/verify/matrix.go`)**: Maps all 42 probes into 21 `InvariantCellSpec` definitions verified under `Cell 2.3 (gate_integrity_proof)`.
3. **Pipeline Executor (`substrate/src/nomos/modules/verify/pipeline.go`)**: Dispatches registered gates deterministically and enforces `Cell 4.2 (contract_parity)` and `Cell 7.1 (config_immutability)` against `nomos.yaml` (`nomos.schema.json`).
4. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
