# Contributing to Nomos

Welcome to **Nomos**! This guide covers the ground rules and local verification workflow for contributing to the open-core substrate.

## Open-Core Scope

This repository (`nomos`) contains the Apache 2.0 indirect-control governance harness for **Tier 1 coding agents** (Claude Code, Cursor, Antigravity, OpenCode, Windsurf, Aider), the 42-gate polyglot AST & toolchain verification engine, pure-Go SQLite Living Ledger persistence, and **Nomos Mneme** hybrid vector memory.

Enterprise multi-seat orchestration, autonomous Tier 2 coding agents, and hosted control-plane surfaces live outside this repository (see [`open_core_license_spec.md`](intent/architecture/specs/open_core_license_spec.md)). Never commit hardcoded host paths, secrets, or proprietary dependencies.

## The 7 Statutory Laws

Every contribution is verified deterministically against the 7 core architectural laws (`nomos verify`):

1. **Dual-Core Inversion (`Law 1`):** The compiled Go substrate governs state and verification; Tier 1 coding agents operate under indirect control.
2. **Non-Verbal Proof (`Law 2`):** Code correctness requires compiler exit code `0`, automated tests, and zero hollow stubs or skipped assertions.
3. **Hermetic Worktrees (`Law 3`):** Protected trunk (`main`) is physically read-only; edits occur strictly inside isolated task worktrees (`worktrees/nomos-<KEY>`).
4. **Single Source of Truth (`Law 4`):** Every configuration and contract schema binds to a single compiled Go SSoT (`nomos.schema.json`).
5. **Zero Hardcoded Paths (`Law 5`):** Machine-specific absolute paths and unconfigured ports are rejected at AST level.
6. **Canonical Singularity (`Law 6`):** Pure `INT-*` / `SUB-*` task keys, zero Cobra command aliases, and zero filesystem symlinks.
7. **Fail-Closed Pre-Flight (`Law 7`):** Strict 2-plane separation (`INT-*` for `intent/`, `SUB-*` for `substrate/`), dependency/secret shields, and test coverage floors.

## Local Development Workflow

1. **Build & install locally** (Go `1.26+`, `CGO_ENABLED=0`):
   ```bash
   go -C substrate install ./cmd/nomos
   nomos handshake
   ```
2. **Provision an isolated task worktree** (`INT-*` for specs in `intent/`, `SUB-*` for Go code in `substrate/`, or `nomos task pair` when changing both planes):
   ```bash
   # Fast-lane 1-turn worktree for focused fixes:
   nomos task chore "Fix parser edge case" --plane substrate

   # Or full 3-section card / twinned spec + code pair:
   nomos task pair "Add verification probe" \
     --int-scope "Document contract in intent/architecture/specs/..." \
     --sub-scope "Implement probe in substrate/src/nomos/modules/verify/..." \
     --ac "Pass all 42 DoD gates and unit tests" \
     --verify "go -C substrate test ./... && nomos verify"
   nomos task start SUB-<N> --edit
   ```
3. **Implement & test inside `worktrees/nomos-<KEY>`**:
   ```bash
   cd worktrees/nomos-<KEY>
   go -C substrate test ./...
   nomos verify
   ```
   Keep Go source files `<= 400 LOC`, functions `<= 60 LOC`, cyclomatic complexity `<= 15`, and cognitive debt score `<= 45`. If `substrate/src/nomos/core/config/config.go` changes, regenerate `nomos.schema.json` to maintain byte parity.
4. **Commit & verify**:
   ```bash
   nomos task commit <KEY> -m "feat(verify): describe change concisely"
   nomos verify --strict
   ```
   Locally on trunk, maintainers merge via atomic Two-Phase Commit (`nomos task sync <KEY> -F .tmp/walkthrough-<KEY>.md`); external contributors can push their `feature/<KEY>` branch and open a Pull Request.

## Pull Requests & Contributor Provenance (DCO)

All Pull Requests run the full Go unit/cleanroom test suite and `nomos verify --all --strict` (`42/42` Definition of Done quality gates) in GitHub Actions CI. Please include table-driven unit tests alongside all behavioral changes.

Contributions are governed inbound-equals-outbound under Section 5 of the [Apache License, Version 2.0](LICENSE) (no separate Contributor License Agreement is required). External contributors must certify provenance under the [Developer Certificate of Origin (DCO 1.1)](https://developercertificate.org/) by signing off commits (`git commit -s`), adding:

```text
Signed-off-by: Your Name <your.email@example.com>
```
