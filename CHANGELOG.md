# Changelog

All notable changes to the **Nomos** deterministic engineering substrate (Apache 2.0) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Precompiled static binaries (`CGO_ENABLED=0`) and SHA-256 `checksums.txt` manifests are published on [GitHub Releases](https://github.com/mgantlett/nomos/releases).

---

## [v4.13.1] - 2026-11-10

> Zero-Ceremony Multi-Agent Onboarding Invisibility, 4-Layer Progressive Intent Compiler Charter (`TC-INT-01..06`), and Ledger-First GitHub Release Notes (Apache 2.0)

### Zero-Ceremony First-Time Onboarding & Ambient Hook Hardening (`nomos init`, `nomos hook`)
- **Out-of-the-Box Claude Code & Gemini CLI Hook Scaffolding (`SUB-2133`, `INT-1940`)**: `nomos init` now deterministically scaffolds and additively merges `.claude/settings.json` and `.gemini/settings.json` alongside `.agents/hooks.json` and `.opencode/plugins/nomos.ts`, activating compiled `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` governance immediately on a developer's first session without requiring a separate `nomos hook install` command.
- **Hermetic `--only` Baseline Governance Commit (`SUB-2133`)**: Scopes `nomos init`'s initial Git baseline commit via `git commit --only -- <governance-paths>` so pre-staged developer index changes are never swept into the governance commit on either unborn or existing `HEAD` repositories, and prints a 1-line `worktrees/<repo>-<task>` sandbox orientation hint on initialization.
- **GUI IDE `$PATH` Fallback, Teammate Missing-Binary Grace & Trunk Escape Hints (`SUB-2133`)**: Embeds POSIX `$PATH` fallback resolution (`$GOBIN`, `$HOME/go/bin`, `$HOME/.local/bin`, `$HOME/bin`) with graceful `{}` (`exit 0`) fallback when `nomos` is absent across `DefaultClaudeSettingsHooksJSON`, `DefaultGeminiSettingsHooksJSON`, and `.opencode/plugins/nomos.ts`, and adds copy-paste coding-agent and `git stash && nomos task chore` instructions when a human `git commit` hits a protected trunk branch.

### 4-Layer Progressive Intent Compiler Charter & Release Ledger Ordering (`intent/intent.md`, `packaging/render-release-notes.sh`)
- **Stage 0 Intent Compiler Charter & Executable Obligation Proofs (`INT-1941`, `SUB-2134`)**: Codifies the 4-Layer Progressive Intent Compiler (`Layer 0` Charter -> `Layer 1` `nomos.yaml` Physics -> `Layer 2` JIT Slice Specs -> `Layer 3` Hermetic Worktree & Git Notes SHA-256 Receipts) in `intent/intent.md`, enables `strict_types`, `verify_obligations`, and `fail_on_stale_obligations` in `intent/config/nomos.yaml`, and binds executable verification vectors `TC-INT-01..06` in `substrate/cli/intent_manifesto_obligations_test.go` with sealed SHA-256 `spec_digests`.
- **Ledger-First GitHub Release Notes & Collapsible Milestone Rollups (`SUB-2129`)**: Reorders `packaging/render-release-notes.sh` so GitHub Releases lead with the 1-line release summary and chronological SHA-linked `[INT-*]` / `[SUB-*]` commit ledger (`## Changelog`), followed by collapsible `<details>` disclosures for detailed architecture notes and parent `vX.Y.0` milestone rollups on patch releases.

---

## [v4.13.0] - 2026-11-09

> 11-Language Polyglot Architectural Obligation Proofs: Declarative Hexagonal Layer Boundaries (`<5ms`), Executable `TC-*` Vector Binding, Fenced Spec Contract Signature Parity, Cryptographic `spec_digests` SHA-256 Taint Receipts & Turn-0 Handshake Alerts, Plus Polyglot Subprocess `GIT_*` Isolation & POSIX Installer Hardening (Apache 2.0)

### 11-Language Polyglot Architectural Obligation Proofs (`nomos verify`, `nomos handshake`, `nomos status`, `nomos.yaml`)
- **Declarative Polyglot Layer Boundary Enforcement (`INT-1935`, `SUB-2004`, `INT-1936`, `SUB-2007`)**: Adds `architecture_boundaries` rules (`name`, `from`, `forbid_imports`, `allow_imports`) under `governance.quality_gates` in `nomos.yaml` and `nomos.schema.json`. Evaluates package and module imports in `<5ms` on `PostToolUse` (`EvaluateFastFileRadar`) and during `nomos verify` across Go AST, TypeScript/JavaScript/Svelte (`import`/`export from`/`require`), Python (`import`/`from ... import`), Rust (`use`/`mod`), Ruby (`require`/`require_relative`), C/C++ (`#include`), Java/Kotlin (`import`), and C# (`using`), with worktree governance-downgrade protection against pinned `main` and live self-dogfooding (`core-purity`, `modules-independence`) in Nomos itself.
- **Polyglot Fenced Spec Contract Signature Parity (`SUB-2002`, `SUB-2007`)**: Adds `strict_types: true` enforcement across fenced Markdown specification code blocks (`go`, `ts`/`tsx`/`js`/`mjs`, `python`, `rust`, `java`, `kotlin`, `csharp`) so declared struct, interface, `type`, `class`, and `@dataclass` fields fail closed on implementation drift across all supported ecosystems, alongside polyglot `@intent <spec>.md #<TypeName>` anchor verification.
- **Executable `TC-*` Specification Obligation Binding (`SUB-2002`, `SUB-2007`, `SUB-2010`, `INT-1937`, `SUB-2011`)**: Adds `verify_obligations: true` scanning Markdown specification vector tables (`TC-[A-Z0-9]+-[0-9]+`) against verification suites across all 11 polyglot test conventions (`*_test.go`, `*.test.ts`/`*.spec.mjs`, `test_*.py`/`*_test.py`, `*_test.rs`, `*_test.rb`/`*_spec.rb`, `*Test.java`, `*Test.kt`, `*Tests.cs`, `*_test.cpp`) via `@verifies TC-...` comments or canonical test identifiers, while exempting upstream `INT-*` specification-authoring tasks from chicken-and-egg failures before their paired `SUB-*` task binds tests.
- **Cryptographic `spec_digests` SHA-256 Taint Tracking & Turn-0 Handshake Alerts (`SUB-2003`, `SUB-2007`, `SUB-2010`)**: Stamps `spec_digests` (`map[spec_path]sha256`) into `refs/notes/nomos/releases` 2PC receipts on `SUB-*` releases (resolving `@intent` refs, `@verifies TC-*` vectors, `--twin INT-*` diffs, and active `spec_roots` contracts), surfaces stale downstream obligations immediately on Turn 0 in `nomos handshake` (`--json` and CLI) and `nomos status`, and blocks stale merges when `fail_on_stale_obligations: true` is active.
- **Paired Live-Model Brownfield Benchmark (`Qwen3.8-27B`, `temp=0.0`)**: Across paired Python, TypeScript, and Go multi-module brownfield repositories with tempting `infra/` helpers and passing unit tests, `nomos v4.13.0` achieved **`100%` (`6/6`)** architectural obligation compliance (`+9s` average self-remediation overhead) versus **`50%` (`3/6`, `0/3` on hexagonal layer purity)** under stock `CLAUDE.md` + unit test harnesses.

### Polyglot Runner Isolation, Sync & Installer Hardening (`nomos task sync`, `install.sh`)
- **Subprocess `GIT_*` Sanitization & Whitespace Duplicate-Commit Guard (`SUB-1990`)**: Strips inherited `GIT_DIR` / `GIT_WORK_TREE` / `GIT_INDEX_FILE` variables across all 22 subprocess invocations in all 7 polyglot test and coverage runners, self-heals accidental `core.bare=true` config mutations during `nomos task sync`, and suppresses pure trailing-whitespace IDE auto-save diffs after `nomos task commit` across all languages.
- **Optional Post-Sync Lifecycle Hook & POSIX `sh` Installer Compatibility (`SUB-1994`, `SUB-1989`, `INT-1933`)**: Invokes `.agents/scripts/post-sync.sh` when executable after `nomos task sync`, makes `install.sh` and `uninstall.sh` strictly POSIX `sh`-compatible (`curl ... | sh`), and streamlines `README.md` header badges.

---

## [v4.12.1] - 2026-11-09

> Episodic Pitfall Scope Hygiene, Polyglot TAP/Exception Diagnostic Windowing, Hybrid Release-Notes Ledger Compiler & Coverage Persistence (Apache 2.0)

### Episodic Memory & Pitfall Distillation Hardening (`nomos mem`, `nomos handshake`)
- **Automatic Dead-Scope Tombstoning & Cross-Folder Habit Consolidation (`SUB-1965`, `SUB-1960`)**: Auto-tombstones stale pitfall scopes whose target directories no longer exist, skips ephemeral test-assertion noise, prioritizes `touchedFiles` during multi-violation gate trap distillation, and scopes cold-start `nomos handshake` / task-start recalls in read-only mode so `access_count` is never artificially inflated.

### Polyglot Verification & Release Pipeline Hardening (`nomos verify`, `packaging/render-release-notes.sh`)
- **Salient Polyglot Toolchain Diagnostic Window (`SUB-1962`, `SUB-1960`)**: Anchors the 400-byte `extractSalientToolchainWindow` diagnostic excerpt on the first `not ok`, `FAIL`, or exception marker across multi-hundred-line Node TAP, Python, Rust, `.ts`, and `.svelte` test outputs rather than truncating inside `TAP version 13` passing headers.
- **Live Statement Coverage Persistence & Test Complexity Ratchet (`SUB-1952`)**: Persists `.tmp/coverage.out` during `nomos verify`, extracts a testable CLI verification seam, and enforces test cyclomatic complexity alongside local-symbol TDD coupling.
- **Hybrid GitHub Release Notes & Commit Ledger Compiler (`SUB-1953`)**: Wires `.github/workflows/release.yaml` to `packaging/render-release-notes.sh` so curated `CHANGELOG.md` highlights, automatic `vX.Y.0` milestone patch rollups, and the chronological SHA-linked `[INT-*]` / `[SUB-*]` commit ledger render together on every GitHub Release.

---

## [v4.12.0] - 2026-11-08

> Zero-Config Brownfield Auto-Detection & Monotonic AST Ratchet, Full 11-Language Polyglot Verification Parity, Switchable `nomos.yaml` Brownfield Modes, Navigation-First Handshake & Read-Spiral Gyroscope (Apache 2.0)

### Zero-Config Brownfield Onboarding & Monotonic AST Ratcheting (`nomos init`, `nomos verify`, `nomos.yaml`)
- **Automatic Brownfield Detection (`nomos init`)**: Detects non-empty repositories lacking dual-hemisphere `intent/` + `substrate/` roots, configures `strict_mode: false` automatically, and discovers active top-level source directories out of the box without Day-1 directory migrations.
- **Monotonic AST Merge-Base Ratchet (`git merge-base`)**: Under `strict_mode: false`, ratchets file LOC (`>400`), function LOC (`>60`), McCabe cyclomatic complexity `V(G)` (`>15`), Cognitive Debt Score (`CDS`), structural clones, magic-string duplication, forbidden path literals, and unchecked error hygiene strictly against the trunk baseline so legacy God files pass untouched while net complexity or debt regressions fail closed with exact `[BASELINE main: ... | delta ...]` diagnostics.
- **Declaratively Switchable Brownfield Modes (`nomos.yaml`, `nomos.schema.json`)**: Adds granular brownfield switches under `governance.quality_gates`: `verify_scope` (`"diff"` | `"repo"`), `tdd_mode` (`"off"` | `"ratchet"` | `"diff"` | `"strict"`), `coverage_mode` (`"off"` | `"artifact"` | `"live"`), and `tdd_exclude` glob patterns.

### Full 11-Language Polyglot Verification Parity (`nomos verify`)
- **11-Language Structural AST, Anti-Shim & Error-Hygiene Parity**: Closes all 12 polyglot verification gaps across Go, Rust, C/C++, Python, Ruby, TypeScript, JavaScript, Svelte (`.svelte` `<script>` extraction), Java, Kotlin, and C-Sharp, including overloaded-signature baseline metric retention (`max(V(G), LOC)`) so TypeScript/C++/JVM overloads never mask real complexity growth.
- **Zero-Dependency Live & Artifact Statement Coverage**: Adds zero-dependency `NODE_V8_COVERAGE` live statement coverage probes for JS/TS, zero-dependency `python3 -m trace` fallback alongside `coverage.py` / LCOV / Cobertura XML, Ruby `Coverage` / SimpleCov ingestion, and JaCoCo XML / Cobertura report verification for Java, Kotlin, C-Sharp, Rust, and C/C++.

### Navigation-First Brownfield Handshake & Read-Spiral Pacing (`nomos handshake`, `nomos hook post-tool`)
- **Noise-Filtered Cold-Start Topology Brief (`SynthesizeBrownfieldTopology`)**: Emits `Governance Mode: BROWNFIELD (scope=diff, tdd=ratchet, cov=live)`, lowers `Core Modules` discovery to `>= 120` LOC, removes upfront prompt scolding that induced reasoning-model analysis paralysis, and filters `_`-prefixed demo directories (`_examples/`), `typings/`, `.test-d.ts`, `*-test` harnesses, and `quality_gates.exclude` globs from cold-start orientation.
- **Worktree Read-Spiral Gyroscope (`cmd_hook_pacing.go`)**: Tracks consecutive read/search calls (`Read`, `Grep`, `Glob`, read-only shell searches) inside active `EDIT` worktrees and emits non-blocking 1-line pacing nudges at `12` and `20` consecutive reads without a source edit, resetting immediately upon any file mutation.

---

## [v4.11.0] - 2026-11-07

> Turn-Termination `nomos hook stop` Gate, Outbound Egress Airlock, Additive `nomos hook install`, Scratch-Artifact & Trunk-Redirect Guards, GoReleaser v2 Schema Gate & Mermaid Spec Diagrams (Apache 2.0)

### Indirect-Control Ambient Hook Suite (`nomos hook`, `nomos.yaml`)
- **Turn-Termination Anti-Phantom Gate (`nomos hook stop`)**: Blocks unverified active `EDIT` worktree exits until `nomos verify` clears `DirtyUnverifiedEdits`, rejects `--container-sandbox` exits containing syntax errors or protected verifier mutations, and honors `stop_hook_active: true` for single-pass re-entry safety.
- **Additive Multi-Harness Installer (`nomos hook install`)**: Non-destructively merges `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` hook bindings across Claude Code (`.claude/settings.json`), Gemini CLI (`.gemini/settings.json`), OpenCode (`.opencode/plugins/nomos.ts`), and Cursor (`--global` or `--project`) while preserving existing user configuration.
- **Outbound Egress Airlock & Loop Circuit-Breaker (`nomos hook pre-tool`, `nomos hook post-tool`)**: Inspects outbound HTTP/shell calls against cloud metadata SSRF endpoints (`169.254.169.254`, `metadata.google.internal`, `fd00:ec2::254`), inline credential exfiltration tokens, and declarative `egress_allowlist` domains in `nomos.yaml` (`nomos.schema.json`), alongside upgraded loop-signature circuit breaking and additive Git Notes refspec sync.

### Hermetic Boundary & Scratch-Artifact Enforcement (`nomos verify`, `nomos init`)
- **Scratch-Artifact DoD Gate (`StagedBoundariesGate`)**: Rejects staged or committed `*.tmp`, `*.bak`, `*.orig`, `*.swp`, `*~`, and `.DS_Store` scratch artifacts outside `.tmp/` across both uncommitted index state and active task branch commits (`ERR_BOUNDARY`), paired with self-healing `.gitignore` and `.git/info/exclude` defaults.
- **Protected-Trunk Shell Redirect Guard (`cmd_hook_trunk_guard.go`)**: Blocks shell redirections (`>`, `>>`, `tee`, `sed -i`, `cp`, `mv`) targeting protected `main` trunk paths during `PreToolUse` before filesystem mutation occurs.

### Release Pipeline & Canonical Spec Architecture (`release.yaml`, `intent/architecture/specs/`)
- **Strict GoReleaser v2 Schema Gate & Patch Note Rollup**: Enforces `yaml.Decoder.KnownFields(true)` against GoReleaser v2 config structs in `goreleaser_contract_test.go`, runs `goreleaser check` in CI on every push/PR, and rolls parent `vX.Y.0` milestone notes into `vX.Y.Z` patch releases so `/releases/latest` stays self-contained.
- **Machine-Verified Mermaid Architecture Diagrams**: Converts ASCII box-drawing architecture diagrams across Open-Core specifications into standard `flowchart` Mermaid v11 blocks validated against orphan-state and syntax invariants.

---

## [v4.10.1] - 2026-11-06

> GoReleaser v2 AUR Schema Alignment & Precompiled Multi-Arch Release Artifacts (Apache 2.0)

### Release Pipeline Hardening (`.goreleaser.yaml`, `release.yaml`)
- **GoReleaser v2 `config.AUR` Schema Parity**: Removes unsupported `options` array from `.goreleaser.yaml` `aurs:` (relying on GoReleaser v2's native `!strip` PKGBUILD template) and guards `AUR_KEY` lookup via `index .Env "AUR_KEY"` so GitHub Release tarballs, `checksums.txt`, and rendered AUR `PKGBUILD` artifacts build deterministically whether or not `secrets.AUR_KEY` is configured.

---

## [v4.10.0] - 2026-11-06

> Ambient OpenCode Plugin Hooks, First-Class Rails 8 Minitest Runner, Arch/Omarchy `nomos-bin` AUR Packaging, Pure NixOS Zero-FHS Hooks & Tree-Hash Toolchain Receipt Caching (Apache 2.0)

### Ambient OpenCode Plugin Hook Scaffolding (`nomos init`, `nomos hook`)
- **Zero-Touch `.opencode/plugins/nomos.ts` Scaffolding**: Scaffolds a zero-dependency TypeScript OpenCode plugin during `nomos init` with idempotent `NOMOS_OPENCODE_PLUGIN_START`/`END` markers that preserve existing user plugins.
- **Sub-5ms OpenCode Payload Normalization**: Normalizes OpenCode `tool.execute.before` and `tool.execute.after` payloads across `bash`, `write`, `edit`, `apply_patch` (`*** Update File:` / `*** Add File:` / `*** Delete File:` target extraction), `read`, `list`, `glob`, `grep`, and `lsp`.

### First-Class Rails 8 Minitest Suite Runner (`nomos verify`)
- **Automatic `bin/rails test` & `bundle exec rake test` Execution**: Detects `test/**/*_test.rb` Minitest suites alongside Ruby `InstructionSequence` bytecode compilation, executes `bin/rails test` (or `bundle exec rake test` under `RAILS_ENV=test`), and rejects both failing assertions and non-zero Minitest skip counts (`Ruby(compile+minitest)`).

### Arch Linux & Omarchy Distribution, Container Portability & Receipt Caching
- **Arch Linux / Omarchy `nomos-bin` AUR Target**: Adds `packaging/aur/PKGBUILD` (`x86_64` and `aarch64` static `CGO_ENABLED=0` binaries with Apache-2.0 license staging) and automated GoReleaser `aurs:` publishing.
- **Zero-FHS NixOS Hooks & Static Auto-Recompilation**: Removes external `sed` dependencies from `.git/hooks/commit-msg` via pure Bash parameter expansion for minimal NixOS sandboxes, and enforces `CGO_ENABLED=0` (`-trimpath -ldflags="-s -w"`, zero ELF `PT_INTERP` dynamic linker leakage) during `nomos task sync` binary recompilation.
- **Content-Addressed Tree-Hash Receipt Cache (`nomos-toolchain-receipt-v2`)**: Canonicalizes relative and absolute worktree paths and hashes live Git blob OIDs (`ls-tree` + working-tree overlay + polyglot stack results) so `nomos verify`, `pre-commit`, `nomos task sync`, and fast-forward trunk checks share cryptographic toolchain receipts without redundant test suite runs.

---

## [v4.9.0] - 2026-11-06

> First-Class Ruby & Rails Polyglot AST Gating, Shell-Segment Sentinel Tokenizer, Mneme Locality Boost, RLVR/DPO Session Export & Flat 39-Spec Blueprint (Apache 2.0)

### First-Class Ruby & Rails Polyglot AST Verification (`nomos verify`)
- **Embedded `ruby.scm` & Structural Ruby Scanner**: Adds first-class `.rb` and `.rake` verification covering `def`, singleton `def self.*`, predicate/bang/setter methods, Ruby 3 endless methods (`def valid? = ...`), SQL/HTML heredoc (`<<~SQL`) and `=begin`/`=end` stripping, modifier `if`/`unless`/`while`/`until` disambiguation, cyclomatic complexity, and `raise NotImplementedError` / empty-block anti-shim rejection.
- **RSpec/Minitest Skip Guards & Rails Impact Mapping**: Rejects skipped test declarations (`xit`, `xdescribe`, `xcontext`, Minitest `skip`), maps Rails `app/<layer>/<name>.rb` edits directly to paired `spec/` and `test/` suites, and validates syntax via `RubyVM::InstructionSequence.compile_file`.

### Sentinel Shell-Segment Tokenizer, Mneme Locality & Session Export (`nomos hook`, `nomos mem`, `nomos session`)
- **Shell-Segment-Aware `evalHermeticWorktree`**: Tokenizes chained shell segments (`&&`, `;`, `||`, `|`, `\n`), tracks sequential `cd <dir>` state and `git -C <dir>` targets when ambient working directory metadata is absent, and ignores quoted `git commit` strings inside read-only search commands.
- **Mneme Repo-Locality Boost & Global Axiom Cap**: Applies a `1.25x` ranking boost to exact `repo_id` matches during scoped recall and caps unscoped `global` `axiom` slots at `max(1, limit/3)` with sparse backfill.
- **Joined `nomos session export` & Churn Advisories**: Synthesizes RLVR/DPO-ready trajectory datasets joined with `refs/notes/nomos/releases` receipts and adds `PostToolUse` slow-step timeout plus high-churn worktree checkpoint advisories.

### Canonical Intent Blueprint, Mermaid Guards & GoReleaser Changelog Pipeline (`IntentSteelmanGate`)
- **Flat 39-Spec Steel-Man Singularity**: Consolidates all 39 canonical specifications under `intent/architecture/specs/*_spec.md` on a single 1-indexed NCTE-100 blueprint with compiled Go SSoT contracts, `TC-*` verification vectors, Mermaid syntax validation, and orphan-state guards.
- **Automated `CHANGELOG.md` Release Pipeline**: Bundles root `CHANGELOG.md` into release archives and extracts matching `## [vX.Y.Z]` sections directly into GoReleaser GitHub Releases (`release.yaml`), failing closed if release notes are missing.

---

## [v4.8.0] - 2026-11-05

> Verified Host/Tool-Health Snapshots, Receipt-Trace Parity, Configurable Model-Profile Advisories & Terminal-Bench 2.0 Harbor Results (Apache 2.0)

### Verified Host-Health Snapshots & Receipt-Trace Parity (`nomos hook`, `nomos task sync`)
- **Session-Start Host & Toolchain Snapshot**: Captures deterministic host and language-toolchain health diagnostics on `SessionStart` without external network probes or global state mutation.
- **Graceful 2PC Receipt-Trace Parity**: Seals hook-trace provenance into `refs/notes/nomos/releases` receipts during `nomos task sync` with deterministic fallback when tasks execute outside ambient hook wrappers.

### Configurable Model-Profile Advisory Thresholds (`nomos hook post-tool`, `nomos.yaml`)
- **Per-Model Turn & Churn Calibration**: Adds configurable model-profile advisory thresholds in `nomos.yaml` and `nomos.schema.json` so high-autonomy frontier reasoning models and local MoE models receive calibrated checkpoint guidance without false-positive interruptions.

### Empirical Benchmark Publication (`Terminal-Bench 2.0`)
- **89-Task Harbor Cleanroom A/B Evaluation**: Publishes full containerized A/B benchmark artifacts (`42.7% -> 51.7%`, `+9.0 pp`, `38 -> 46` solved tasks, zero regressions across 38 baseline passes).

---

## [v4.7.0] - 2026-11-05

> Self-Cleaning Nomos Mneme, Binary Context Guard, Interactive Hang Guard & Session Budget Radar (Apache 2.0)

### Self-Cleaning Nomos Mneme & Recurrence-Aware Graduation (`nomos mem`, `nomos handshake`)
- **Per-Task Reinforcement Deduplication & Read-Only Recall**: Deduplicates active task handshake reinforcement to at most `+1` per `(task_id, memory_id)`, marks interactive `nomos mem recall` read-only, and gates built-in Tier-C AST graduation candidates on true cross-task topic recurrence (`>= 2`).
- **Symbol-Delta Fixes, Unscoped Cooling & Active-Default `nomos mem list`**: Prepends refactored file and verified symbol inventories into distilled `Verified Fix` entries, cools single-occurrence pitfalls older than 14 days out of unscoped handshakes, and defaults `nomos mem list` to `active` records with `--status`.

### Execution Hygiene & Context Protection (`nomos hook pre-tool`, `nomos hook post-tool`)
- **Binary Read Guard & Interactive Hang Guard**: Blocks accidental full-file reads of binary/archive/sqlite blobs before context saturation and intercepts unflagged interactive CLI prompts (`apt-get install` without `-y`, unpaged `git log`, interactive editors) before session hangs.
- **Session Budget Radar**: Surfaces deterministic step-budget consumption advisories in `PostToolUse` before lease exhaustion.

---

## [v4.5.2] - 2026-11-04

> Legal Notice Packaging, `SecretGate` Cache Pruning & 42-Gate CI Release Pre-Flight (Apache 2.0)

### Legal Notices & Distribution Hygiene
- **Statutory Apache-2.0 Packaging**: Bundles `NOTICE`, `THIRD_PARTY_NOTICES.md`, and the verbatim SPDX `Apache-2.0` appendix across GoReleaser archives and updates `uninstall.sh` for clean `NOMOS_HOME` removal.
- **`SecretGate` & Release Pre-Flight Hardening**: Prunes gitignored framework build caches during secret scans, enforces word-bounded credential regexes, aligns Cell 2.3 deterministic gate proofs, and runs full 42-gate CI pre-flight verification prior to release cuts.

---

## [v4.5.1] - 2026-11-04

> Native Gemini CLI (`BeforeTool` / `AfterTool`) Hook Protocol Parity for Harbor Benchmarks (Apache 2.0)

### Multi-Agent Hook Protocol Normalization (`nomos hook pre-tool`, `nomos hook post-tool`)
- **Native `@google/gemini-cli` Hook Parity**: Distinguishes Gemini CLI `BeforeTool` and `AfterTool` JSON hook payloads from Claude Code `PreToolUse` / `PostToolUse` events, emitting canonical `{"decision":"allow"|"deny","reason":"..."}` responses on `BeforeTool` and `{"hookSpecificOutput":{"additionalContext":"..."}}` diagnostics on `AfterTool`.
- **Directory & Search Path Target Extraction**: Inspects `dir_path` arguments across `list_directory`, `glob`, and `search_file_content` so directory scans targeting protected Harbor verifier or oracle paths (`/logs/verifier`, `/solution`, `/tests`) are deterministically blocked in `--container-sandbox` mode.

---

## [v4.5.0] - 2026-11-04

> Container Sandbox Anti-Reward-Hacking Hooks, Configurable Intent Twin Mode & Zero-Footprint Telemetry (Apache 2.0)

### Container Sandbox Benchmark Mode (`nomos hook --container-sandbox`)
- **Runtime Anti-Reward-Hacking Guard**: Physically blocks reads, writes, and shell commands targeting protected benchmark verifier and oracle paths (`/logs/verifier`, `/solution`, `/tests`) while permitting full-container OS sysadmin operations (`/etc`, `/tmp`, `/var`, `sqlite3`).
- **Zero-Footprint Container Execution**: Skips global SQLite telemetry initialization during `nomos hook` and `nomos version` invocations so disposable evaluation containers remain 100% free of global state.

### Configurable Bilateral Intent Twin Enforcement (`intent_twin_mode`)
- **Opt-In Spec-vs-Code Twin Policy**: Adds `intent_twin_mode` (`off`, `feat`, `strict`) to `nomos.yaml` and `nomos.schema.json`, defaulting to `off` for friction-free solo adoption while enabling `feat` or `strict` bilateral `INT-*`/`SUB-*` pairing on team and regulated repositories.

---

## [v4.4.1] - 2026-11-03

> Brownfield Go Workspace Hardening & Golden Conventional Commits with Git Trailers (Apache 2.0)

### Brownfield Go Workspace & Verification Hardening (`nomos init`, `nomos verify`, `nomos hook`)
- **Ancestor `go.work` Discovery & Dynamic Go Directives**: Honors explicit ancestor `go.work` module membership in `nomos verify` and derives transient `go.work` Go directives dynamically from module `go.mod` versions.
- **Scoped Pre-Tool & Git Exclude Hygiene**: Anchors operational `.git/info/exclude` entries cleanly, removes legacy `jq` shell fallback guards from hook templates, and scopes `PreToolUse` toolchain install guards to dirty or non-main trees.

### Golden Conventional Commits & RFC 822 Git Trailers (`nomos task commit`, `nomos verify`)
- **Flowing Commit Bodies & `Task:` / `Twin:` Trailers**: Upgrades commit schema enforcement from rigid Markdown headers to readable Golden Conventional Commits (`[KEY] type(scope): summary`) paired with machine-readable `Task:` and `Twin:` RFC 822 Git trailers.

---

## [v4.4.0] - 2026-11-03

> Compositional Capability-Flow Taint Guard, Zero-Trust MCP Boundary Interceptor (IFC) & Multi-Repo Ledger Isolation (Apache 2.0)

### Compositional Capability-Flow Graph & Cross-Tool Taint Guard (`nomos hook pre-tool`)
- **Cross-Tool Taint Propagation**: Tracks untrusted external inputs across sequential tool invocations in `PreToolUse` so prompt-injected payloads read from files, web fetches, or logs cannot exfiltrate secrets or execute privileged shell mutations.
- **Deterministic Pre-Flight Blocking**: Evaluates compositional capability-flow rules in sub-millisecond compiled Go outside the model context window.

### Zero-Trust MCP Boundary Interceptor & Information Flow Control (`nomos verify`, `capbac`)
- **Cross-Server Information Flow Control (IFC)**: Enforces explicit server-to-server trust boundaries across Model Context Protocol (MCP) tool calls, preventing untrusted third-party MCP tools from bridging into sensitive workspace capabilities.
- **Self-Hosted JSON Schema Parity**: Regenerates `nomos.schema.json` with MCP boundary and IFC policy fields verified by byte-identical AST schema parity tests.

### Multi-Repo Ledger Isolation & Nomos Learn Signal Hygiene (`nomos handshake`, `nomos mem`)
- **Strict Per-Repository Ledger Scoping**: Isolates wavefront and Living Ledger views per `repo_id` across multi-repository workstations.
- **High-Signal Memory Curation**: Refines `Nomos Learn` distillation thresholds and adds `nomos mem dismiss` to prune superseded episodic entries cleanly.

---

## [v4.3.0] - 2026-11-03

> Compiled Go AST SSoT, Reflected Draft 2020-12 JSON Schema & 32.8% Smaller Static Binary (Apache 2.0)

### Pure Go AST & Reflected JSON Schema SSoT (`nomos.schema.json`)
- **Single Compiled Go SSoT**: Retires external CUE runtime dependencies in favor of compiled Go struct contracts reflected directly into Draft 2020-12 `nomos.schema.json`.
- **Zero-Copy IDE Autocompletion**: Pins `yaml-language-server` `$schema` headers in `nomos init` templates to versioned GitHub raw schema URLs for instant editor validation.

### Binary Footprint & Storage Hygiene (`-32.8%` Size Reduction)
- **Leaner Static Binary**: Cuts the `CGO_ENABLED=0` static Go binary footprint by nearly one-third (`~28 MB`) while accelerating startup and `nomos verify` cold-boot latency.
- **Storage Table Pruning**: Excises retired v3 journal and legacy telemetry tables from local SQLite state.

---

## [v4.2.0] - 2026-11-02

> NCTE-100 Controlled English Engine, Unified Lipgloss Terminal UI, `nomos physics` CLI & Claude Code v2 Hook Parity (Apache 2.0)

### NCTE-100 Controlled English Engine & Succinct Action Lexicon (`nomos directive`)
- **Compiled NCTE-100 Lexicon & Grammar**: Ships the deterministic Controlled Technical English validator (`ncte.cue`) and succinct operator-to-agent action lexicon CLI for unambiguous, token-disciplined agent steering.
- **Agent-Steering Protocol Parity**: Aligns JIT phase directives (`IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN`) and CLI guidance directly with the published Agent-Steering Protocol.

### Software Physics CLI & Cobra Contract Completeness (`nomos physics`, `nomos task`)
- **Real-Time AST Mass & Collision Vectors**: Wires `nomos physics` to inspect active worktree AST mass and detect real-time symbol and file collision vectors across concurrent task worktrees.
- **Deterministic CLI & Probe Hardening**: Adds complete `--json` and task filter flags across Cobra commands, closes `nomos verify` probe blind spots, and auto-hydrates Dual-Layer commit rationale sections from Living Ledger cards while rejecting canned commit shims.

### Unified Lipgloss Terminal UI & Claude Code v2 Hook Compatibility
- **Adaptive Lipgloss Styling**: Introduces a unified terminal theme across human-facing CLI outputs (`nomos handshake`, `nomos status`, `nomos verify`, `nomos version`) with crisp gradients, pill badges, and clean non-TTY fallback.
- **Claude Code v2 `PreToolUse` Parity**: Aligns `nomos hook pre-tool` with the Claude Code v2 `PreToolUse` JSON schema, adds `Read` environment-variable guards, and keeps background polling exempt from the 3-strike loop circuit-breaker.

---

## [v4.1.0] - 2026-11-02

> Polyglot Wazero + Tree-sitter AST Engine, Native `nomos hook` CLI & 3-Strike Loop Circuit-Breaker (Apache 2.0)

### Polyglot Wazero + Tree-sitter AST Engine (`nomos verify`)
- **Zero-CGO WebAssembly + Tree-sitter Evaluation**: Combines a pure-Go `wazero` WebAssembly linear-memory source scanner (`scan_source_bytes`) with `.scm` S-expression structural evaluators across Go, TypeScript/Svelte, Python, Rust, C/C++, Java, Kotlin, and C#.
- **Polyglot Test, Coverage & Anti-Skip Ratchets**: Auto-detects `go test`, `cargo test`, `pytest`, `gradle test`, `mvn test`, and `dotnet test`, enforces LCOV + Cobertura XML coverage thresholds (`>= 55%`), and rejects reward-hacked skipped tests (`t.Skip`, `#[ignore]`, `@pytest.mark.skip`, `it.skip`, `@Disabled`, `[Fact(Skip=...)]`).

### Native Compiled `nomos hook` CLI & Loop Circuit-Breaker
- **Compiled Hook Subcommands**: Replaces external shell hook scripts with sub-millisecond `nomos hook session-start`, `nomos hook pre-tool`, and `nomos hook post-tool` commands, preserving existing user hooks additively during `nomos init`.
- **3-Strike Loop Circuit-Breaker**: Fingerprints tool invocations (`ArgsDigest` + file mtime in `0700` user-private XDG cache) to halt identical retry loops after 3 consecutive failures (`0/20` runaway loops across the 95-task `TraceTrial-Bench` suite; `83.2%` guarded pass rate vs `45.3%` unguarded).

### Fail-Closed 3-Section Task Cards & Supply-Chain Hardening
- **Structured Living Ledger Cards**: Enforces `## Scope & Context`, `## Acceptance Criteria`, and `## Verification` across `nomos task create`, `chore`, and `pair` with `--scope`, `--ac`, `--verify`, and `--depends-on` flags.
- **Verified Supply-Chain Installer**: Enforces mandatory SHA-256 archive verification against `checksums.txt` in `install.sh` and `os.Lstat` symlink/world-writable rejection across plugins and caches.

---

## [v4.0.0] - 2026-11-01

> Initial Open-Core Public Release (Apache 2.0): Deterministic Go Control Harness for AI Coding Agents

### 7x3 Canonical Invariant Suite (`nomos verify`)
- **7 Laws & 21 Compiled Go AST Invariant Cells**: Enforces complexity ceilings, explicit error handling, anti-shim detection, test-deletion guards, zero-alias singularity, and 2-plane separation in milliseconds.
- **Fail-Closed Pre-Flight Verification**: Zero reliance on probabilistic prompt honor systems; code mutations require exit code `0` and deterministic AST proof before review.

### Hermetic Worktrees & 2PC Atomic Sync (`nomos task`)
- **Disposable Task Worktrees**: Physically locks `main` as read-only and isolates every AI coding session inside `worktrees/<repo>-<task>` across the `IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN` state machine.
- **Cryptographic Release Provenance**: Attaches verifiable Git Notes receipts and enforces strict `intent/` vs `substrate/` plane separation.

### Nomos Mneme Episodic Memory (`nomos mem`)
- **Repo-Scoped Local SQLite FTS5 Engine**: Persists distilled architectural axioms and verified engineering pitfalls locally on your machine with sub-millisecond recall.

### Zero-Lock-In Tier 1 Agent Harness (`nomos init`, `nomos handshake`)
- **Universal Indirect Control**: Works out of the box with Claude Code, Cursor, Antigravity IDE, Windsurf, Cline, and OpenCode with zero cloud accounts, background daemons, or telemetry phone-home.
- **Cross-Platform Single Binary (`CGO_ENABLED=0`)**: Precompiled `linux`, `darwin`, and `windows` (`amd64` / `arm64`) archives published with SHA-256 `checksums.txt`.
