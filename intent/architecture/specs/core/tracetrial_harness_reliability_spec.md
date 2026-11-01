---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-11-01"
last_updated: "2026-11-01"
plane: intent
status: active
tags:
    - INT-858
    - SUB-1527
title: "TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification"
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify the empirical 7-pillar harness reliability evaluation framework aligned with *TraceTrial-Bench* (`arXiv:2610.32285`) and harden three critical fail-closed adversarial boundary invariants across worktree isolation, statutory configuration pinning, and gate crash recovery.

---

## 1. Context & Epistemology

Contemporary agent benchmarks evaluate model code generation in isolation while holding the orchestration harness constant. Empirical evaluation in *TraceTrial-Bench* (`arXiv:2610.32285`) demonstrates that enterprise coding agents succeed or fail across **seven orthogonal harness pillars**: *Context*, *Execution*, *Governance*, *Lifecycle*, *Observability*, *Tools*, and *Verification*.

In Nomos, prompts are advisory while the compiled Go substrate is statutory. A governance harness is only statutory if it **fails closed** under adversarial mutation, boundary escape attempts, and runtime probe failures:

1. **Worktree Boundary Escape (Law 3):** An agent must be physically blocked from mutating files outside `worktrees/<repo>-<task>/` or committing directly to protected trunk.
2. **Statutory Configuration Tampering (Law 4 & Law 7):** An agent inside a task worktree must be incapable of relaxing gate thresholds, disabling `strict_mode`, or disabling `boundary.enforce` relative to the trunk baseline (`nomos.yaml`).
3. **Gate Crash / Panic Abort (Law 2 & Law 5):** If any verification gate panics, errors, or fails during evaluation, the 2-Phase Commit (2PC) release engine (`nomos task sync`) must deterministically recover, abort the merge transaction, emit zero cryptographic release receipts, and leave trunk (`main`) untouched.

---

## 2. Seven-Pillar Statutory Mapping

| Pillar | TraceTrial-Bench Dimension | Nomos Statutory Law & Enforcement Seam |
| :--- | :--- | :--- |
| **1. Context** | Bounded, repository-scoped orientation without cross-repo leakage | **Law 1 & Law 6:** `SynthesizeDynamicHandshake` (`nomos handshake`) injects repository-scoped wavefronts, directives, and verified Mneme patterns (`mneme.db`). |
| **2. Execution** | Hermetic filesystem isolation and path-traversal confinement | **Law 3:** `worktree-guard.sh` pre-tool boundary interceptor and Git `pre-commit` hooks reject writes outside `worktrees/<repo>-<task>/` and block `..` traversal. |
| **3. Governance** | Immutability of grading rubrics, plane separation, and config pinning | **Law 4 & Law 7:** `PlaneSeparationGate` blocks `SUB-*` config mutations; `ThresholdIntegrityGate` pins `strict_mode` and `boundary.enforce` to trunk `RepoRoot`. |
| **4. Lifecycle** | Atomic 2PC state transitions and crash-safe abort semantics | **Law 5:** `TimedEvaluate` recovers runtime gate panics into `ERR_CORE` fail-closed errors; `nomos task sync` aborts before rebase/FF-merge on any non-zero exit. |
| **5. Observability** | Cryptographic release provenance and empirical pattern distillation | **Law 2:** `nomos task sync` stamps SHA-256 receipts onto `refs/notes/nomos/releases` and distills verified positive-path patterns into `mneme.db`. |
| **6. Tools** | Sanctioned toolchain wrappers and zero filesystem aliasing | **Law 3 & Law 4:** `nomos-build-guard.sh` intercepts raw `go build` bypasses; `ZeroFilesystemAliasGate` rejects symlinks inside task worktrees. |
| **7. Verification** | Multi-gate AST anti-shortcut enforcement (`0.0%` trunk defect rate) | **Law 2 & Law 4:** 42 DoD gates (`AntiShimGate`, `SecretScanGate`, `UncheckedErrorsGate`, `GateIntegrityProofGate`) reject shortcuts fail-closed. |

---

## 3. Fail-Closed Hardening Invariants

### 3.1 Trunk Statutory Baseline Pinning (`ThresholdIntegrityGate`)
When `nomos verify` executes inside an isolated task worktree (`ctx.WorktreePath != ""` and `filepath.Clean(ctx.WorktreePath) != filepath.Clean(ctx.RepoRoot)`):
1. **Substrate Plane Rejection:** `PlaneSeparationGate` rejects any modification to root `nomos.yaml` or `intent/` files within a `SUB-*` task (`ERR_CROSS_PLANE_MUTATION`).
2. **Intent Plane Downgrade Rejection:** Even within an `INT-*` worktree permitted to edit intent/governance artifacts, `ThresholdIntegrityGate` loads the canonical trunk configuration from `ctx.RepoRoot`.
   - If trunk `nomos.yaml` enforces `governance.quality_gates.strict_mode: true` and the worktree `nomos.yaml` downgrades `strict_mode: false`, `ThresholdIntegrityGate` **must** return `Passed: false`.
   - If trunk `nomos.yaml` enforces `governance.boundary.enforce: true` and the worktree `nomos.yaml` downgrades `boundary.enforce: false`, `ThresholdIntegrityGate` **must** return `Passed: false`.
   - Any threshold relaxation violating statutory ceilings (`max_file_loc > 400`, `max_func_loc > 60`, `max_complexity > 15`, `min_coverage < 50.0`) continues to fail closed.
3. **Brownfield Root Compatibility:** When evaluating directly on a brownfield repository root (`ctx.WorktreePath == ""`) where the repository owner explicitly configured `strict_mode: false` on trunk, `ThresholdIntegrityGate` honors the root baseline.

### 3.2 Gate Runtime Panic Recovery (`TimedEvaluate`)
In `substrate/src/nomos/modules/verify/gate.go`, `TimedEvaluate(g Gate, ctx *VerifyContext)` wraps `g.Evaluate(ctx)` in a `defer recover()` boundary:
- Any runtime panic `r` inside a gate probe is caught and converted into a structured `NomosError` (`ERR_CORE`: `"gate <Name> panicked during evaluation (fail-closed abort): <r>"`) with `res = nil`.
- `Pipeline.Execute` treats any non-nil `err` from `TimedEvaluate` as an immediate pipeline abort, ensuring `nomos verify` and `nomos task sync` never fail open or leave half-committed state.

---

## 4. Deterministic Verification Vectors (`VEC-TTB-01` .. `VEC-TTB-07`)

The cleanroom harness suite (`substrate/cli/cleanroom_tracetrial_bench_test.go`) and unit hardening tests (`substrate/src/nomos/modules/verify/tracetrial_hardening_test.go`) execute seven deterministic end-to-end vectors:

- **`VEC-TTB-01` (Pillar 1 — Context):** Provision cleanroom workspace, seed Mneme records across two repo scopes, execute `SynthesizeDynamicHandshake`, and assert strict repository isolation plus active wavefront orientation.
- **`VEC-TTB-02` (Pillar 2 — Execution / Worktree Escape):** Invoke `worktree-guard.sh` against root trunk paths and `worktrees/repo-SUB-1/../../trunk.go` path traversal, plus attempt a direct commit on `main`; assert non-zero fail-closed denial across all vectors.
- **`VEC-TTB-03` (Pillar 3 — Governance / Config Tampering):**
  - *Sub-vector 3a:* Mutate `nomos.yaml` inside a `SUB-*` worktree; assert `PlaneSeparationGate` fails closed.
  - *Sub-vector 3b:* Flip `strict_mode: false` and `max_file_loc: 9999` inside an `INT-*` worktree against a `strict_mode: true` trunk; assert `ThresholdIntegrityGate` detects the downgrade against `RepoRoot` and fails closed.
- **`VEC-TTB-04` (Pillar 4 — Lifecycle / Gate Crash & 2PC Abort):** Trigger a panicking gate in `TimedEvaluate` / `Pipeline.Execute` (asserting structured `ERR_CORE` recovery without process crash) and run `nomos task sync` on a failing worktree; assert trunk `HEAD` SHA is unchanged and `refs/notes/nomos/releases` contains zero receipts for the aborted task.
- **`VEC-TTB-05` (Pillar 5 — Observability / Attestation & Mneme Provenance):** Sync a compliant worktree through 2PC `nomos task sync`; assert a cryptographic SHA-256 receipt is attached to `refs/notes/nomos/releases` and a statutory `pattern` record is distilled into `mneme.db`.
- **`VEC-TTB-06` (Pillar 6 — Tools / Toolchain & Symlink Confinement):** Invoke `nomos-build-guard.sh` on raw `go build` (asserting fail-closed block) and plant a symlink inside a worktree (asserting `ZeroFilesystemAliasGate` rejection).
- **`VEC-TTB-07` (Pillar 7 — Verification / Anti-Shortcut Enforcement):** Inject an empty function stub (`func Shortcut() {}`) into a worktree; assert `AntiShimGate` rejects the shortcut (`0.0%` trunk defect leakage).
