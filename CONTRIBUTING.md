# Contributing to Nomos Open-Core Substrate

Welcome to the Nomos project! We are thrilled to have you contribute. This document sets out the ground rules and guidelines for participating in the deterministic open-core substrate.

## The Open-Core Philosophy

The `nomos` repository contains the control harness and foundational logic for the agentic engineering ecosystem. Downstream logic, private sovereign layers, and Tier 2 proprietary agents exist separately in closed repositories (`gsi-sov`).
If you find yourself hardcoding internal IP, paths, or secrets, **do not commit**. All code must abide by the **Zero-Hardcoded-Path Law (Law 5)**.

## The 7 Laws of Deterministic Engineering

Before contributing, please review the 7 core invariants enforced by our CI and quality gates:

1. **The Dual-Core Inversion Law:** The open-core substrate indirect-controls the cognitive coding agent.
2. **The Non-Verbal Proof Law:** Claims don't matter; compile exit code 0 and AST proofs are required.
3. **The Hermetic Worktree Law:** Trunk branches (`main`) are read-only. Work strictly in isolated transient worktrees (`worktrees/<repo>-<task>`).
4. **The Single Source of Truth Law:** Every contract has exactly one compiled Go SSoT.
5. **The Zero-Hardcoded-Path Law:** No path literals or topology hardcoding. Resolve paths dynamically.
6. **The Canonical Singularity Law:** Zero aliases or duplicates in schemas or CLI.
7. **The Fail-Closed Pre-Flight Law:** Mutations require declarative explicit configuration.

## Development Workflow

1. Start your task using `nomos task start <KEY>`. This provisions an isolated hermetic worktree.
2. Make your edits exclusively within `worktrees/nomos-<KEY>`.
3. Before submitting, ensure that `nomos verify` passes with **0 failures**. This executes 42 statutory Definition of Done (DoD) Go-native quality gates locally.
4. If everything passes, use `nomos task sync <KEY> -F .tmp/walkthrough.md` to trigger the atomic 2PC release.

## Pull Requests

Since the release pipeline relies on `goreleaser` for deterministic binary snapshots, all PRs must successfully pass the verification gates and GitHub Actions CI. Code must be accompanied by comprehensive tests satisfying our TDD invariant.

Thank you for building the future of agentic coding with us!
