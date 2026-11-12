# Changelog

All notable changes to the **Nomos** deterministic engineering substrate (Apache 2.0) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Precompiled static binaries (`CGO_ENABLED=0`) and SHA-256 `checksums.txt` manifests are published on [GitHub Releases](https://github.com/mgantlett/nomos/releases).

---

## [v4.17.0] - 2026-11-12

> Package Fan-Out Cohesion Gate (`max_package_files`) & Rename-Aware Brownfield Ratchets, 1-Turn AST Refactoring & CWE-22 Confinement Recipes, Working-Tree Verification Receipt Caching, Closed-Loop Mneme Hot-Scope Reflexes, Native MCP Catalog Pinning, Plus Pre-Tag GoReleaser & SemVer Airlock Gates (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **`max_package_files` Cohesion Gate & Core Sub-Package Decomposition (`INT-1986`, `SUB-2281`)**: Adds configurable `governance.quality_gates.thresholds.max_package_files` (default `40` non-test source files per directory) across Go SSoT configuration and `nomos.schema.json` (`TC-SSOT-06`, `TC-TS-12`) with monotonic `git ls-tree` brownfield ratcheting and strict enforcement on `nomos`, decomposing `substrate/src/nomos/modules/verify` (`70` -> `38` files across `verify/astdebt`, `verify/vscope`, `verify/impact`, and `verify/polyrun`) and `substrate/cli` (`79` -> `36` files across `cli/tui`, `cli/commitmsg`, `cli/inspectcmd`, `cli/diag`, `cli/selfupdate`, and `cli/taskui`).
- **Rename-Aware Baseline Lookup & Monotonic Fan-Out Ratcheting (`INT-1989`, `SUB-2292`)**: Wires `ResolveRenamedBaselinePath` (`git diff --find-renames=50%`) into `loadBaselineFileBytes` (`TC-TS-13`) so moved files (`git mv`) inherit their pre-move AST baselines rather than being treated as net-new code, and permits zero-net-file edits inside pre-existing `> max_package_files` brownfield directories without false-positive fan-out blocks.
- **1-Turn AST Refactoring & Polyglot CWE-22 Confinement Recipes (`INT-1985`, `INT-1989`, `SUB-2280`, `SUB-2292`)**: Enriches cognitive debt (`dCDS`), cyclomatic complexity (`V(G)`), function LOC, and parameter arity gate diagnostics across Go, Python, and JS/TS with `file:start-end` spans, exact numeric excess deltas, and explicit helper or `<fn>Options` extraction targets (`TC-PHY-05`, `TC-PHY-06`, `TC-TS-11`), and refines `FastFileRadar` CWE-22 path-sink precision (`TC-TS-14`) with Go, JS/TS, and Python path-confinement recipes.

### CLI, Distribution & Hooks (`nomos hook`, `capbac`)
- **Native MCP Catalog Pinning & 1-Hop `PostToolUse` Impact Recipes (`SUB-2278`, `SUB-2290`)**: Enforces native MCP catalog pinning (`mcp_catalog_pin.go`) alongside polyglot CWE and structural-clone fast-radar checks, surfaces 1-hop `PostToolUse` blast-radius alerts (`cmd_hook_impact.go`), and emits 1-turn `PostToolUse` recipes for above-ceiling file LOC, inline `dCDS`, CWE sinks, and first-read hotspot peers.

### Task, Memory & Session (`nomos verify`, `nomos task sync`, `nomos mem`)
- **Working-Tree SHA Verification Receipt Cache (`INT-1989`, `SUB-2292`)**: Computes unstaged working-tree SHA digests via an isolated temporary `GIT_INDEX_FILE` (`cmd_verify_working_tree_cache.go`) so verified clean-tree receipts are reused across `nomos verify`, `pre-commit`, and `nomos task sync` without redundant full-suite gate runs.
- **Closed-Loop Mneme Hot-Scope Reflexes & Clean-Streak Cooling (`INT-1984`, `SUB-2279`)**: Escalates recurring directory pitfalls (`recurrence >= 3`, active head `access_count < 5`) into sensitized `65%` `FastFileRadar` warnings and inline `arity`, `unchecked_error_hygiene`, and `anti_shim` micro-probes (`TC-MEM-16`), cools reflexes automatically after `5` consecutive clean recalls (`TC-MEM-17`), and suppresses `nomos.yaml` graduation prompts for built-in compiled AST gates (`TC-MEM-18`).

### Release & Packaging (`nomos verify`, `CHANGELOG.md`)
- **Pre-Tag GoReleaser Template, SemVer Parity & `CHANGELOG.md` Airlock (`INT-1987`, `SUB-2283`, `SUB-2256`)**: Enforces `EvaluatePreTagReleaseAirlock` inside `ReleaseReceiptGate` (`Cell 3.3`) prior to release commits or tags (`TC-SYNC-022`, `TC-SYNC-023`), validating `.goreleaser.yaml` Go `text/template` and v2.20+ `{{ .Env.<VAR> }}` secret-token syntax, statutory SemVer pin parity across `root.go`, `CHANGELOG.md`, `templates/nomos.yaml`, and `packaging/aur/PKGBUILD`, and cleanroom-safe `CHANGELOG.md` prose free of host user-home prefixes or Unicode dashes.

---

## [v4.16.2] - 2026-11-11

> GoReleaser v2.20 Secret-Token Interpolation Fix Enabling Live Automated `mgantlett/homebrew-tap` Cask Publishing & AUR Secret Hygiene (Apache 2.0)

### Release & Packaging (`.goreleaser.yaml`)
- **GoReleaser v2.20+ Secret-Token Interpolation (`SUB-2252`)**: Updates `homebrew_casks[].repository.token` (`{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}`) and `aurs[].private_key` (`{{ .Env.AUR_KEY }}`) to GoReleaser v2.20+ strict secret-only template syntax while preserving `index .Env` guards in `skip_upload`, enabling zero-touch `mgantlett/homebrew-tap` Cask updates on every tag push.

---

## [v4.16.1] - 2026-11-11

> Canonical `@AGENTS.md` `CLAUDE.md` SSoT Import Collapse, Root `nomos.yaml` Inline Extension Envelope, Plus Automated Homebrew Tap Publishing & GitHub Discussions Forms (Apache 2.0)

### CLI & Workspace Contracts (`nomos init`, `nomos update --sync-workspace`, `nomos verify`)
- **Canonical `@AGENTS.md` Import Singularity for `CLAUDE.md` (`SUB-2244`, `SUB-2245`)**: Switches `CLAUDE.md` scaffolding (`nomos init`), workspace contract synchronization (`nomos update --sync-workspace`), and template parity verification (`IntentParityGate`) from duplicating the full `AGENTS.md` protocol block to a canonical `@AGENTS.md` SSoT import pointer (Law 4 & Law 6), idempotently collapsing legacy marker blocks while preserving custom repository-specific Claude instructions.

### Configuration & Schema (`nomos.yaml`)
- **Inline Extension Envelope with Strict Governance Validation (`SUB-2247`)**: Preserves strict `yaml.v3` `KnownFields(true)` verification across the `governance` configuration tree (`TC-SSOT-05`) and root-level governance key typos (`quality_gates`, `boundary`, `thresholds`, `contracts`, `strict_mode`, `governence`, `defaultBranch`) while allowing external `nomos-plugin-*` and embedded library inline extension blocks in unified root `nomos.yaml` files.

### Packaging & Community (`packaging/aur/PKGBUILD`, `.github/DISCUSSION_TEMPLATE/`)
- **Automated Homebrew Tap Sync, Arch `PKGBUILD` Alignment & Discussion Forms (`SUB-2246`, `SUB-2248`)**: Enables automated GoReleaser `mgantlett/homebrew-tap` cask publishing on release tags, aligns `packaging/aur/PKGBUILD` Linux SHA-256 digests, adds structured `.github/DISCUSSION_TEMPLATE/` forms (`q-a.yml`, `ideas.yml`, `show-and-tell.yml`), and links GitHub Discussions across `README.md` and `CONTRIBUTING.md`.

---

## [v4.16.0] - 2026-11-11

> `PreToolUse` Host-Credential Read Airlock, 2PC Walkthrough Path & Credential Sanitization, Cross-Platform `nomos update` Provenance & Windows Swap Hardening, Strict `nomos.yaml` Schema Decoding, Plus Neutral External Plugin Discovery & CI Git Notes Verification (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook`)
- **Host-Credential Read Airlock & MCP `ArgsDigest` Scoping (`INT-1973`, `SUB-2239`, `SUB-2240`)**: Enforces `governance.boundary.credential_read_mode` (`host_deny` default, `strict`, `taint_only`) and `blocked_read_paths` globs in `nomos hook pre-tool` to fail closed on agent reads of host credential stores (`~/.aws`, `~/.ssh`, `~/.kube`, `~/.gnupg`, `~/.config/gcloud`, `.netrc`, `.npmrc`, `.pypirc`, SSH private keys, and cloud key JSONs) while tainting or blocking non-template `.env` reads (`TC-HK-025`), and scopes `ArgsDigest` credential path extraction to MCP file/resource tools so task titles or search strings mentioning credential filenames never false-positive.

### Task Lifecycle & 2PC Sync (`nomos task sync`)
- **2PC Walkthrough Path Scrubbing & Credential Airlock (`INT-1976`, `SUB-2241`)**: Enforces `SanitizeAndValidateWalkthrough` in `nomos task sync` prior to fast-forward merge (`TC-SYNC-021`), stripping `file://` schemes and repository/worktree root prefixes to repo-relative paths, scrubbing residual host home directory prefixes (`$HOME` and macOS, Linux, and Windows user home roots) to `~`, and failing closed on raw credentials (`ERR_SECRET_DETECTED`) or configured `prohibited_packages` tokens (`ERR_BOUNDARY_VIOLATION`) before persisting `refs/notes/nomos/releases` receipts.

### CLI & Distribution (`nomos update`, `plugin.go`, `install.sh`, `uninstall.sh`)
- **Cross-Platform `nomos update` Provenance & Rate-Limit Resilience (`INT-1976`, `SUB-2241`)**: Adds `AtomicReplaceBinaryWithBackup` so Windows self-updates move the active `.exe` to `.old` prior to atomic rename (`TC-UPD-006`), distinguishes Arch Linux `aur` (`/etc/arch-release`) from generic Linux `system` package-manager provenance on `/usr/bin/nomos`, resolves `/releases/latest` HTTP redirects before falling back to `api.github.com` (with `GITHUB_TOKEN` / `GH_TOKEN` Bearer authentication) to avoid anonymous GitHub API rate limits, and hardens `uninstall.sh` path handling for directories containing spaces.
- **Neutral `External Plugins` CLI Discovery (`SUB-2241`)**: Generalizes external `nomos-plugin-*` binary discovery under a neutral `External Plugins:` Cobra command group across the sibling executable directory, `GOBIN`, `GOPATH/bin` (`filepath.SplitList`), `~/.local/bin`, and `PATH`.

### Configuration & Workspace (`nomos.yaml`, `nomos.schema.json`)
- **Strict `KnownFields(true)` Config Decoding & Schema Disambiguation (`SUB-2241`)**: Enforces `yaml.NewDecoder` with `KnownFields(true)` in `config.LoadConfig` so unknown or misspelled `nomos.yaml` keys fail closed with `ERR_CONFIG_PARSE` (`TC-SSOT-05`), and disambiguates outer `GovernanceConfig.contracts` vs inner `QualityGatesConfig.contracts` descriptions in `nomos.schema.json`.

### Release, CI & Community Health (`.github/`, `CODE_OF_CONDUCT.md`)
- **CI Git Notes Verification & Open-Core Community Health (`SUB-2241`)**: Fetches `+refs/notes/nomos/releases:refs/notes/nomos/releases` in `.github/workflows/ci.yaml` and `release.yaml` so `nomos verify --all --strict` validates live `spec_digests` receipts in CI, adds `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1), aligns `.github/PULL_REQUEST_TEMPLATE.md` with DCO sign-off and 2-plane (`INT-*` / `SUB-*`) governance, and removes non-public ADR citations across embedded agent directives and CLI help surfaces.

---

## [v4.15.0] - 2026-11-11

> Verified `nomos update` Self-Updater & 24h Zero-Egress Notice, Canonical Root `nomos.yaml` Singularity & Scoped `spec_roots` Steel-Man Gates, Pre-Verify Rebase & SHA-256 Receipt Caching, Plus JIT Mneme Pitfall Recall & Area-Grouped Release Ledgers (Apache 2.0)

### CLI & Distribution (`nomos update`, `install.sh`, `uninstall.sh`)
- **Verified `nomos update` Self-Updater & Provenance Quarantine (`INT-1966`, `SUB-2219`, `SUB-2225`)**: Introduces `nomos update` (`--check`, `--json`, `--sync-workspace`) with SHA-256 `checksums.txt` verification prior to atomic binary `os.Rename`, package-manager provenance guards blocking standalone overwrites of Homebrew (`/opt/homebrew`), AUR/system (`/usr/bin`), and Nix store (`/nix/store`) binaries with exact upgrade remedies, a 24-hour cached update notice on `nomos version`, `nomos handshake`, and `nomos hook session-start` (`NOMOS_NO_UPDATE_CHECK=1` opt-out), and bounded `file://` cleanroom vector tests (`CDV_UP_01..04`).
- **Idempotent Downstream Workspace Sync (`INT-1966`, `SUB-2219`)**: Adds `nomos update --sync-workspace` to refresh downstream `nomos.yaml` `$schema` pins, `AGENTS.md` / `CLAUDE.md` protocol marker blocks (preserving repository-specific instructions outside markers), and `.agents/` hook scripts without overwriting custom project rules.
- **Zero-FHS POSIX `install.sh` Checksum Hardening (`SUB-2202`)**: Replaces `awk` and `sed` invocations in `install.sh` with pure POSIX parameter expansion for Zero-FHS NixOS and BusyBox/Ash environments, adds `NOMOS_LOCAL_CHECKSUMS` offline verification support, and enforces end-to-end POSIX `sh install.sh` and `sh uninstall.sh` (`NOMOS_PURGE_DATA=1`) lifecycle round-trip tests.

### Configuration & Workspace (`nomos.yaml`)
- **Canonical Root `nomos.yaml` Promotion (`INT-1967`, `SUB-2224`)**: Promotes Layer 1 governance configuration to canonical repository root `nomos.yaml` (`$schema=./nomos.schema.json`), aligns Layer 0 `intent/intent.md` references, and seals updated Layer 0 SHA-256 `spec_digests` receipts.
- **Zero-Fallback Config Lookup Singularity (`SUB-2220`, `SUB-2229`)**: Removes legacy `.agents/nomos.yaml`, `.nomos/nomos.yaml`, `intent/config/nomos.yaml`, `nomos.yml`, and `~/.nomos/` home fallback lookup aliases across `config`, `verify`, and `cli` so `<repoRoot>/nomos.yaml` (or explicit `NOMOS_CONFIG`) is the single authoritative configuration path under Law 6.

### Verification & Quality Gates (`nomos verify`)
- **Scoped `contracts.spec_roots` Steel-Man & Optional Schema Parity (`INT-1969`, `SUB-2226`)**: Declares explicit `verify_scope: diff`, `tdd_mode: strict`, and `coverage_mode: live` in `nomos.yaml`, scopes `IntentSteelmanGate` (`TC-TRC-005`) to configured `contracts.spec_roots` when targeting explicit files or subdirectories, and skips default `nomos.schema.json` checks in `ContractParityGate` when `schema_files` is omitted and no root schema file exists.
- **Isolated Sibling `INT-*` Parity & Template `$schema` Pin Normalization (`SUB-2223`, `SUB-2232`)**: Skips Substrate AST contract parity inside isolated `INT-*` worktrees so concurrent uncompiled sibling specs do not block Intent-plane configuration updates, and normalizes statutory `$schema` SemVer comment pins in `templates/nomos.yaml` during `chore(release)` commits.

### Task Lifecycle & 2PC Sync (`nomos task sync`)
- **Pre-Verify Trunk Rebase & 5-Minute Clean-Tree Receipt Cache (`INT-1961`, `SUB-2210`, `SUB-2222`)**: Reorders `rebaseOntoTrunkIfDiverged` ahead of gate execution in `nomos task sync` and caches 5-minute SHA-256 verification receipts keyed by `(treeSHA, trunkHeadSHA)` (`TC-SYNC-020`, `NOMOS_NO_VERIFY_CACHE=1` bypass), eliminating duplicate full-suite gate runs across `nomos task commit` -> `nomos task sync`.

### Ambient Hooks & Pacing (`nomos hook`)
- **JIT Post-Edit Pitfall Recall & Session Trunk Pacing (`INT-1962`, `SUB-2211`)**: Injects one-time directory-scoped Mneme pitfall recall on the first edit to a package in `nomos hook post-tool` and enforces session-keyed idle-trunk read-spiral pacing (`7/12` Carbon, `12/20` Standard) (`TC-MNM-030`).

### Episodic Memory (`nomos mem`)
- **Subword Feature Hashing for Local Vector Synthesis (`INT-1956`, `SUB-2203`)**: Upgrades pure-Go 64-dimensional local vector synthesis in `nomos mem` with subword feature hashing for morphological and identifier-stem recall accuracy.

### Session & Boundary Telemetry (`nomos session`)
- **Authorized `nomos-plugin-*` Workspace Boundaries in Sentinel (`INT-1960`, `SUB-2209`)**: Authorizes workspaces declaring `substrate/cmd/nomos-plugin-*` entrypoints and self-owned packages under `CHARTER-4-BOUNDARY` (`TC-SEN-010`) while preserving `80/100 SUSPICIOUS` boundary detection on external downstream consumers.

### Release & Packaging (`packaging/render-release-notes.sh`, `CHANGELOG.md`)
- **Area-Grouped Release Ledgers & Historical Backfill (`SUB-2233`)**: Groups SHA-linked `[INT-*]` and `[SUB-*]` commit ledgers in `packaging/render-release-notes.sh` across the 6 canonical Open-Core areas while preserving collapsible `<details>` architecture disclosures and parent `vX.Y.0` milestone rollups, and aligns historical `v4.0.0`..`v4.14.0` `CHANGELOG.md` entries to clean per-area headings.

---

## [v4.14.0] - 2026-11-10

> 5-Provider Ambient Lifecycle Parity & Native Cursor Hooks, Antigravity 2.0 Calibration & Deliberation Telemetry, AST Normative-Surface Twin Parity & Spec-Taint Scoping, Plus CapBAC Containment, Verify Memoization & Homebrew Cask Distribution (Apache 2.0)

### CLI & Distribution (`install.sh`, `.goreleaser.yaml`)
- **Homebrew Cask Distribution & Execution Substrate Framing (`INT-1952`, `SUB-2189`)**: Adds GoReleaser v2 Homebrew Cask packaging (`brew install --cask mgantlett/tap/nomos`), clears macOS Gatekeeper quarantine attributes in `install.sh` and Cask post-install hooks, and aligns deterministic execution substrate category framing across `README.md` and `intent/`.

### Ambient Hooks & Pacing (`nomos init`, `nomos hook`, `nomos handshake`)
- **Native Cursor Hook Scaffolding & 5-Provider Default Parity (`INT-1951`, `SUB-2188`)**: Scaffolds `.cursor/hooks.json` (`version: 1`) out of the box in `nomos init` and `nomos hook install --cursor`, establishes full 5-provider default lifecycle parity across `Claude Code`, `Gemini CLI`, `OpenCode`, `Cursor`, and `Antigravity`, and normalizes Cursor stdin/stdout lifecycle events (`sessionStart`, `preToolUse`, `beforeShellExecution`, `postToolUse`, `afterFileEdit`, `stop`).
- **Antigravity 2.0 Lifecycle Calibration & Turn-0 Pacing (`SUB-2177`, `SUB-2178`)**: Aligns Antigravity 2.0 `PostInvocation` hook lifecycle handling, Turn-0 prefix stability, loop digest telemetry, and Carbon pacing profiles.

### Verification & Quality Gates (`nomos verify`)
- **AST Normative-Surface Twin Parity (`INT-1948`, `SUB-2185`)**: Enforces `NormativeSurfaceDelta` and `ValidateWorktreeIntentTwin` across exported Go contracts, CLI flag surfaces, and embedded configuration templates.
- **Concurrent Sibling-Twin Isolation & Spec-Taint Scoping (`SUB-2188`, `SUB-2194`)**: Isolates concurrent sibling-twin contract obligations (`ResolveActiveWorktreeSpecRoots`), scopes spec-taint receipt obligations to modified files, and expands Go verification to 1-hop reverse-importer test packages.
- **CapBAC Glob Containment & `VerifyContext` Pipeline Memoization (`SUB-2180`, `SUB-2181`)**: Hardens workspace glob containment and protected-branch file deletion blocking, adds a SQLite `PRAGMA user_version` schema initialization fast-path, and memoizes `VerifyContext` configuration and Git file-scope lookups across all 42 Definition of Done quality gates.

### Task Lifecycle & 2PC Sync (`nomos task`)
- **Zero-Drift `spec_digests` Auto-Sealing & Walkthrough Diff Provenance (`SUB-2170`, `SUB-2172`)**: Automatically seals `spec_digests` on standalone zero-drift `INT-*` syncs and verifies walkthrough `## Changes` file provenance against the active worktree diff.

### Episodic Memory (`nomos mem`)
- **Automatic Dead-End Memory Capture (`SUB-2172`)**: Automatically records `dead_end` Mneme memories on reverted worktree edits.

### Session & Boundary Telemetry (`nomos session`)
- **Polymorphic `DeliberationMetrics` & Antigravity Brain Discovery (`INT-1954`, `SUB-2191`, `SUB-2179`)**: Adds `DeliberationMetrics` (`thinking_bytes`, `thinking_step_count`, `max_read_streak`, `mutation_step_count`, `deliberation_to_mutation_ratio`, and `transcript_full.jsonl` recovery) in `nomos session show` alongside `antigravity/brain` session discovery across supported agent transcript formats.

---

## [v4.13.2] - 2026-11-10

> Self-Hosted 4-Layer Progressive Intent Compiler, 8 Canonical Layer 2 Domain Specifications (`99` Compiled Go SSoT Contracts, `186` Executable `@verifies TC-*` Vectors, `0` `NCTE-100` Diagnostics), and `N=108` Multi-Lab Open-Weight Brownfield Audit (Apache 2.0)

### Intent & Specifications (`intent/intent.md`, `intent/architecture/specs/`)
- **8 Canonical Layer 2 Domain Specifications (`INT-1943`, `SUB-2147`, `INT-1944`, `SUB-2157`)**: Consolidated 37 historical per-task specifications into **8 Canonical Layer 2 Domain Specifications** under `intent/architecture/specs/` mapped 1-to-1 to `substrate/` packages (`go_ssot_and_json_schema_spec.md`, `nomos_hermetic_worktree_spec.md`, `task_lifecycle_cli_spec.md`, `nomos_intent_traceability_spec.md`, `tree_sitter_ast_gate_spec.md`, `mneme_memory_spec.md`, `nomos_controlled_technical_english_spec.md`, and `software_physics_engine_spec.md`) anchored by the Layer 0 North Star Charter (`intent/intent.md`).
- **NCTE-100 Controlled-English Lexicon Alignment (`SUB-2146`)**: Aligned the `NCTE-100` controlled lexicon (`Plane` vs. architectural `layer` terminology) and validated all 9 canonical Intent documents (`2,671` lines) with zero `NCTE-100` diagnostics under `IntentSteelmanGate` (`Cell 4.1`).

### Verification & Quality Gates (`nomos verify`, `intent/config/nomos.yaml`)
- **Full Compiled Go SSoT & Executable `TC-*` Obligation Parity (`INT-1944`, `SUB-2157`, `SUB-2150`, `SUB-2158`)**: Synthesized all `99` compiled Go SSoT struct and interface contracts (`VerifySpecTypeSignatures`) and `186` executable `@verifies TC-*` verification vectors (`VerifyExecutableObligations`) across `core/config`, `core/types`, `core/workspace`, `core/capbac`, `core/storage`, `modules/task`, `modules/verify`, `modules/verify/treesitter`, `modules/mem`, `modules/ncte`, `modules/handshake`, `modules/session`, `modules/telemetry`, and `cli`. Enabled repository-wide `strict_types: true`, `verify_obligations: true`, and `fail_on_stale_obligations: true` across `intent/architecture/specs` in `intent/config/nomos.yaml` with sealed Layer 3 SHA-256 `spec_digests` receipts in `refs/notes/nomos/releases`.

### Empirical Benchmarks (`README.md`)
- **`N=108` Multi-Lab Open-Weight Brownfield Benchmark (`SUB-2154`)**: Documented the `108`-episode paired brownfield audit (`54` Stock vs. `54` Nomos `temp=0.0` episodes across Python, TypeScript, and Go on `6` open-weight models from `4` labs sized for single-card `16GB-24GB` GPUs), lifting verified resolution on the `14B-32B` production tier from **`27.8%` (`10 / 36`, `0%` hexagonal layer purity)** to **`86.1%` (`31 / 36`, `+58.3` pp, `100%` layer purity on all merged diffs)** while cutting token burn **`-43%` to `-57%`** on multi-file refactors, alongside Day-2 brownfield escape-hatch documentation.

---

## [v4.13.1] - 2026-11-10

> Zero-Ceremony Multi-Agent Onboarding Invisibility, 4-Layer Progressive Intent Compiler Charter (`TC-INT-01..06`), and Ledger-First GitHub Release Notes (Apache 2.0)

### CLI & Distribution (`nomos init`)
- **Hermetic `--only` Baseline Governance Commit (`SUB-2133`)**: Scopes `nomos init`'s initial Git baseline commit via `git commit --only -- <governance-paths>` so pre-staged developer index changes are never swept into the governance commit on either unborn or existing `HEAD` repositories, and prints a 1-line `worktrees/<repo>-<task>` sandbox orientation hint on initialization.

### Ambient Hooks & Pacing (`nomos init`, `nomos hook`)
- **Out-of-the-Box Claude Code & Gemini CLI Hook Scaffolding (`SUB-2133`, `INT-1940`)**: `nomos init` now deterministically scaffolds and additively merges `.claude/settings.json` and `.gemini/settings.json` alongside `.agents/hooks.json` and `.opencode/plugins/nomos.ts`, activating compiled `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` governance immediately on a developer's first session without requiring a separate `nomos hook install` command.
- **GUI IDE `$PATH` Fallback, Teammate Missing-Binary Grace & Trunk Escape Hints (`SUB-2133`)**: Embeds POSIX `$PATH` fallback resolution (`$GOBIN`, `$HOME/go/bin`, `$HOME/.local/bin`, `$HOME/bin`) with graceful `{}` (`exit 0`) fallback when `nomos` is absent across `DefaultClaudeSettingsHooksJSON`, `DefaultGeminiSettingsHooksJSON`, and `.opencode/plugins/nomos.ts`, and adds copy-paste coding-agent and `git stash && nomos task chore` instructions when a human `git commit` hits a protected trunk branch.

### Intent & Specifications (`intent/intent.md`)
- **Stage 0 Intent Compiler Charter & Executable Obligation Proofs (`INT-1941`, `SUB-2134`)**: Codifies the 4-Layer Progressive Intent Compiler (`Layer 0` Charter -> `Layer 1` `nomos.yaml` Physics -> `Layer 2` JIT Slice Specs -> `Layer 3` Hermetic Worktree & Git Notes SHA-256 Receipts) in `intent/intent.md`, enables `strict_types`, `verify_obligations`, and `fail_on_stale_obligations` in `intent/config/nomos.yaml`, and binds executable verification vectors `TC-INT-01..06` in `substrate/cli/intent_manifesto_obligations_test.go` with sealed SHA-256 `spec_digests`.

### Release & Packaging (`packaging/render-release-notes.sh`)
- **Ledger-First GitHub Release Notes & Collapsible Milestone Rollups (`SUB-2129`)**: Reorders `packaging/render-release-notes.sh` so GitHub Releases lead with the 1-line release summary and chronological SHA-linked `[INT-*]` / `[SUB-*]` commit ledger (`## Changelog`), followed by collapsible `<details>` disclosures for detailed architecture notes and parent `vX.Y.0` milestone rollups on patch releases.

---

## [v4.13.0] - 2026-11-09

> 11-Language Polyglot Architectural Obligation Proofs: Declarative Hexagonal Layer Boundaries (`<5ms`), Executable `TC-*` Vector Binding, Fenced Spec Contract Signature Parity, Cryptographic `spec_digests` SHA-256 Taint Receipts & Turn-0 Handshake Alerts, Plus Polyglot Subprocess `GIT_*` Isolation & POSIX Installer Hardening (Apache 2.0)

### Verification & Quality Gates (`nomos verify`, `nomos handshake`, `nomos status`, `nomos.yaml`)
- **Declarative Polyglot Layer Boundary Enforcement (`INT-1935`, `SUB-2004`, `INT-1936`, `SUB-2007`)**: Adds `architecture_boundaries` rules (`name`, `from`, `forbid_imports`, `allow_imports`) under `governance.quality_gates` in `nomos.yaml` and `nomos.schema.json`. Evaluates package and module imports in `<5ms` on `PostToolUse` (`EvaluateFastFileRadar`) and during `nomos verify` across Go AST, TypeScript/JavaScript/Svelte (`import`/`export from`/`require`), Python (`import`/`from ... import`), Rust (`use`/`mod`), Ruby (`require`/`require_relative`), C/C++ (`#include`), Java/Kotlin (`import`), and C# (`using`), with worktree governance-downgrade protection against pinned `main` and live self-dogfooding (`core-purity`, `modules-independence`) in Nomos itself.
- **Polyglot Fenced Spec Contract Signature Parity (`SUB-2002`, `SUB-2007`)**: Adds `strict_types: true` enforcement across fenced Markdown specification code blocks (`go`, `ts`/`tsx`/`js`/`mjs`, `python`, `rust`, `java`, `kotlin`, `csharp`) so declared struct, interface, `type`, `class`, and `@dataclass` fields fail closed on implementation drift across all supported ecosystems, alongside polyglot `@intent <spec>.md #<TypeName>` anchor verification.
- **Executable `TC-*` Specification Obligation Binding (`SUB-2002`, `SUB-2007`, `SUB-2010`, `INT-1937`, `SUB-2011`)**: Adds `verify_obligations: true` scanning Markdown specification vector tables (`TC-[A-Z0-9]+-[0-9]+`) against verification suites across all 11 polyglot test conventions (`*_test.go`, `*.test.ts`/`*.spec.mjs`, `test_*.py`/`*_test.py`, `*_test.rs`, `*_test.rb`/`*_spec.rb`, `*Test.java`, `*Test.kt`, `*Tests.cs`, `*_test.cpp`) via `@verifies TC-...` comments or canonical test identifiers, while exempting upstream `INT-*` specification-authoring tasks from chicken-and-egg failures before their paired `SUB-*` task binds tests.
- **Cryptographic `spec_digests` SHA-256 Taint Tracking & Turn-0 Handshake Alerts (`SUB-2003`, `SUB-2007`, `SUB-2010`)**: Stamps `spec_digests` (`map[spec_path]sha256`) into `refs/notes/nomos/releases` 2PC receipts on `SUB-*` releases (resolving `@intent` refs, `@verifies TC-*` vectors, `--twin INT-*` diffs, and active `spec_roots` contracts), surfaces stale downstream obligations immediately on Turn 0 in `nomos handshake` (`--json` and CLI) and `nomos status`, and blocks stale merges when `fail_on_stale_obligations: true` is active.
- **Subprocess `GIT_*` Sanitization (`SUB-1990`)**: Strips inherited `GIT_DIR` / `GIT_WORK_TREE` / `GIT_INDEX_FILE` variables across all 22 subprocess invocations in all 7 polyglot test and coverage runners.

### Task Lifecycle & 2PC Sync (`nomos task commit`, `nomos task sync`)
- **Bare-Config Self-Healing & Whitespace Duplicate-Commit Guard (`SUB-1990`)**: Self-heals accidental `core.bare=true` config mutations during `nomos task sync` and suppresses pure trailing-whitespace IDE auto-save diffs after `nomos task commit` across all languages.
- **Optional Post-Sync Lifecycle Hook (`SUB-1994`)**: Invokes `.agents/scripts/post-sync.sh` when executable after `nomos task sync`.

### CLI & Distribution (`install.sh`, `uninstall.sh`)
- **POSIX `sh` Installer Compatibility (`SUB-1989`, `INT-1933`)**: Makes `install.sh` and `uninstall.sh` strictly POSIX `sh`-compatible (`curl ... | sh`) and streamlines `README.md` header badges.

### Empirical Benchmarks (`Qwen3.8-27B`)
- **Paired Live-Model Brownfield Benchmark (`Qwen3.8-27B`, `temp=0.0`)**: Across paired Python, TypeScript, and Go multi-module brownfield repositories with tempting `infra/` helpers and passing unit tests, `nomos v4.13.0` achieved **`100%` (`6/6`)** architectural obligation compliance (`+9s` average self-remediation overhead) versus **`50%` (`3/6`, `0/3` on hexagonal layer purity)** under stock `CLAUDE.md` + unit test harnesses.

---

## [v4.12.1] - 2026-11-09

> Episodic Pitfall Scope Hygiene, Polyglot TAP/Exception Diagnostic Windowing, Hybrid Release-Notes Ledger Compiler & Coverage Persistence (Apache 2.0)

### Episodic Memory (`nomos mem`, `nomos handshake`)
- **Automatic Dead-Scope Tombstoning & Cross-Folder Habit Consolidation (`SUB-1965`, `SUB-1960`)**: Auto-tombstones stale pitfall scopes whose target directories no longer exist, skips ephemeral test-assertion noise, prioritizes `touchedFiles` during multi-violation gate trap distillation, and scopes cold-start `nomos handshake` / task-start recalls in read-only mode so `access_count` is never artificially inflated.

### Verification & Quality Gates (`nomos verify`)
- **Salient Polyglot Toolchain Diagnostic Window (`SUB-1962`, `SUB-1960`)**: Anchors the 400-byte `extractSalientToolchainWindow` diagnostic excerpt on the first `not ok`, `FAIL`, or exception marker across multi-hundred-line Node TAP, Python, Rust, `.ts`, and `.svelte` test outputs rather than truncating inside `TAP version 13` passing headers.
- **Live Statement Coverage Persistence & Test Complexity Ratchet (`SUB-1952`)**: Persists `.tmp/coverage.out` during `nomos verify`, extracts a testable CLI verification seam, and enforces test cyclomatic complexity alongside local-symbol TDD coupling.

### Release & Packaging (`packaging/render-release-notes.sh`)
- **Hybrid GitHub Release Notes & Commit Ledger Compiler (`SUB-1953`)**: Wires `.github/workflows/release.yaml` to `packaging/render-release-notes.sh` so curated `CHANGELOG.md` highlights, automatic `vX.Y.0` milestone patch rollups, and the chronological SHA-linked `[INT-*]` / `[SUB-*]` commit ledger render together on every GitHub Release.

---

## [v4.12.0] - 2026-11-08

> Zero-Config Brownfield Auto-Detection & Monotonic AST Ratchet, Full 11-Language Polyglot Verification Parity, Switchable `nomos.yaml` Brownfield Modes, Navigation-First Handshake & Read-Spiral Gyroscope (Apache 2.0)

### CLI & Distribution (`nomos init`)
- **Zero-Config Brownfield Onboarding (`nomos init`)**: Detects non-empty repositories lacking dual-hemisphere `intent/` + `substrate/` roots, configures `strict_mode: false` automatically, and discovers active top-level source directories out of the box without Day-1 directory migrations.

### Configuration & Workspace (`nomos.yaml`, `nomos.schema.json`)
- **Declaratively Switchable Brownfield Modes (`nomos.yaml`, `nomos.schema.json`)**: Adds granular brownfield switches under `governance.quality_gates`: `verify_scope` (`"diff"` | `"repo"`), `tdd_mode` (`"off"` | `"ratchet"` | `"diff"` | `"strict"`), `coverage_mode` (`"off"` | `"artifact"` | `"live"`), and `tdd_exclude` glob patterns.

### Verification & Quality Gates (`nomos verify`)
- **Monotonic AST Merge-Base Ratchet (`git merge-base`)**: Under `strict_mode: false`, ratchets file LOC (`>400`), function LOC (`>60`), McCabe cyclomatic complexity `V(G)` (`>15`), Cognitive Debt Score (`CDS`), structural clones, magic-string duplication, forbidden path literals, and unchecked error hygiene strictly against the trunk baseline so legacy God files pass untouched while net complexity or debt regressions fail closed with exact `[BASELINE main: ... | delta ...]` diagnostics.
- **11-Language Structural AST, Anti-Shim & Error-Hygiene Parity**: Closes all 12 polyglot verification gaps across Go, Rust, C/C++, Python, Ruby, TypeScript, JavaScript, Svelte (`.svelte` `<script>` extraction), Java, Kotlin, and C-Sharp, including overloaded-signature baseline metric retention (`max(V(G), LOC)`) so TypeScript/C++/JVM overloads never mask real complexity growth.
- **Zero-Dependency Live & Artifact Statement Coverage**: Adds zero-dependency `NODE_V8_COVERAGE` live statement coverage probes for JS/TS, zero-dependency `python3 -m trace` fallback alongside `coverage.py` / LCOV / Cobertura XML, Ruby `Coverage` / SimpleCov ingestion, and JaCoCo XML / Cobertura report verification for Java, Kotlin, C-Sharp, Rust, and C/C++.

### Ambient Hooks & Pacing (`nomos handshake`, `nomos hook post-tool`)
- **Noise-Filtered Cold-Start Topology Brief (`SynthesizeBrownfieldTopology`)**: Emits `Governance Mode: BROWNFIELD (scope=diff, tdd=ratchet, cov=live)`, lowers `Core Modules` discovery to `>= 120` LOC, removes upfront prompt scolding that induced reasoning-model analysis paralysis, and filters `_`-prefixed demo directories (`_examples/`), `typings/`, `.test-d.ts`, `*-test` harnesses, and `quality_gates.exclude` globs from cold-start orientation.
- **Worktree Read-Spiral Gyroscope (`cmd_hook_pacing.go`)**: Tracks consecutive read/search calls (`Read`, `Grep`, `Glob`, read-only shell searches) inside active `EDIT` worktrees and emits non-blocking 1-line pacing nudges at `12` and `20` consecutive reads without a source edit, resetting immediately upon any file mutation.

---

## [v4.11.0] - 2026-11-07

> Turn-Termination `nomos hook stop` Gate, Outbound Egress Airlock, Additive `nomos hook install`, Scratch-Artifact & Trunk-Redirect Guards, GoReleaser v2 Schema Gate & Mermaid Spec Diagrams (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook`, `nomos.yaml`)
- **Turn-Termination Anti-Phantom Gate (`nomos hook stop`)**: Blocks unverified active `EDIT` worktree exits until `nomos verify` clears `DirtyUnverifiedEdits`, rejects `--container-sandbox` exits containing syntax errors or protected verifier mutations, and honors `stop_hook_active: true` for single-pass re-entry safety.
- **Additive Multi-Harness Installer (`nomos hook install`)**: Non-destructively merges `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` hook bindings across Claude Code (`.claude/settings.json`), Gemini CLI (`.gemini/settings.json`), OpenCode (`.opencode/plugins/nomos.ts`), and Cursor (`--global` or `--project`) while preserving existing user configuration.
- **Outbound Egress Airlock & Loop Circuit-Breaker (`nomos hook pre-tool`, `nomos hook post-tool`)**: Inspects outbound HTTP/shell calls against cloud metadata SSRF endpoints (`169.254.169.254`, `metadata.google.internal`, `fd00:ec2::254`), inline credential exfiltration tokens, and declarative `egress_allowlist` domains in `nomos.yaml` (`nomos.schema.json`), alongside upgraded loop-signature circuit breaking and additive Git Notes refspec sync.
- **Protected-Trunk Shell Redirect Guard (`cmd_hook_trunk_guard.go`)**: Blocks shell redirections (`>`, `>>`, `tee`, `sed -i`, `cp`, `mv`) targeting protected `main` trunk paths during `PreToolUse` before filesystem mutation occurs.

### Verification & Quality Gates (`nomos verify`, `nomos init`)
- **Scratch-Artifact DoD Gate (`StagedBoundariesGate`)**: Rejects staged or committed `*.tmp`, `*.bak`, `*.orig`, `*.swp`, `*~`, and `.DS_Store` scratch artifacts outside `.tmp/` across both uncommitted index state and active task branch commits (`ERR_BOUNDARY`), paired with self-healing `.gitignore` and `.git/info/exclude` defaults.

### Intent & Specifications (`intent/architecture/specs/`)
- **Machine-Verified Mermaid Architecture Diagrams**: Converts ASCII box-drawing architecture diagrams across Open-Core specifications into standard `flowchart` Mermaid v11 blocks validated against orphan-state and syntax invariants.

### Release & Packaging (`release.yaml`, `.github/workflows/ci.yaml`)
- **Strict GoReleaser v2 Schema Gate & Patch Note Rollup**: Enforces `yaml.Decoder.KnownFields(true)` against GoReleaser v2 config structs in `goreleaser_contract_test.go`, runs `goreleaser check` in CI on every push/PR, and rolls parent `vX.Y.0` milestone notes into `vX.Y.Z` patch releases so `/releases/latest` stays self-contained.

---

## [v4.10.1] - 2026-11-06

> GoReleaser v2 AUR Schema Alignment & Precompiled Multi-Arch Release Artifacts (Apache 2.0)

### Release & Packaging (`.goreleaser.yaml`, `release.yaml`)
- **GoReleaser v2 `config.AUR` Schema Parity**: Removes unsupported `options` array from `.goreleaser.yaml` `aurs:` (relying on GoReleaser v2's native `!strip` PKGBUILD template) and guards `AUR_KEY` lookup via `index .Env "AUR_KEY"` so GitHub Release tarballs, `checksums.txt`, and rendered AUR `PKGBUILD` artifacts build deterministically whether or not `secrets.AUR_KEY` is configured.

---

## [v4.10.0] - 2026-11-06

> Ambient OpenCode Plugin Hooks, First-Class Rails 8 Minitest Runner, Arch/Omarchy `nomos-bin` AUR Packaging, Pure NixOS Zero-FHS Hooks & Tree-Hash Toolchain Receipt Caching (Apache 2.0)

### Ambient Hooks & Pacing (`nomos init`, `nomos hook`)
- **Zero-Touch `.opencode/plugins/nomos.ts` Scaffolding**: Scaffolds a zero-dependency TypeScript OpenCode plugin during `nomos init` with idempotent `NOMOS_OPENCODE_PLUGIN_START`/`END` markers that preserve existing user plugins.
- **Sub-5ms OpenCode Payload Normalization**: Normalizes OpenCode `tool.execute.before` and `tool.execute.after` payloads across `bash`, `write`, `edit`, `apply_patch` (`*** Update File:` / `*** Add File:` / `*** Delete File:` target extraction), `read`, `list`, `glob`, `grep`, and `lsp`.

### Verification & Quality Gates (`nomos verify`)
- **Automatic `bin/rails test` & `bundle exec rake test` Execution**: Detects `test/**/*_test.rb` Minitest suites alongside Ruby `InstructionSequence` bytecode compilation, executes `bin/rails test` (or `bundle exec rake test` under `RAILS_ENV=test`), and rejects both failing assertions and non-zero Minitest skip counts (`Ruby(compile+minitest)`).
- **Content-Addressed Tree-Hash Receipt Cache (`nomos-toolchain-receipt-v2`)**: Canonicalizes relative and absolute worktree paths and hashes live Git blob OIDs (`ls-tree` + working-tree overlay + polyglot stack results) so `nomos verify`, `pre-commit`, `nomos task sync`, and fast-forward trunk checks share cryptographic toolchain receipts without redundant test suite runs.

### Task Lifecycle & 2PC Sync (`nomos task sync`, `.git/hooks/`)
- **Zero-FHS NixOS Hooks & Static Auto-Recompilation**: Removes external `sed` dependencies from `.git/hooks/commit-msg` via pure Bash parameter expansion for minimal NixOS sandboxes, and enforces `CGO_ENABLED=0` (`-trimpath -ldflags="-s -w"`, zero ELF `PT_INTERP` dynamic linker leakage) during `nomos task sync` binary recompilation.

### Release & Packaging (`packaging/aur/PKGBUILD`, `.goreleaser.yaml`)
- **Arch Linux / Omarchy `nomos-bin` AUR Target**: Adds `packaging/aur/PKGBUILD` (`x86_64` and `aarch64` static `CGO_ENABLED=0` binaries with Apache-2.0 license staging) and automated GoReleaser `aurs:` publishing.

---

## [v4.9.0] - 2026-11-06

> First-Class Ruby & Rails Polyglot AST Gating, Shell-Segment Sentinel Tokenizer, Mneme Locality Boost, RLVR/DPO Session Export & Flat 39-Spec Blueprint (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **Embedded `ruby.scm` & Structural Ruby Scanner**: Adds first-class `.rb` and `.rake` verification covering `def`, singleton `def self.*`, predicate/bang/setter methods, Ruby 3 endless methods (`def valid? = ...`), SQL/HTML heredoc (`<<~SQL`) and `=begin`/`=end` stripping, modifier `if`/`unless`/`while`/`until` disambiguation, cyclomatic complexity, and `raise NotImplementedError` / empty-block anti-shim rejection.
- **RSpec/Minitest Skip Guards & Rails Impact Mapping**: Rejects skipped test declarations (`xit`, `xdescribe`, `xcontext`, Minitest `skip`), maps Rails `app/<layer>/<name>.rb` edits directly to paired `spec/` and `test/` suites, and validates syntax via `RubyVM::InstructionSequence.compile_file`.

### Ambient Hooks & Pacing (`nomos hook`)
- **Shell-Segment-Aware `evalHermeticWorktree`**: Tokenizes chained shell segments (`&&`, `;`, `||`, `|`, `\n`), tracks sequential `cd <dir>` state and `git -C <dir>` targets when ambient working directory metadata is absent, and ignores quoted `git commit` strings inside read-only search commands.
- **Slow-Step Timeout & High-Churn Checkpoint Advisories**: Emits `PostToolUse` slow-step timeout alerts and high-churn worktree checkpoint advisories before context drift accumulates.

### Episodic Memory (`nomos mem`)
- **Mneme Repo-Locality Boost & Global Axiom Cap**: Applies a `1.25x` ranking boost to exact `repo_id` matches during scoped recall and caps unscoped `global` `axiom` slots at `max(1, limit/3)` with sparse backfill.

### Session & Boundary Telemetry (`nomos session`)
- **Joined `nomos session export`**: Synthesizes RLVR/DPO-ready trajectory datasets joined directly with `refs/notes/nomos/releases` 2PC receipts.

### Intent & Specifications (`intent/architecture/specs/`)
- **Flat 39-Spec Steel-Man Singularity (`IntentSteelmanGate`)**: Consolidates all 39 canonical specifications under `intent/architecture/specs/*_spec.md` on a single 1-indexed NCTE-100 blueprint with compiled Go SSoT contracts, `TC-*` verification vectors, Mermaid syntax validation, and orphan-state guards.

### Release & Packaging (`release.yaml`)
- **Automated `CHANGELOG.md` Release Pipeline**: Bundles root `CHANGELOG.md` into release archives and extracts matching `## [vX.Y.Z]` sections directly into GoReleaser GitHub Releases (`release.yaml`), failing closed if release notes are missing.

---

## [v4.8.0] - 2026-11-05

> Verified Host/Tool-Health Snapshots, Receipt-Trace Parity, Configurable Model-Profile Advisories & Terminal-Bench 2.0 Harbor Results (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook`, `nomos.yaml`)
- **Session-Start Host & Toolchain Snapshot**: Captures deterministic host and language-toolchain health diagnostics on `SessionStart` without external network probes or global state mutation.
- **Per-Model Turn & Churn Calibration**: Adds configurable model-profile advisory thresholds in `nomos.yaml` and `nomos.schema.json` so high-autonomy frontier reasoning models and local MoE models receive calibrated checkpoint guidance without false-positive interruptions.

### Task Lifecycle & 2PC Sync (`nomos task sync`)
- **Graceful 2PC Receipt-Trace Parity**: Seals hook-trace provenance into `refs/notes/nomos/releases` receipts during `nomos task sync` with deterministic fallback when tasks execute outside ambient hook wrappers.

### Empirical Benchmarks (`Terminal-Bench 2.0`)
- **89-Task Harbor Cleanroom A/B Evaluation**: Publishes full containerized A/B benchmark artifacts (`42.7% -> 51.7%`, `+9.0 pp`, `38 -> 46` solved tasks, zero regressions across 38 baseline passes).

---

## [v4.7.0] - 2026-11-05

> Self-Cleaning Nomos Mneme, Binary Context Guard, Interactive Hang Guard & Session Budget Radar (Apache 2.0)

### Episodic Memory (`nomos mem`, `nomos handshake`)
- **Per-Task Reinforcement Deduplication & Read-Only Recall**: Deduplicates active task handshake reinforcement to at most `+1` per `(task_id, memory_id)`, marks interactive `nomos mem recall` read-only, and gates built-in Tier-C AST graduation candidates on true cross-task topic recurrence (`>= 2`).
- **Symbol-Delta Fixes, Unscoped Cooling & Active-Default `nomos mem list`**: Prepends refactored file and verified symbol inventories into distilled `Verified Fix` entries, cools single-occurrence pitfalls older than 14 days out of unscoped handshakes, and defaults `nomos mem list` to `active` records with `--status`.

### Ambient Hooks & Pacing (`nomos hook pre-tool`, `nomos hook post-tool`)
- **Binary Read Guard & Interactive Hang Guard**: Blocks accidental full-file reads of binary/archive/sqlite blobs before context saturation and intercepts unflagged interactive CLI prompts (`apt-get install` without `-y`, unpaged `git log`, interactive editors) before session hangs.
- **Session Budget Radar**: Surfaces deterministic step-budget consumption advisories in `PostToolUse` before lease exhaustion.

---

## [v4.5.2] - 2026-11-04

> Legal Notice Packaging, `SecretGate` Cache Pruning & 42-Gate CI Release Pre-Flight (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **`SecretGate` Cache Pruning & Credential Bounds**: Prunes gitignored framework build caches during secret scans, enforces word-bounded credential regexes, and aligns Cell 2.3 deterministic gate proofs.

### Release & Packaging (`.goreleaser.yaml`, `uninstall.sh`, `ci.yaml`)
- **Statutory Apache-2.0 Packaging & Uninstall Hygiene**: Bundles `NOTICE`, `THIRD_PARTY_NOTICES.md`, and the verbatim SPDX `Apache-2.0` appendix across GoReleaser archives, updates `uninstall.sh` for clean `NOMOS_HOME` removal, and runs full 42-gate CI pre-flight verification prior to release cuts.

---

## [v4.5.1] - 2026-11-04

> Native Gemini CLI (`BeforeTool` / `AfterTool`) Hook Protocol Parity for Harbor Benchmarks (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook pre-tool`, `nomos hook post-tool`)
- **Native `@google/gemini-cli` Hook Parity**: Distinguishes Gemini CLI `BeforeTool` and `AfterTool` JSON hook payloads from Claude Code `PreToolUse` / `PostToolUse` events, emitting canonical `{"decision":"allow"|"deny","reason":"..."}` responses on `BeforeTool` and `{"hookSpecificOutput":{"additionalContext":"..."}}` diagnostics on `AfterTool`.
- **Directory & Search Path Target Extraction**: Inspects `dir_path` arguments across `list_directory`, `glob`, and `search_file_content` so directory scans targeting protected Harbor verifier or oracle paths (`/logs/verifier`, `/solution`, `/tests`) are deterministically blocked in `--container-sandbox` mode.

---

## [v4.5.0] - 2026-11-04

> Container Sandbox Anti-Reward-Hacking Hooks, Configurable Intent Twin Mode & Zero-Footprint Telemetry (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook --container-sandbox`)
- **Runtime Anti-Reward-Hacking Guard**: Physically blocks reads, writes, and shell commands targeting protected benchmark verifier and oracle paths (`/logs/verifier`, `/solution`, `/tests`) while permitting full-container OS sysadmin operations (`/etc`, `/tmp`, `/var`, `sqlite3`).
- **Zero-Footprint Container Execution**: Skips global SQLite telemetry initialization during `nomos hook` and `nomos version` invocations so disposable evaluation containers remain 100% free of global state.

### Configuration & Workspace (`nomos.yaml`, `nomos.schema.json`)
- **Configurable Bilateral Intent Twin Policy (`intent_twin_mode`)**: Adds `intent_twin_mode` (`off`, `feat`, `strict`) to `nomos.yaml` and `nomos.schema.json`, defaulting to `off` for friction-free solo adoption while enabling `feat` or `strict` bilateral `INT-*`/`SUB-*` pairing on team and regulated repositories.

---

## [v4.4.1] - 2026-11-03

> Brownfield Go Workspace Hardening & Golden Conventional Commits with Git Trailers (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **Ancestor `go.work` Discovery & Dynamic Go Directives**: Honors explicit ancestor `go.work` module membership in `nomos verify` and derives transient `go.work` Go directives dynamically from module `go.mod` versions.

### Ambient Hooks & Pacing (`nomos init`, `nomos hook`)
- **Scoped Pre-Tool & Git Exclude Hygiene**: Anchors operational `.git/info/exclude` entries cleanly, removes legacy `jq` shell fallback guards from hook templates, and scopes `PreToolUse` toolchain install guards to dirty or non-main trees.

### Task Lifecycle & 2PC Sync (`nomos task commit`, `nomos verify`)
- **Flowing Commit Bodies & `Task:` / `Twin:` Trailers**: Upgrades commit schema enforcement from rigid Markdown headers to readable Golden Conventional Commits (`[KEY] type(scope): summary`) paired with machine-readable `Task:` and `Twin:` RFC 822 Git trailers.

---

## [v4.4.0] - 2026-11-03

> Compositional Capability-Flow Taint Guard, Zero-Trust MCP Boundary Interceptor (IFC) & Multi-Repo Ledger Isolation (Apache 2.0)

### Ambient Hooks & Pacing (`nomos hook pre-tool`)
- **Cross-Tool Taint Propagation**: Tracks untrusted external inputs across sequential tool invocations in `PreToolUse` so prompt-injected payloads read from files, web fetches, or logs cannot exfiltrate secrets or execute privileged shell mutations.
- **Deterministic Pre-Flight Blocking**: Evaluates compositional capability-flow rules in sub-millisecond compiled Go outside the model context window.

### Verification & Quality Gates (`nomos verify`, `capbac`)
- **Cross-Server Information Flow Control (IFC)**: Enforces explicit server-to-server trust boundaries across Model Context Protocol (MCP) tool calls, preventing untrusted third-party MCP tools from bridging into sensitive workspace capabilities.

### Configuration & Workspace (`nomos.schema.json`, `nomos handshake`)
- **Self-Hosted JSON Schema Parity**: Regenerates `nomos.schema.json` with MCP boundary and IFC policy fields verified by byte-identical AST schema parity tests.
- **Strict Per-Repository Ledger Scoping**: Isolates wavefront and Living Ledger views per `repo_id` across multi-repository workstations.

### Episodic Memory (`nomos mem`)
- **High-Signal Memory Curation**: Refines `Nomos Learn` distillation thresholds and adds `nomos mem dismiss` to prune superseded episodic entries cleanly.

---

## [v4.3.0] - 2026-11-03

> Compiled Go AST SSoT, Reflected Draft 2020-12 JSON Schema & 32.8% Smaller Static Binary (Apache 2.0)

### Configuration & Workspace (`nomos.schema.json`, `nomos init`)
- **Single Compiled Go SSoT**: Retires external CUE runtime dependencies in favor of compiled Go struct contracts reflected directly into Draft 2020-12 `nomos.schema.json`.
- **Zero-Copy IDE Autocompletion**: Pins `yaml-language-server` `$schema` headers in `nomos init` templates to versioned GitHub raw schema URLs for instant editor validation.
- **Storage Table Pruning**: Excises retired v3 journal and legacy telemetry tables from local SQLite state.

### CLI & Distribution (`CGO_ENABLED=0`)
- **Leaner Static Binary (`-32.8%` Size Reduction)**: Cuts the `CGO_ENABLED=0` static Go binary footprint by nearly one-third (`~28 MB`) while accelerating startup and `nomos verify` cold-boot latency.

---

## [v4.2.0] - 2026-11-02

> NCTE-100 Controlled English Engine, Unified Lipgloss Terminal UI, `nomos physics` CLI & Claude Code v2 Hook Parity (Apache 2.0)

### Intent & Specifications (`nomos directive`)
- **Compiled NCTE-100 Lexicon & Grammar**: Ships the deterministic Controlled Technical English validator (`ncte.cue`) and succinct operator-to-agent action lexicon CLI for unambiguous, token-disciplined agent steering.
- **Agent-Steering Protocol Parity**: Aligns JIT phase directives (`IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN`) and CLI guidance directly with the published Agent-Steering Protocol.

### Verification & Quality Gates (`nomos physics`, `nomos verify`)
- **Real-Time AST Mass & Collision Vectors**: Wires `nomos physics` to inspect active worktree AST mass and detect real-time symbol and file collision vectors across concurrent task worktrees, alongside closing `nomos verify` probe blind spots.

### Task Lifecycle & 2PC Sync (`nomos task`)
- **Dual-Layer Commit Rationale Hydration**: Adds complete `--json` and task filter flags across Cobra commands and auto-hydrates Dual-Layer commit rationale sections from Living Ledger cards while rejecting canned commit shims.

### CLI & Distribution (`nomos handshake`, `nomos status`, `nomos verify`, `nomos version`)
- **Adaptive Lipgloss Terminal UI**: Introduces a unified terminal theme across human-facing CLI outputs with crisp gradients, pill badges, and clean non-TTY fallback.

### Ambient Hooks & Pacing (`nomos hook pre-tool`)
- **Claude Code v2 `PreToolUse` Parity**: Aligns `nomos hook pre-tool` with the Claude Code v2 `PreToolUse` JSON schema, adds `Read` environment-variable guards, and keeps background polling exempt from the 3-strike loop circuit-breaker.

---

## [v4.1.0] - 2026-11-02

> Polyglot Wazero + Tree-sitter AST Engine, Native `nomos hook` CLI & 3-Strike Loop Circuit-Breaker (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **Zero-CGO WebAssembly + Tree-sitter Evaluation**: Combines a pure-Go `wazero` WebAssembly linear-memory source scanner (`scan_source_bytes`) with `.scm` S-expression structural evaluators across Go, TypeScript/Svelte, Python, Rust, C/C++, Java, Kotlin, and C#.
- **Polyglot Test, Coverage & Anti-Skip Ratchets**: Auto-detects `go test`, `cargo test`, `pytest`, `gradle test`, `mvn test`, and `dotnet test`, enforces LCOV + Cobertura XML coverage thresholds (`>= 55%`), and rejects reward-hacked skipped tests (`t.Skip`, `#[ignore]`, `@pytest.mark.skip`, `it.skip`, `@Disabled`, `[Fact(Skip=...)]`).

### Ambient Hooks & Pacing (`nomos hook`)
- **Compiled Hook Subcommands**: Replaces external shell hook scripts with sub-millisecond `nomos hook session-start`, `nomos hook pre-tool`, and `nomos hook post-tool` commands, preserving existing user hooks additively during `nomos init`.
- **3-Strike Loop Circuit-Breaker**: Fingerprints tool invocations (`ArgsDigest` + file mtime in `0700` user-private XDG cache) to halt identical retry loops after 3 consecutive failures (`0/20` runaway loops across the 95-task `TraceTrial-Bench` suite; `83.2%` guarded pass rate vs `45.3%` unguarded).

### Task Lifecycle & 2PC Sync (`nomos task`)
- **Structured Living Ledger Cards**: Enforces `## Scope & Context`, `## Acceptance Criteria`, and `## Verification` across `nomos task create`, `chore`, and `pair` with `--scope`, `--ac`, `--verify`, and `--depends-on` flags.

### CLI & Distribution (`install.sh`)
- **Verified Supply-Chain Installer**: Enforces mandatory SHA-256 archive verification against `checksums.txt` in `install.sh` and `os.Lstat` symlink/world-writable rejection across plugins and caches.

---

## [v4.0.0] - 2026-11-01

> Initial Open-Core Public Release (Apache 2.0): Deterministic Go Control Harness for AI Coding Agents

### Verification & Quality Gates (`nomos verify`)
- **7 Laws & 21 Compiled Go AST Invariant Cells**: Enforces complexity ceilings, explicit error handling, anti-shim detection, test-deletion guards, zero-alias singularity, and 2-plane separation in milliseconds.
- **Fail-Closed Pre-Flight Verification**: Zero reliance on probabilistic prompt honor systems; code mutations require exit code `0` and deterministic AST proof before review.

### Task Lifecycle & 2PC Sync (`nomos task`)
- **Disposable Task Worktrees**: Physically locks `main` as read-only and isolates every AI coding session inside `worktrees/<repo>-<task>` across the `IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN` state machine.
- **Cryptographic Release Provenance**: Attaches verifiable Git Notes receipts and enforces strict `intent/` vs `substrate/` plane separation.

### Episodic Memory (`nomos mem`)
- **Repo-Scoped Local SQLite FTS5 Engine**: Persists distilled architectural axioms and verified engineering pitfalls locally on your machine with sub-millisecond recall.

### CLI & Distribution (`nomos init`, `nomos handshake`)
- **Universal Indirect Control**: Works out of the box with Claude Code, Cursor, Antigravity IDE, Windsurf, Cline, and OpenCode with zero cloud accounts, background daemons, or telemetry phone-home.
- **Cross-Platform Single Binary (`CGO_ENABLED=0`)**: Precompiled `linux`, `darwin`, and `windows` (`amd64` / `arm64`) archives published with SHA-256 `checksums.txt`.
