---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - INT-731
    - SUB-1207
title: Anti-Stub Invariants, Empirical Verification, and Contract Boundaries Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Anti-Stub Invariants, Empirical Verification, and Contract Boundaries Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify machine-enforced invariants prohibiting trivial constant-return stubs in Definition of Done (DoD) quality gates, mandating dynamic asset discovery in bilateral parity audits, and formalizing Contract Boundaries across hemispheres and substrate domains.

---

## 1. Context & Problem Statement

During the initial bootstrap of the Gen 4 substrate (`nomos-v4`), an insidious class of architectural defect emerged: **False Positive Verification ("Green-Check Syndrome")**.

1. **The Trivial Constant Return Loophole**:
   The AST anti-shim validator originally checked only whether functions had zero statements (`len(Body.List) == 0`) or contained `panic("todo")`. Consequently, verification gates returning a single literal statement:
   ```go
   return &GateResult{GateName: g.Name(), Passed: true, Message: "..."}, nil
   ```
   were evaluated as having 1 valid statement. These stubs passed `nomos verify` cleanly while performing zero actual checks.

2. **The Vacuous Parity Audit**:
   `workspace.CheckBilateralParity(sourceRoot, targetRoot)` invoked `CheckAssetParity(sourceRoot, targetRoot, nil)`. Passing `nil` resulted in `TotalAssets: 0, Mismatches: 0, Passed: true`, yielding false confidence of "Zero Drift" when no assets were actually compared.

3. **Unimplemented Contract Boundary Gates**:
   ADR-0066 declared Contract Boundaries (`contracts.contracts`, `contracts.membrane`, `contracts.infra`), but substrate gate implementations were missing or unported, creating a contract governance blind spot.

---

## 2. Invariant Contracts

### `INV-STUB-001`: Gate Non-Triviality Invariant (Law 2: Non-Verbal Proof)
Any function implementing the `verify.Gate` interface (`Evaluate(*VerifyContext) (*GateResult, error)`) is strictly prohibited from returning a static literal `&GateResult{Passed: true}` or `nil` error without executing substantive inspection logic (e.g. AST traversal, filesystem walk, configuration inspection, or subprocess execution).
- **Enforcement**: `AntiShimGate` inspects gate ASTs. Functions whose body consists solely of a single `ast.ReturnStmt` yielding `Passed: true` are rejected with `ERR_TRIVIAL_PASS_SHIM`.

### `INV-PARITY-002`: Empirical Asset Discovery Invariant (Law 4: SSoT)
Any bilateral or cross-hemisphere parity check executed via `workspace.CheckAssetParity` must discover and audit $\ge 1$ canonical assets.
- If `TotalAssets == 0`, the parity audit fails closed with `ERR_ZERO_ASSETS_DISCOVERED`.
- Canonical assets dynamically discovered must include:
  1. The canonical root `./AGENTS.md`.
  2. All CUE contracts under `intent/contracts/cue/*.cue`.
  3. Ambient lifecycle hook manifests and scripts under `.agents/hooks.json` and `.agents/scripts/`.
  4. Repository configuration manifests (`nomos.yaml`).

### `INV-CONTRACT-003`: Contract Boundaries Invariant (ADR-0066 & Law 7)
Contract Boundaries govern the interfaces between Intent, Substrate Core, Membrane, and Infra. Each boundary defined in `nomos.yaml` binds to a registered substrate gate:
1. `contracts.contracts`: `ContractSchemaGate` (Cell 4_2). Audits CUE schemas vs Go DTOs.
2. `contracts.infra`: `InfraPortIngressGate` (Cell 5_2). Audits source `os.Getenv` usages vs environment declarations (`.env.example`) and port configurations.
3. `contracts.membrane`: `UIAntiMaskSentinelGate` (Cell 7_1). Audits UI component bindings and eliminates defensive fallback masks.
When a boundary is declared `enabled: false` in `nomos.yaml` (e.g. open-core boundary omitting UI membrane), the corresponding gate must emit an explicit skip result rather than failing or silently returning an unverified pass.

---

## 3. Table-Driven Protocol Compliance Vectors

| Compliance Vector | Invariant | Target Substrate Gate | Expected Failure / Outcome |
| :--- | :--- | :--- | :--- |
| `TC-AGT-07` | Law 2 (Non-Verbal Proof) | `AntiShimGate` | Rejects trivial constant return stubs with `ERR_TRIVIAL_PASS_SHIM` |
| `TC-AGT-08` | Law 5 (Zero-Hardcoded-Path) | `ForbiddenLiteralsGate` | Rejects forbidden literal paths with `ERR_FORBIDDEN_LITERAL` |
| `TC-AGT-09` | Law 4 (SSoT) | `CheckBilateralParity` | Rejects zero-asset parity audits with `ERR_ZERO_ASSETS_DISCOVERED` |

