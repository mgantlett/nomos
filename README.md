<div align="center">

<img src="intent/assets/nomos_hero_emblem.jpg" alt="SophiaLabs // Nomos Unified Emblem" width="220" />

# Nomos (Νόμος)

**Agents guess; Git cannot. The deterministic execution substrate between any coding agent and Git.**

[![CI](https://github.com/mgantlett/nomos/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/mgantlett/nomos/actions/workflows/ci.yaml)
[![Release](https://img.shields.io/github/v/release/mgantlett/nomos?color=0EA5E9)](https://github.com/mgantlett/nomos/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mgantlett/nomos?filename=substrate%2Fgo.mod&logo=go&logoColor=white)](substrate/go.mod)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/Docs-sophialabs.ca-0EA5E9)](https://sophialabs.ca/docs/introduction)
[![Discussions](https://img.shields.io/github/discussions/mgantlett/nomos?color=0EA5E9&label=Discussions)](https://github.com/mgantlett/nomos/discussions)

</div>

---

AI coding agents optimize for making their prompt finish, even when that means taking shortcuts. Without a deterministic boundary, agents routinely introduce three silent failures into production repositories:

- **Reward Hacking Tests:** Deleting, skipping, or weakening failing assertions instead of fixing the root cause.
- **Hollow Shims & Stubs:** Leaving empty `// TODO` stubs, mock returns, or dead unreferenced code to make a build compile.
- **Unbounded Workspace Drift:** Mutating `main` directly, polluting unrelated modules, or forgetting architectural context across sessions.

Just as **PostgreSQL placed ACID transactions** between concurrent applications and raw disk files, **Nomos** introduces a new infrastructure layer between your coding agent (**Claude Code**, **Cursor**, **Antigravity**, **OpenCode**, **Windsurf**, or **Aider**) and your Git repository - physically enforcing **causal `INT-*`/`SUB-*` spec-vs-code DAGs**, **hermetic Git worktree sandboxes**, **42 polyglot anti-reward-hacking AST quality gates** (with trunk-pinned `nomos.yaml`), **receipt-gated SQLite memory (`mneme.db`)**, and **atomic Two-Phase Commit (2PC) releases** before a single line can touch `main`:

| What Nomos Is *Not* | Why Legacy Layers Fail on Coding Agents | What the Nomos Layer Enforces |
| :--- | :--- | :--- |
| **Not Another Coding Agent** | Switching models or IDEs does not stop a stochastic optimizer from taking shortcuts under context pressure. | Keep Claude Code, Cursor, Antigravity, OpenCode, Windsurf, or Aider; Nomos governs state isolation and merge safety underneath them. |
| **Not a Linter or Post-Hoc CI** | Linters (`golangci-lint`, `eslint`) assume cooperative humans *after* edits already dirtied your active branch. | Isolates tasks in disposable `git worktree` sandboxes, separates `INT-*` specs from `SUB-*` code so agents cannot rewrite specs to excuse broken code, pins `nomos.yaml` to `main`, and blocks deleted assertions or hollow shims *before* merge. |
| **Not a `.cursorrules` Prompt Pack** | Advisory markdown rules decay once context crosses 60%, and failed runs poison repo notes. | Enforces compiled Go, Git hook, and AST exit codes (`exit 1`) outside the model context window, promoting `mneme.db` lessons strictly after a verified 2PC Git Notes receipt. |

> **⚡ Two commands to install and initialize. Invisible after that.**
> Run `curl -fsSL https://sophialabs.ca/install | bash` and `nomos init` once. There are zero custom slash commands to memorize, zero prompt templates to maintain, and zero manual handshake steps (`nomos hook session-start` runs `nomos handshake` automatically on session boot). You prompt Claude Code, Cursor, Antigravity, or OpenCode in plain English the exact way you already do, while Nomos governs worktree isolation, `~24ms` single-file AST checks, and two-phase commits silently under the hood.

<div align="center">
  <img src="intent/assets/nomos_hero_demo.gif" alt="Nomos Live Terminal Demo - Blocking main commits, catching an empty AST stub, and releasing via 2PC" width="100%" />
</div>

---

## ⚡ Quick Start (Under 60 Seconds)

### 1. Install the `nomos` Binary

```bash
curl -fsSL https://sophialabs.ca/install | bash
```

Installs the SHA-256-verified release binary directly to standard Go binary paths (`$GOBIN` / `$(go env GOPATH)/bin` / `~/go/bin`; pass `NOMOS_VERSION=v4.16.2` to pin a specific release tag). Or install on **macOS / Linux** via Homebrew, on **Arch Linux / Omarchy** via `makepkg`, or directly from `main` via the Go toolchain (`substrate/` subdirectory module):

```bash
# macOS & Linux (Homebrew):
brew install --cask mgantlett/tap/nomos

# Arch Linux / Omarchy (verified PKGBUILD):
(tmp="$(mktemp -d)" && cd "$tmp" && curl -fsSLO https://raw.githubusercontent.com/mgantlett/nomos/main/packaging/aur/PKGBUILD && makepkg -si && rm -rf "$tmp")

# Build from source (requires Go 1.26+; GOTOOLCHAIN=auto auto-fetches Go 1.26+ on older Go installs):
GOTOOLCHAIN=auto CGO_ENABLED=0 go install github.com/mgantlett/nomos/substrate/cmd/nomos@main
```

To update an existing standalone install (verifies SHA-256 checksums, guards Homebrew/AUR installs, and optionally refreshes downstream workspace contracts), or to cleanly uninstall at any time:

```bash
# Update standalone nomos binary (and optionally refresh workspace AGENTS.md & ambient hooks):
nomos update
nomos update --sync-workspace

# Uninstall cleanly:
curl -fsSL https://sophialabs.ca/uninstall | bash
```

### 2. Initialize Any Repository

```bash
cd your-project
nomos init
```

`nomos init` detects your language stack, scaffolds [`nomos.yaml`](intent/architecture/specs/go_ssot_and_json_schema_spec.md) (validated against [`nomos.schema.json`](nomos.schema.json)), installs self-healing Git defense hooks (`pre-commit`, `commit-msg`, `pre-push`), and drops indirect-control agent contracts (`AGENTS.md`, `CLAUDE.md`) plus ambient lifecycle hooks across Claude Code, Gemini CLI, OpenCode, Cursor, and Antigravity (`.claude/settings.json`, `.gemini/settings.json`, `.opencode/plugins/nomos.ts`, `.cursor/hooks.json`, `.agents/hooks.json`) into your repo.

### 3. Prompt Your Coding Agent Naturally

You are done with the CLI. Open **Claude Code**, **Cursor**, **Antigravity**, **OpenCode**, or **Windsurf** and prompt in plain English:

```text
> "Add TTL cache eviction to Store with unit tests"
```

Because `nomos init` injected `AGENTS.md` / `CLAUDE.md` and Git defense hooks, **your coding agent autonomously executes the Nomos lifecycle under the hood**:

```bash
# Executed autonomously by your coding agent (not typed by you):
nomos task chore "Add TTL cache eviction"          # 1. Agent provisions isolated worktree (worktrees/<repo>-SUB-1)
nomos verify -w worktrees/your-project-SUB-1       # 2. Agent runs 42 AST & DoD gates (self-correcting any failures)
nomos task sync SUB-1 -F .tmp/walkthrough.md       # 3. Agent atomically merges to main with a Git Notes audit receipt
```

---

## 🏗️ How It Works in Your Repository

<div align="center">
  <img src="intent/assets/nomos_hero_blueprint_v1_5.jpg" alt="Nomos v1.5 Downstream Closed-Loop Lifecycle Architecture" width="100%" />
</div>

1. **Indirect Control & Receipt-Gated Episodic Memory (`nomos init` / `nomos hook` / `nomos mem`):** Your coding agent reads the bootstrapped `AGENTS.md` / `CLAUDE.md` protocol, runs sub-5ms compiled lifecycle hooks (`nomos hook session-start`, `pre-tool`, `post-tool`, `stop`, and zero-ceremony `nomos hook install` across Claude Code, Gemini CLI, OpenCode, Cursor, and Antigravity), and queries **Nomos Mneme** (`nomos mem recall`) for sub-50ms receipt-gated episodic memory (AST directory-scoped pitfalls/patterns + pure-Go subword feature hashing, lexical ranking, and optional local neural embeddings) before touching code.
2. **Hermetic Worktree Sandbox & Causal Plane DAG (`nomos task`):** Direct commits on `main` fail closed. Tasks separate intent/contract specs (`INT-*`) from substrate code (`SUB-*`) in a local causal DAG (`nomos task pair`, `nomos task chore`, `nomos task dag`), and every task executes inside an isolated Git worktree (`worktrees/<repo>-<task>`) governed by a strict state machine (`IDLE → PLAN → EDIT → REVIEW → SYNC → LEARN`).
3. **42-Gate Polyglot AST & Toolchain Verification (`nomos verify`):** Before any code can be reviewed or merged, Nomos evaluates `go/ast`, embedded `.scm` S-expression query rules, `wazero` WebAssembly linear-memory scanning, pure-Go multiline-aware structural scanners, and native compiler/coverage probes (`tsc --noEmit`, `pytest`, `cargo check`, `ruby -c`, `cmake`, `gradle`/`mvn`, `dotnet build`, plus LCOV and Cobertura XML ratchets) across **Go, TypeScript/Svelte, Python, Rust, Ruby, C/C++, Java, Kotlin, and C#** - catching empty stubs, `todo!()`/`pass`/`raise NotImplementedError` shims, skipped-test annotations (`t.Skip`, `@pytest.mark.skip`, `#[ignore]`, `xit`, `@Disabled`, `[Fact(Skip=...)]`), functions over `60 LOC` (`400 LOC` file ceiling), cyclomatic complexity spikes (`> 15`), hardcoded paths, security flaws, and test coverage drops.
4. **1-Turn Auto-Remediation Loop (`NomosError`):** When a gate fails, `nomos verify` emits a structured [`NomosError`](intent/architecture/specs/go_ssot_and_json_schema_spec.md) with the exact file, line, violated law, and remediation instruction so the coding agent fixes its own mistake on the next turn.
5. **Physically Locked Trunk & Atomic 2PC Release (`nomos task sync`):** Once all 42 gates pass, Nomos fast-forward merges the worktree into `main`, attaches a cryptographic **Git Notes (`refs/notes/nomos/releases`)** verification receipt, and prunes the sandbox.

---

## 🧩 Core Capabilities & Deep-Dive Specifications

| Capability | Canonical CLI Surface | What It Enforces | Architecture Specification |
| :--- | :--- | :--- | :--- |
| **Indirect Agent Control** | `nomos init`, `nomos handshake`, `nomos hook` | Bootstraps agent rules (`AGENTS.md`, `CLAUDE.md`), user/project hook installation (`nomos hook install --claude \| --gemini \| --opencode \| --cursor \| --antigravity \| --user`), additive lifecycle hooks (`nomos hook session-start \| pre-tool \| post-tool \| stop` at `~4ms` cold start with pure-Go syntax radar, anti-phantom `Stop` verification, and 3-strike + no-edit test-rerun circuit-breaking), and self-healing Git hooks without locking you into a specific model or IDE. | [`intent.md`](intent/intent.md) |
| **Capability-Flow, Host-Credential Airlock & Zero-Trust MCP IFC** | `nomos hook pre-tool \| session-start` | Tracks multi-turn compositional taint (`CapabilityFlowGraph`) across web ingress, local credentials, and release/exfiltration sinks; blocks host credential reads (`~/.ssh`, `~/.aws`, `~/.kube`, `~/.gnupg`, `~/.config/gcloud`, `.netrc`, `.npmrc`, `.pypirc` + `blocked_read_paths`) via `credential_read_mode`; blocks cloud metadata SSRF (`169.254.169.254`), credential/email leaks, and disallowed domains via `BoundaryConfig` (`egress_mode`, `allowed_egress_domains`, `blocked_egress_domains`); and enforces zero-trust MCP server allow/block lists. | [`nomos_hermetic_worktree_spec.md`](intent/architecture/specs/nomos_hermetic_worktree_spec.md) |
| **Phase Discipline & Worktrees** | `nomos task chore \| pair \| create \| start \| edit \| review \| dag` | Separates `INT-*` spec tasks from `SUB-*` code tasks in a causal DAG wavefront and enforces `IDLE → PLAN → EDIT → REVIEW → SYNC → LEARN` state transitions inside hermetic `worktrees/<repo>-<task>` sandboxes. | [`nomos_hermetic_worktree_spec.md`](intent/architecture/specs/nomos_hermetic_worktree_spec.md) |
| **42-Gate AST & DoD Engine** | `nomos verify`, `nomos verify proof` | Runs 42 statutory Definition of Done (DoD) quality gates across the 7x3 Canonical Invariant Matrix (21 Cells) via Go AST, `.scm` S-expression query rules, `wazero` WASM linear-memory scanning, polyglot structural scanners, and native toolchain/coverage probes (`tsc`, `pytest`, `cargo`, `cmake`, `gradle`/`mvn`, `dotnet`). | [`tree_sitter_ast_gate_spec.md`](intent/architecture/specs/tree_sitter_ast_gate_spec.md) · [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md) |
| **Polyglot Architectural Obligation Proofs** | `nomos verify`, `nomos handshake`, `nomos status` | Enforces `<5ms` declarative hexagonal `architecture_boundaries` across 11 languages, polyglot fenced spec contract signature parity (`strict_types`), executable `TC-*` test obligation binding (`verify_obligations`), and cryptographic `spec_digests` SHA-256 taint alerts when upstream specs evolve. | [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md) |
| **Auto-Remediation & NCTE-100 Directives** | `nomos directive`, `nomos directive --lint \| --lexicon` | Translates gate failures into deterministic JSON/terminal `NomosError` payloads for 1-turn agent self-correction and enforces `NCTE-100` Controlled Technical English across specifications and directives. | [`nomos_controlled_technical_english_spec.md`](intent/architecture/specs/nomos_controlled_technical_english_spec.md) |
| **Nomos Mneme Episodic Memory** | `nomos mem save \| recall \| list \| dismiss` | Pure-Go SQLite WAL receipt-gated episodic store (`mneme.db`) combining AST directory-scoped pitfall/pattern injection with 64-dim subword feature hashing, lexical ranking, and optional local neural embeddings (`LOCAL_EMBEDDING_URL`) scoped per repository (`repo_id`). | [`mneme_memory_spec.md`](intent/architecture/specs/mneme_memory_spec.md) |
| **Atomic 2PC Sync & Git Notes** | `nomos task sync`, `nomos verify proof` | Two-Phase Commit release to `main` with mandatory walkthrough synthesis and immutable `refs/notes/nomos/releases` provenance receipts. | [`task_lifecycle_cli_spec.md`](intent/architecture/specs/task_lifecycle_cli_spec.md) |
| **Git Hooks & Sanitized Issues** | `nomos issue` | Self-healing pre-commit/commit-msg/pre-push defense hooks and zero-leak sanitized diagnostic issue reporting. | [`nomos_hermetic_worktree_spec.md`](intent/architecture/specs/nomos_hermetic_worktree_spec.md) |
| **Complete CLI & Config Reference** | `nomos --help` | Full flag-by-flag reference for all commands and Draft 2020-12 `nomos.schema.json` configuration contracts. | [`software_physics_engine_spec.md`](intent/architecture/specs/software_physics_engine_spec.md) · [`go_ssot_and_json_schema_spec.md`](intent/architecture/specs/go_ssot_and_json_schema_spec.md) |

### Scope of Proof & Day-2 Brownfield Ergonomics

- **Structural & Trace Obligations (Not Formal Semantic Theorem Proving):** Nomos does not run an LLM-as-a-Judge critic or require formal theorem provers (`Coq`, `Lean`, `TLA+`). Its `<5ms` obligation checks verify **compiled structural and traceability invariants** across 11 languages: directional import boundaries (`architecture_boundaries`), exported contract field parity (`strict_types`), anti-shim/anti-skip AST hygiene, executable `TC-*` test bindings (`verify_obligations`), and SHA-256 specification taint (`spec_digests`). Dynamic runtime behavior, cross-language FFI semantics, and domain business logic remain governed by your actual test suite.
- **2-Tier Enforcement & Threat Model Boundary:**
  1. **Ambient Lifecycle + Git Hooks (`Claude Code`, `Gemini CLI`, `OpenCode`, `Cursor`, `Antigravity`):** Compiled `~4ms` `pre-tool`/`post-tool`/`stop` hooks intercept tool execution, `--no-verify` bypass flags, and direct trunk writes alongside self-healing Git hooks.
  2. **Git-Hook Mode & Container Sandbox (`Windsurf`, `Aider`, or Untrusted Workloads):** Harnesses without native pre-tool hooks enforce `AGENTS.md` + `pre-commit`/`commit-msg`/`pre-push` gates out of the box; pair with `--container-sandbox` (OCI container / separate host UID) when hard OS process isolation is required.
- **Day-2 Escape Hatches (Zero Brownfield Lock-In):**
  1. **Diff-Scoped Monotonic Ratchet (`baseline_commit`):** Existing `200k+ LOC` legacy files are never required to pass greenfield LOC or complexity ceilings upfront. Nomos evaluates structural regressions strictly on files touched by the active worktree diff against `main`.
  2. **Trunk-Pinned Carve-Outs vs. Blocked Agent Shims:** Agents are blocked from adding inline linter suppression comments inside worktrees to reward-hack a gate. Human engineers declare legitimate adapter exceptions (`allow_only`, `forbid_imports`, `exclude`) in `nomos.yaml` on `main`, protected by `ConfigImmutabilityGate`.
  3. **Progressive Spec Evolution (`nomos task pair` + `spec_digests`):** Updating a Layer 2 domain contract alongside a `SUB-*` implementation stamps a fresh SHA-256 receipt in one workflow; if an `INT-*` spec changes independently, Turn-0 `nomos handshake` surfaces the exact stale `spec_digests` delta instead of silently drifting.
  4. **Bounded 3-Strike Circuit-Breaker:** If a smaller local model or misconfigured rule fails verification 3 consecutive turns without progress, `nomos hook` halts the loop cleanly rather than burning context tokens.

---

## 🛡️ The 7 Laws of Nomos

Every check in `nomos verify` maps to one of seven compiled architectural laws (full 42-gate matrix in [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md)):

| Law | Name | What It Prevents |
| :---: | :--- | :--- |
| **1** | **Dual-Core Inversion** | Prevents unverified agent execution outside the phase state machine (`PhaseDisciplineGate`). |
| **2** | **Non-Verbal Proof** | Rejects conversational "tests pass" claims, `// TODO` shims, and empty function stubs (`AntiShimGate`). |
| **3** | **Hermetic Worktree** | Physically blocks direct commits on `main` and path escapes outside the task worktree (`HermeticWorktreeGate`, `StagedBoundariesGate`). |
| **4** | **Single Source of Truth** | Eliminates broken documentation links, orphaned specs, and unaligned schemas (`BrokenWireGate`, `ContractParityGate`). |
| **5** | **Zero Hardcoded Paths** | Blocks machine-specific absolute paths (`/Users/...`, `/home/...`) and secret literals (`ForbiddenLiteralsGate`). |
| **6** | **Canonical Singularity** | Prohibits duplicate CLI aliases, rogue task keys, and filesystem symlinks (`ZeroAliasGate`, `ZeroFilesystemAliasGate`). |
| **7** | **Fail-Closed Pre-Flight** | Enforces strict plane separation, test coverage floors, static analysis, and vulnerability checks (`PlaneSeparationGate`). |

---

## 📊 Verified Benchmarks & Runtime Footprint

Nomos is engineered so deterministic governance adds sub-second overhead on every agent turn while failing closed under adversarial stress (see [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md)):

> **🔬 Reproduce locally in one command:** Run `go -C substrate test ./cli -run "TestTraceTrialBench|TestCleanroomPolyglotObligation|TestCleanroomDownstreamVectors" -v` to execute the adversarial fail-closed, polyglot obligation, and 2PC lifecycle harnesses directly on your machine, or inspect the public third-party Harbor Hub trajectories below.

| Engineering Metric | Verified Result | Reproducible Proof / Implementation |
| :--- | :--- | :--- |
| **42-Gate Structural & AST Overhead** | **`~460 ms` (`~24 ms` `--fast-file`)** | `nomos verify` structural/AST gate pass across Go/TS/Svelte/Py/Rust/C/JVM/C# (plus project `go test`/`tsc`/`pytest`/`cargo`/`gradle`/`dotnet` runtime) |
| **Static Binary Footprint** | **`~20 MB` stripped (`CGO_ENABLED=0`)** | Single Go release binary (`-ldflags="-s -w"`) with embedded `modernc.org/sqlite` WAL + `wazero` WASM runtime host; zero host runtime deps |
| **Adversarial Fail-Closed Suite (`VEC-TTB-01..07`)** | **`7/7` vectors blocked in `1.98s`** | [`cleanroom_tracetrial_bench_test.go`](substrate/cli/cleanroom_tracetrial_bench_test.go) (`go -C substrate test ./cli -run TestTraceTrialBench -v`) |
| **Multi-Lab Polyglot Brownfield Audit (`N = 108`)** | **`86.1%` (`31/36`) vs. `27.8%` (`10/36` Stock; `0%` layer purity)** | `108` paired Python/TS/Go brownfield episodes across `14B-32B` (`Qwen`, `Mistral`, local `16GB VRAM`) + [`cleanroom_obligation_polyglot_stress_test.go`](substrate/cli/cleanroom_obligation_polyglot_stress_test.go) |
| **Config & Boundary Tamper Resistance** | **`0` trunk leaks** | Worktree edits to `nomos.yaml` fail closed against pinned `main`; mid-2PC gate panics abort before `main` fast-forwards |
| **Nomos Mneme Episodic Recall** | **`< 50 ms` local query** | 64-dim subword feature-hash + lexical SQLite recall (`mneme.db`, plus optional `LOCAL_EMBEDDING_URL`); mints `pattern`/`pitfall` records strictly after verified 2PC `Git Notes` release |
| **Open-Core Automated Test Suite** | **`860+` test cases across `16` packages + `8` cleanroom suites** | Pure Apache 2.0 local test execution with zero cloud or telemetry dependencies |

### Terminal-Bench 2.0 Controlled A/B ([`Harbor`](https://github.com/harbor-framework/harbor) · [`nomos-harbor`](https://github.com/mgantlett/nomos-harbor) · `89 / 89` Tasks)

Across all `89` containerized tasks in [`terminal-bench@2.0`](https://github.com/laude-institute/terminal-bench-2) holding `Gemini CLI 0.67.0`, `vertex_ai/gemini-3.5-flash`, and unmodified task timeouts fixed, adding compiled `nomos hook` pre-tool/post-tool governance lifted single-shot `pass@1` from **`65.17%` (`58 / 89`)** to **`70.79%` (`63 / 89`, `+5.62` pp, `0` API crashes)**, and **`84.27%` (`75 / 89`, `+19.10` pp, `pass<=2`)** when resuming the `26` failed trials with `nomos v4.7.0` (`--budget-sec` wall-clock advisories + interactive/binary stdout guards, cutting timeouts from `24` to `13`):

| Arm | Agent Configuration | Score (`89` Tasks) | Lift vs. Control | Timeouts | Public Harbor Hub Trajectories |
| :--- | :--- | :--- | :--- | :---: | :--- |
| **Pure Stock Control (`pass@1`)** | Stock `Gemini CLI 0.67.0` (zero Nomos hooks) | **`58 / 89` (`65.17%`)** | Baseline | `24` | [`49f21995-f52a-4f33-837e-a3f86bbfadd0`](https://hub.harborframework.com/jobs/49f21995-f52a-4f33-837e-a3f86bbfadd0) |
| **Nomos Single-Shot (`pass@1`)** | `Gemini CLI 0.67.0` + `nomos hook` (`--container-sandbox`) | **`63 / 89` (`70.79%`)** | **`+5.62` pp** (`14W / 9L`) | `16` | [`39059e9a-8236-472a-a43b-ab500146abe1`](https://hub.harborframework.com/jobs/39059e9a-8236-472a-a43b-ab500146abe1) |
| **Nomos `v4.7.0` Resume (`pass<=2`)** | `v4.7.0` Budget Radar + interactive/binary guards resumed on `26` `0.0` trials | **`75 / 89` (`84.27%`)** | **`+19.10` pp** (`20W / 3L`) | `13` | [`6b9f81a5-0750-4242-80f2-0b5d0830355e`](https://hub.harborframework.com/jobs/6b9f81a5-0750-4242-80f2-0b5d0830355e) |

### External Harness Reliability ([`TraceTrial-Bench`](https://github.com/TraceLite-AI/TraceTrial-Bench) · [`arXiv:2610.32285`](https://arxiv.org/abs/2610.32285))

Across all 95 containerized tasks in [*Trial by Trace* (`TraceTrial-Bench`, `arXiv:2610.32285`)](https://arxiv.org/abs/2610.32285) (`190` paired Docker runs holding `Claude Code v2.1.220` and `Gemini 2.5 Flash-Lite` fixed), adding compiled `~4ms` `nomos hook` governance lifted **Agent deliverables by `+15.2%`** (`+78.3%` in Context & Continuity), lifted **Harness reliability by `+2.51`** (`44.89` → **`48.78`** Overall, `36W / 30T / 29L`), and cut **`harness_caused` failures by `-18.2%`** (`55` → `45`). Reproduce the 7-pillar fail-closed stress vectors locally via [`cleanroom_tracetrial_bench_test.go`](substrate/cli/cleanroom_tracetrial_bench_test.go) (`go -C substrate test ./cli -run TestTraceTrialBench -v`).

### Multi-Lab Legacy-GPU & Open-Weight Brownfield Audit (`N = 108` Paired Episodes · `8B` to `32B`)

Across `108` paired zero-hint brownfield episodes (`54` Stock vs. `54` Nomos `v4.13.1`, `temp=0.0`) spanning Python (`py-hex-01`, `py-debt-02`), TypeScript (`ts-hex-01`, `ts-debt-02`), and Go (`go-hex-01`, `go-debt-02`) on `6` open-weight models from `4` labs sized for single-card `16GB-24GB` GPUs (`RTX 4080`, `RTX 3090`, `A10G`, `L4`), deterministic AST & obligation ratchets lifted verified resolution on the `14B-32B` production tier from **`27.8%` (`10 / 36`, `0%` hexagonal layer purity)** to **`86.1%` (`31 / 36`, `+58.3` pp, `100%` layer purity on all merged diffs)** while cutting token burn **`-43%` to `-57%`** on multi-file refactors (polyglot boundary and obligation checks reproducible via [`cleanroom_obligation_polyglot_stress_test.go`](substrate/cli/cleanroom_obligation_polyglot_stress_test.go)):

| Open-Weight Model (`Lab · VRAM Class`) | Stock Verified (`9` Tasks) | Stock Hexagonal Layer Purity | Nomos `v4.13.1` Verified (`9` Tasks) | Nomos Layer Purity | Governance Delta |
| :--- | :---: | :---: | :---: | :---: | :--- |
| **`Llama 3.1 8B Instruct`** (`Meta · 8GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`33.3%` (`3/9`)** | `33.3%` (`1/3`) | **`+11.1` pp** *(Fail-closed capacity floor)* |
| **`Gemma 3 12B IT`** (`Google · 12GB`) | `33.3%` (`3/9`) | `0.0%` (`0/3`) | **`66.7%` (`6/9`)** | **`100%` (`3/3`)** | **`+33.3` pp** (`+100%` relative) |
| **`Qwen 3 14B`** (`Alibaba · 12-16GB`) | `33.3%` (`3/9`) | `0.0%` (`0/3`) | **`77.8%` (`7/9`)** | **`100%` (`3/3`)** | **`+44.4` pp** (`+133%` relative) |
| **`Mistral Small 3.2 24B`** (`Mistral · 16-24GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`88.9%` (`8/9`)** | **`100%` (`3/3`)** | **`+66.7` pp** (`4.0x` lift) |
| **`Qwen 3.8 27B IQ3_M`** (`Local RTX 4080 16GB`) | `50.0%` (`3/6`) | `0.0%` (`0/3`) | **`100.0%` (`6/6`)** | **`100%` (`3/3`)** | **`+50.0` pp** (`-43%` to `-57%` tokens) |
| **`Qwen 3 32B`** (`Alibaba · 24GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`88.9%` (`8/9`)** | **`100%` (`3/3`)** | **`+66.7` pp** (`4.0x` lift) |

---

## 🧭 Open-Core Boundary

Nomos Community Edition is **100% open-source under Apache 2.0** with zero telemetry, zero cloud account requirements, and zero crippled CLI commands. Every single-developer and single-repository governance capability runs locally out of the box (see [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and [`intent/intent.md`](intent/intent.md)):

| Capability Layer | Nomos Community Edition (Apache 2.0) | SophiaLabs Sovereign Enterprise |
| :--- | :--- | :--- |
| **Indirect Control for Tier 1 Coding Agents** (`AGENTS.md`, `CLAUDE.md`, `.claude/settings.json`, `.gemini/settings.json`, `.opencode/plugins/nomos.ts`, `.cursor/hooks.json`, `.agents/hooks.json`) | ✅ Included | ✅ Included |
| **42-Gate Polyglot Verification Engine** (`nomos verify`, Go AST + Polyglot Scanners, 7 Laws + DoD) | ✅ Included | ✅ Included |
| **Causal `INT-*`/`SUB-*` DAG, Hermetic Worktrees & 2PC Sync** (`nomos task`, trunk-pinned `nomos.yaml`, `refs/notes/nomos/releases`) | ✅ Included | ✅ Included |
| **Nomos Mneme Receipt-Gated Episodic Memory & Sanitized Issues** (`nomos mem`, `nomos issue`) | ✅ Included | ✅ Included |
| **Multi-Repository Causal DAGs & Cross-Team Fleet Governance** | Not Included | 🔒 Enterprise Only |
| **Organization Control Plane, Centralized Audit Ledger & Hardware Attestation** | Not Included | 🔒 Enterprise Only |

---

## 🤝 Community & License

- **Community & Governance:** Join [GitHub Discussions](https://github.com/mgantlett/nomos/discussions) for setup Q&A, `nomos.yaml` tuning, and early ideas, see [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) for contribution guidelines, and see [`SECURITY.md`](SECURITY.md) for responsible vulnerability disclosure.
- **License:** Licensed under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) for full text and attribution terms.

<div align="center">
  <sub>Built with deterministic rigor by <a href="https://sophialabs.ca"><b>SophiaLabs</b></a> · Governed by Nomos</sub>
</div>
