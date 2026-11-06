---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-11-01"
last_updated: "2026-11-04"
lineage:
  - INT-858
  - INT-863
  - INT-868
  - INT-886
  - INT-901
  - INT-907
  - INT-908
  - INT-922
  - SUB-1527
  - SUB-1546
  - SUB-1560
  - SUB-1610
  - SUB-1693
  - SUB-1696
  - SUB-1708
  - SUB-1710
plane: intent
ssot_repository: nomos
status: active
title: "TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification"
type: spec
version: "4.1.1"
---

# TraceTrial-Bench 7-Pillar Harness Reliability & Fail-Closed Stress Specification

---

## 1. Executive Summary & Problem Statement

Contemporary agent benchmarks evaluate model generation in isolation while holding the orchestration harness constant. *Trial by Trace* ([TraceTrial-Bench](https://github.com/TraceLite-AI/TraceTrial-Bench), [arXiv:2610.32285](https://arxiv.org/abs/2610.32285)) shows coding agents succeed or collapse across **seven pillars**: *Context*, *Execution*, *Governance*, *Lifecycle*, *Observability*, *Tools*, and *Verification*.

In Nomos, prompts remain advisory while the compiled Go substrate remains statutory. A statutory harness rejects execution fail-closed under adversarial mutation, boundary escape attempts, and runtime probe panics:

1. **Worktree Boundary Escape (Law 3):** Git hooks and `nomos hook pre-tool` block writes outside `worktrees/<repo>-<task>/` and block commits on trunk.
2. **Statutory Configuration Tampering (Law 4 & Law 7):** Task worktrees cannot relax gate thresholds, disable `strict_mode`, or disable `boundary.enforce` relative to trunk `nomos.yaml`.
3. **Gate Panic Abort (Law 2 & Law 5):** `TimedEvaluate` catches gate panics and `nomos task sync` aborts before merging or writing release receipts.

---

## 2. Axiomatic Invariants & Governing Laws

### 2.1 Trunk Statutory Baseline Pinning (`ThresholdIntegrityGate`)
When `nomos verify` executes inside an isolated task worktree (`ctx.WorktreePath != ""` and `filepath.Clean(ctx.WorktreePath) != filepath.Clean(ctx.RepoRoot)`):
1. **Substrate Plane Rejection:** `PlaneSeparationGate` rejects any modification to root `nomos.yaml` or `intent/` files within a `SUB-*` task (`ERR_CROSS_PLANE_MUTATION`).
2. **Intent Plane Downgrade Rejection:** Inside an `INT-*` worktree, `ThresholdIntegrityGate` loads the canonical trunk configuration from `ctx.RepoRoot`:
   - Downgrading `governance.quality_gates.strict_mode` from `true` to `false` returns `Passed: false`.
   - Downgrading `governance.boundary.enforce` from `true` to `false` returns `Passed: false`.
   - Lowering `governance.quality_gates.intent_twin_mode` rank (for example `"feat"` to `"off"`) returns `Passed: false`.
   - Relaxing statutory ceilings (`max_file_loc > 400`, `max_func_loc > 60`, `max_complexity > 15`, `min_coverage < 55.0`) rejects verification.
3. **Brownfield Root Compatibility:** Evaluating directly on a brownfield root (`ctx.WorktreePath == ""`) honors the repository owner's trunk `strict_mode` baseline.

### 2.2 Gate Runtime Panic Recovery (`TimedEvaluate`)
In `substrate/src/nomos/modules/verify/gate.go`, `TimedEvaluate(g Gate, ctx *VerifyContext)` wraps `g.Evaluate(ctx)` in a `defer recover()` boundary:
- A runtime panic `r` converts into a typed `NomosError` (`ERR_CORE`: `"gate <Name> panicked during evaluation (fail-closed abort): <r>"`).
- `Pipeline.Execute` treats any non-nil `err` from `TimedEvaluate` as an immediate abort, leaving trunk untouched.

### 2.3 Native Compiled `nomos hook` Subcommands & Container Mode (`INT-863` / `SUB-1546`)
Nomos ships zero-dependency Go subcommands (`nomos hook session-start`, `nomos hook pre-tool`, `nomos hook post-tool`) removing external `jq`, `bash`, or `python3` requirements:

1. **Unified JSON Payload Ingestion:** Each subcommand parses standard `stdin` hook payloads from Claude Code (`tool_name`, `tool_input`), Cursor, and Antigravity (`toolCall.name`, `toolCall.args`, `cwd`).
2. **PreToolUse Guards (`nomos hook pre-tool` - Law 3 & Law 7):**
   - **Unexpanded Variable Guard:** Rejects native file-edit tool calls whose target path contains unexpanded `$VAR`, `${VAR}`, or leading `~` tokens.
   - **Boundary Containment:** Standard mode rejects writes outside `/worktrees/`, `/.tmp/`, and `/brain/` with a HATEOAS remedy (`nomos task chore "<title>"`). Passing `--hermetic-dir <dir>` confines writes to `<dir>` while protecting OS roots (`/etc`, `/usr`, `/bin`, `/var`, `/proc`, `/sys`, `/dev`, `/boot`). Passing `--container-sandbox` permits container OS paths while blocking verifier and oracle roots (`/logs/verifier`, `/solution`, `/tests`).
   - **Command Safety & Encapsulation Guard:** Inspects shell tool invocations (`Bash`, `bash`, `run_command`, `execute_command`) and denies:
     - Wildcard or root recursive deletions (`rm -rf /`, `rm -rf /*`, `rm -rf *`, `rm -rf ~`, `rm -rf ..`).
     - Quote-aware `go install` of `nomos` or `nomos-*` outside a clean `IDLE` primary `main` trunk, or with `@latest` / `-overlay` / `-modfile` / `-C` flags (Law 4).
     - Direct `sqlite3` mutations or queries against `.nomos` or `.db` files under Law 1 and the Database Encapsulation Invariant.
     - Stateful `git stash` mutations (allowing read-only `git stash list` / `git stash clear`).
     - Tampering with `.git/hooks` defense scripts (`rm`, `unlink`, `chmod`, `mv`).
3. **PostToolUse Channel Purity (`nomos hook post-tool` - Law 4):**
   - **Silent Non-Edit Channels:** Emits `{}` immediately for `Bash`, `Read`, and `MCP` calls so native OS errors (`EACCES`, `ENOENT`) remain unmasked.
   - **In-Memory Syntax & Radar Verification:** Validates edited `.go`, `.py`, `.json`, `.jsonl`, and `.yaml` files in memory without writing `.tmp/` scratch files.
4. **Additive Hook Chaining & 3-Strike Circuit-Breaker (`INT-868` / `SUB-1560`):**
   - **Additive Hook Chaining:** `nomos init` and `nomos hook` chain existing `.claude/settings.json` hooks additively instead of shadowing task-local `O` and `T` plugins.
   - **3-Strike Loop Breaker:** `nomos hook pre-tool` hashes `(tool_name, sha256(canonical_args), workspace_mtime_digest)`. Three identical consecutive calls without a `workspace_mtime_digest` mutation trigger a 1-line remediation directive halting the retry loop.

---

## 3. Seven-Pillar Statutory Mapping & Go SSoT Contracts

| Pillar | Benchmark Dimension | Nomos Enforcement Seam |
| :--- | :--- | :--- |
| **1. Context** | Repository-scoped orientation | `SynthesizeDynamicHandshake` injects scoped wavefronts, directives, and `mneme.db` patterns. |
| **2. Execution** | Hermetic filesystem confinement | `nomos hook pre-tool` and `pre-commit` block path escapes. |
| **3. Governance** | Rubric immutability and plane separation | `PlaneSeparationGate` and `ThresholdIntegrityGate` pin `nomos.yaml` to trunk. |
| **4. Lifecycle** | Crash-safe 2PC state transitions | `TimedEvaluate` catches panics; `nomos task sync` aborts cleanly. |
| **5. Observability** | Cryptographic provenance and distillation | `nomos task sync` seals `refs/notes/nomos/releases` and distills patterns. |
| **6. Tools** | Sanctioned wrappers and zero symlinks | `nomos hook pre-tool` and `ZeroFilesystemAliasGate` block unsafe calls. |
| **7. Verification** | Multi-gate AST shortcut rejection | `AntiShimGate`, `SecretGate`, and `GateIntegrityProofGate` reject shortcuts. |

### 3.1 Compiled Go SSoT Hardening Contracts (`substrate/src/nomos/modules/verify/`)
`gate.go` and `gates_intent.go` compile the fail-closed evaluation wrapper and trunk baseline guard:

```go
package verify

// VerifyContext provides contextual metadata for quality gate evaluation.
type VerifyContext struct {
	RepoRoot         string `json:"repo_root"`
	WorktreePath     string `json:"worktree_path"`
	TaskID           string `json:"task_id"`
	CommitRange      string `json:"commit_range,omitempty"`
	Phase            string `json:"phase"`
	IsBaseline       bool   `json:"is_baseline"`
	AllFiles         bool   `json:"all_files,omitempty"`
	Strict           bool   `json:"strict,omitempty"`
	AdvisoryDebtMode bool   `json:"advisory_debt_mode,omitempty"`
}

// Gate defines the interface for an invariant evaluation step.
type Gate interface {
	Name() string
	Evaluate(ctx *VerifyContext) (*GateResult, error)
}

// TimedEvaluate executes a gate while measuring its elapsed duration in microseconds
// and recovering runtime panics into fail-closed ERR_CORE errors.
func TimedEvaluate(g Gate, ctx *VerifyContext) (res *GateResult, err error)

// ThresholdIntegrityGate verifies gate thresholds are never relaxed (Cell 7.3, Law 7).
type ThresholdIntegrityGate struct{}

func (g *ThresholdIntegrityGate) Evaluate(ctx *VerifyContext) (*GateResult, error)
```

---

## 4. Fail-Closed Adversarial Suite Architecture

The cleanroom suite (`substrate/cli/cleanroom_tracetrial_bench_test.go`) and unit suite (`substrate/src/nomos/modules/verify/tracetrial_hardening_test.go`) execute seven end-to-end adversarial pillars (`VEC-TTB-01` .. `VEC-TTB-07` in Go subtests):

- **Pillar 1 - Context Isolation (`VEC-TTB-01`):** Seeds two repo scopes in Mneme, runs `SynthesizeDynamicHandshake`, and verifies strict repository isolation plus wavefront orientation.
- **Pillar 2 - Worktree Escape Containment (`VEC-TTB-02`):** Runs `worktree-guard.sh` against trunk paths, `../../trunk.go` traversal, and direct `main` commits; verifies non-zero rejection.
- **Pillar 3 - Governance Config Tampering (`VEC-TTB-03`):**
  - *Sub-vector 3a:* Mutates `nomos.yaml` inside a `SUB-*` worktree; verifies `PlaneSeparationGate` rejects the edit.
  - *Sub-vector 3b:* Flips `strict_mode: false` and `max_file_loc: 9999` inside an `INT-*` worktree; verifies `ThresholdIntegrityGate` rejects the downgrade.
- **Pillar 4 - Panic & 2PC Abort (`VEC-TTB-04`):** Triggers a panicking gate in `TimedEvaluate` and verifies `nomos task sync` leaves trunk `HEAD` and `refs/notes/nomos/releases` untouched.
- **Pillar 5 - Receipts & Mneme Provenance (`VEC-TTB-05`):** Syncs a compliant worktree and verifies both the `refs/notes/nomos/releases` SHA-256 receipt and `mneme.db` pattern record.
- **Pillar 6 - Build & Symlink Guards (`VEC-TTB-06`):** Runs `nomos-build-guard.sh` on a dirty tree and plants a worktree symlink; verifies both reject execution.
- **Pillar 7 - Anti-Shortcut Verification (`VEC-TTB-07`):** Injects an empty function stub (`func Shortcut() {}`) into a worktree; verifies `AntiShimGate` rejects the stub.

---

## 5. External Containerized Paired Evaluation (`v4.1.0`, `n = 95`)

[TraceTrial-Bench](https://github.com/TraceLite-AI/TraceTrial-Bench) ([arXiv:2610.32285](https://arxiv.org/abs/2610.32285)) separates **harness reliability** from **model capability** across 95 containerized tasks (`190` paired Docker runs). Both arms run `Claude Code v2.1.220` with `gemini-2.5-flash-lite` (scored by `gemini-3.5-flash-lite`), comparing stock Control against `nomos-claudecode:4.1.0`:

| Pillar / Cohort (`n = 95`, `0` errors) | Control Overall | Nomos `v4.1.0` | Overall Δ | Agent Deliverable Axis (`0..100`) | Harness Reliability Axis (`0..100`) |
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

---

## 7. Table-Driven Verification Vectors

| Vector ID | Go Test Anchor | Invariant Evaluated | Input Condition | Expected Deterministic Verdict |
| :--- | :--- | :--- | :--- | :--- |
| `TC-TTB-01` | `Pillar1_Context_VEC_TTB_01` | Pillar 1 Context isolation | Two repo scopes seeded in `mneme.db`; `SynthesizeDynamicHandshake` called | Isolates repository memories and wavefront |
| `TC-TTB-02` | `Pillar2_Execution_WorktreeEscape_VEC_TTB_02` | Pillar 2 Worktree escape guard | `worktree-guard.sh` and `pre-tool` against trunk and `../../trunk.go` | Rejects non-worktree write and trunk commit |
| `TC-TTB-03` | `Pillar3_Governance_ConfigTamper_VEC_TTB_03` | Pillar 3 Config pinning | `SUB-*` edits `nomos.yaml`; `INT-*` flips `strict_mode: false` | `PlaneSeparationGate` and `ThresholdIntegrityGate` fail |
| `TC-TTB-04` | `Pillar4_Lifecycle_GateCrashAborts2PC_VEC_TTB_04` | Pillar 4 Panic & 2PC abort | Panicking gate inside `TimedEvaluate` during `nomos task sync` | Converts panic to `ERR_CORE`; trunk `HEAD` untouched |
| `TC-TTB-05` | `Pillar5_Observability_ReceiptAndMneme_VEC_TTB_05` | Pillar 5 Receipts & Mneme | Compliant worktree synced via `nomos task sync` | Writes `refs/notes/nomos/releases` receipt and pattern |
| `TC-TTB-06` | `Pillar6_Tools_ToolchainAndSymlinkGuard_VEC_TTB_06` | Pillar 6 Build & symlink guard | Dirty-tree `nomos` install and planted worktree symlink | Both rejected fail-closed (`ZeroFilesystemAliasGate`) |
| `TC-TTB-07` | `Pillar7_Verification_AntiShortcut_VEC_TTB_07` | Pillar 7 Anti-shortcut AST | Empty stub `func Shortcut() {}` injected in worktree | `AntiShimGate` rejects placeholder implementation |
| `TC-TTB-08` | `TestGoWorkModuleBoundary` | `go.work` module boundary | Multi-module workspace evaluated via `GoToolchainGate` and `BoundaryGate` | Resolves `go.work` roots without boundary escape |
| `TC-TTB-09` | `TestLoopBreaker3Strike` | 3-strike circuit-breaker | Three identical calls in `nomos hook pre-tool` with unchanged mtime digest | Emits 1-line remediation directive and halts loop |

---

## 8. Cross-Plane Traceability & DoD Verification

| Intent Reliability Pillar | Compiled Go SSoT Path | Enforcing 7x3 Matrix Cell |
| :--- | :--- | :--- |
| Panic-safe gate wrapper (`TimedEvaluate`) | `substrate/src/nomos/modules/verify/gate.go` | `Cell 2.3 (gate_integrity_proof)`, `Cell 7.2 (unchecked_errors)` |
| Trunk config pinning (`ThresholdIntegrityGate`) | `substrate/src/nomos/modules/verify/gates_intent.go` | `Cell 7.1 (plane_separation)`, `Cell 7.3 (threshold_integrity)` |
| Compiled hook guards & loop breaker (`nomos hook`) | `substrate/cli/cmd_hook.go` | `Cell 3.2 (hermetic_worktree)`, `Cell 6.2 (zero_filesystem_alias)` |
| Cleanroom & unit adversarial suites | `substrate/cli/cleanroom_tracetrial_bench_test.go`, `substrate/src/nomos/modules/verify/tracetrial_hardening_test.go` | `Cell 2.1 (anti_shim)`, `Cell 2.2 (go_toolchain)` |

Run `nomos verify` to execute all 42 Definition of Done quality gates and confirm exit code `0`.
