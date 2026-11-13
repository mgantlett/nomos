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

Coding agents optimize for making their prompt finish, even when that means deleting failing tests, returning empty `// TODO` stubs, editing linter configs, or stuffing new complexity into fragile legacy files.

**Nomos** is a single `~20 MB` static Go binary (`Apache 2.0`, zero cloud account, zero telemetry) that sits between your coding agent (**Claude Code**, **Cursor**, **OpenCode**, **Gemini CLI**, **Antigravity**, **Windsurf**, or **Aider**) and Git. Just as PostgreSQL placed ACID transactions between applications and raw disk, Nomos places **human-authored repository laws underneath machine speed** so nothing touches `main` unless it passes isolated `git worktree` sandboxes and compiled AST verification.

```bash
curl -fsSL https://sophialabs.ca/install | bash
cd your-project && nomos init
```

> **Two commands to install. Invisible after that.** You keep prompting your coding agent in plain English with zero custom slash commands or prompt templates to maintain, while Nomos governs sandboxes, `~24ms` AST checks, and atomic merges silently under the hood.

<div align="center">
  <img src="intent/assets/nomos_hero_demo.gif" alt="Nomos Live Terminal Demo - Blocking main commits, catching an empty AST stub, and releasing via 2PC" width="100%" />
</div>

---

## ✨ What's Included Out of the Box (100% Apache 2.0 Open-Core)

Every single-repository governance capability ships locally in the open-source `nomos` binary with zero crippled commands:

### 1. Zero-Migration Brownfield Debt Ratchet (`nomos init`)
- **Drop Into `200k+ LOC` Legacy Repos on Day 1:** Auto-detects existing brownfield repositories (`strict_mode: false`) with **zero upfront warning cleanup** required.
- **Monotonic Merge-Base Ratchet:** Untouched `10,000`-line legacy files pass automatically, while **every file an agent touches** is ratcheted against `main` (`<= 0` added cyclomatic/cognitive debt, `>= 0` test coverage delta, `0` new structural clones or swallowed errors).
- **Tamper-Proof Human Exceptions:** Agents are blocked from adding inline linter-ignore comments or editing `nomos.yaml` inside a task branch; human architects declare legitimate adapter carve-outs (`allow_only`, `forbid_imports`, `exclude`) directly on protected `main`.

### 2. Hermetic Git Worktrees & Constitutional `main` (`nomos task`)
- **Physically Read-Only `main`:** Direct agent commits to `main` fail closed; tasks execute inside disposable `worktrees/<repo>-<task>` sandboxes (`nomos task chore`, `nomos task pair`, `nomos task dag`).
- **Spec-vs-Code Separation:** Separates human contracts (`INT-*`) from implementation (`SUB-*`) across a `PLAN -> EDIT -> REVIEW -> SYNC -> LEARN` state machine so an agent cannot rewrite a spec to excuse broken code.
- **Atomic Two-Phase Commit & Audit Receipts:** `nomos task sync` verifies all gates, fast-forwards `main`, stamps a tamper-evident SHA-256 **Git Notes** receipt (`refs/notes/nomos/releases`), and prunes the worktree.

### 3. 42 Polyglot Anti-Cheat AST & Obligation Gates (`nomos verify`)
- **11 Languages in `~460ms` (`~24ms` Single-File Radar, `0` LLM Judge Tokens):** Built-in AST, `.scm` query, `wazero` WASM, and native toolchain/coverage probes across **Go, TypeScript/Svelte, Python, Rust, Ruby, C/C++, Java, Kotlin, and C#**.
- **Anti-Reward-Hacking Checks:** Blocks deleted or skipped tests (`t.Skip`, `@pytest.mark.skip`, `#[ignore]`, `xit`, `@Disabled`, `[Fact(Skip=...)]`), hollow stubs (`todo!()`, `pass`, `NotImplementedError`), functions `> 60 LOC` (`400 LOC` file ceiling), cyclomatic complexity `> 15`, and hardcoded local paths or secrets.
- **Sub-5ms Architectural Obligations:** Enforces directional hexagonal `architecture_boundaries`, exported contract field parity (`strict_types`), `TC-*` test-to-spec bindings (`verify_obligations`), and SHA-256 `spec_digests` staleness alerts.
- **1-Turn Self-Correction (`NomosError`):** Gate failures emit exact file, line, violated law, and remediation instructions so agents fix issues on the next turn without thrashing.

### 4. `~4ms` Pre-Tool Security Airlock & Loop Circuit-Breaker (`nomos hook`)
- **Host Credential & SSRF Firewall:** Blocks agent reads of `~/.ssh`, `~/.aws`, `~/.kube`, `~/.gnupg`, `~/.config/gcloud`, `.netrc`, `.npmrc`, and `.pypirc`, blocks cloud metadata SSRF (`169.254.169.254`), and tracks multi-turn compositional taint (`CapabilityFlowGraph`) across web ingress and egress/MCP tools.
- **Anti-Phantom Stop & 3-Strike Circuit-Breaker:** Prevents agents from claiming "tests pass" without a verified exit code and halts 3 consecutive non-progressing failures before burning context budgets.

### 5. Receipt-Gated Episodic Memory (`nomos mem` / `mneme.db`)
- **Zero-Cloud Local SQLite Recall (`<50ms`):** Pure-Go SQLite WAL store combining AST directory-scoped pitfalls/patterns with 64-dim subword hashing, lexical ranking, and optional local neural embeddings (`LOCAL_EMBEDDING_URL`).
- **Zero Poisoned Notes:** Promotes `pitfall` and `pattern` lessons strictly after a verified 2PC release receipt so failed runs never pollute repository memory.

### 6. Drop-In Hooks Across Every Major Coding Agent (`nomos hook install`)
- **Works With Your Existing Stack:** Auto-scaffolds `AGENTS.md`, `CLAUDE.md`, self-healing Git hooks (`pre-commit`, `commit-msg`, `pre-push`), and native lifecycle hooks for **Claude Code** (`.claude/settings.json`), **Cursor** (`.cursor/hooks.json`), **OpenCode** (`.opencode/plugins/nomos.ts`), **Gemini CLI** (`.gemini/settings.json`), and **Antigravity** (`.agents/hooks.json`), plus `--container-sandbox` for Docker/CI workloads.

---

## ⚡ Quick Start (Under 60 Seconds)

### 1. Install `nomos`

```bash
# Linux, macOS & WSL2 (SHA-256 verified release binary; pin with NOMOS_VERSION=v4.18.0):
curl -fsSL https://sophialabs.ca/install | bash

# macOS & Linux (Homebrew):
brew install --cask mgantlett/tap/nomos

# Arch Linux / Omarchy (verified PKGBUILD):
(tmp="$(mktemp -d)" && cd "$tmp" && curl -fsSLO https://raw.githubusercontent.com/mgantlett/nomos/main/packaging/aur/PKGBUILD && makepkg -si && rm -rf "$tmp")

# Build from source (Go 1.26+; GOTOOLCHAIN=auto fetches Go 1.26+ automatically):
GOTOOLCHAIN=auto CGO_ENABLED=0 go install github.com/mgantlett/nomos/substrate/cmd/nomos@main
```

Update or cleanly uninstall anytime:

```bash
nomos update && nomos update --sync-workspace
curl -fsSL https://sophialabs.ca/uninstall | bash
```

### 2. Initialize Any Greenfield or Brownfield Repository

```bash
cd your-project
nomos init
```

### 3. Prompt Your Agent in Plain English

Open **Claude Code**, **Cursor**, **OpenCode**, **Gemini CLI**, **Antigravity**, or **Windsurf** and ask for a feature normally:

```text
> "Add TTL cache eviction to Store with unit tests"
```

Your agent autonomously executes the sandboxed lifecycle under the hood (in as few as 2 CLI calls on small chores):

```bash
nomos task chore "Add TTL cache eviction"                              # 1. Opens isolated worktree (worktrees/<repo>-SUB-1)
nomos task sync SUB-1 -m "Add TTL cache eviction and unit coverage"    # 2. Verifies 42 gates, merges to main & seals Git Note
```

> **💡 Cheap Workhorse Stack (`OpenCode` + `OpenRouter` / Local `16GB` GPU):** Because `.opencode/plugins/nomos.ts` enforces frontier merge rules in local compiled Go (`0` middleman token tax), `14B-32B` open-weight models jump from `27.8%` to **`86.1%`** brownfield resolution (`3.1x` lift) while cutting token spend `-43%` to `-57%`.

---

## 🧩 Why Nomos Is a New Layer of the Stack

| What Nomos Is *Not* | Why Existing Tools Leave `main` Exposed | What the Nomos Transaction Layer Adds |
| :--- | :--- | :--- |
| **Not Another Coding Agent** | Switching models or IDEs does not stop a stochastic optimizer from cutting corners at 70% context. | Keep your favorite agent; Nomos enforces sandbox isolation and merge rules underneath it. |
| **Not a Linter or Post-Hoc CI** | Traditional linters assume honest humans *after* a branch is dirty; agents edit configs or add ignore comments. | Evaluates isolated `git worktree` diffs against `main` while locking `nomos.yaml`, specs, and assertions outside agent write scope. |
| **Not a `.cursorrules` Prompt Pack** | Markdown rules decay past 60% context, and unverified agent notes poison future sessions. | Enforces compiled Go exit codes (`exit 1`) outside context windows and records `mneme.db` lessons only after verified merges. |

---

## 📊 Verified Benchmarks & Runtime Footprint

> **🔬 Reproduce locally in one command:** `go -C substrate test ./cli -run "TestTraceTrialBench|TestCleanroomPolyglotObligation|TestCleanroomDownstreamVectors" -v`

| Engineering Metric | Verified Result | Proof / Suite |
| :--- | :--- | :--- |
| **Workhorse Brownfield Lift (`N = 108` Paired)** | **`86.1%` (`31/36`) vs. `27.8%` Stock (`3.1x` lift) · `-43%` to `-57%` Tokens** | `14B-32B` models across Python/TS/Go; `100%` hexagonal purity vs. `0%` Stock ([`cleanroom_obligation_polyglot_stress_test.go`](substrate/cli/cleanroom_obligation_polyglot_stress_test.go)) |
| **Prompt-Injection & Security (`1,481` Traces)** | **`0.00%` ASR (`0/949` `AgentDojo` · `0/200` `SecBench-Live`) · `85.04%` UA** | Official `AgentDojo v1.2.2` (`-0.53 pp` benign delta) + 4-model `SecBench-Live 1.0` (`-2.6%` turns, [Dispatch D-11](https://sophialabs.ca/dispatches/paired-security-and-brownfield-ablation-v417)) |
| **`233k`-LOC Mature OSS Brownfield Ratchet** | **`6/6` `42`-Gate Clean · `2x` Debt-Free · `-14.7%` Tokens** | Zero-hint refactors across `sqlglot` (`152k` LOC), `commander.js`, `go-chi`, and `BurntSushi/toml` (blocked `+26` hidden complexity debt) |
| **`Terminal-Bench 2.0` (`k=5` · `445` Docker Runs)** | **`76.40%` `Pass@1` (`340/445`) · `92.13%` `Pass@5` (`82/89` Tasks)** | Lightweight `Gemini Flash` across `89` tasks (`91.20%` cached tokens, [`nomos-harbor`](https://github.com/mgantlett/nomos-harbor)) |
| **Footprint & Gate Latency** | **`~20 MB` binary (`CGO_ENABLED=0`) · `~4ms` hook · `~24ms` `--fast-file`** | `860+` automated tests across `16` packages + `7/7` adversarial fail-closed vectors ([`cleanroom_tracetrial_bench_test.go`](substrate/cli/cleanroom_tracetrial_bench_test.go)) |

<details>
<summary><b>🔬 Expand Full Per-Model Benchmark Tables (108-Run GPU Matrix, 1,541-Run Security/OSS Audit, Harbor & TraceTrial-Bench)</b></summary>

<br />

#### 1. Open-Weight & Commodity Workhorse Brownfield Lift (`N = 108` Paired Episodes · `8B` to `32B`)

Across `108` paired zero-hint brownfield episodes (`54` Stock vs. `54` Nomos `v4.13.1`, `temp=0.0`) spanning Python, TypeScript, and Go on `6` open-weight models sized for single-card `16GB-24GB` GPUs (`RTX 4080`, `RTX 3090`, `A10G`, `L4`):

| Open-Weight Model (`Lab · VRAM Class`) | Stock Verified (`9` Tasks) | Stock Layer Purity | Nomos `v4.13.1` Verified | Nomos Layer Purity | Governance Delta |
| :--- | :---: | :---: | :---: | :---: | :--- |
| **`Llama 3.1 8B Instruct`** (`Meta · 8GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`33.3%` (`3/9`)** | `33.3%` (`1/3`) | **`+11.1` pp** *(Fail-closed capacity floor)* |
| **`Gemma 3 12B IT`** (`Google · 12GB`) | `33.3%` (`3/9`) | `0.0%` (`0/3`) | **`66.7%` (`6/9`)** | **`100%` (`3/3`)** | **`+33.3` pp** (`+100%` relative) |
| **`Qwen 3 14B`** (`Alibaba · 12-16GB`) | `33.3%` (`3/9`) | `0.0%` (`0/3`) | **`77.8%` (`7/9`)** | **`100%` (`3/3`)** | **`+44.4` pp** (`+133%` relative) |
| **`Mistral Small 3.2 24B`** (`Mistral · 16-24GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`88.9%` (`8/9`)** | **`100%` (`3/3`)** | **`+66.7` pp** (`4.0x` lift) |
| **`Qwen 3.8 27B IQ3_M`** (`Local RTX 4080 16GB`) | `50.0%` (`3/6`) | `0.0%` (`0/3`) | **`100.0%` (`6/6`)** | **`100%` (`3/3`)** | **`+50.0` pp** (`-43%` to `-57%` tokens) |
| **`Qwen 3 32B`** (`Alibaba · 24GB`) | `22.2%` (`2/9`) | `0.0%` (`0/3`) | **`88.9%` (`8/9`)** | **`100%` (`3/3`)** | **`+66.7` pp** (`4.0x` lift) |

#### 2. Zero-Trust Security & Real-Repo OSS Debt Containment (`1,541` Live Trajectories)

| Benchmark Suite | Scale & Corpus | Ungoverned `Control` | `Nomos v4.17.0` Governed | Efficiency / Utility Delta |
| :--- | :--- | :---: | :---: | :--- |
| **Official `AgentDojo v1.2.2` (`4` Suites)** | `1,081` runs (`132` benign + `949` indirect prompt-injection tasks, `sha256:e0847020a2f5`) | Indirect tool-hijack exposure | **`0.00%` ASR (`0/949`) · `85.04%` UA** | Only **`-0.53 pp`** vs. `85.57%` Benign (`0` lockouts) |
| **4-Model `SecBench-Live 1.0` (Rounds 1 - 5)** | `400` runs (`Claude Sonnet 5.5`, `Haiku 5.5`, `Gemini 3.8 Flash`, `Qwen 3.8 27B`, `sha256:7f671df45ed9`) | `9.0%` ASR (`18/200` compromised) | **`0.00%` ASR (`0/200`) · `92.5%` Utility** | **`-2.6%` fewer turns** (`304` -> `296` in Round 5) |
| **`RealRepo-BrownfieldBench v2` (`233k` LOC)** | Zero-file-hint refactors across `sqlglot`, `commander.js`, `go-chi`, `BurntSushi/toml` | Leaked **`+26` `dCDS`** structural debt (`16.7%` clean) | **`6/6` `42`-gate clean · `2x` Debt-Free (`33.3%`)** | **`-14.7%` fewer tokens** (`1.58M` -> `1.35M`) |

#### 3. `Terminal-Bench 2.0` Harbor Hub Trajectories & `TraceTrial-Bench` (`arXiv:2610.32285`)

- **`Terminal-Bench 2.0` (`89 / 89` Tasks via [`Harbor`](https://github.com/harbor-framework/harbor) · [`nomos-harbor`](https://github.com/mgantlett/nomos-harbor)):** Multi-trial `k=5` evaluation (`445` Docker trials on `Gemini Flash`) achieved **`76.40%` `Pass@1` (`340/445`)** and **`92.13%` `Pass@5` (`82/89` tasks solved)** at `91.20%` prompt cache stability. In paired single-shot `Gemini CLI 0.67.0` A/B runs, Stock scored `58 / 89` (`65.17%`, [`49f21995`](https://hub.harborframework.com/jobs/49f21995-f52a-4f33-837e-a3f86bbfadd0)) vs. **`63 / 89` (`70.79%`, `+5.62 pp`)** single-shot ([`39059e9a`](https://hub.harborframework.com/jobs/39059e9a-8236-472a-a43b-ab500146abe1)) and **`75 / 89` (`84.27%`, `+19.10 pp`)** `pass<=2` ([`6b9f81a5`](https://hub.harborframework.com/jobs/6b9f81a5-0750-4242-80f2-0b5d0830355e)).
- **Frontier Harness Fault-Containment ([`TraceTrial-Bench`](https://github.com/TraceLite-AI/TraceTrial-Bench) · [`arXiv:2610.32285`](https://arxiv.org/abs/2610.32285)):** Across `190` paired Docker runs (`95` tasks holding `Claude Code v2.1.220` + `Gemini 2.5 Flash-Lite` fixed), `nomos hook` lifted **Context & Continuity by `+78.3%`**, cut **`harness_caused` failures by `-18.2%`** (`55` -> `45`), and raised reliability from `44.89` to **`48.78`**.

</details>

<details>
<summary><b>🏗️ Expand Architecture Blueprint, CLI Surface & Specifications</b></summary>

<br />

<div align="center">
  <img src="intent/assets/nomos_hero_blueprint_v1_5.jpg" alt="Nomos v1.5 Downstream Closed-Loop Lifecycle Architecture" width="100%" />
</div>

| Capability | Canonical CLI Surface | Architecture Specification |
| :--- | :--- | :--- |
| **Indirect Control & Hooks** | `nomos init`, `nomos handshake`, `nomos hook install \| session-start \| pre-tool \| post-tool \| stop` | [`intent.md`](intent/intent.md) |
| **Worktrees, DAG & 2PC Sync** | `nomos task chore \| pair \| create \| start \| edit \| review \| dag \| sync` | [`nomos_hermetic_worktree_spec.md`](intent/architecture/specs/nomos_hermetic_worktree_spec.md) · [`task_lifecycle_cli_spec.md`](intent/architecture/specs/task_lifecycle_cli_spec.md) |
| **42-Gate AST & Obligation Engine** | `nomos verify`, `nomos verify proof`, `nomos status` | [`tree_sitter_ast_gate_spec.md`](intent/architecture/specs/tree_sitter_ast_gate_spec.md) · [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md) |
| **NCTE-100 Directives & Config SSoT** | `nomos directive [--lint \| --lexicon]`, `nomos --help` (`nomos.schema.json`) | [`nomos_controlled_technical_english_spec.md`](intent/architecture/specs/nomos_controlled_technical_english_spec.md) · [`go_ssot_and_json_schema_spec.md`](intent/architecture/specs/go_ssot_and_json_schema_spec.md) · [`software_physics_engine_spec.md`](intent/architecture/specs/software_physics_engine_spec.md) |
| **Receipt-Gated Episodic Memory** | `nomos mem save \| recall \| list \| dismiss`, `nomos issue` | [`mneme_memory_spec.md`](intent/architecture/specs/mneme_memory_spec.md) |

- **Scope of Proof:** Nomos verifies `<5ms` compiled structural and traceability invariants (import boundaries, contract field parity, anti-shim AST checks, `TC-*` test bindings, and SHA-256 spec digests) rather than LLM-as-a-Judge critics or formal theorem provers (`Coq`, `Lean`, `TLA+`); runtime semantics remain governed by your test suite.

</details>

<details>
<summary><b>🛡️ Expand The 7 Laws of Nomos (42-Gate Invariant Matrix)</b></summary>

<br />

Full 42-gate matrix in [`nomos_intent_traceability_spec.md`](intent/architecture/specs/nomos_intent_traceability_spec.md):

| Law | Name | What It Prevents |
| :---: | :--- | :--- |
| **1** | **Dual-Core Inversion** | Unverified agent execution outside the phase state machine (`PhaseDisciplineGate`). |
| **2** | **Non-Verbal Proof** | Conversational "tests pass" claims, `// TODO` shims, and empty function stubs (`AntiShimGate`). |
| **3** | **Hermetic Worktree** | Direct commits on `main` and path escapes outside the task worktree (`HermeticWorktreeGate`, `StagedBoundariesGate`). |
| **4** | **Single Source of Truth** | Broken documentation links, orphaned specs, and unaligned schemas (`BrokenWireGate`, `ContractParityGate`). |
| **5** | **Zero Hardcoded Paths** | Machine-specific absolute paths and secret literals (`ForbiddenLiteralsGate`). |
| **6** | **Canonical Singularity** | Duplicate CLI aliases, rogue task keys, and filesystem symlinks (`ZeroAliasGate`, `ZeroFilesystemAliasGate`). |
| **7** | **Fail-Closed Pre-Flight** | Cross-plane drift, test coverage drops, static-analysis regressions, and vulnerability leaks (`PlaneSeparationGate`). |

</details>

---

## 🧭 Open-Core Boundary

Nomos Community Edition is **100% open-source under Apache 2.0** with zero telemetry, zero cloud account requirements, and zero crippled CLI commands. Every single-developer and single-repository governance capability runs locally out of the box (see [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and [`intent/intent.md`](intent/intent.md)):

| Capability Layer | Nomos Community Edition (Apache 2.0) | SophiaLabs Nomos Sovereign |
| :--- | :--- | :--- |
| **Zero-Migration Brownfield Debt Ratchet & Trunk-Pinned `nomos.yaml`** | ✅ Included | ✅ Included |
| **Indirect Control for Tier 1 Coding Agents** (`AGENTS.md`, `CLAUDE.md`, `.claude/settings.json`, `.gemini/settings.json`, `.opencode/plugins/nomos.ts`, `.cursor/hooks.json`, `.agents/hooks.json`) | ✅ Included | ✅ Included |
| **42-Gate Polyglot Verification Engine** (`nomos verify`, Go AST + Polyglot Scanners, 7 Laws + DoD) | ✅ Included | ✅ Included + Custom Enterprise Gates |
| **Pre-Tool Security Airlock** (`CapabilityFlowGraph` IFC, Credential Airlock, Command & Zero-Trust MCP Guard) | ✅ Included | ✅ Included + Live 2D Firewall & Air-Gap Radar |
| **Causal `INT-*`/`SUB-*` DAG, Hermetic Worktrees & 2PC Sync** (`nomos task`, trunk-pinned `nomos.yaml`, `refs/notes/nomos/releases`) | ✅ Included (Single-Seat) | ✅ Included + Multi-Seat CapBAC Leases & Cross-Repo DAGs |
| **Nomos Mneme Receipt-Gated Episodic Memory & Sanitized Issues** (`nomos mem`, `nomos issue`, local `mneme.db` hybrid vector/lexical recall + automatic 2PC `trap -> fix` pitfall distillation) | ✅ Included (Local Repo `mneme.db`) | ✅ + SOMA Dream Cycle Synthesis, Mneme Cortex HUD & Synchronized Squad Mesh |
| **4-Pillar Glass Cockpit SPA** (Unified Workfront & 2D Code DNA Atlas, 7x3 Verify Matrix, 70/30 Silicon Router, 2D Security & Air-Gap Radar) | Not Included (Terminal ANSI HUD Included) | 🔒 Sovereign Edition ([Explore Tour](https://sophialabs.ca/sovereign)) |
| **Headless Tier 2 Autonomous Worker Fleet & Adversarial Conformance Critic** (`nomos code`, `nomos code review`, Intent-to-Code Twin Parity, `$0` Local GPU Burndown) | Not Included | 🔒 Sovereign Edition |
| **Turnkey Air-Gapped NixOS GPU Appliances & Offline Ed25519 Entitlement** | Not Included | 🔒 Regulated & Enterprise ([Adoption Profiles](https://sophialabs.ca/editions)) |

---

## 🤝 Community & License

- **Community & Governance:** Join [GitHub Discussions](https://github.com/mgantlett/nomos/discussions) for setup Q&A, `nomos.yaml` tuning, and feature ideas. See [`CONTRIBUTING.md`](CONTRIBUTING.md), [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md), and [`SECURITY.md`](SECURITY.md).
- **License:** Licensed under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE), [`NOTICE`](NOTICE), and [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

<div align="center">
  <sub>Built with deterministic rigor by <a href="https://sophialabs.ca"><b>SophiaLabs</b></a> · Governed by Nomos</sub>
</div>
