# Changelog

All notable changes to the **Nomos** deterministic engineering substrate (Apache 2.0) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). Precompiled static binaries (`CGO_ENABLED=0`) and SHA-256 `checksums.txt` manifests are published on [GitHub Releases](https://github.com/mgantlett/nomos/releases).

---

## [v4.35.0] - 2026-11-28

> Add `nomos.yaml` threshold loading and CLI overrides (--max-file-loc, --max-func-loc, --max-complexity) to nomos xray; Add machine-actionable verify --fixes repair recipes and deterministic safe --fix auto-remediation; Add Linux deb, rpm, and apk nfpms packages to GoReleaser configuration; Refresh MTPO base commit after verified sync even when worktree was manually rebased (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `nomos cli`, `nomos verify`, `.goreleaser.yaml`.

### CLI, Distribution & Hooks (`nomos cli`)
- **Add machine-actionable verify --fixes repair recipes and deterministic safe --fix auto-remediation (`INT-2340`, `SUB-3942`)**: Support nomos verify --fixes (terminal and --json VerifyFixPlan) and --fix deterministic safe auto-remediation (gofmt on modified Go files and `nomos.schema.json` regeneration inside worktrees) with re-verification proof.
- **Refresh MTPO base commit after verified sync even when worktree was manually rebased (`INT-2341`, `SUB-3943`)**: Refresh MTPO base commit after verified sync even when manually rebased.
- **Add nfpms schema to GoReleaser v2 strict contract test (`INT-2337`, `SUB-3940`)**: Add strict nfpms schema to GoReleaser v2 contract test.

### Verification & Quality Gates (`nomos verify`)
- **Add `nomos.yaml` threshold loading and CLI overrides (--max-file-loc, --max-func-loc, --max-complexity) to nomos xray (`INT-2343`, `SUB-3945`)**: Respect `nomos.yaml` governance.quality_gates.thresholds in nomos xray and support --max-file-loc, --max-func-loc, and --max-complexity CLI overrides with dynamic rendered labels.
- **Add PR-scoped xray --diff and shareable --badge/--badge-svg outputs (`INT-2339`, `SUB-3941`)**: Support nomos xray --diff <base-ref> PR delta scanning and --badge / --badge-svg shareable README outputs across terminal, markdown, json, and sarif.
- **Allow zero-dependency packaging/*/package.json under boundary.lock_dependencies (`INT-2334`, `SUB-3937`)**: Allow new zero-dependency zero-script packaging/*/package.json manifests under `lock_dependencies`.

### Release & Packaging (`.goreleaser.yaml`, `.github/workflows/ci.yaml`)
- **Add Linux deb, rpm, and apk nfpms packages to GoReleaser configuration (`INT-2336`, `SUB-3939`)**: Add Linux deb, rpm, and apk nfpms packages to GoReleaser.
- **Add Apache-2.0 VS Code and Open VSX extension (sophialabs.nomos) with release packaging (`INT-2332`, `SUB-3935`)**: Add Apache-2.0 VS Code and Open VSX extension (sophialabs.nomos) with SHA-256 binary provisioning and CI VSIX release sync.

---

## [v4.34.1] - 2026-11-27

> Fix budget critical and final-deadline deliverable flush advisories (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `nomos cli`.

### CLI, Distribution & Hooks (`nomos cli`)
- **Fix budget critical and final-deadline deliverable flush advisories (`INT-2330`, `SUB-3928`)**: Strengthen critical budget deliverable-flush and add final-deadline checkpoint.

---

## [v4.34.0] - 2026-11-26
> Suppression & Type-Escape Diff-Ratchet, Native Zod 4 Contract Parity, Held-Out Test Oracle Lock (`--lock-tests`), Hard `--scope` Write Fence, 15-Second Architectural Intent Brief (`--intent-brief`), Optional Runtime `benchmark_ratchet`, and Polyglot `SiblingDelta` Multi-Agent Isolation (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `nomos verify --intent-brief`, `nomos verify --update-bench-baseline`, `nomos task start --lock-tests`, `nomos task pair --lock-tests`, `nomos.yaml` (`forbid_suppressions`, `immutable_test_paths`, `benchmark_ratchet`).

### Static Escape & Polyglot Contract Hardening (`INT-2305`, `SUB-3875`, `SUB-3878`)
- **Diff-Ratcheted Suppression & Type-Escape Guard (`SUB-3878`)**: `AntiShimGate` rejects net-new linter suppression pragmas and TypeScript type escapes (`//nolint`, `eslint-disable`, `@ts-ignore`, `@ts-expect-error`, `# noqa`, `# type: ignore`, `#[allow(`, `: any`, `as any`) by default while grandfathering pre-existing baseline occurrences (`forbid_suppressions: false` opt-out; `TC-AGT-12`).
- **Native Zod 4 Contract Parity & Polyglot `SiblingDelta` Isolation (`SUB-3875`, `SUB-3878`)**: Extracts contract field shapes from TypeScript `z.object` / `z.strictObject` / `z.looseObject` schemas (`TC-OBL-017`), extends `SiblingDelta` deferral to polyglot contract blocks and bidirectional multi-`INT-*` twins, and bounds trunk deferral strictly to non-terminal sibling `SUB-*` worktrees present on disk.

### Pre-Tool Hook & Verification Oracle Boundaries (`INT-2307`, `SUB-3879`)
- **Held-Out Test Oracle Lock (`--lock-tests`, `immutable_test_paths`)**: Persists `LockTests` in SQLite schema v5 and blocks edits to pre-existing committed verification suites while allowing net-new test files; `immutable_test_paths` globs remain write-protected across every `SUB-*` task in both 4ms Pre-Tool Hooks and `PlaneSeparationGate` (`TC-TSK-23`).
- **Hard `--scope` Write Fence**: Enforces declared `scope_paths` in both 4ms Pre-Tool Hooks and `nomos verify` while automatically permitting paired unit test files so TDD never deadlocks (`TC-TSK-24`).

### Review Compression & Runtime Ratchet (`INT-2308`, `SUB-3880`)
- **15-Second Architectural Intent Brief (`--pr`, `--intent-brief`)**: Embeds `#ArchitecturalIntentBrief` (mutated contracts, dependency/boundary manifests, `TC-*` obligation deltas, and mechanical debt seal) directly into `PRProofReceipt` Markdown and exposes standalone `nomos verify --intent-brief` output (`TC-PR-03`).
- **Optional Runtime Benchmark Ratchet (`benchmark_ratchet`, `--update-bench-baseline`)**: Runs opt-in Go `-bench` (`ns/op`, `allocs/op`, `B/op`) or generic JSON metric commands inside `GoToolchainGate`, rejects regressions exceeding `max_regression_pct` (default `15%`), pins `benchmark_ratchet.command` to trunk in `ConfigImmutabilityGate`, and refreshes baselines via `nomos verify --update-bench-baseline` (`TC-BENCH-01`).

### Release & Packaging (`README.md`, `CHANGELOG.md`)
- **Prepare v4.34.0 Open-Core milestone release and align version pins (`INT-2308`, `SUB-3883`)**: Bump all statutory version pins to v4.34.0 and record v4.34.0 `CHANGELOG.md` section.

---

## [v4.33.0] - 2026-11-25

> Ratchet-by-Default Compact Init, Repo-Scoped Shield Scorecard Telemetry, Smart Home Agent Detection, and Branch-Guard 4-Rung Graduation Ladder (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `nomos init`, `nomos status`, `nomos hook post-commit`, `mneme.db` (`shield_events`).

### Intent & Specifications (`intent/architecture/specs/`)
- **Specify Shield telemetry, ratchet-by-default init, and Branch-Guard status ladder (`INT-2294`)**: Add `shield_events` SQLite table, `#ShieldSummary` Go SSoT, compact `templates/nomos_branch.yaml`, smart agent detection without 5-folder fallback, Branch-Guard `nomos status` 4-rung graduation ladder, quiet post-commit Shield Receipts, and compliance vectors `TC-SHIELD-01..03`, `TC-INIT-08..10`, and `TC-STATUS-BG-01..02`.

### CLI, Onboarding & Cognitive Memory (`nomos init`, `nomos status`, `mneme.db`)
- **Add Ratchet-by-default compact init, smart `$HOME` agent detection, and idempotent `--strict` promotion (`SUB-3851`)**: Plain `nomos init` always starts in safe Ratchet posture (`strict_mode: false`) with compact `~18`-line `nomos.yaml`, checks repo -> `$PATH` -> `$HOME` agent config directories without dumping unused IDE folders, prints plain-English ratchet guarantees and auto-locked boundaries, and promotes existing `nomos.yaml` cleanly via `nomos init --strict` while preserving historical `nomos.baseline.json`.
- **Add Shield Scorecard telemetry, Branch-Guard `nomos status`, and quiet 1-line post-commit receipts (`SUB-3851`, `SUB-3853`)**: Record `PreToolUse` denies, `Stop` blocks, verification gate catches, and verified commits into repo-scoped `shield_events` in `mneme.db` (`0ms` cost on clean allows), render the Branch Guard Status Card with a 4-rung graduation ladder (`Stealth` -> `Branch Guard` -> `CI PR Proofs` -> `Strict/Worktrees`), and emit a 1-line `post-commit` Shield Receipt only on first commit, branch catches, or pitfall distillation.

---

## [v4.32.1] - 2026-11-25

> Wrap `@verifies` and #StructName in backticks in v4.32.0 changelog (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.

### Release & Packaging (`CHANGELOG.md`)
- **Wrap `@verifies` and #StructName in backticks in v4.32.0 changelog (`SUB-3711`)**.

---

## [v4.32.0] - 2026-11-24

> Add polyglot dependency spine, reachable branch, and unreached file topology to xray and verify; Add intent contract lookup, impact boundary hints, and Mneme lineage recall; Add Windows Scoop, Winget, Devcontainer, and mise distribution manifests; Specify GateResult sensor scopes and orphan fixture checks (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `intent/architecture/specs/`, `nomos cli`, `nomos verify`, `capbac`.

### Intent & Specifications (`intent/architecture/specs/`)
- **Specify GateResult sensor scopes and orphan fixture checks (`INT-2258`, `SUB-3527`)**: Specify diff-ratcheted orphan fixture detection in AntiShimGate and sensor execution summaries across GateResult, PipelineReport, SARIF, and ReleaseNotePayload.
- **Remove closed Smithery reference from distribution manifest spec (`INT-2253`)**.

### CLI, Distribution & Hooks (`nomos cli`)
- **Add intent contract lookup, impact boundary hints, and Mneme lineage recall (`INT-2260`, `SUB-3533`)**: Nomos capabilities and `nomos_capabilities` resolve `#StructName`, `TC-*`, `ERR_*`, and spec filenames with zero-markup extraction, reverse `@intent`/`@verifies` links, and `sha256` digest.

### Verification & Quality Gates (`nomos verify`, `capbac`)
- **Add polyglot dependency spine, reachable branch, and unreached file topology to xray and verify (`INT-2272`, `SUB-3608`)**: XrayReport emits SpineDepth, ReachableFiles, UnreachedFiles, and XrayTopologyGraph with zero false positives on library exemplars.
- **Harden shell lexer near-misses, place-scoped container guards, and config/symlink preflight (`SUB-3529`)**: Harden shell lexer near-misses, dangling escaping symlinks, and `corpus_command` trunk pin.
- **Propagate gate sensor execution summaries and orphan fixture checks (`INT-2259`, `SUB-3527`)**: Enforce diff-ratcheted ScanOrphanedFixtures in AntiShimGate and emit ExecutionScope, SensorSummary, and UnindexedExtensions across GateResult, PipelineReport, SARIF, CLI verify output, and ReleaseNotePayload.

### Release & Packaging (`README.md`, `.goreleaser.yaml`, `.github/workflows/ci.yaml`)
- **Align Open-Core boundary row with Sovereign Pro, Squad, SLSA DSSE audit, and Air-Gap editions (`SUB-3666`)**: Align Open-Core boundary row with Sovereign Pro, Squad, SLSA DSSE, and Air-Gap editions.
- **Add Glama remote MCP connector backlink in collapsed MCP details (`SUB-3658`)**.
- **Surface Topology Spine and Unreached Dust row in TUI xray card (`SUB-3614`)**: Surface Topology Spine and Unreached Dust row in CLI xray summary.
- **Close cp/subshell binary-copy TOCTOU on workspace escaping symlinks (`SUB-3531`)**: Close cp/subshell binary-copy TOCTOU and derive escaping symlink parent roots.
- **Add Windows Scoop, Winget, Devcontainer, and mise distribution manifests (`INT-2256`, `SUB-3523`)**: Goreleaser check and strict KnownFields tests validate scoops and winget targets.
- **Assert closed Smithery manifest remains absent (`SUB-3519`)**: Assert Marketplace shield branding in distribution manifest suite.
- **Add pre-commit hooks, Nix flake, and multi-arch GHCR OCI container distribution (`INT-2252`, `SUB-3517`)**: Pre-commit hooks manifest, Nix flake, multi-arch GHCR OCI container config, and GoReleaser contract tests pass all 42 DoD gates.

---

## [v4.31.0] - 2026-11-23

> Catch silent JSON `snake_case` wire drift and untyped DTO escapes across Go, TypeScript, and Python; expose Page 0 capability discovery across stdio MCP, npm, PyPI, and `llms.txt` (Apache 2.0)

### Agent Upgrade & New Surfaces
- **Upgrade**: `nomos update --sync-workspace` (package-manager installs: check `nomos update --check --json` for `upgrade_command`, then run `nomos update --sync-workspace`).
- **Zero-Install Scan**: `npx -y @mgantlett/nomos@latest xray . --json` or `uvx --refresh nomos-cli xray . --json`.
- **Active Surfaces**: `nomos verify`, `nomos xray`, `nomos mcp` (`nomos://capabilities/catalog`), `CAPABILITIES.md`.

### CLI, Distribution & Hooks (`nomos cli`, `README.md`)
- **Align two-tier root help, plugin boundary checks, and README CLI parity (`INT-2242`, `SUB-3483`)**: Default `nomos --help` renders Core Lifecycle and Cognitive Memory commands cleanly with full `README.md` flag parity.

### Verification & Quality Gates (`nomos verify`, `nomos xray`)
- **Catch untyped DTO boundary reads and phantom wire keys in X-Ray and Law 4 verify (`INT-2248`, `SUB-3493`)**: `XrayReport` and `XrayHotspot` surface `untyped_boundary_reads` (`Record<string, any|unknown>`, `dict[str, Any]`, `map[string]any`) and `phantom_wire_keys` (unmatched `snake_case` consumer reads) with JSON Schema parity.
- **Scope Law 4 wire-boundary gate to modified worktree files in brownfield repos (`SUB-3495`)**: Diff-scoped ratcheting prevents legacy brownfield files outside the active diff from blocking unrelated PRs.
- **Emit canonical `probe_id`, `law`, and `ring` metadata across all 42 `GateResults` and 21 `CellReports` (`SUB-3509`)**: Enables deterministic coding-agent self-repair directly from `nomos verify --json` output.

### Release & Packaging (`CAPABILITIES.md`, `nomos mcp`, `llms.txt`)
- **Publish Page 0 `CAPABILITIES.md` with 8 skimmable Architectural Engine breakdowns (`SUB-3499`, `SUB-3505`)**: Documents Purpose, Mechanics, Formulas, Connections, and bijective TOC anchors across `CAPABILITIES.md`, `README.md`, and `llms.txt`.
- **Expose `nomos://capabilities/catalog` across stdio MCP, npm, PyPI, and `SKILL.md` (`SUB-3502`)**: Coding agents discover all 42 AST sensor probes and CLI surfaces in a single MCP resource read.

---

## [v4.30.0] - 2026-11-22

> Upgrade Living Ledger DAG and 4-Layer Compiler to Agent-to-Agent Handoff IR; Scope subdir git ls-files, target Go; Scope IntentTraceability, IPLeak, and brownfield; Add hotspot AST symbol outlines and 22-read search circuit breaker (Apache 2.0)

### CLI, Distribution & Hooks (`nomos cli`, `nomos hook`)
- **Upgrade Living Ledger DAG and 4-Layer Compiler to Agent-to-Agent Handoff IR (`INT-2226`, `SUB-3456`)**: Structured TaskRecord IR fields (`scope_paths`, obligations, `acceptance_criteria`, `verify_cmd`) persist in SQLite while auto-rendering read-only Markdown Description.
- **Add hotspot AST symbol outlines and 22-read search circuit breaker (`INT-2218`, `SUB-3433`)**: Append Tree-sitter function line-range outlines on >=500-LOC hotspot reads and pause broad Grep/Glob/list_dir calls at >=22 consecutive zero-edit reads in active worktrees/branches.

### Verification & Quality Gates (`nomos verify`)
- **Scope IntentTraceability, IPLeak, and brownfield (`SUB-3428`)**: Scope IntentTraceability, IPLeak, and brownfield env secret checks to O(diff).
- **Parallelize xray scanner, eliminate O(repo) diff walks, and surface 1-hop seam radar for 1M+ LOC monorepos (`INT-2214`, `SUB-3424`)**: Parallelize multi-million-LOC AST scanning, index git 1-hop impact lookups, and enable branch-mode seam radar.

### Release & Packaging (`README.md`, `nomos hook`, `.github/workflows/ci.yaml`)
- **Align Sovereign table row in `README.md` with Sophia & Nomos and Sophia Flight Deck (`SUB-3462`)**: Align Sovereign table with Sophia & Nomos and Sophia Flight Deck.
- **Lead with text pitch and quickstart above split-terminal graphic (`SUB-3443`)**: Lead with text pitch and quickstart above split-terminal graphic.
- **Add 3.6M-LOC monorepo scaling & 24-episode benchmark receipts (`SUB-3431`)**: Add 3.6M-LOC monorepo scaling & 30-episode benchmark receipts.
- **Scope subdir git ls-files, target Go (`INT-2216`, `SUB-3430`)**: Scope subdir git ls-files, target Go reverse imports, and emit monorepo 1-hop seam read hints.
- **Harmonize README 6-badge hero Title Case and high-contrast #0284C7 palette (`SUB-3427`)**: Harmonize 6-badge hero Title Case and high-contrast #0284C7 palette.
- **Swap static Web X-Ray shield in `README.md` to live self-scanning Nomos X-Ray SVG badge (`SUB-3425`)**: Swap static Web X-Ray shield to live self-scanning Nomos X-Ray SVG badge.
- **Tighten README hero badges to single-row high-signal bar and relocate Marketplace and DOI links (`SUB-3423`)**: Tighten hero badges to single-row bar and relocate Marketplace and DOI links.
- **Link sophialabs.ca/system/analytics badge and dogfooding velocity row in `README.md` (`SUB-3421`)**: Link sophialabs.ca/system/analytics badge and dogfooding velocity row.
- **Fix npm publish working directory in CI `sync_registries` job (`SUB-3414`)**: Cd into packaging/npm before npm publish in `sync_registries`.

---

## [v4.29.0] - 2026-11-21

> Unify convention-family axiom consolidation, scrub axiom stack traces, and ground KindDeadEnd hygiene in open-core mem; Fail-closed CommitSchemaGate twin preflight and Go const/SQL DML surface fingerprinting; Add zero-dependency PyPI nomos-cli thin binary launcher for uvx and pip; Skip non-shell heredoc stdin lines during trunk (Apache 2.0)

### Intent & Specifications (`intent/architecture/specs/`)
- **Codify polyglot 24h runner assertion TTL (`INT-2192`)**: Updated `mneme_memory_spec.md` to specify 24h polyglot runner assertion expiry, bare-header tombstoning, 42-gate Tier-C exclusion, and cooled 1x guardrail retirement.

### CLI, Distribution & Hooks (`nomos hook`, `nomos cli`)
- **Skip non-shell heredoc stdin lines during trunk (`SUB-3409`)**: Skip non-shell heredoc stdin lines during trunk shell redirect checks.
- **Harden Tier 1 agent radar, worktree loop digest, and concurrency guidance (`INT-2198`, `SUB-3392`)**: Add pre-latch Contract Radar preview, worktree loop digest resolution, and ActiveWorktrees concurrency guidance.

### Verification & Quality Gates (`nomos verify`, `nomos cli`)
- **Fail-closed CommitSchemaGate twin preflight and Go const/SQL DML surface fingerprinting (`INT-2194`, `SUB-3387`)**: Wired EvaluateWorktreeTwinPreflight fail-closed into CommitSchemaGate on nomos verify with immediate latch arming, added Go const threshold and SQL DDL/DML surface tokens, and removed soft CLI warning.
- **Polyglot TS/Svelte normative surface fingerprinting, verify twin preflight, and anti-evasion ratchet (`INT-2193`, `SUB-3386`)**: Implement TS/Svelte surface fingerprinting in astdebt/verify, sticky worktree twin-rejection latch against export-hiding evasion, and pre-commit twin preflight reporting during nomos verify.

### Task, Memory & Session (`nomos mem`, `nomos handshake`)
- **Unify convention-family axiom consolidation, scrub axiom stack traces, and ground KindDeadEnd hygiene in open-core mem (`INT-2208`, `SUB-3407`)**: Unified 24-gate convention axiom consolidation, title/path scrubbing, pattern:habit retirement, and KindDeadEnd deleted-file/cooling hygiene in open-core modules/mem.
- **Ground absent Trap files and healed ceilings (`INT-2206`, `SUB-3404`)**: SweepStaleRepoMemories and read-time pitfall grounding suppress deleted Trap files and healed LOC/package ceilings in open-core, Verified Patterns excludes negative pattern:habit AST trap clones, and unscoped IDLE pitfalls prioritize >=2x recurring traps.
- **Recall repo axioms in handshake and task start and cap stale directory patterns (`INT-2201`, `SUB-3397`)**: Handshake and task start/chore recall and reinforce top-2 repo axioms alongside pitfalls and patterns, and SweepRepoPitfallHygiene retires >30d zero-recall pattern overflows beyond top-3 per directory scope.
- **Add polyglot ephemeral test expiry, bare-header sweep (`SUB-3383`)**: Expanded polyglot single-test assertion expiry (24h), swept bare-header pitfalls, retired cooled 1x guardrails, and completed built-in 42-gate classification.

### Release & Packaging (`.github/workflows/ci.yaml`, `nomos verify`)
- **Verify pyproject.toml version parity in TestOpenCoreManifestVersionParity (`SUB-3395`)**: Verify pyproject.toml version parity against RootCmdVersion.
- **Wire GitHub OIDC PyPI Trusted Publishing and mcp-name ownership marker for nomos-cli (`SUB-3381`)**: Added Official MCP Registry mcp-name ownership marker to `packaging/pypi/README.md`, added pypi nomos-cli stdio package to `server.json`, and wired idempotent GitHub OIDC PyPI + npm + Official MCP Registry `sync_registries` job into `.github/workflows/ci.yaml`.
- **Enable passwordless OIDC Trusted Publisher and CDN wait in `ci.yaml` npm sync (`SUB-3380`)**: Enable passwordless npm OIDC Trusted Publisher and CDN wait.
- **Add zero-dependency PyPI nomos-cli thin binary launcher for uvx and pip (`INT-2189`, `SUB-3377`)**: Implement packaging/pypi (pyproject.toml, nomos_cli/__init__.py, `README.md`, and `test_nomos_cli`.py) verifying OS/arch resolution and SHA-256 checksum enforcement.

---

## [v4.28.0] - 2026-11-20

> Upgrade stdio MCP tool TDQS schemas and add X-Ray badge link to PR proof receipts; Markdown enumeration count parity in intentlint Steel-Man structure checks; Add MCP 2026-07-28 stateless discovery and server/discover; Update `tree_sitter_ast_gate_spec` probe count from six to eight (Apache 2.0)

### Intent & Specifications (`intent/architecture/specs/`)
- **Update `tree_sitter_ast_gate_spec` probe count from six to eight (`INT-2174`)**: Update `tree_sitter_ast_gate_spec` sensor probe count from six to eight.

### CLI, Distribution & Hooks (`nomos cli`)
- **Add MCP 2026-07-28 stateless discovery and server/discover (`INT-2169`, `SUB-3328`)**: Nomos mcp negotiates 2026-07-28, serves server/discover with supportedVersions, rejects unsupported _meta protocol versions with -32022, and passes `TC-MCP-02`.

### Verification & Quality Gates (`nomos verify`)
- **Consolidate canonical npm `package.json`, `server.json`, and CI registry publishing in open-core (`SUB-3368`)**: Allow metadata-only `package.json` updates under `lock_dependencies`.
- **Enforce Markdown enumeration count parity in intentlint Steel-Man structure checks (`INT-2176`, `SUB-3336`)**: Enforce Markdown enumeration count parity in intentlint Steel-Man structure checks.

### Release & Packaging (`CHANGELOG.md`, `.github/workflows/ci.yaml`, `README.md`)
- **Replace `@mgantlett/nomos` with `@mgantlett/nomos` in CHANGELOG and launcher User-Agent (`SUB-3370`)**: Replace stale `@mgantlett/nomos` with `@mgantlett/nomos` in CHANGELOG and launcher.
- **Align canonical `@mgantlett/nomos` `package.json`, version parity contract, and CI registry sync (`SUB-3369`)**: Align canonical `@mgantlett/nomos` `package.json`, manifest version parity, and CI registry sync.
- **Upgrade stdio MCP tool TDQS schemas and add X-Ray badge link to PR proof receipts (`INT-2182`, `SUB-3362`)**: Add MCP tool TDQS schemas and GitHub X-Ray badge footer on PR proof receipts.
- **Support NOMOS_RELEASE_BASE_URL for offline SHA-256 launcher verification (`SUB-3356`)**: Support NOMOS_RELEASE_BASE_URL in nomos-cli.mjs for offline SHA-256 verification.
- **Align npm package scope to `@mgantlett/nomos` (`SUB-3353`)**: Align npm package scope to `@mgantlett/nomos` across docs, llms.txt, skills, and MCP `server.json`.
- **Fix npm launcher GoReleaser archive naming, add mcpName, and add root `server.json` + `glama.json` (`SUB-3350`)**: Fix npm GoReleaser archive naming and add root `server.json` and `glama.json`.

---

## [v4.27.1] - 2026-11-19

> Upgrade `nomos mcp` stdio server to negotiate MCP `2025-11-25` / `2025-06-18` and expose read-only `resources/*`, `prompts/*`, and `nomos_capabilities`; Add SHA-256 verified `@mgantlett/nomos` `npx` launcher and synchronize Homebrew, OpenAPI 3.1, and remote Streamable HTTP MCP discovery across `llms.txt`, `README.md`, `SKILL.md`, and `plugin.json` (Apache 2.0)

### CLI, MCP & Agent Discovery (`nomos mcp`, `packaging/npm`, `llms.txt`)
- **Upgrade `nomos mcp` to negotiate `2025-11-25` and serve read-only resources, prompts, and capabilities (`INT-2167`, `SUB-3322`)**: Negotiate MCP protocol versions `2025-11-25`, `2025-06-18`, `2025-03-26`, and `2024-11-05` in `nomos mcp` `initialize`, expose read-only `resources/list` + `resources/read` (`nomos://status`, `nomos://capabilities`, `nomos://directives`) and `prompts/list` + `prompts/get` (`xray-audit`, `verify-impact`, `recall-pitfalls`), and add the read-only `nomos_capabilities` tool (`TC-MCP-02`).
- **Add SHA-256 verified `@mgantlett/nomos` `npx` launcher and remote HTTP MCP discovery (`SUB-3320`, `SUB-3321`, `SUB-3314`)**: Ship zero-dependency Node.js launcher (`packaging/npm/bin/nomos-cli.mjs`) with automatic latest GitHub Release resolution and `checksums.txt` SHA-256 verification, wire remote Streamable HTTP MCP (`https://sophialabs.ca/mcp`) into `.claude-plugin/plugin.json`, and synchronize Homebrew (`mgantlett/tap/nomos`), `npx`, and OpenAPI 3.1 discovery across `llms.txt`, `README.md`, and `skills/nomos/SKILL.md`.

---

## [v4.27.0] - 2026-11-19

> Numeric ratchet guard and refactor AST ceiling hotspots; Eliminate self-referential harness terminology in favor of deterministic substrate; Ratchet `max_package_files` to 20 and lock subpackage DAG boundaries in `nomos.yaml`; Extract polygate and ratchet subpackages, co-locate astdebt tests, and cap xray CDS <= 45 (Apache 2.0)

### Intent & Specifications (`nomos.yaml`)
- **Ratchet `max_package_files` to 20 and lock subpackage DAG boundaries in `nomos.yaml` (`INT-2157`)**: Ratchet `max_package_files` from 30 to 20 and append 4 leaf subpackage `architecture_boundaries` rules (hookcmd-shellguard-leaf, hookcmd-containerguard-leaf, verify-ratchet-leaf, verify-polygate-leaf) in `nomos.yaml`.
- **Ratchet self-dogfooding thresholds and lock 4-tier DAG in `nomos.yaml` (`INT-2154`)**: Ratchet self-dogfooding quality thresholds (complexity<=14, func_loc<=55, params<=7, clone_lines>=14, magic_strings<=8, package_files<=30, coverage>=65%, docstrings>=90%), enable boundary.lock_dependencies, and lock 4-tier `architecture_boundaries` DAG in root `nomos.yaml`.

### CLI, Distribution & Hooks (`nomos cli`)
- **Extract containerguard and shellguard subpackages and trim cli package fan-out (`INT-2155`, `SUB-3299`)**: Extract hookcmd/shellguard and hookcmd/containerguard leaf subpackages with unit suites (trimming hookcmd from 26 to 19 prod files), consolidate 3 small cli helpers (trimming cli from 22 to 19 prod files), and split `inspectcmd/cmd_session_name.go` below xray CDS 45.
- **Eliminate self-referential harness terminology in favor of deterministic substrate (`INT-2152`, `SUB-3296`)**: Replace self-referential Nomos harness phrasing with Nomos Substrate across templates, root help, and contract tests.
- **Preserve present agent adapters and local stealth mode during nomos update --sync-workspace (`INT-2149`, `SUB-3293`)**: Detect per-adapter hook drift and preserve present adapters and local stealth excludes on nomos update --sync-workspace.

### Verification & Quality Gates (`nomos verify`, `nomos handshake`, `nomos cli`)
- **Extract polygate and ratchet subpackages, co-locate astdebt tests, and cap xray CDS <= 45 (`INT-2156`, `SUB-3300`)**: Extract verify/ratchet (4 files) and verify/polygate (7 files) leaf subpackages with unit suites (trimming modules/verify from 29 to 19 production files), fold astdebt micro-files with co-located blueprint tests (21 to 19 production files), and decompose handshake and task lifecycle hotspots so xray `observed_max_cds` <= 45 and `observed_max_package_files` <= 19 across all packages.
- **Complete exported gate method docstrings for diff-scope parity (`SUB-3298`)**: Complete Go doc comments across all remaining exported gate methods in `gates_standard.go`, `gates_capbac_release.go`, and `gates_toolchain.go` for 100% per-file docstring parity under `verify_scope=diff`.
- **Harden numeric ratchet guard and refactor AST ceiling hotspots (`INT-2151`, `SUB-3295`)**: Enforce worktree numeric-threshold downgrade guard in ThresholdIntegrityGate (`TC-COV-007`), refactor all complexity/LOC/arity/clone/magic-string ceiling hotspots below ratcheted bounds, and split oversized CLI test suites under 400 LOC.

---

## [v4.26.0] - 2026-11-18

> Fast static binary install and baseline-only zero-config PR bootstrap in `action.yml`; Fix SQLite strftime CWE-89 false positive in DetectPolyglotCWESinks and add sophialabs.ca/xray link to README; Add Thin Binary Bootstrapper Agent Skill and Claude Plugin manifests for native agent discovery; Pin SemVer tag on README GitHub Action snippet and document sophialabs.ca URL swap (Apache 2.0)

### Verification & Quality Gates (`nomos verify`, `README.md`)
- **Fix SQLite strftime CWE-89 false positive in DetectPolyglotCWESinks and add sophialabs.ca/xray link to README (`SUB-3281`)**: Eliminate SQLite strftime CWE-89 false positive for 0/100 self-xray and surface sophialabs.ca/xray in README.

### Release & Packaging (`README.md`)
- **Pin SemVer tag on README GitHub Action snippet and document sophialabs.ca URL swap (`SUB-3283`)**: Pin SemVer tag on GitHub Action snippet and document sophialabs.ca URL swap.
- **Fast static binary install and baseline-only zero-config PR bootstrap in `action.yml` (`INT-2144`, `SUB-3280`)**: Fast static binary install, zero-yaml baseline bootstrap, and sarif-path in `action.yml`.
- **Add Thin Binary Bootstrapper Agent Skill and Claude Plugin manifests for native agent discovery (`INT-2139`, `SUB-3274`)**: `Skills/nomos/SKILL.md` and .claude-plugin manifests ship as strict Thin Binary Bootstrappers with zero Sovereign leaks.

---

## [v4.25.0] - 2026-11-18

> Clarify Init-and-Forget stack model, Agent-first setup, and Branch Guard stewardship; Log debug parse error in OpenCode `nomos.ts` template; Pre-push release-note sync, diff-scoped corpus clone index, and micro-file guard; Tighten `README.md` front-door tagline and subhead around 0-token physics powered by your coding agent (Apache 2.0)

### CLI, Distribution & Hooks (`nomos cli`, `README.md`)
- **Clarify Init-and-Forget stack model, Agent-first setup, and Branch Guard stewardship (`INT-2136`, `SUB-3265`)**: Clarify Init-and-Forget stack layer, Agent-first setup, and Branch Guard stewardship.

### Verification & Quality Gates (`nomos verify`, `capbac`)
- **Enforce pre-push release-note sync, diff-scoped corpus clone index, and micro-file guard (`INT-2133`, `SUB-3255`)**: PrePushHookScript syncs refs/notes/nomos/releases, CognitiveDebtGate indexes live corpus AST skeletons for net-new clones, MaxFileLOCGate+FastFileRadar block net-new micro-files in high-fanout packages.
- **Consolidate modules/verify gate fragments and reclaim package file headroom (`SUB-3251`)**: Consolidate gate fragments, reclaim package headroom, and prune surviving `gov_rule` moves.
- **Share capbac shell quote lexer and consolidate hookcmd handlers (`INT-2130`, `SUB-3250`)**: Extract SplitUnquotedShellSegments, TokenizeQuotedShellWords, and HasUnquotedShellChainingOrSub into `core/capbac/shell_lex.go` and consolidate fragmented cli/hookcmd files.

### Release & Packaging (`nomos init`, `README.md`)
- **Log debug parse error in OpenCode `nomos.ts` template (`INT-2137`, `SUB-3267`)**: Log debug parse error in OpenCode `nomos.ts` template catch block.
- **Tighten `README.md` front-door tagline and subhead around 0-token physics powered by your coding agent (`SUB-3269`)**: Tighten front-door tagline around zero-token physics powered by your coding agent.
- **Route hookcmd and astdebt scopes in `render-release-notes.sh` (`SUB-3260`)**: Route hookcmd, inspectcmd, init, mcp, astdebt, xray, sarif, and vscope commit scopes in `render-release-notes.sh`.
- **Unify polyglot wrapper, idiom, exception, and blueprint scanners in verify/astdebt (`INT-2129`, `SUB-3249`)**: Unify polyglot wrapper, idiom, exception, and blueprint scanners.

---

## [v4.24.0] - 2026-11-17

> Add polyglot design-slop, coverage-exclusion, manifest ImportError fallback, and wrapper-fragmentation AST sensors; Track Bash file mutations natively across PostToolUse and Stop hooks, surface manifest/module hull in SessionStart, harden NormativeSurfaceDelta Go AST extraction, and codify v4.24.0 specs (Apache 2.0)

### Intent & Specifications (`intent/architecture/specs/`)
- **Defer sibling removed spec fields and TCs across disjoint in-flight commits (`INT-2126`)**: Specify deferral for removed spec fields and verification vectors alongside disjoint spec filtering in `nomos_intent_traceability_spec.md`.
- **Specify intermediate worktree exclusions, heredoc SQL airlock, quoted-DDL CWE-89, and defensive return sprawl (`INT-2121`)**: Codify `.venv`, `venv`, `node_modules`, and `.tmp` segment skipping in `walkExplicitRelativeFiles`, identifier-quoted DDL exemption in `detectCWE89SQLSink`, four-plus `isinstance`/`typeof` early-return sprawl detection (`TC-TS-24`), static heredoc source-write airlock precision, and the 6-strike headless container `Stop` refactoring mandate (`TC-BG-11`).
- **Specify polyglot wrapper, idiom, clone, and test-rigor gates (`INT-2114`)**: Codify the polyglot two-tier one-line return wrapper rule, indent cognitive `elif`/`else` child-nesting parity, short sibling clones (`>=5` LOC intra-file, `>=8` LOC cross-file), cross-language control-flow/idiom checks, `.tmp/tests/` hygiene steering, and polyglot test-breadth (`>=8` cases) / isolated boundary-mutant spot-checks (`TC-TS-23`, `TC-COV-006`, `TC-BG-10`).
- **Tighten polyglot complexity bounds, trace realpath normalization, and guard contracts (`INT-2111`, `INT-2112`)**: Scope `MaxPolyglotCyclomaticComplexity` and `MaxFunctionCognitiveComplexity` (`10`) to Python/Ruby indent languages while preserving `15` on Go and brace languages (`.ts`/`.svelte`), count inline Python ternaries and comprehension filters, and codify `>=3`/`>=5` `isinstance` sprawl, `1`-caller middleman wrappers, non-worktree brownfield `verify` parity, `os.path.realpath` trace matching, and `--container-sandbox` fixture hull exclusions.
- **Codify diff-scoped Python trace coverage and Turn-0 container guidance (`INT-2110`)**: Document Git-diff-scoped Python trace coverage (`CoverageGate`) and Turn-0 `--container-sandbox` `SessionStart` `.tmp/tests/` guidance across `nomos_intent_traceability_spec.md` and `nomos_hermetic_worktree_spec.md`.
- **Codify `.tmp/tests/` discovery, behavioral `Stop` gating, and universal AST invariants (`INT-2109`)**: Document per-function polyglot Cognitive Complexity bounds (`15`), Python `(val, err)` tuple-return, empty `Exception` sprawl, and `SIM103` boolean-return checks (`TC-TS-22`), `HookSessionTrace.DirtyBehavioralEdits` and `SessionStart` `.tmp/tests/` guidance (`TC-BG-09`), plus scratch regression suite discovery with `requirements.txt` manifest detection (`TC-COV-005`).
- **Codify v4.24.0 polyglot design-slop sensors, shell hook tracking, and surface delta invariants (`INT-2104`, `INT-2106`)**: Document 2nd-gen exception gaming, comment-aware coverage exclusions, manifest ImportError fallbacks, wrapper fragmentation, 3-branch chains, `venv`/`site-packages` exclusions, `Bash` content-digest tracking with 4-strike `--container-sandbox` `Stop` retries and inline `FastFileRadar`, `SessionStart` hull injection, and `NormativeSurfaceDelta` extraction rules (`TC-TS-19`..`21`, `TC-CMT-009`, `TC-BG-08`).

### Verification & AST Gates (`nomos verify`)
- **Pass bound `INT` twin key in in-flight `SUB` spec digest collection (`SUB-3241`)**: Thread the bound Intent twin key through `collectInFlightSubSpecDigests` so sibling obligation verification resolves active twin specs cleanly.
- **Defer sibling removed fields/TCs and filter disjoint specs (`SUB-3239`, Twin: `INT-2126`)**: Tighten `sibling_delta` and `intent_obligations` so removed fields and verification vectors on disjoint deferred specs do not block parallel sibling deliveries.
- **Exclude `.venv` symlinks in worktree walk, refine quoted-DDL `CWE-89`, and catch `isinstance`/`typeof` return sprawl (`SUB-3233`, Twin: `INT-2121`)**: Skip default intermediate segments (`.venv`, `venv`, `node_modules`, `.tmp`) in `walkExplicitRelativeFiles` so virtualenv symlinks never fail `ZeroFilesystemAliasGate`; exempt identifier-quoted SQLite migration DDL from `CWE-89` while keeping value interpolation and unquoted table names fail-closed; flag modules repeating four-plus `isinstance` or `typeof` early-return guards while exempting pre-existing brownfield files; raise headless configured container `Stop` retries to `6` with explicit autonomous refactoring pre-authorization.
- **Gate container test hygiene on `nomos.yaml`, normalize brownfield count drift, and probe compound boundary operators (`SUB-3229`)**: Silence `evaluateTestPlacementHygieneAdvisory` in workspaces lacking `nomos.yaml`, normalize embedded numeric counts during brownfield design-slop ratcheting across polyglot extensions, and scan all relational operators across chained expressions, comprehensions, and ternaries up to `6` boundary-mutant probes per checkpoint.
- **Implement polyglot wrapper, idiom, clone, and boundary test-rigor gates (`SUB-3223`, Twin: `INT-2114`)**: Enforce polyglot two-tier single-return micro-wrapper limits across Python, Ruby, TypeScript/JS, Rust, Java, and C# (`0/1`-caller bans + `<=2`/`<=3` multi-caller caps, keeping Go exempt), align indent cognitive `elif`/`else` nesting depth with AST trees, catch `>=5`-LOC intra-file sibling clones and `>=8`-LOC cross-file polyglot clones, add helper-call guard, `2`-branch jump `if`, generalized bool-return, `(ok, val)`/`[ok, val]` Go-tuple, and collection/nullish idiom scans, emit non-blocking `.tmp/tests/` hygiene steering in zero-baseline containers, and enforce `>=8`-case test breadth plus fast isolated `<`/`<=` boundary-mutant spot-checks.
- **Scope `CC/Cog = 10` default to Python/Ruby while preserving `15` on Go/TS/Svelte (`SUB-3218`, Twin: `INT-2112`)**: Keep `MaxPolyglotCyclomaticComplexity` and `MaxFunctionCognitiveComplexity` (`10`) focused on `.py`/`.rb`/`.rake` indent files while retaining `15` on Go, TypeScript, Svelte, and other brace languages.
- **Enforce polyglot CC/Cog `10`, realpath `.tmp/tests/` trace, and non-worktree verify parity (`SUB-3217`, Twin: `INT-2111`)**: Wire `EffectiveMaxComplexity` (`10` non-Go, `15` Go) and Python inline ternary/filter cognitive scoring, close non-worktree brownfield/strict `nomos verify` gaps for design slop, clones, cognitive complexity, and arity, flag `>=3`/`>=5` inline `isinstance`-`raise` sprawl, `>=2` single-use helpers, and `1`-caller middlemen, normalize Python `trace` paths via `os.path.realpath`, skip sample fixture dirs in `SessionStart` hulls, and omit `git commit` in `--container-sandbox` Next Actions.
- **Diff-scope Python trace coverage and emit Turn-0 container `.tmp/tests/` guidance (`SUB-3214`, Twin: `INT-2110`)**: Pass modified/added non-test `.py` files via `NOMOS_PY_TRACE_FILES` when Git scope contains changes so untouched trunk sample fixtures never dilute `CoverageGate`; put `.tmp/tests/test_*.py` first in `TDDGate` remedies and wire `--container-sandbox` on `nomos hook session-start` to emit `.tmp/tests/` regression-suite guidance even on empty Turn-0 repos.
- **Enforce `.tmp/tests/` regression discovery, `DirtyBehavioralEdits` `Stop` gating, and universal cognitive/idiom AST sensors (`SUB-3213`, Twin: `INT-2109`)**: Discover untracked `.tmp/tests/` suites in `TDDGate`, `GoToolchainGate`, `CoverageGate` (`>=80%` Python trace coverage), and `ResolvePairedTestStatus` while including `requirements.txt` in manifest checks; enforce `MaxFunctionCognitiveComplexity` (`15`) and universal Python anti-slop checks (`(val, err)` tuple returns, `>=3` empty `Exception` subclasses, `SIM103` boolean returns) while permitting multi-caller 1-line helpers; gate post-edit behavioral execution in `HookSessionTrace` and emit `Regression Suite` `.tmp/tests/` guidance in `SessionStart`.
- **Exclude `venv`/`site-packages`, tighten 3-branch chains and 0-caller class methods, and harden container `Stop` (`SUB-3207`, Twin: `INT-2106`)**: Exclude `venv`, `.venv`, and `site-packages` across `workspace.Walk` and `vscope` git-status polyglot scoping; flag >=3 consecutive same-variable `if ==` / `if isinstance` chains without `elif`, 0-caller 1-line class methods, and comprehension/ternary return helpers; seed shell mutations in `loadOrSyncStopTrace` even when `ObservedDigests` starts empty after failing `Bash` writes, scope `Stop` strictly to `DirtyUnverifiedEdits`, and allow up to 4 `--container-sandbox` `Stop` retries with inline `FastFileRadar` diagnostics.
- **Harden `NormativeSurfaceDelta` Go AST extraction and v4.24.0 AST edge cases (`SUB-3203`, Twin: `INT-2104`)**: Extend `ParseGoSurfaceFingerprintForPath` to track exported Go functions/types, exported struct `json` tags, `@intent` bindings, and `verify`/`hookcmd` diagnostic/banner literals with balanced intra-package split cancellation; collapse multi-line Python `except (...)` tuples, raise design-slop workspace corpus scanning to `512` files with fail-open skip when exceeded, detect compatibility/legacy/deprecated/checkpoint public pass-through shims with 0 callers, and bind `TC-TS-19`..`21`, `TC-CMT-009`, and `TC-BG-08` `@verifies` test vectors.
- **Enforce 2nd-gen exception gaming, coverage exclusion, and design-slop AST sensors (`SUB-3200`)**: Detect >=3-type bug-masking `except` tuples and silent fallback returns across broad/boundary catches, reject comment-level coverage exclusion directives (`pragma: no cover`, `istanbul/v8/c8 ignore`, `#[coverage(off)]`), reject multi-statement `except ImportError` fallbacks when libraries are declared in manifests or stdlib, catch 0-caller private pass-through wrappers, flag >=3 single-use 1-line wrapper fragmentation, detect inline `isinstance` guard ladders and same-variable `if ==` chains, and enforce polyglot control-flow nesting depth across `verify` and `FastFileRadar`.

### CLI, Distribution & Hooks (`nomos hook`)
- **Support compound `cat` heredocs and allow `.tmp/tests/` scratch suites (`SUB-3235`)**: Parse `splitUnquotedShellSegments` per command segment in container `PreToolUse` so chained setup plus static heredoc source writes containing `DROP TABLE` pass cleanly, and allow `.tmp/tests/` scratch suites generated per `SessionStart` guidance while continuing to block arbitrary hidden dot-dir scripts.
- **Track shell file mutations and surface manifest hull (`SUB-3201`)**: Record content digests to detect files created or edited via `Bash` across `PostToolUse` and `Stop` hooks (including `--container-sandbox` Branch Guard repos), enable `PostToolUse` AST radar on active Branch Guard feature branches, and surface declared dependencies (`Manifest Deps`) and workspace symbols (`Workspace Modules`) during `SessionStart`.

---

## [v4.23.0] - 2026-11-17

> Document HarnessSecurity-Bench 46-container paired audit in README; Container mechanism policies, symlink path isolation, and token-aware runner loop detection; Fix content-hash loop breaker and container test-plane write guard in hookcmd; Tighten README conversion flow and sync Branch Guard hero terminal SVG (Apache 2.0)

### Intent & Specifications (`README.md`)
- **Document HarnessSecurity-Bench 46-container paired audit in README (`INT-2099`)**: Add HarnessSecurity-Bench 46-container paired audit to benchmark matrix.
- **Tighten README conversion flow and sync Branch Guard hero terminal SVG (`INT-2089`)**: Tighten Open-Core README conversion, add mobile-legible 24ms catch block, remove section emojis, update Action pin to v4.22.1, and align hero SVG with Branch Guard.

### CLI, Distribution & Hooks (`nomos cli`, `nomos hook`)
- **Enforce container mechanism policies, symlink path isolation, and token-aware runner loop detection (`SUB-3181`)**: Add container sandbox policies, symlink escape guard, and IFC sink enforcement.
- **Fix content-hash loop breaker and container test-plane write guard in hookcmd (`SUB-3180`)**: Switch loop-breaker workspace digests from mtime to content SHA-256 with per-digest runner strike tracking, detect python test scripts, run unconditional container anti-shim checks, and support NOMOS_PROTECT_WORKSPACE_TESTS=1 write-locking.

---

## [v4.22.1] - 2026-11-16

> Skip vendors/third_party dirs and separate `path_leaks` from `secret_leaks`; Emit stderr interactive hint when nomos mcp runs on a TTY; Add GitHub Marketplace Action badge to `README.md` (Apache 2.0)

### CLI, Distribution & Hooks (`nomos cli`)
- **Emit stderr interactive hint when nomos mcp runs on a TTY (`SUB-3168`)**: Emit stderr interactive hint when nomos mcp runs on a TTY.

### Release & Packaging (`README.md`)
- **Skip vendors/third_party dirs and separate `path_leaks` from `secret_leaks` (`SUB-3169`)**: Skip vendors/third_party dirs and separate `path_leaks` from `secret_leaks`.
- **Add GitHub Marketplace Action badge to `README.md` (`SUB-3163`)**: Add GitHub Marketplace Action badge to `README.md`.

---

## [v4.22.0] - 2026-11-16

> Add zero-dependency nomos mcp stdio server for read, orient, and verify affordances; Add Java and C# WASM Tree-sitter drivers to polyglot AST verification and brownfield xray; Pin NOMOS_INSTALL_DIR in init --ci workflows and preserve PR step summary; Ship nomos ci-init workflow generator and OASIS SARIF 2.1.0 output for xray and verify (Apache 2.0)

### CLI, Distribution & Hooks (`nomos cli`)
- **Add zero-dependency nomos mcp stdio server for read, orient, and verify affordances (`INT-2075`, `SUB-3124`)**: Implement zero-dependency read-only nomos mcp stdio JSON-RPC 2.0 server exposing handshake, status, `task_dag`, `mem_recall`, `verify_impact`, and xray under --help-agent.
- **Ship nomos ci-init workflow generator and OASIS SARIF 2.1.0 output for xray and verify (`INT-2074`, `SUB-3123`)**: Implement 'nomos ci-init' deterministic CI workflow writer (`.github/workflows/nomos-verify.yml` and `.gitlab-ci.yml` snippet) and OASIS SARIF 2.1.0 serializer for 'nomos xray --sarif' and 'nomos verify --sarif' with unit tests.

### Verification & Quality Gates (`nomos verify`)
- **Add Java and C# WASM Tree-sitter drivers to polyglot AST verification and brownfield xray (`INT-2076`, `SUB-3125`)**: Embed wazero Java and C# WASM Tree-sitter grammar drivers, add parameterized constant-return anti-shim detection, wire public symbol export and xray stub scanning, and align SupportedLanguage and `nomos.yaml` templates.

### Release & Packaging (`.github/workflows/ci.yaml`, `README.md`)
- **Pin NOMOS_INSTALL_DIR in init --ci workflows and preserve PR step summary (`SUB-3154`)**: Pin NOMOS_INSTALL_DIR in init --ci workflows, add Marketplace branding, and preserve PR step summary.

---

## [v4.21.1] - 2026-11-16

> Exempt external Go modules without nomos/substrate (Apache 2.0)

### Verification & Quality Gates (`nomos verify`)
- **Exempt external Go modules without nomos/substrate (`SUB-3145`)**: Scope GitSubprocessHygieneGate to Go modules referencing github.com/mgantlett/nomos/substrate so external Go repos calling exec.Command("git") pass nomos verify.

---

## [v4.21.0] - 2026-11-16

> Resolve repo root and git HEAD in pure Go for sub-millisecond PreToolUse; Frictionless Branch Guard nomos init, stealth --local mode, and 2-tier --help-agent; Branch Guard progressive onboarding and frictionless nomos init; Dual-gate pattern filesystem grounding, periodic sweep, and nomos mem dismiss --stale (Apache 2.0)

### CLI, Distribution & Hooks (`nomos cli`, `nomos init`)
- **Implement frictionless Branch Guard nomos init, stealth --local mode, and 2-tier --help-agent (`INT-2083`, `SUB-3133`)**: Implement progressive Branch Guard nomos init, smart auto-baseline, stealth --local .git/info/exclude mode, --agents auto-detection, --worktrees/--strict overrides, and 2-tier nomos --help-agent.
- **Add multi-language cross-PR contract hint in xray and missing external plugin remedy in root CLI (`INT-2071`, `SUB-3122`)**: RenderXrayTerminal appends zero-nag cross-PR contract/compliance affordance only when FilesScanned >= 50 and len(Languages) >= 2.

### Configuration & Workspace (`nomos.yaml`, `nomos hook`, `nomos verify`)
- **Implement Branch Guard progressive onboarding and frictionless nomos init (`INT-2082`, `SUB-3132`)**: Implement BoundaryConfig.IsolationMode SSoT and `nomos.schema.json` parity, Branch Guard PreToolUse/Stop/CommitSchemaGate/InstallHooks enforcement, branch-scoped trace and post-commit Mneme distillation in substrate/.

### Task, Memory & Session (`nomos mem`, `nomos cli`)
- **Implement dual-gate pattern filesystem grounding, periodic sweep, and nomos mem dismiss --stale (`INT-2055`, `SUB-3099`)**: Implemented dual-gate filesystem grounding for pattern: and pitfall: scopes and file blueprints across SweepStaleRepoMemories, RecallRepoPatterns, RecallRepoPitfalls, automatic maintenance, and nomos mem dismiss --stale.

### Release & Packaging (`README.md`)
- **Resolve repo root and git HEAD in pure Go for sub-millisecond PreToolUse (`SUB-3138`)**: Replace git rev-parse subprocess spawns on PreToolUse with pure-Go .git and .git/HEAD resolution, deduplicate candidate scopes, and satisfy <5ms PreToolUse SLA under full parallel verify load.
- **Reframe top-of-funnel onboarding around progressive Branch Guard ramp (`SUB-3134`)**: Reframe hero and 60-second quickstart around nomos xray, Branch Guard default init (--local stealth), and optional --worktrees escalation while moving internal protocol deep-dives into expandable sections.
- **Add Zenodo Concept DOI badge to `README.md` and CITATION.cff (`SUB-3097`)**: Added minted Zenodo Concept DOI (10.5281/zenodo.24240539) badge to `README.md` and doi field to CITATION.cff following v4.20.1 release.

---

## [v4.20.1] - 2026-11-15

> Remove `packaging/mcp/server.json` since Nomos is strictly CLI and hooks; Add root llms.txt, CITATION.cff, and MCP registry `server.json` manifests (Apache 2.0)

### Release & Packaging
- **Remove `packaging/mcp/server.json` since Nomos is strictly CLI and hooks (`SUB-3092`)**: Remove `packaging/mcp/server.json` since Nomos Open-Core is strictly a CLI and ambient hook substrate.
- **Add root llms.txt, CITATION.cff, and MCP registry `server.json` manifests (`SUB-3089`)**: Add root llms.txt, CITATION.cff, and MCP registry `server.json` manifests.

---

## [v4.20.0] - 2026-11-15

> Upgrade InferCleanArchitectureBoundaries to Git-index file reuse and TS/Svelte alias layers; Fix x86_64 SHA-256 digest in packaging/aur/PKGBUILD; Extract cli/hookcmd and verify/intentgate subpackages and split >385 LOC ceiling hotspots; Increase hero SVG typography scale by 36% for GitHub 880px column width (Apache 2.0)

### Intent & Specifications (`intent/`)
- **Increase hero SVG typography scale by 36% for GitHub 880px column width (`INT-2042`)**: Enlarge hero SVG typography by 36% for GitHub 880px README width.

### CLI, Distribution & Hooks (`nomos cli`)
- **Extract cli/hookcmd and verify/intentgate subpackages and split >385 LOC ceiling hotspots (`INT-2046`, `SUB-3059`)**: Extract cli/hookcmd and verify/intentlint helpers and split proof and task-sync hotspots.
- **Unify nomos task status into nomos status, streamline EDIT HATEOAS, and fix polyglot --mutate-diff help (`INT-2044`, `SUB-3057`)**: Top-level 'nomos status' (--json and TUI) surfaces active phase, bound task key, CapBAC lease budget, HATEOAS Next Actions, and wavefront while 'nomos task status' is removed under Law 6.

### Verification & Quality Gates (`nomos verify`)
- **Upgrade InferCleanArchitectureBoundaries to Git-index file reuse and TS/Svelte alias layers (`INT-2045`, `SUB-3058`)**: InferCleanArchitectureBoundaries reuses Git-tracked source files (skipping .gitignore build trees like .svelte-kit and cutting xray latency).

### Release & Packaging
- **Fix x86_64 SHA-256 digest in packaging/aur/PKGBUILD (`SUB-3072`)**: Set exact v4.19.0 x86_64 SHA-256 digest in PKGBUILD.
- **Sync packaging/aur/PKGBUILD SHA-256 digests with published v4.19.0 release (`SUB-3070`)**: Sync PKGBUILD Linux SHA-256 digests with published v4.19.0 release.

---

## [v4.19.0] - 2026-11-14

> Zero-Init Brownfield Debt Scanner (`nomos xray`) & Day-0 Ratchet Baseline (`nomos init --baseline`), Zero-Trust PR Proof & Diff-Scoped AST Mutation Testing (`nomos verify --pr --mutate-diff` + GitHub Action), Multi-Worktree AST Write-Time Collision Radar (`MTPO`), Plus Canonical Lipgloss Terminal Rendering (Apache 2.0)

### Brownfield Onboarding & X-Ray (`nomos xray`, `nomos init`)
- **Zero-Init Brownfield AI Hangover Scanner & Day-0 Ratchet Baseline (`INT-2023`, `SUB-2408`, `SUB-2414`, `SUB-2416`, `INT-2037`, `SUB-3022`, `SUB-3033`)**: Adds read-only Git-index-aware (`git ls-files --cached --others --exclude-standard`) polyglot `nomos xray [dir]` (`--json`, `--markdown`, `--top`) computing a deterministic `0-100` `AI Hangover Score` (`Grade A` to `Grade F`), structural clones, `>400 LOC` monoliths, oversized functions, high-arity (`>8` params) and deep-nesting (`>5` levels) signatures, magic-string duplication clusters, empty/TODO stubs, skipped tests, swallowed errors, secret/host-path leaks, CWE security sinks, and zero-violation hexagonal `architecture_boundaries` in milliseconds without requiring `nomos init` or repository writes (`TC-XRY-01`, `TC-XRY-02`). Aligns Go skip/stub/error/secret detection with `nomos verify`, honors `.gitignore` plus `build.*`/`_app`/hashed webdist exclusions, and adds `nomos init --baseline` (`TC-XRY-03`) to scaffold `nomos.yaml` and freeze Day-0 structural debt bounds (`ObservedMaxParams`, `ObservedMaxNesting`).

### Verification, Mutation Proofs & CI (`nomos verify`, `action.yml`)
- **Zero-Trust PR Proof Receipts & Diff-Scoped AST Mutation Testing (`INT-2024`, `SUB-2409`)**: Adds `nomos verify --pr` (`--pr-output`, `--json`) generating deterministic GitHub PR governance receipts (`GatePostureSummary`, hotspot complexity/LOC deltas, CWE taint summary, boundary purity, and SHA-256 receipt digest, `TC-PRP-01`), pure-Go diff-scoped AST mutation testing via `nomos verify --mutate-diff` (`--mutate-max`) across Go, TypeScript/JS, and Python (`TC-PRP-02`, `TC-PRP-03`) proving modified tests fail when newly added conditionals or return values are mutated, and a root composite GitHub Action (`action.yml`) for 1-step pull-request attestation.

### Multi-Agent Worktree Physics & CLI TUI (`nomos physics`, `nomos hook`, `nomos cli`)
- **Multi-Worktree AST Write-Time Collision Prevention (`INT-2025`, `SUB-2410`)**: Implements `MultiWorktreeSymbolHull` (`MTPO`) across sibling `worktrees/*` sandboxes (`TC-MTP-01..03`), surfacing real-time file and exported AST symbol overlap advisories during `nomos physics` (`--json`), `nomos task start`, and `nomos hook post-tool` before parallel coding agents drift into semantic merge conflicts.
- **Canonical Lipgloss TUI Rendering & Self-Hosting Release Alignment (`SUB-3003`, `INT-2031`, `SUB-3013`, `SUB-3014`)**: Upgrades `nomos xray`, `nomos init --baseline`, `nomos verify --pr`, `nomos verify --mutate-diff`, and `nomos physics` terminal output to the canonical SophiaLabs Lipgloss design system (`substrate/cli/tui/gen2_render.go`), unifies `xray.FormatLanguageBreakdown` for a `0/100` (`Grade A`) self-hosted X-Ray score, canonicalizes `hooks.json` template parity, guards `nomos init` auto-commits on release-provenanced trunks, aligns the `README.md` Open-Core vs Sovereign Gen-2 boundary matrix (`TC-REL-012`), and promotes root `action.yml` with `EvaluatePreTagReleaseAirlock` Go toolchain parity checks.

---

## [v4.18.0] - 2026-11-13

> Upstream surgical ratchet, switch default AST check, and `corpus_command` gate; Close binary-only brownfield init gaps with inferred boundaries, Day-0 Mneme seeding, and lean steering; Restructure `README.md` for progressive disclosure, scannable open-core feature checklist, and elevated brownfield ratchet; Add v4.17.0 5-round paired security and brownfield benchmark receipts (Apache 2.0)

### Intent & Specifications (`README.md`, `intent/`)
- **Restructure `README.md` for progressive disclosure, scannable open-core feature checklist, and elevated brownfield ratchet (`INT-2005`)**: Restructured `README.md` around progressive disclosure with a scannable 6-part Apache 2.0 Open-Core capability list leading with the Zero-Migration Brownfield Debt Ratchet, updated Terminal-Bench 2.0 k=5 metrics (76.40% Pass@1 / 92.13% Pass@5), collapsible detail drawers, and full INT-2008 Sovereign boundary parity.
- **Clarify Open-Core vs Sovereign Enterprise capability boundary in README (`INT-2008`)**: Clarified Open-Core vs Nomos Sovereign capability boundary in `README.md` across Pre-Tool Security Airlock, local Mneme pitfall distillation vs SOMA Dream Cycle/Squad Mesh, 4-Pillar Glass Cockpit, nomos code, and air-gapped NixOS GPU appliances.
- **Pure Lineage SSoT Frontmatter Specification (`INT-2001`)**: Updated `intent/architecture/specs/nomos_controlled_technical_english_spec.md` (Section 6.2 & Section 7) to replace SemVer `version` in canonical Intent YAML frontmatter with the 9 metadata keys plus required non-empty `lineage` (`INT-<N>` / `SUB-<N>`) as the single document revision SSoT, plus active `INT-<N>` append-on-edit enforcement (`TC-NCTE-09`, `TC-NCTE-10`).
- **Lead README benchmark section with cheap-workhorse brownfield and security proofs (`INT-1998`)**: Reorder `README.md` benchmarks to lead with +58.3pp workhorse brownfield lift, 0.00% ASR AgentDojo/SecBench proof, and Terminal-Bench 2.0 container receipts.

### CLI, Distribution & Hooks (`nomos cli`, `nomos hook`)
- **Harden CapBAC PreToolUse compound-command airlock and surface SQLite migration errors (`SUB-2349`)**: Block compound-command airlock/sanitizer bypasses, surface SQLite migration errors, and align `TC-REL-012` with INT-2008 README.
- **Close binary-only brownfield init gaps with inferred boundaries, Day-0 Mneme seeding, and lean steering (`INT-1999`, `SUB-2340`)**: Auto-infer zero-violation `architecture_boundaries` on nomos init, document progressive contracts in `templates/nomos.yaml`, seed Day-0 Mneme architecture baseline with cold-start recall fallback, and steer `intent_twin_mode=off` workspaces directly to lean chore + sync -m.
- **Inline 1-turn task sync (-m) and full-lifecycle OpenCode + OpenRouter parity (`INT-1997`, `SUB-2339`)**: Implement inline '-m, --message' walkthrough summary flag on 'nomos task sync <KEY>' (mutually exclusive with '-F, --file', wired through autoCommitDirtyWorktree, SanitizeAndValidateWalkthrough, validateReceiptTraceParity, Git Notes receipt, and Mneme distillation), surface 1-turn sync in BuildPhaseNextActions and `AGENTS.md` template, upgrade `templates/hooks/nomos.ts.tmpl` with Turn-0 session-start orientation injection and session.idle anti-phantom stop verification, and add E2E cleanroom tests measuring small-task CLI ceremony reduction.

### Verification & Quality Gates (`nomos verify`)
- **Implement upstream surgical ratchet, switch default AST check, and `corpus_command` gate (`INT-2011`, `SUB-2378`)**: Implement RatchetMode (standard|upstream) with collateral extraction and copyright email checks, Go non-terminal switch default check, and CorpusCommand execution in GoToolchainGate with `TC-SSOT-07` and `TC-TS-15..18` tests.
- **Optimize verify pipeline with shared Go AST cache, parallel static gates, in-memory proof checks, and verify subpackage extraction (`SUB-2350`)**: Add shared AST/walk cache, parallel gate pool, in-memory proof checks, and intentlint subpackage extraction.
- **Harden verify proof subpackage scan and backfill astdebt/polyrun unit tests (`SUB-2348`)**: Hardened verify proof subpackage AST scan and selector-type GateResult vacuity detection, diversified mutation sampling across distinct files, backfilled astdebt/polyrun unit tests, and scrubbed test fixture naming.
- **Disambiguate Unqualified Spec Struct Collisions by Nearest Field Distance (`SUB-2346`)**: Replaced naive multi-package struct field merging in `DiffSpecStructsBijective` (`substrate/src/nomos/modules/verify/impact/bijective_contracts.go`) with `resolveBestCandidateFields` / `structFieldDistance` so unqualified specification `go` blocks (`key.Pkg == ""`) select the nearest matching compiled package struct rather than merging unrelated same-named structs across packages (such as `social.Violation` vs `domaincalc.Violation`).
- **Seal 8-Key Static Intent Frontmatter Allowlist (`INT-2002`, `SUB-2344`)**: Removed `version`, `lineage`, and `last_updated` from `allowedStrictFrontmatterKeys` and `intentFrontmatter` in `substrate/src/nomos/modules/verify/intent_steelman.go` (deleting `semverPattern` and `lineageTaskKeyPattern`), sealing canonical Intent frontmatter to the 8 static identity keys (`authors`, `classification`, `date`, `plane`, `ssot_repository`, `status`, `title`, `type`).
- **Expand Static 8-Key Intent Frontmatter Support (`SUB-2343`)**: Updated `validateStrictIntentFrontmatter` in `substrate/src/nomos/modules/verify/intent_steelman.go` so strict canonical Intent documents require only the 8 static identity keys (`authors`, `classification`, `date`, `plane`, `ssot_repository`, `status`, `title`, `type: spec`), preparing the corpus to drop `version`, `lineage`, and `last_updated`.
- **Enforce bijective 4-layer intent compiler parity and commit-delta sibling isolation (`INT-2000`, `SUB-2342`)**: Implemented bijective Go struct field and canonicalized type parity in VerifySpecTypeSignatures (`TC-OBL-012`), AST-based orphaned `@verifies` TC-* detection in VerifyNoOrphanedTestObligations (`TC-OBL-013`), self-hosted normative registry surjection across Gate ProbeIDs, core/config structs, CLI flags, and ERR_* codes in VerifyNormativeRegistrySurjection (`TC-OBL-014`), and commit-delta sibling obligation isolation across shared Layer 2 domain specifications in `intent_obligations_sibling.go` (`TC-OBL-015`).
- **Enforce strict `spec_roots` Steel-Man validation for downstream multi-directory intent repositories (`INT-1996`, `SUB-2316`)**: Implemented downstream canonical `spec_roots` Steel-Man scoping in `substrate/src/nomos/modules/verify/intent_steelman_scoped.go` (`twinned with INT-1996`, `TC-TRC-005`).

### Release & Packaging (`README.md`, `.github/workflows/ci.yaml`)
- **Add v4.17.0 5-round paired security and brownfield benchmark receipts (`SUB-2329`)**: Added `5-Round Live Multi-Model Security Audit (N = 400)` (`0.00%` ASR across `0/200` governed trials vs. `9.0%` Control; `92.5%` benign utility in `-2.6%` fewer turns) and `Real-Repo Zero-Hint Brownfield Audit (233k LOC OSS)` (`2x` debt-free resolution in `-14.7%` fewer tokens) to the top-level benchmark summary table in `README.md`.
- **Consolidate CI and Release GitHub Actions into single verify-then-release pipeline (`SUB-2311`)**: `.github/workflows/ci.yaml`: Added `workflow_dispatch` (`inputs.tag`), `detect_release` step in `verify` (detecting `vX.Y.Z` tags at `HEAD` on `main` pushes when not yet published, or explicit `workflow_dispatch` tags), and a downstream `goreleaser` job (`needs: verify`) gated on `should_release == 'true'`.

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
