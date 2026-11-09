---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-05"
last_updated: "2026-11-09"
lineage:
  - INT-541
  - INT-1935
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Intent Traceability & Architectural Obligation Proofs Specification"
type: spec
version: "4.13.0"
---

# Nomos Intent Traceability & Architectural Obligation Proofs Specification

---

## 1. Executive Summary & Problem Statement

Universal AST gates prove local code hygiene and worktree isolation. However, universal AST gates alone do not prove repository-specific architectural obligations:
1. A specification can declare `TC-*` vectors or Go structs while downstream code omits matching verification suites or alters fields.
2. An `INT-*` commit on Trunk can mutate a contract after a `SUB-*` merge, leaving the implementation verified against an older digest.
3. Functions can satisfy complexity ceilings while violating package dependency boundaries (for example, `domain/` importing `infra/`).

This specification upgrades Law 4 and Law 7 to enforce deterministic **Architectural Obligation Proofs**:
- **Executable `TC-*` Binding & Signature Parity (`SUB-2002`):** Binds specification `TC-*` vectors to verification suites (`@verifies TC-*`) and matches specification Go structs against compiled AST declarations.
- **Trunk Spec SHA-256 Receipt Stamping (`SUB-2003`):** Stamps referenced `@intent` specification SHA-256 digests into `ReleaseNotePayload.SpecDigests` and flags stale obligations after `INT-*` mutations.
- **Declarative Architecture Boundaries (`SUB-2004`):** Enforces directional package import rules (`governance.quality_gates.architecture_boundaries`) across Go AST and polyglot files in under `5ms`.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Enforcement |
| :---: | :--- | :--- |
| **Law 1** | **Dual-Core Inversion** | Compiled Go AST and Tree-sitter analyzers verify obligations independently. |
| **Law 2** | **Non-Verbal Proof** | `IntentTraceabilityGate` requires executable `TC-*` bindings and struct field parity. |
| **Law 4** | **Single Source of Truth** | Specification contracts, `nomos.schema.json`, and Go AST structs maintain structural parity. |
| **Law 5** | **Zero-Hardcoded-Path** | Obligation and boundary globs resolve dynamically from repository root. |
| **Law 6** | **Canonical Singularity** | Extends `IntentTraceabilityGate`, `ContractParityGate`, and `ReleaseNotePayload` with zero aliases. |
| **Law 7** | **Fail-Closed Pre-Flight** | Unbound `TC-*` vectors, drifted fields, and forbidden imports fail closed. |

---

## 3. Traceability & Obligation Binding Syntax

### 3.1 `@intent` Source Annotations

Exported packages, structs, interfaces, and functions in `substrate/` declare Trunk lineage via `@intent` docstrings:

1. **Canonical Intent Task Key**:
   ```go
   // @intent INT-541
   ```
2. **Semantic Heading Anchor**:
   ```go
   // @intent INT-541#continuous-collision-detection
   ```
3. **Compiled Go Type & Specification Anchor**:
   ```go
   // @intent #PhysicsFrame (software_physics_engine_spec.md)
   // @intent nomos_intent_traceability_spec.md #ArchitectureBoundaryRule
   ```

### 3.2 `@verifies` Executable Obligation Annotations

Verification files (`*_test.go`, `*.test.ts`, `*.spec.ts`, `test_*.py`, `*_test.py`) bind directly to `TC-[A-Z0-9-]+` vectors declared in specifications using:
1. Explicit comment annotations (`// @verifies TC-OBL-001` or `# @verifies TC-OBL-001`), or
2. Subtest or table-vector literals containing the canonical identifier (`"TC-OBL-001"` or `TestTC_OBL_001`).

### 3.3 Declarative Architecture Boundaries in `nomos.yaml`

Repositories declare directional package import obligations in `nomos.yaml` under `governance.quality_gates.architecture_boundaries`:

```yaml
governance:
  quality_gates:
    architecture_boundaries:
      - name: "domain-isolation"
        from: "internal/domain/**"
        forbid_imports:
          - "internal/infra/**"
          - "database/sql"
      - name: "ui-service-seam"
        from: "src/routes/**"
        allow_only:
          - "$lib/**"
          - "svelte"
```

---

## 4. Compiled Go SSoT Contracts

### 4.1 Configuration Contracts (`substrate/src/nomos/core/config/config.go`)

```go
package config

// ContractsConfig defines contract schema, obligation binding, and backend verification roots (#ContractsConfig).
type ContractsConfig struct {
	Enabled                bool     `yaml:"enabled" json:"enabled"`
	SpecRoots              []string `yaml:"spec_roots,omitempty" json:"spec_roots,omitempty"`
	SchemaFiles            []string `yaml:"schema_files,omitempty" json:"schema_files,omitempty"`
	BackendRoots           []string `yaml:"backend_roots,omitempty" json:"backend_roots,omitempty"`
	StrictTypes            bool     `yaml:"strict_types" json:"strict_types"`
	VerifyObligations      bool     `yaml:"verify_obligations,omitempty" json:"verify_obligations,omitempty"`
	FailOnStaleObligations bool     `yaml:"fail_on_stale_obligations,omitempty" json:"fail_on_stale_obligations,omitempty"`
}

// ArchitectureBoundaryRule declares directional import obligations for source globs (#ArchitectureBoundaryRule).
type ArchitectureBoundaryRule struct {
	Name          string   `yaml:"name,omitempty" json:"name,omitempty"`
	From          string   `yaml:"from" json:"from"`
	ForbidImports []string `yaml:"forbid_imports,omitempty" json:"forbid_imports,omitempty"`
	AllowOnly     []string `yaml:"allow_only,omitempty" json:"allow_only,omitempty"`
}
```

`QualityGatesConfig` exposes `ArchitectureBoundaries []ArchitectureBoundaryRule` (`yaml:"architecture_boundaries,omitempty" json:"architecture_boundaries,omitempty"`), reflected into Draft 2020-12 `nomos.schema.json`. `ConfigImmutabilityGate` blocks worktree deletions or weakenings of Trunk `architecture_boundaries` rules.

### 4.2 Release Provenance & Obligation Taint Contracts (`substrate/src/nomos/modules/task/git_notes.go`)

```go
package task

// StaleSpecObligation identifies an Intent specification modified after its latest SUB-* verification receipt (#StaleSpecObligation).
type StaleSpecObligation struct {
	SpecPath       string `json:"spec_path"`
	CurrentSHA256  string `json:"current_sha256"`
	VerifiedSHA256 string `json:"verified_sha256"`
	LastVerifiedBy string `json:"last_verified_by"`
}
```

`ReleaseNotePayload` includes `SpecDigests map[string]string` (`json:"spec_digests,omitempty"`), mapping workspace-relative specification paths to 64-character hex SHA-256 digests recorded at `2PC` sync.

### 4.3 Gate & Fast-Radar Evaluators (`substrate/src/nomos/modules/verify/`)

```go
package verify

// VerifySpecTypeSignatures checks exported Go structs in spec code blocks against compiled substrate AST declarations.
func VerifySpecTypeSignatures(root, targetDir string, specPaths []string) []string

// VerifyExecutableObligations checks that TC-* vectors declared in active specs appear in repository verification suites.
func VerifyExecutableObligations(root string, specPaths []string) []string

// EvaluateArchitectureBoundaries checks Go AST and polyglot import statements against configured ArchitectureBoundaryRule entries.
func EvaluateArchitectureBoundaries(root string, files []string, rules []config.ArchitectureBoundaryRule) []string
```

---

## 5. Deterministic Execution Semantics

1. **Base `@intent` Resolution (`Cell 4.1`):**
   - Scans exported Go symbols in `substrate/` and enforces $\ge 50\%$ `@intent` docstring density on modified Go files.
   - Verifies every `#<TypeName>` resolves to a compiled Go `type <TypeName>` declaration and every `<name>_spec.md` resolves under `intent/`.
2. **Spec Signature AST Parity (`contracts.strict_types: true`):**
   - Extracts fenced ````go` blocks from target specifications and parses `type <Name> struct` declarations via `go/parser`.
   - Verifies every exported field declared in the specification exists on the compiled Go struct in `substrate/`.
3. **Executable `TC-*` Vector Binding (`contracts.verify_obligations: true`):**
   - Extracts `TC-[A-Z0-9]+-[0-9]+` identifiers from Markdown tables in target specifications and scans repository verification suites (`*_test.go`, `*.test.*`, `*.spec.*`, `test_*.*`).
   - Missing bindings reject execution with a HATEOAS remedy naming the unbound `TC-*` identifier and specification file.
4. **Spec SHA-256 Stamping & Taint Detection (`ReleaseNotePayload.SpecDigests`):**
   - During `nomos task sync` for `SUB-*` tasks, Nomos hashes `@intent`-referenced specifications and writes `spec_digests` into `refs/notes/nomos/releases`.
   - `ComputeStaleSpecObligations` compares `HEAD` specification digests against the newest `spec_digests` receipt.
   - `nomos status` displays pending obligation deltas, and `IntentTraceabilityGate` fails closed when `contracts.fail_on_stale_obligations: true`.
5. **Polyglot Architecture Boundary Enforcement (`architecture_boundaries`):**
   - Evaluates imports across Go (`go/parser`) and polyglot files (`import`, `require`, `from`, `use`, `using`, `#include`) against matching `from` globs.
   - Executes in under `5ms` inside both `EvaluateFastFileRadar` (`PreToolUse` / `PostToolUse` hooks) and `nomos verify`.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Target Component | Input Vector | Expected Outcome | Invariant |
| :--- | :--- | :--- | :--- | :--- |
| `TC-TRC-001` | `IntentTraceabilityGate` | `"// @intent INT-541"` | Passes with valid docstring density | Law 4 & Law 6 |
| `TC-TRC-002` | `verifyIntentASTAnchors` | `"// @intent #HandshakeEvent (jit_handshake_and_session_journal_spec.md)"` | Resolves `HandshakeEvent` and spec file | Law 4 |
| `TC-TRC-003` | `verifyIntentASTAnchors` | `"// @intent #NonExistentPhantomType"` | Fails closed naming `#NonExistentPhantomType` | Law 2 & Law 7 |
| `TC-TRC-004` | `verifyIntentASTAnchors` | `"// @intent missing_ghost_spec.md"` | Fails closed naming missing spec | Law 4 & Law 7 |
| `TC-OBL-001` | `VerifyExecutableObligations` | Spec declares `TC-AUTH-01`; `auth_test.go` tags `@verifies TC-AUTH-01` | Passes with zero unbound obligations | Law 2 & Law 4 |
| `TC-OBL-002` | `VerifyExecutableObligations` | Spec declares `TC-AUTH-99` with zero verification references | Fails closed naming `TC-AUTH-99` | Law 2 & Law 7 |
| `TC-OBL-003` | `VerifySpecTypeSignatures` | Spec declares `Strict bool` on `ContractsConfig`; substrate omits field | Fails closed naming field drift | Law 4 & Law 7 |
| `TC-OBL-004` | `ComputeStaleSpecObligations` | `SUB-10` stamps `spec_digests`; later `INT-11` mutates `auth_spec.md` | Reports `auth_spec.md` stale obligation delta | Law 4 |
| `TC-OBL-005` | `ComputeStaleSpecObligations` | Later `SUB-12` stamps updated `auth_spec.md` SHA-256 | Clears stale obligation delta | Law 2 & Law 4 |
| `TC-OBL-006` | `EvaluateArchitectureBoundaries` | `internal/domain/order.go` imports `internal/infra/db` matching `forbid_imports` | Fails closed with boundary remedy | Law 7 |
| `TC-OBL-007` | `EvaluateArchitectureBoundaries` | `src/routes/page.ts` imports `../db/client` outside `allow_only` | Fails closed in radar and verify | Law 7 |
| `TC-OBL-008` | `ConfigImmutabilityGate` | Worktree `nomos.yaml` deletes Trunk `architecture_boundaries` rule | Fails closed blocking boundary downgrade | Law 1 & Law 7 |

---

## 7. Cross-Plane Traceability & DoD Verification

1. **Compiled Go SSoT Paths:**
   - `substrate/src/nomos/core/config/config.go` (`ContractsConfig`, `ArchitectureBoundaryRule`, `QualityGatesConfig`).
   - `substrate/src/nomos/modules/task/git_notes.go` (`ReleaseNotePayload`, `StaleSpecObligation`, `ComputeStaleSpecObligations`).
   - `substrate/src/nomos/modules/verify/gates_intent.go` & `substrate/src/nomos/modules/verify/intent_schema_helpers.go` (`IntentTraceabilityGate`, `VerifyExecutableObligations`, `VerifySpecTypeSignatures`, `EvaluateArchitectureBoundaries`).
2. **7x3 Invariant Matrix Coordinates:**
   - `Cell 4.1 (intent_traceability)` and `Cell 4.2 (contract_parity)` enforce `@intent` anchors, executable `TC-*` bindings, struct parity, taint checks, and import boundaries.
   - `Cell 3.3 (release_receipt)` validates `ReleaseNotePayload` integrity including optional `spec_digests` SHA-256 entries.
3. **Verification Command:**
   - Execute `nomos verify` across all 42 Definition of Done quality gates and assert exit code `0`.
