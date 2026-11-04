---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-11-01"
last_updated: "2026-11-04"
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
    - INT-907
    - SUB-1708
    - INT-908
    - SUB-1710
    - INT-922
title: "TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification"
trigger: always_on
type: spec
version: "4.1.1"
ssot_repository: nomos
---

# TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify the 7-pillar harness reliability framework aligned with [TraceTrial-Bench](https://github.com/TraceLite-AI/TraceTrial-Bench) ([arXiv:2610.32285](https://arxiv.org/abs/2610.32285)) and harden fail-closed isolation, configuration pinning, and crash recovery.

---

## 1. Context & Epistemology

Contemporary agent benchmarks evaluate model generation in isolation while holding the orchestration harness constant. *Trial by Trace* ([arXiv:2610.32285](https://arxiv.org/abs/2610.32285)) shows coding agents succeed or collapse across **seven pillars**: *Context*, *Execution*, *Governance*, *Lifecycle*, *Observability*, *Tools*, and *Verification*.

In Nomos, prompts remain advisory while the compiled Go substrate remains statutory. A statutory harness rejects execution fail-closed under adversarial mutation, boundary escape attempts, and runtime probe panics:

1. **Worktree Boundary Escape (Law 3):** Git hooks and `nomos hook pre-tool` block writes outside `worktrees/<repo>-<task>/` and block commits on trunk.
2. **Statutory Configuration Tampering (Law 4 & Law 7):** Task worktrees cannot relax gate thresholds, disable `strict_mode`, or disable `boundary.enforce` relative to trunk `nomos.yaml`.
3. **Gate Panic Abort (Law 2 & Law 5):** `TimedEvaluate` catches gate panics and `nomos task sync` aborts before merging or writing release receipts.

---

## 2. Seven-Pillar Statutory Mapping

| Pillar | Benchmark Dimension | Nomos Enforcement Seam |
| :--- | :--- | :--- |
| **1. Context** | Repository-scoped orientation | `SynthesizeDynamicHandshake` injects scoped wavefronts, directives, and `mneme.db` patterns. |
| **2. Execution** | Hermetic filesystem confinement | `nomos hook pre-tool` and `pre-commit` block path escapes. |
| **3. Governance** | Rubric immutability and plane separation | `PlaneSeparationGate` and `ThresholdIntegrityGate` pin `nomos.yaml` to trunk. |
| **4. Lifecycle** | Crash-safe 2PC state transitions | `TimedEvaluate` catches panics; `nomos task sync` aborts cleanly. |
| **5. Observability** | Cryptographic provenance and distillation | `nomos task sync` seals `refs/notes/nomos/releases` and distills patterns. |
| **6. Tools** | Sanctioned wrappers and zero symlinks | `nomos hook pre-tool` and `ZeroFilesystemAliasGate` block unsafe calls. |
| **7. Verification** | Multi-gate AST shortcut rejection | `AntiShimGate`, `SecretGate`, and `GateIntegrityProofGate` reject shortcuts. |

---

## 3. Fail-Closed Hardening Invariants

### 3.1 Trunk Statutory Baseline Pinning (`ThresholdIntegrityGate`)
When `nomos verify` executes inside an isolated task worktree (`ctx.WorktreePath != ""` and `filepath.Clean(ctx.WorktreePath) != filepath.Clean(ctx.RepoRoot)`):
1. **Substrate Plane Rejection:** `PlaneSeparationGate` rejects any modification to root `nomos.yaml` or `intent/` files within a `SUB-*` task (`ERR_CROSS_PLANE_MUTATION`).
2. **Intent Plane Downgrade Rejection:** Inside an `INT-*` worktree, `ThresholdIntegrityGate` loads the canonical trunk configuration from `ctx.RepoRoot`:
   - Downgrading `governance.quality_gates.strict_mode` from `true` to `false` returns `Passed: false`.
   - Downgrading `governance.boundary.enforce` from `true` to `false` returns `Passed: false`.
   - Lowering `governance.quality_gates.intent_twin_mode` rank (for example `"feat"` to `"off"`) returns `Passed: false`.
   - Relaxing statutory ceilings (`max_file_loc > 400`, `max_func_loc > 60`, `max_complexity > 15`, `min_coverage < 50.0`) rejects verification.
3. **Brownfield Root Compatibility:** Evaluating directly on a brownfield root (`ctx.WorktreePath == ""`) honors the repository owner's trunk `strict_mode` baseline.

### 3.2 Gate Runtime Panic Recovery (`TimedEvaluate`)
In `substrate/src/nomos/modules/verify/gate.go`, `TimedEvaluate(g Gate, ctx *VerifyContext)` wraps `g.Evaluate(ctx)` in a `defer recover()` boundary:
- A runtime panic `r` converts into a typed `NomosError` (`ERR_CORE`: `"gate <Name> panicked during evaluation (fail-closed abort): <r>"`).
- `Pipeline.Execute` treats any non-nil `err` from `TimedEvaluate` as an immediate abort, leaving trunk untouched.

### 3.3 Native Compiled `nomos hook` Subcommands & Container Mode (`INT-863` / `SUB-1546`)
Nomos ships zero-dependency Go subcommands (`nomos hook session-start`, `nomos hook pre-tool`, `nomos hook post-tool`) removing external `jq`, `bash`, or `python3` requirements:

1. **Unified JSON Payload Ingestion:** Each subcommand parses standard `stdin` hook payloads from Claude Code (`tool_name`, `tool_input`), Cursor, and Antigravity (`toolCall.name`, `toolCall.args`, `cwd`).
2. **PreToolUse Guards (`nomos hook pre-tool` — Law 3 & Law 7):**
   - **Unexpanded Variable Guard:** Rejects native file-edit tool calls whose target path contains unexpanded `$VAR`, `${VAR}`, or leading `~` tokens.
   - **Boundary Containment:** Standard mode rejects writes outside `/worktrees/`, `/.tmp/`, and `/brain/` with a HATEOAS remedy (`nomos task chore "<title>"`). Passing `--hermetic-dir <dir>` confines writes to `<dir>` while protecting OS roots (`/etc`, `/usr`, `/bin`, `/var`, `/proc`, `/sys`, `/dev`, `/boot`). Passing `--container-sandbox` permits container OS paths while blocking verifier and oracle roots (`/logs/verifier`, `/solution`, `/tests`).
   - **Command Safety & Encapsulation Guard:** Inspects shell tool invocations (`Bash`, `bash`, `run_command`, `execute_command`) and denies:
     - Wildcard or root recursive deletions (`rm -rf /`, `rm -rf /*`, `rm -rf *`, `rm -rf ~`, `rm -rf ..`).
     - Quote-aware `go install` of `nomos` or `nomos-*` outside a clean `IDLE` primary `main` trunk, or with `@latest` / `-overlay` / `-modfile` / `-C` flags (Law 4).
     - Direct `sqlite3` mutations or queries against `.nomos` or `.db` files (Law 1 & ADR-0062).
     - Stateful `git stash` mutations (allowing read-only `git stash list` / `git stash clear`).
     - Tampering with `.git/hooks` defense scripts (`rm`, `unlink`, `chmod`, `mv`).
3. **PostToolUse Channel Purity (`nomos hook post-tool` — Law 4):**
   - **Silent Non-Edit Channels:** Emits `{}` immediately for `Bash`, `Read`, and `MCP` calls so native OS errors (`EACCES`, `ENOENT`) remain unmasked.
   - **In-Memory Syntax & Radar Verification:** Validates edited `.go`, `.py`, `.json`, `.jsonl`, and `.yaml` files in memory without writing `.tmp/` scratch files.
4. **Additive Hook Chaining & 3-Strike Circuit-Breaker (`INT-868` / `SUB-1560`):**
   - **Additive Hook Chaining:** `nomos init` and `nomos hook` chain existing `.claude/settings.json` hooks additively instead of shadowing task-local `O` and `T` plugins.
   - **3-Strike Loop Breaker:** `nomos hook pre-tool` hashes `(tool_name, sha256(canonical_args), workspace_mtime_digest)`. Three identical consecutive calls without a `workspace_mtime_digest` mutation trigger a 1-line remediation directive halting the retry loop.

---

## 4. Deterministic Verification Vectors (`VEC-TTB-01` .. `VEC-TTB-07`)

The cleanroom suite (`substrate/cli/cleanroom_tracetrial_bench_test.go`) and unit suite (`substrate/src/nomos/modules/verify/tracetrial_hardening_test.go`) execute seven end-to-end vectors:

- **`VEC-TTB-01` (Pillar 1 — Context):** Seed two repo scopes in Mneme, run `SynthesizeDynamicHandshake`, and verify strict repository isolation plus wavefront orientation.
- **`VEC-TTB-02` (Pillar 2 — Execution / Worktree Escape):** Run `worktree-guard.sh` against trunk paths, `../../trunk.go` traversal, and direct `main` commits; verify non-zero rejection.
- **`VEC-TTB-03` (Pillar 3 — Governance / Config Tampering):**
  - *Sub-vector 3a:* Mutate `nomos.yaml` inside a `SUB-*` worktree; verify `PlaneSeparationGate` rejects the edit.
  - *Sub-vector 3b:* Flip `strict_mode: false` and `max_file_loc: 9999` inside an `INT-*` worktree; verify `ThresholdIntegrityGate` rejects the downgrade.
- **`VEC-TTB-04` (Pillar 4 — Lifecycle / Panic & 2PC Abort):** Trigger a panicking gate in `TimedEvaluate` and verify `nomos task sync` leaves trunk `HEAD` and `refs/notes/nomos/releases` untouched.
- **`VEC-TTB-05` (Pillar 5 — Observability / Receipts & Mneme):** Sync a compliant worktree and verify both the `refs/notes/nomos/releases` SHA-256 receipt and `mneme.db` pattern record.
- **`VEC-TTB-06` (Pillar 6 — Tools / Build & Symlink Guards):** Run `nomos-build-guard.sh` on a dirty tree and plant a worktree symlink; verify both reject execution.
- **`VEC-TTB-07` (Pillar 7 — Verification / Anti-Shortcut):** Inject an empty function stub (`func Shortcut() {}`) into a worktree; verify `AntiShimGate` rejects the stub.

---

## 5. External Containerized Paired Evaluation (`v4.1.0`, `n = 95`)

[TraceTrial-Bench](https://github.com/TraceLite-AI/TraceTrial-Bench) ([arXiv:2610.32285](https://arxiv.org/abs/2610.32285)) separates **harness reliability** from **model capability** across 95 containerized tasks (`190` paired Docker runs). Both arms run `Claude Code v2.1.220` with `gemini-2.5-flash-lite` (scored by `gemini-3.5-flash-lite`), comparing stock Control against `nomos-claudecode:4.1.0`:

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

1. **Majority of Agent Failures Originate in the Harness (`57.9%` Baseline):** In stock `Claude Code`, `55 / 95` failures trace to harness mechanics rather than model intelligence. Deterministic compiled gates lift budget-model deliverable output by **`+15.2%`** (`+78.3%` in Context & Continuity) without changing weights.
2. **Governance Overlays Must Chain Additively (`O` `+10.39` & `T` `+5.92` Recovery):** CLI `--settings` overrides shadow workspace hooks (`-6.51` in `v4.0.0`). Additive `nomos hook` merging flips Observability positive (`+4.17` over stock).
3. **Zero-Noise Channel Purity Beats Verbose Coaching:** Synthetic `PostToolUse` coaching prose masked native `EACCES` errors (`-39.3%` on `G`). Emitting `{}` on allow preserves native error provenance.
4. **Input Hygiene & Loop Breakers Multiply Continuity (`+42.8%` on `C`):** Budget models pass unexpanded `$WORKSPACE`/`~` paths and repeat broken calls (`700+` turns). Pre-flight variable guards and 3-strike circuit-breakers prevent context collapse.
