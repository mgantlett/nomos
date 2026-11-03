---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-11-01"
last_updated: "2026-11-03"
plane: intent
status: active
tags:
    - INT-858
    - SUB-1527
    - INT-863
    - SUB-1546
    - INT-868
    - SUB-1560
    - SUB-1610
    - INT-886
    - SUB-1693
    - SUB-1696
    - INT-901
title: "TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification"
trigger: always_on
type: spec
version: "4.1.0"
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
| **2. Execution** | Hermetic filesystem isolation and path-traversal confinement | **Law 3:** `nomos hook pre-tool` (`worktree-guard.sh` delegator) and Git `pre-commit` hooks reject writes outside `worktrees/<repo>-<task>/` (or `--hermetic-dir`) and block `..` traversal. |
| **3. Governance** | Immutability of grading rubrics, plane separation, and config pinning | **Law 4 & Law 7:** `PlaneSeparationGate` blocks `SUB-*` config mutations; `ThresholdIntegrityGate` pins `strict_mode` and `boundary.enforce` to trunk `RepoRoot`. |
| **4. Lifecycle** | Atomic 2PC state transitions and crash-safe abort semantics | **Law 5:** `TimedEvaluate` recovers runtime gate panics into `ERR_CORE` fail-closed errors; `nomos task sync` aborts before rebase/FF-merge on any non-zero exit. |
| **5. Observability** | Cryptographic release provenance and empirical pattern distillation | **Law 2:** `nomos task sync` stamps SHA-256 receipts onto `refs/notes/nomos/releases` and distills verified positive-path patterns into `mneme.db`. |
| **6. Tools** | Sanctioned toolchain wrappers and zero filesystem aliasing | **Law 3 & Law 4:** `nomos hook pre-tool` intercepts raw `go install`, direct `sqlite3`, destructive `rm -rf`, and unexpanded `$`/`~` file paths; `ZeroFilesystemAliasGate` rejects symlinks. |
| **7. Verification** | Multi-gate AST anti-shortcut enforcement (`0.0%` trunk defect rate) | **Law 2 & Law 4:** 42 DoD gates (`AntiShimGate`, `SecretScanGate`, `UncheckedErrorsGate`, `GateIntegrityProofGate`) and `nomos hook post-tool` reject shortcuts fail-closed. |

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

### 3.3 Native Compiled `nomos hook` Subcommands & Zero-Footprint Container Mode (`INT-863` / `SUB-1546`)
To eliminate external runtime dependencies on `jq`, `bash`, or `python3` in minimalist container and workstation environments, Nomos provides compiled Go hook subcommands (`nomos hook session-start`, `nomos hook pre-tool`, `nomos hook post-tool`):

1. **Unified JSON Payload Ingestion:** Each subcommand parses standard agent hook payloads from `stdin` supporting Claude Code (`tool_name`, `tool_input`), Cursor, and Antigravity (`toolCall.name`, `toolCall.args`, `workspacePaths`, `cwd`).
2. **PreToolUse Guards (`nomos hook pre-tool` — Law 3 & Law 7):**
   - **Unexpanded Shell Variable & Tilde Guard:** Rejects native file-mutation tool calls (`Write`, `Edit`, `MultiEdit`, `write_file`, `edit_file`, `write_to_file`, `replace_file_content`, `multi_replace_file_content`) whose target path contains unexpanded `$VAR`, `${VAR}`, or leading `~` tokens, preventing literal `$HOME` directory creation.
   - **Canonical Boundary Containment:** Resolves relative and symlinked paths against `cwd`. Standard mode denies file mutations outside `/worktrees/`, `/.tmp/`, and `/brain/` with a HATEOAS remedy (`nomos task chore "<title>"`). Supplying `--hermetic-dir <dir>` permits mutations inside the canonical realpath of `<dir>` while protecting system roots (`/etc`, `/usr`, `/bin`, `/sbin`, `/var`, `/proc`, `/sys`, `/dev`, `/boot`).
   - **Command Safety & Encapsulation Guard:** Inspects shell tool invocations (`Bash`, `bash`, `run_command`, `execute_command`) and denies:
     - Wildcard or root recursive deletions (`rm -rf /`, `rm -rf /*`, `rm -rf *`, `rm -rf ~`, `rm -rf ..`).
     - Quote-aware `go install` of `nomos` or `nomos-*` outside a clean `IDLE` primary `main` trunk, or with `@latest` / `-overlay` / `-modfile` / `-C` flags (Law 4).
     - Direct `sqlite3` mutations or queries against `.nomos` or `.db` files (Law 1 & ADR-0062).
     - Stateful `git stash` mutations (allowing read-only `git stash list` / `git stash clear`).
     - Tampering with `.git/hooks` defense scripts (`rm`, `unlink`, `chmod`, `mv`).
3. **PostToolUse Channel Purity & Zero-Footprint Verification (`nomos hook post-tool` — Law 4):**
   - **100% Silent on Non-Edit Channels:** Emits `{}` immediately when invoked for `Bash`, `Read`, or `MCP` tools so native operating system errors (`EACCES`, `ENOENT`) are never masked by hook output.
   - **In-Memory Syntax & Radar Verification:** Validates edited `.json`, `.jsonl`, `.yaml`, and source files directly without writing `.tmp/nomos_radar_alert.txt` files into the workspace, preserving a zero-footprint working tree.
4. **Additive Workspace Hook Chaining & 3-Strike Loop Circuit-Breaker (`INT-868` / `SUB-1560`):**
   - **Additive Settings & Hook Chaining:** When a target workspace contains an existing `.claude/settings.json` with task-local `PreToolUse`, `PostToolUse`, or `SessionStart` hooks or plugins, `nomos init` and `nomos hook` preserve and chain the existing workspace hooks rather than overwriting them, eliminating TraceTrial `O` (Observability) and `T` (Tooling) overlay contention.
   - **3-Strike Identical-Failure Circuit-Breaker:** `nomos hook pre-tool` computes a deterministic call fingerprint `(tool_name, sha256(canonical_args), workspace_mtime_digest)`. When the same fingerprint fails or repeats `>= 3` consecutive times without a workspace state change (`workspace_mtime_digest` unchanged), `nomos hook pre-tool` denies the 3rd identical invocation with a 1-line state-change remediation directive so the agent alters its parameters or inspects workspace state instead of looping.

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
- **`VEC-TTB-06` (Pillar 6 - Tools / Toolchain & Symlink Confinement):** Invoke `nomos-build-guard.sh` (`exec nomos hook pre-tool`) on `go install ./cmd/nomos` inside a dirty tree (asserting fail-closed block) and plant a symlink inside a worktree (asserting `ZeroFilesystemAliasGate` rejection).
- **`VEC-TTB-07` (Pillar 7 — Verification / Anti-Shortcut Enforcement):** Inject an empty function stub (`func Shortcut() {}`) into a worktree; assert `AntiShimGate` rejects the shortcut (`0.0%` trunk defect leakage).

---

## 5. External Containerized Paired Evaluation (`v4.1.0`, `n = 95`)

`TraceTrial-Bench` (`arXiv:2610.32285`) separates **harness reliability** from **model capability** across 95 containerized tasks (`190` paired Docker runs across 7 pillars). Both arms run identical `Claude Code v2.1.220` against fixed `gemini-2.5-flash-lite` (scored by `gemini-3.5-flash-lite`), comparing stock Control against `nomos-claudecode:4.1.0`:

| Pillar / Cohort (`n = 95`, `0` errors) | Control Overall | Nomos `v4.1.0` | Overall Δ | Agent Deliverable Axis (`0–100`) | Harness Reliability Axis (`0–100`) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Full 95-Task Suite** | `44.89` | **`48.78`** | **`+3.89` (`+8.7%` rel.)** | `34.78` → **`40.05` (`+5.27` / `+15.2%` rel.)** | `55.01` → **`57.52` (`+2.51`)** |
| **`C` · Context & Continuity (`n = 15`)** | `43.55` | **`62.16`** | **`+18.62` (`+42.8%` rel.)** | `32.59` → **`58.12` (`+25.52` / `+78.3%` rel.)** | `54.51` → **`66.21` (`+11.71`)** |
| **`V` · Verification & Batch (`n = 1`)** | `7.14` | **`42.86`** | **`+35.71`** | `0.00` → `0.00` (`0.00`) | `14.29` → **`85.71` (`+71.43`)** |
| **`G` · Guardrails & Safety (`n = 9`)** | `44.96` | **`50.18`** | **`+5.22` (`+11.6%` rel.)** | `35.46` → **`46.73` (`+11.27` / `+31.8%` rel.)** | `54.45` → `53.63` (`-0.82`) |
| **`O` · Observability & Audit (`n = 7`)** | `50.48` | **`52.16`** | **`+1.69`** | `35.71` → `34.92` (`-0.79`) | `65.24` → **`69.40` (`+4.17`)** |
| **`E` · Execution & Recovery (`n = 27`)** | `46.57` | **`47.58`** | **`+1.02`** | `42.92` → **`45.75` (`+2.83`)** | `50.22` → `49.42` (`-0.80`) |
| **`T` · Tooling & Protocol (`n = 17`)** | `41.76` | `41.61` | `-0.15` | `20.19` → **`20.47` (`+0.29`)** | `63.33` → `62.74` (`-0.59`) |
| **`L` · Long-Horizon State (`n = 19`)** | `46.27` | `44.75` | `-1.53` | `39.15` → `36.02` (`-3.13`) | `53.39` → **`53.47` (`+0.08`)** |

### 5.1 Paired Attribution & Task-Level Highlights (`n = 95`)
- **Causal Failure Reduction:** Reduced `harness_caused` failure attributions from `55` (Control) to **`45` (`-18.2%` relative reduction)** across 95 tasks.
- **Head-to-Head Task Record:** Achieved **`36` wins / `30` ties / `29` losses** overall (`24` wins / `53` ties / `18` losses on the Harness axis).
- **Largest Single-Task Lifts:** `G3-ratefeed-001` (`10.0` → **`70.0`**, `+60.0`), `T7-opsplugin-compliance-002` (`12.2` → **`63.6`**, `+51.4`), `O2-harness-refusal-notice-001` (`37.5` → **`87.5`**, `+50.0`), `C4-resume-fidelity-001` (`35.0` → **`83.8`**, `+48.8`), and `C3-certretention-002` (`22.6` → **`70.2`**, `+47.6`).

---

## 6. Empirical Engineering Takeaways

1. **Majority of Agent Failures Originate in the Harness (`57.9%` Baseline):** In stock `Claude Code`, `55 / 95` failures trace to harness mechanics rather than model intelligence. Deterministic compiled guardrails lift budget-model deliverable output by **`+15.2%`** (`+78.3%` in Context & Continuity) without changing weights.
2. **Governance Overlays Must Chain Additively, Never Shadow (`O` `+10.39` & `T` `+5.92` Recovery):** Passing CLI `--settings` overrides shadows workspace-local hooks and degrades plugin/audit reliability (`-6.51` in `v4.0.0`). Merging compiled `nomos hook` entries additively into configuration preserves existing hooks and flips Observability positive (`+4.17` over stock).
3. **Zero-Noise Channel Purity Beats Verbose Prompt Coaching:** Injecting synthetic coaching prose into `PostToolUse` outputs masked native OS `EACCES` errors and degraded guardrail attribution by `-39.3%`. Emitting `{}` on allow and firing only on deterministic syntax or boundary violations preserves native error provenance.
4. **Input-Channel Hygiene & Stateful Loop Breakers Multiply Continuity (`+42.8%` on `C`):** Budget models routinely pass unexpanded `$WORKSPACE`/`~` literals to `Read`/`Write` tools and repeat identical failing calls (`700+` turns). Blocking unexpanded shell tokens pre-flight and enforcing a 3-strike identical-failure circuit-breaker (with async status-polling exemptions) prevents context collapse.
