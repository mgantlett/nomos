<div align="center">

<img src="intent/assets/nomos_hero_emblem.jpg" alt="SophiaLabs // Nomos Unified Emblem" width="220" />

# Nomos (Νόμος)

**Peace of mind for AI-assisted engineering: the deterministic governance harness between any coding agent and your repository.**

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Polyglot Gates](https://img.shields.io/badge/AST_%26_Toolchains-Go_%7C_TS_%7C_Py_%7C_Rust-654FF0)](intent/architecture/specs/core/seven_laws_canonical_gate_matrix_spec.md)
[![Quality Gates](https://img.shields.io/badge/DoD_Gates-42_Enforced-success)](intent/architecture/specs/core/seven_laws_canonical_gate_matrix_spec.md)

</div>

---

AI coding agents optimize for making their prompt finish, even when that means taking shortcuts. Without a deterministic boundary, agents routinely introduce three silent failures into production repositories:

- **Reward Hacking Tests:** Deleting, skipping, or weakening failing assertions instead of fixing the root cause.
- **Hollow Shims & Stubs:** Leaving empty `// TODO` stubs, mock returns, or dead unreferenced code to make a build compile.
- **Unbounded Workspace Drift:** Mutating `main` directly, polluting unrelated modules, or forgetting architectural context across sessions.

**Nomos** gives you genuine **peace of mind** when delegating to **Claude Code**, **Cursor**, **Antigravity**, **OpenCode**, **Windsurf**, or **Aider**. Sitting between your coding agent and your Git repository, Nomos doesn't rely on prompt instructions that models ignore under context pressure - it physically enforces **hermetic Git worktree sandboxes**, **42 polyglot AST quality gates**, and **atomic Two-Phase Commit (2PC) releases** before a single line can touch `main`.

> **⚡ Two commands to install and initialize. Invisible after that.**
> Run `curl -fsSL https://sophialabs.ca/install | bash` and `nomos init` once. There are zero custom slash commands to memorize, zero prompt templates to maintain, and zero manual handshake steps. You prompt Claude Code, Cursor, Antigravity, or OpenCode in plain English the exact way you already do, while Nomos governs worktree isolation, sub-15ms AST checks, and two-phase commits silently under the hood.

<div align="center">
  <img src="intent/assets/nomos_hero_demo.gif" alt="Nomos Live Terminal Demo - Blocking main commits, catching an empty AST stub, and releasing via 2PC" width="100%" />
</div>

---

## ⚡ Quick Start (Under 60 Seconds)

### 1. Install the `nomos` Binary

```bash
curl -fsSL https://sophialabs.ca/install | bash
```

Installs directly to standard Go binary paths (`$GOBIN` / `$(go env GOPATH)/bin` / `~/go/bin`). Or build directly via the Go toolchain:

```bash
go install github.com/mgantlett/nomos/substrate/cmd/nomos@latest
```

To cleanly uninstall at any time:

```bash
curl -fsSL https://sophialabs.ca/uninstall | bash
```

### 2. Initialize Any Repository

```bash
cd your-project
nomos init
```

`nomos init` detects your language stack, scaffolds [`nomos.yaml`](intent/architecture/specs/core/declarative_pipeline_spec.md), installs self-healing Git defense hooks (`pre-commit`, `commit-msg`, `pre-push`), and drops indirect-control agent contracts (`AGENTS.md`, `CLAUDE.md`) plus ambient lifecycle hooks (`.agents/hooks.json`) into your repo.

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

1. **Indirect Control & Persistent Memory (`nomos init` / `nomos mem`):** Your coding agent reads the bootstrapped `AGENTS.md` / `CLAUDE.md` protocol and queries **Nomos Mneme** (`nomos mem recall`) for sub-50ms hybrid vector and lexical architectural context before touching code.
2. **Hermetic Worktree Sandbox (`nomos task`):** Direct commits on `main` fail closed. Every task executes inside an isolated Git worktree (`worktrees/<repo>-<task>`) governed by a strict state machine (`IDLE → PLAN → EDIT → REVIEW → SYNC → LEARN`).
3. **42-Gate Polyglot AST & Toolchain Verification (`nomos verify`):** Before any code can be reviewed or merged, Nomos evaluates `go/ast`, pure-Go polyglot structural scanners (`wazero` sandbox), and native compiler/test probes (`tsc`, `pytest`, `cargo check`) across **Go, TypeScript/Svelte, Python, Rust, and C/C++** - catching empty stubs, `todo!()`/`pass` shims, complexity spikes, hardcoded paths, security flaws, and test coverage drops.
4. **1-Turn Auto-Remediation Loop (`NomosError`):** When a gate fails, `nomos verify` emits a structured [`NomosError`](intent/architecture/specs/core/nomos_actionable_error_framework_spec.md) with the exact file, line, violated law, and remediation instruction so the coding agent fixes its own mistake on the next turn.
5. **Physically Locked Trunk & Atomic 2PC Release (`nomos task sync`):** Once all 42 gates pass, Nomos fast-forward merges the worktree into `main`, attaches a cryptographic **Git Notes (`refs/notes/nomos/releases`)** verification receipt, and prunes the sandbox.

---

## 🧩 Core Capabilities & Deep-Dive Specifications

| Capability | Canonical CLI Surface | What It Enforces | Architecture Specification |
| :--- | :--- | :--- | :--- |
| **Indirect Agent Control** | `nomos init`, `nomos handshake` | Bootstraps agent rules (`AGENTS.md`, `CLAUDE.md`), ambient lifecycle hooks (`.agents/hooks.json`), and self-healing Git hooks without locking you into a specific model or IDE. | [`agent_tiers_and_indirect_control_spec.md`](intent/architecture/specs/core/agent_tiers_and_indirect_control_spec.md) |
| **Phase Discipline & Worktrees** | `nomos task create \| start \| edit \| review` | Enforces `IDLE → PLAN → EDIT → REVIEW → SYNC → LEARN` state transitions inside hermetic `worktrees/<repo>-<task>` sandboxes. | [`nomos_hermetic_worktree_spec.md`](intent/architecture/specs/core/nomos_hermetic_worktree_spec.md) |
| **42-Gate AST & DoD Engine** | `nomos verify`, `nomos verify proof` | Runs 42 statutory Definition of Done (DoD) quality gates across the 7x3 Canonical Invariant Matrix (21 Cells) via Go AST, pure-Go polyglot structural scanners (`wazero`), and native toolchain probes (`tsc`, `pytest`, `cargo`). | [`seven_laws_canonical_gate_matrix_spec.md`](intent/architecture/specs/core/seven_laws_canonical_gate_matrix_spec.md) · [`full_dod_gate_suite_and_polyglot_spec.md`](intent/architecture/specs/core/full_dod_gate_suite_and_polyglot_spec.md) |
| **Auto-Remediation Directives** | `nomos directive` | Translates gate failures into deterministic JSON/terminal `NomosError` payloads and next-action directives for 1-turn agent self-correction. | [`nomos_actionable_error_framework_spec.md`](intent/architecture/specs/core/nomos_actionable_error_framework_spec.md) |
| **Nomos Mneme Memory** | `nomos mem save \| recall \| list` | Pure-Go SQLite WAL + hybrid 64-dim vector & lexical memory bank (`mneme.db`) scoped per repository (`repo_id`) so agents retain architectural decisions across sessions. | [`mneme_memory_spec.md`](intent/architecture/specs/core/mneme_memory_spec.md) |
| **Atomic 2PC Sync & Git Notes** | `nomos task sync`, `nomos verify proof` | Two-Phase Commit release to `main` with mandatory walkthrough synthesis and immutable `refs/notes/nomos/releases` provenance receipts. | [`task_lifecycle_cli_spec.md`](intent/architecture/specs/core/task_lifecycle_cli_spec.md) |
| **Git Hooks & Sanitized Issues** | `nomos issue` | Self-healing pre-commit/commit-msg/pre-push defense hooks and zero-leak sanitized diagnostic issue reporting. | [`git_hooks_and_commit_schema_spec.md`](intent/architecture/specs/core/git_hooks_and_commit_schema_spec.md) |
| **Complete CLI & Config Reference** | `nomos --help` | Full flag-by-flag reference for all commands and declarative `nomos.yaml` quality thresholds. | [`cli_and_flags_parity_expansion_spec.md`](intent/architecture/specs/core/cli_and_flags_parity_expansion_spec.md) · [`declarative_pipeline_spec.md`](intent/architecture/specs/core/declarative_pipeline_spec.md) |

---

## 🛡️ The 7 Laws of Nomos

Every check in `nomos verify` maps to one of seven compiled architectural laws (full 42-gate matrix in [`seven_laws_canonical_gate_matrix_spec.md`](intent/architecture/specs/core/seven_laws_canonical_gate_matrix_spec.md)):

| Law | Name | What It Prevents |
| :---: | :--- | :--- |
| **1** | **Dual-Core Inversion** | Prevents unverified agent execution outside the phase state machine (`PhaseStateGate`). |
| **2** | **Non-Verbal Proof** | Rejects conversational "tests pass" claims, `// TODO` shims, and empty function stubs (`AntiShimGate`). |
| **3** | **Hermetic Worktree** | Physically blocks direct commits on `main` and path escapes outside the task worktree (`WorktreeBoundaryGate`). |
| **4** | **Single Source of Truth** | Eliminates broken documentation links, orphaned specs, and unaligned schemas (`BrokenWireGate`, `ParityGate`). |
| **5** | **Zero Hardcoded Paths** | Blocks machine-specific absolute paths (`/Users/...`, `/home/...`) and secret literals (`ForbiddenLiteralsGate`). |
| **6** | **Canonical Singularity** | Prohibits duplicate CLI aliases, rogue task keys, and filesystem symlinks (`ZeroFilesystemAliasGate`). |
| **7** | **Fail-Closed Pre-Flight** | Enforces strict plane separation, test coverage floors, static analysis, and vulnerability checks (`PlaneSeparationGate`). |

---

## 🧭 Open-Core Boundary

Nomos Community Edition is **100% open-source under Apache 2.0** with zero telemetry, zero cloud account requirements, and zero crippled CLI commands. Every single-developer and single-repository governance capability runs locally out of the box (see [`open_core_license_spec.md`](intent/architecture/specs/core/open_core_license_spec.md)):

| Capability Layer | Nomos Community Edition (Apache 2.0) | SophiaLabs Sovereign Enterprise |
| :--- | :--- | :--- |
| **Indirect Control for Tier 1 Coding Agents** (`AGENTS.md`, `CLAUDE.md`, Skills) | ✅ Included | ✅ Included |
| **42-Gate Polyglot Verification Engine** (`nomos verify`, Go AST + Polyglot Scanners, 7 Laws + DoD) | ✅ Included | ✅ Included |
| **Hermetic Worktree Lifecycle & 2PC Sync** (`nomos task`, `refs/notes/nomos/releases`) | ✅ Included | ✅ Included |
| **Nomos Mneme Hybrid Vector Memory & Sanitized Diagnostic Issues** (`nomos mem`, `nomos issue`) | ✅ Included | ✅ Included |
| **Autonomous Tier 2 Coding Agent & Multi-Agent Swarm** (`nomos code`) | Not Included | 🔒 Enterprise Only |
| **Multi-Seat Mesh Sync, Cockpit Control Plane & Hardware Attestation** | Not Included | 🔒 Enterprise Only |

---

## 📄 License

Licensed under the **Apache License, Version 2.0**. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE) for full text and attribution terms.

<div align="center">
  <sub>Built with deterministic rigor by <a href="https://sophialabs.ca"><b>SophiaLabs</b></a> · Governed by Nomos</sub>
</div>
