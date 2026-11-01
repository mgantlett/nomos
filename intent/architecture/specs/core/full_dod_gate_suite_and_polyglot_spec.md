---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-24"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - INT-737
    - SUB-1215
title: Full 42 DoD Gate Suite and Polyglot Verification Architecture Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Full 42 DoD Gate Suite and Polyglot Verification Architecture Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify the complete 42-sensor Definition of Done (DoD) verification architecture, mapping each statutory sensor probe to its exact cell in the 7x3 Canonical Matrix, and establishing the pure-Go polyglot verification strategy that guarantees zero CGO dependencies.

---

## 1. Problem Statement & Architectural Context

In Nomos v3, the Definition of Done pipeline grew organically across sequential sensor probes. However, legacy execution suffered from significant architectural liabilities:
1. **CGO Compilation Bottlenecks**: Polyglot AST parsing depended on `smacker/go-tree-sitter` with 10 compiled C grammars, preventing pure-Go `CGO_ENABLED=0` static binary builds.
2. **False Positives & Fragile Discards**: Several gates used naive string matching (e.g. matching forbidden literal keywords inside comments or docstrings) or relied on unchecked error discards (`_ = ...`), violating Law 7.
3. **Parity Confusion**: Transitioning to v4 initially activated 16, then 19, and then 21 statutory gates (ensuring each of the 21 Invariant Cells had at least one active Go-native anchor probe) before unifying all 42 granular sensor probes into the 7x3 matrix.

This specification formalizes the clean-room activation of all 42 statutory sensor probes, raising active gate verification to complete statutory depth while preserving **pure Go execution (`CGO_ENABLED=0`)**, **MaxFuncLOC <= 60**, and **sub-second verification latency**.

---

## 2. The Complete 42-Sensor Probe Taxonomy

The Nomos DoD verification engine binds 42 statutory sensor probes across 7 Laws and 3 Defense Rings:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       THE 7x3 CANONICAL MATRIX                             │
├──────────────────────────┬─────────────────┬────────────────┬───────────────┤
│ Statutory Law            │ Syntactic Ring  │ Dynamic Ring   │ Crypto Ring   │
├──────────────────────────┼─────────────────┼────────────────┼───────────────┤
│ 1: Dual-Core Inversion   │ Cell 1.1        │ Cell 1.2       │ Cell 1.3      │
│ 2: Non-Verbal Proof      │ Cell 2.1        │ Cell 2.2       │ Cell 2.3      │
│ 3: Hermetic Worktree     │ Cell 3.1        │ Cell 3.2       │ Cell 3.3      │
│ 4: Single Source Truth   │ Cell 4.1        │ Cell 4.2       │ Cell 4.3      │
│ 5: Zero-Hardcoded-Path   │ Cell 5.1        │ Cell 5.2       │ Cell 5.3      │
│ 6: Canonical Singularity │ Cell 6.1        │ Cell 6.2       │ Cell 6.3      │
│ 7: Fail-Closed Preflight │ Cell 7.1        │ Cell 7.2       │ Cell 7.3      │
└──────────────────────────┴─────────────────┴────────────────┴───────────────┘
```

### 2.1 Detailed Sensor Probe Allocation

#### Law 1: Dual-Core Inversion (Governance & Control)
- **Cell 1.1 (Syntactic)**:
  - `complexity`: AST cyclomatic complexity per function ($\le 15$).
  - `max_file_loc`: Maximum lines of code per file ($\le 400$).
  - `cognitive_debt`: Cognitive Debt Score (CDS) calculated from control flow nests ($\le 45$).
  - `boundary`: Enforces repository-local workspace boundary, preventing traversal out of workspace root.
  - `ui_anti_mask`: Detects UI shims masking unfinished functionality.
- **Cell 1.2 (Dynamic)**:
  - `phase_discipline`: Enforces valid state machine phase transitions (`IDLE` $\to$ `PLAN` $\to$ `EDIT` $\to$ `REVIEW` $\to$ `SYNC` $\to$ `LEARN`).
- **Cell 1.3 (Cryptographic)**:
  - `capbac_lease`: Validates active cryptographic CapBAC capability lease token for active worktrees.

#### Law 2: Non-Verbal Proof (Epistemology & Verification)
- **Cell 2.1 (Syntactic)**:
  - `tdd`: Validates that new or modified implementation source files possess accompanying test files.
  - `anti_shim`: AST-level inspection rejecting trivial functions returning literal `Passed: true` or constant dummy values.
  - `docstrings`: Validates exported function docstring density ($\ge 10\%$).
  - `dead_code`: Detects unreachable functions, unused local variables, and obsolete declarations.
  - `uasset_dependency`: Audits binary asset links and schema dependencies.
- **Cell 2.2 (Dynamic)**:
  - `go_toolchain`: Machine compilation verification (`go test`, `go vet`).
  - `coverage`: Statement test coverage gating ($\ge 55\%$).
  - `visual_regression`: Verifies UI components against baseline visual renders.
- **Cell 2.3 (Cryptographic)**:
  - `review_receipt`: Cryptographically verifies human/agent review attestation before merge.
  - `gate_integrity_proof`: Verifies that all active DoD gates contain non-trivial algorithmic verification logic.

#### Law 3: Hermetic Worktree (State & Isolation)
- **Cell 3.1 (Syntactic)**:
  - `staged_boundaries`: Machine-enforces that only files within the active task's designated plane boundary are staged.
- **Cell 3.2 (Dynamic)**:
  - `hermetic_worktree`: Verifies execution occurs strictly within transient worktrees (`worktrees/nomos-<KEY>`), and protected trunk `main` is physically read-only.
  - `git_hygiene`: Enforces canonical Git command hygiene and prevents unstaged mutations.
  - `ecosystem_drift`: Detects cross-repository drift against upstream ecosystem baseline.
- **Cell 3.3 (Cryptographic)**:
  - `release_receipt`: Verifies Git Notes release receipt under `refs/notes/nomos/releases`.

#### Law 4: Single Source of Truth (Contracts & Schemas)
- **Cell 4.1 (Syntactic)**:
  - `broken_wire`: Validates all relative file links and documentation references across Intent markdown specs.
  - `intent_traceability`: Validates that implementation symbols include bidirectional traceability annotations back to Intent specs ($\ge 50\%$).
  - `intent_steelman`: Audits intentional architecture docs for counter-argument steelman sections.
- **Cell 4.2 (Dynamic)**:
  - `contract_parity`: CUE schema validation across live operational datasets.
- **Cell 4.3 (Cryptographic)**:
  - `intent_parity`: SHA-256 byte parity audit across 45 canonical Intent-Substrate assets.

#### Law 5: Zero-Hardcoded-Path (Portability & Environment)
- **Cell 5.1 (Syntactic)**:
  - `forbidden_literals`: AST-level token search rejecting hardcoded absolute paths (`/home/`, `/tmp/`, `/Users/`) in source code.
  - `ip_leak`: Scans source code for leaked internal IP markers, proprietary URLs, and confidential credentials.
  - `magic_strings`: Detects inline raw string literals that should be extracted to constants.
  - `magic_string_duplication`: Detects string literals repeated $> 2$ times without constant extraction.
- **Cell 5.2 (Dynamic)**:
  - `infra_ingress`: Verifies dynamic port binding and zero hardcoded port assumptions.
- **Cell 5.3 (Cryptographic)**:
  - `topology_parity`: Cryptographically validates repository directory topology against `#NomosConfig` contract.

#### Law 6: Canonical Singularity (Action Space & Surface)
- **Cell 6.1 (Syntactic)**:
  - `commit_schema`: Enforces Dual-Layer commit message structure (`[KEY] type(scope): summary\n\n- Details`).
- **Cell 6.2 (Dynamic)**:
  - `zero_alias`: Validates that CLI commands and API endpoints possess zero hidden aliases, duplicates, or convenience wrappers.
  - `zero_filesystem_alias`: Rejects all filesystem symlinks in repository source.
- **Cell 6.3 (Cryptographic)**:
  - `canonical_singularity`: Validates task key singularity (`INT-<N>` or `SUB-<N>`).

#### Law 7: Fail-Closed Pre-Flight (Safety & Budgets)
- **Cell 7.1 (Syntactic)**:
  - `secret`: Scans code for private keys, AWS tokens, API secrets, and unmasked credentials.
  - `config_immutability`: Machine-enforces immutability of sealed system configuration files.
  - `plane_separation`: Enforces that `SUB-*` cannot mutate `intent/` and `INT-*` cannot mutate `substrate/`.
- **Cell 7.2 (Dynamic)**:
  - `unchecked_errors`: Rejects unchecked error discards via `_ = ...` or `, _ :=`.
- **Cell 7.3 (Cryptographic)**:
  - `threshold_integrity`: Cryptographically verifies that DoD metric thresholds in `gates.cue` cannot be loosened without cryptographic authorization.

---

## 3. Pure-Go Polyglot AST Engine (Zero CGO + Embedded WASM Tree-sitter)

In Nomos v4, polyglot verification executes natively in pure Go without external C toolchains:
1. **Go Source Analysis**: Standard library `go/parser`, `go/ast`, and `go/token`. Fast, zero dependencies, memory safe.
2. **Polyglot Tree-sitter WASM Analysis**: Embedded `wazero` WebAssembly runtime executing pre-compiled Tree-sitter grammars for **Go, TypeScript, Python, and Rust** (`substrate/src/nomos/modules/verify/treesitter/`).
3. **Markdown & CUE Analysis**: Lexical stream scanner for Markdown relative links/frontmatter, paired with the native `cuelang.org/go/cue` schema unification engine.
4. **Configuration & Script Analysis**: Standard library lexical analyzers for JSON, YAML, and shell scripts.

This architecture ensures static compilation under `CGO_ENABLED=0`, cross-compilation across all target architectures, and sub-second verification runs.

---

## 4. Thresholds and Mathematical Guarantees

All thresholds are unified in `intent/contracts/cue/gates.cue`:

| Metric | Threshold | Enforcement Probe |
| :--- | :--- | :--- |
| **Max Function LOC** | $\le 60$ LOC | `complexity` / `gates_standard.go` |
| **Max File LOC** | $\le 400$ LOC | `max_file_loc` |
| **Max Cyclomatic Complexity** | $\le 15$ | `complexity` |
| **Max Cognitive Debt Score (CDS)** | $\le 45$ | `cognitive_debt` |
| **Min Test Statement Coverage** | $\ge 55\%$ | `coverage` |
| **Min Docstring Density** | $\ge 10\%$ | `docstrings` |
| **Min Intent Traceability Density** | $\ge 50\%$ | `intent_traceability` |
| **Max Magic String Duplications** | $\le 2$ repetitions | `magic_string_duplication` |
| **Symlink Tolerance** | $0$ (strictly forbidden) | `zero_filesystem_alias` |
| **Unchecked Error Discard Tolerance** | $0$ (strictly forbidden) | `unchecked_errors` |

---

## 5. Table-Driven Verification Vectors

| Vector ID | Sensor / Cell | Input Condition | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-DOD-01` | Cell 1.1 (`complexity`) | Function cyclomatic complexity `> 15` or LOC `> 60` | `FAIL` (Exit 1, threshold breach) |
| `TC-DOD-02` | Cell 2.1 (`anti_shim` / `docstrings`) | Trivial constant-return shim or modified file docstring density `< 10%` | `FAIL` (Exit 1, AST proof violation) |
| `TC-DOD-03` | Cell 6.2 (`zero_filesystem_alias`) | Symbolic link committed inside repository tree | `FAIL` (`ERR_SYMLINK_FORBIDDEN`) |
| `TC-DOD-04` | Cell 7.1 (`plane_separation` / `secret`) | Cross-plane `INT-*`/`SUB-*` edit or inline credential literal | `FAIL` (Fail-closed pre-flight block) |
| `TC-DOD-05` | Full 7x3 Matrix (`42` probes) | Clean Go + WASM Tree-sitter polyglot workspace | `PASS` (Exit 0, `42/42` satisfied) |

---

## 6. Verification & Acceptance

The Definition of Done for the statutory gate suite requires:
1. All 42 statutory probes implemented in pure Go and registered in `CanonicalGateOrder`.
2. Clean separation of probes into modular Go packages conforming to $\le 400$ file LOC and $\le 60$ function LOC.
3. `nomos verify` executing and returning exit code 0 across the complete 42-gate statutory suite.
4. 100% deterministic, zero false positives, and zero external C dependencies.
