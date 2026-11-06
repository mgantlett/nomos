---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
lineage:
  - INT-731
  - SUB-1207
plane: intent
ssot_repository: nomos
status: active
title: "Anti-Stub Invariants, Empirical Verification, and Contract Boundaries Specification"
type: spec
version: "4.0.0"
---

# Anti-Stub Invariants, Empirical Verification, and Contract Boundaries Specification

---

## 1. Executive Summary & Problem Statement

During the initial bootstrap of the Gen 4 substrate (`nomos-v4`), an insidious class of architectural defect emerged: **False Positive Verification ("Green-Check Syndrome")**.
This specification prohibits trivial constant-return stubs in DoD gates, mandates dynamic asset discovery in parity audits, and codifies Declarative Contract Boundaries across planes.

1. **The Trivial Constant Return Loophole**:
   The AST anti-shim validator originally checked only whether functions had zero statements (`len(Body.List) == 0`) or contained `panic("todo")`. Consequently, verification gates returning a single literal statement satisfied that naive length predicate as 1 valid statement:
   ```go
   return &GateResult{GateName: g.Name(), Passed: true, Message: "..."}, nil
   ```
   Those stubs passed `nomos verify` cleanly while performing zero actual verification.

2. **The Vacuous Parity Audit**:
   `workspace.CheckBilateralParity(sourceRoot, targetRoot)` invoked `CheckAssetParity(sourceRoot, targetRoot, nil)`. Passing `nil` produced `TotalAssets: 0, Mismatches: 0, Passed: true` and reported false "Zero Drift" while comparing zero assets.

3. **Unimplemented Contract Boundary Gates**:
   Declarative Contract Boundaries (`contracts.contracts`, `contracts.membrane`, `contracts.infra`) lacked substrate gate implementations, creating a contract governance blind spot.

---

## 2. Axiomatic Invariants & Governing Laws

### `INV-STUB-001`: Gate Non-Triviality Invariant (Law 2: Non-Verbal Proof)
Every `verify.Gate` implementation (`Evaluate(*VerifyContext) (*GateResult, error)`) must execute substantive AST, filesystem, configuration, or subprocess analysis before returning `&GateResult{Passed: true}`.
- **Enforcement**: `Cell 2.1 (anti_shim)` (`AntiShimGate`) walks gate ASTs and rejects single-statement `ast.ReturnStmt` functions returning `Passed: true` with `ERR_TRIVIAL_PASS_SHIM`.

### `INV-PARITY-002`: Empirical Asset Discovery Invariant (Law 4: SSoT)
Every bilateral or cross-hemisphere parity audit executed via `workspace.CheckAssetParity` must discover and verify $\ge 1$ canonical assets.
- If `TotalAssets == 0`, the parity audit fails closed with `ERR_ZERO_ASSETS_DISCOVERED`.
- Canonical assets dynamically discovered must include:
  1. The canonical root `./AGENTS.md`.
  2. The Go-reflected configuration JSON Schema (`./nomos.schema.json`).
  3. Ambient lifecycle hook manifests and scripts under `.agents/hooks.json` and `.agents/scripts/`.
  4. Repository configuration manifests (`nomos.yaml`).

### `INV-CONTRACT-003`: Declarative Contract Boundaries Invariant (Law 7)
Declarative Contract Boundaries govern the interfaces between Intent, Substrate Core, Membrane, and Infra. Each boundary defined in `nomos.yaml` binds to a registered substrate gate:
1. `contracts.contracts`: `ContractParityGate` (`Cell 4.2 (contract_parity)`). Audits `nomos.schema.json` byte parity vs `config.EcosystemConfig` and engine grounding.
2. `contracts.infra`: `InfraPortIngressGate` (`Cell 5.2 (infra_ingress)`). Audits source `os.Getenv` usages vs environment declarations (`.env.example`) and port configurations.
3. `contracts.membrane`: `UIAntiMaskSentinelGate` (`Cell 1.1 (ui_anti_mask)`). Audits UI component bindings and eliminates defensive fallback masks.
Setting `enabled: false` on a boundary in `nomos.yaml` causes the bound gate to emit an explicit skip result instead of an unverified pass.

---

## 3. Compiled Go SSoT Contracts & AST Anti-Shim Engine

The substrate compiles anti-shim AST extraction in `substrate/src/nomos/modules/verify/anti_shim.go` and declarative contract boundary configuration in `substrate/src/nomos/core/config/config.go`:

```go
package verify

import "go/ast"

// AntiShimDTO defines the AST shape extracted by the Go substrate (#AntiShimDTO).
type AntiShimDTO struct {
	Name                 string `json:"Name"`
	BodyStatementCount   int    `json:"BodyStatementCount"`
	IsPanicShim          bool   `json:"IsPanicShim"`
	IsTrivialPassShim    bool   `json:"IsTrivialPassShim,omitempty"`
	ForbiddenLiteralShim string `json:"ForbiddenLiteralShim"`
}

// ValidateAntiShim enforces zero-trust AST anti-shim bounds (Law 2).
func ValidateAntiShim(dto AntiShimDTO) error

// ScanFunctionAntiShim extracts anti-shim properties from a Go AST function declaration.
func ScanFunctionAntiShim(fn *ast.FuncDecl) AntiShimDTO
```

Compiled boundary configuration structs in `substrate/src/nomos/core/config/config.go`:

```go
package config

// ContractsConfig defines contract schema and backend verification roots.
type ContractsConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	SpecRoots    []string `yaml:"spec_roots,omitempty" json:"spec_roots,omitempty"`
	SchemaFiles  []string `yaml:"schema_files,omitempty" json:"schema_files,omitempty"`
	BackendRoots []string `yaml:"backend_roots,omitempty" json:"backend_roots,omitempty"`
	StrictTypes  bool     `yaml:"strict_types" json:"strict_types"`
}

// ContractsBoundaryConfig defines cross-plane contract and infrastructure verification boundaries.
type ContractsBoundaryConfig struct {
	Contracts   ContractsConfig `yaml:"contracts" json:"contracts"`
	Membrane    *MembraneConfig `yaml:"membrane,omitempty" json:"membrane,omitempty"`
	Infra       InfraConfig     `yaml:"infra" json:"infra"`
	AutoPromote bool            `yaml:"auto_promote,omitempty" json:"auto_promote,omitempty"`
}
```

---

## 4. Table-Driven Verification Vectors

| Compliance Vector | Invariant | Target Substrate Gate | Expected Failure / Outcome |
| :--- | :--- | :--- | :--- |
| `TC-AGT-07` | Law 2 (`INV-STUB-001`) | `Cell 2.1 (anti_shim)` | Rejects trivial constant return stubs with `ERR_TRIVIAL_PASS_SHIM` |
| `TC-AGT-08` | Law 5 (Zero-Hardcoded-Path) | `Cell 5.1 (forbidden_literals)` | Rejects forbidden literal paths with `ERR_FORBIDDEN_LITERAL` |
| `TC-AGT-09` | Law 4 (`INV-PARITY-002`) | `CheckBilateralParity` (`Cell 4.3`) | Rejects zero-asset parity audits with `ERR_ZERO_ASSETS_DISCOVERED` |
| `TC-AGT-10` | Law 7 (`INV-CONTRACT-003`) | `Cell 4.2 (contract_parity)` | Verifies `nomos.schema.json` reflection and `AuditArchitecturalEngines` grounding |

---

## 5. Cross-Plane Traceability & DoD Verification

| Component | Substrate SSoT Path | Canonical 7x3 Cell |
| :--- | :--- | :--- |
| **AST Anti-Shim Engine** | `substrate/src/nomos/modules/verify/anti_shim.go` | `Cell 2.1 (anti_shim)` |
| **Bilateral Asset Parity** | `substrate/src/nomos/core/workspace/parity.go` | `Cell 4.3 (intent_parity)` |
| **Contract Boundary Gates** | `substrate/src/nomos/modules/verify/gates_contracts_boundary.go` | `Cell 4.2 (contract_parity)`, `Cell 5.2 (infra_ingress)`, `Cell 1.1 (ui_anti_mask)` |
| **Declarative Boundary Config** | `substrate/src/nomos/core/config/config.go` | `Cell 7.1 (config_immutability)` |

Deterministic verification executes via `nomos verify` and passes all 42 Definition of Done quality gates with exit code `0`.
