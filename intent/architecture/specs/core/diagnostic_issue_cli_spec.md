---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-11-03"
last_updated: "2026-11-03"
plane: intent
ssot_repository: nomos
status: active
tags:
    - INT-896
    - SUB-1668
title: "Sanitized Diagnostic Bundle & Issue Reporting CLI Specification"
type: spec
version: "4.2.0"
---

# Sanitized Diagnostic Bundle & Issue Reporting CLI Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Codify the canonical `nomos issue` CLI command (`substrate/cli/cmd_issue.go`), `cli.IssueReport` diagnostic contract, automatic credential/path scrubbing invariants, and fail-closed browser dispatch semantics.

---

## 1. Context & Problem Statement

Open-core adopters reporting bugs or quality gate edge cases need a one-command way to capture accurate runtime context (Nomos version, Go runtime, OS/architecture, Git version, workspace lifecycle phase, worktree state, and SQLite health) without accidentally leaking personal filesystem paths, GitHub Personal Access Tokens, API keys, or HTTP bearer credentials into a public issue tracker.

`nomos issue` provides a zero-dependency, client-side diagnostic collector that scrubs sensitive strings deterministically before previewing (`--dry-run`) or opening a pre-filled GitHub issue URL.

---

## 2. Axiomatic Invariants & Statutory Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Diagnostic telemetry is collected deterministically from Go runtime and git state without LLM summarization. | `collectIssueReport` |
| **2** | **The Non-Verbal Proof Law** | Bug bundles attach machine-verified `NomosVersion`, `GoVersion`, `GitVersion`, `ActivePhase`, and `DatabaseStatus`. | `formatIssueMarkdown` |
| **4** | **The Single Source of Truth Law** | Diagnostic bundle fields bind 1:1 to `cli.IssueReport` in `substrate/cli/cmd_issue.go`. | `cmd_issue_test.go` & Go compiler |
| **5** | **The Zero-Hardcoded-Path Law** | User home directories discovered via `os.UserHomeDir()` are scrubbed to `~` prior to output or URL encoding. | `sanitizeText` |
| **6** | **The Canonical Singularity Law** | `nomos issue` is the sole canonical bug-report command registered on `NewRootCmd()`. | `ZeroAliasGate`, `DeadCodeGate` |
| **7** | **The Fail-Closed Pre-Flight Law** | Browser launch refuses any target URL outside `https://github.com/` and prompts interactively unless `--yes` or `--dry-run` is set. | `openBrowser` URL prefix guard |

---

## 3. Deterministic Secret & Path Scrubbing (`sanitizeText`)

Before populating `#IssueReport.Title` or `#IssueReport.Description`, `sanitizeText` applies five ordered transformations:
1. **Home Path Redaction:** Replaces `os.UserHomeDir()` occurrences with `~`.
2. **GitHub Token Redaction:** Scrubs `ghp_*` and `github_pat_*` tokens to `[REDACTED_GH_TOKEN]`.
3. **API Key Redaction:** Scrubs `sk-*` secret keys to `[REDACTED_API_KEY]`.
4. **Bearer Header Redaction:** Scrubs `Bearer <token>` headers (case-insensitive) to `Bearer [REDACTED]`.
5. **Authenticated URL Redaction:** Scrubs `http(s)://<user>:<pass>@` basic-auth credentials to `http(s)://user:[REDACTED]@`.

---

## 4. CLI Command Syntax & Flags (`substrate/cli/cmd_issue.go`)

```text
nomos issue [title] [--dry-run] [--web|-w] [--yes|-y] [--body|-b <text>]
```

| Flag | Shorthand | Default | Semantics |
| :--- | :---: | :---: | :--- |
| `--dry-run` | — | `false` | Renders the sanitized diagnostic summary, Markdown payload, and encoded GitHub URL to stdout without launching a browser. |
| `--web` | `-w` | `true` | Opens the pre-filled GitHub issue URL in the OS default browser (`xdg-open`, `open`, or `rundll32`). |
| `--yes` | `-y` | `false` | Skips interactive `[y/N]` confirmation prompt when launching the browser in a TTY. |
| `--body` | `-b` | `""` | Supplies additional reproduction details or error logs (automatically passed through `sanitizeText`). |

---

## 5. Table-Driven Verification Vectors

| Vector ID | Scenario | Input Vector | Expected Verdict | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-ISS-01` | Dry-Run Preview | `nomos issue "Gate edge case" --dry-run` | Prints sanitized Markdown payload and `https://github.com/mgantlett/nomos/issues/new?...` without opening browser | Law 2 & Law 6 |
| `TC-ISS-02` | Credential Scrubbing | `--body` containing a `ghp_<20+char>` PAT and `sk-<20+char>` key | Emits `[REDACTED_GH_TOKEN]` and `[REDACTED_API_KEY]` with zero raw secrets | Law 5 & Law 7 |
| `TC-ISS-03` | URL Prefix Guard | `openBrowser("https://example.com/phish")` | Fails closed (`refusing to open non-GitHub URL`) | Law 7 (Fail-Closed) |

