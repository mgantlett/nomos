---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-11-03"
last_updated: "2026-11-06"
lineage:
  - INT-896
  - SUB-1668
  - INT-950
  - SUB-1840
plane: intent
ssot_repository: nomos
status: active
title: "Sanitized Diagnostic Report & Issue Reporting CLI Specification"
type: spec
version: "4.3.0"
---

# Sanitized Diagnostic Report & Issue Reporting CLI Specification

---

## 1. Executive Summary & Problem Statement

Open-core adopters require a zero-network diagnostic collector for runtime context (`Nomos`, `Go`, OS, `Git`, phase, worktree topology, and SQLite health). That collector never leaks repository directory names, private branch names, cloud credentials, or database URIs. When Tier 1 coding agents invoke `nomos` non-interactively, agents mask tool friction without informing the human operator.

This specification codifies offline-first `nomos issue` (`substrate/cli/cmd_issue.go`), topology scrubbing (`substrate/cli/cmd_issue_scrub.go`), and the local `SubstrateAnomalyEvent` ring buffer (`substrate/cli/diagnostics_anomaly.go`). By default, `nomos issue` writes `.tmp/nomos-diagnostic.md` offline with zero network calls and zero unprompted browser spawns. Operators choose between public GitHub issues and private email submission.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Go records local anomalies and alerts operators during `nomos task sync`. | `RecordSubstrateAnomaly`, `renderTaskSyncAnomalyAlert` |
| **2** | **The Non-Verbal Proof Law** | Reports attach verified versions, `ActivePhase`, `BranchTopology`, `DatabaseStatus`, and `RecentAnomalies`. | `formatIssueMarkdown` (`Cell 2.1 (tdd)`) |
| **4** | **The Single Source of Truth Law** | Report fields bind 1:1 to `cli.IssueReport` and `cli.SubstrateAnomalyEvent`. | `cmd_issue_test.go` & Go compiler |
| **5** | **The Zero-Hardcoded-Path Law** | Scrubber redacts repository roots, home paths, and private branch names. | `sanitizeDiagnosticText`, `classifyBranchTopology` (`Cell 5.1 (forbidden_literals)`) |
| **6** | **The Canonical Singularity Law** | `nomos issue` registers as the sole diagnostic command on `NewRootCmd()`. | `Cell 6.2 (zero_alias)`, `Cell 2.1 (dead_code)` |
| **7** | **The Fail-Closed Pre-Flight Law** | Default execution stays offline and blocks non-TTY or non-GitHub browser spawns. | `openBrowser`, `isInteractive` (`Cell 7.1 (secret)`) |

---

## 3. Compiled Go SSoT Contract (`substrate/cli/cmd_issue.go` & `substrate/cli/diagnostics_anomaly.go`)

Compiled Go structures and command constructors define the diagnostic report and local anomaly ring-buffer contracts:

```go
package cli

import "github.com/spf13/cobra"

// SubstrateAnomalyEvent records a scrubbed, local-only substrate anomaly or gate thrash event (#SubstrateAnomalyEvent).
type SubstrateAnomalyEvent struct {
	Timestamp      string `json:"timestamp"`
	NomosVersion   string `json:"nomos_version"`
	TaskKey        string `json:"task_key,omitempty"`
	Command        string `json:"command"`
	Kind           string `json:"kind"`
	Component      string `json:"component"`
	Summary        string `json:"summary"`
	Occurrences    int    `json:"occurrences"`
	DurationMillis int64  `json:"duration_ms,omitempty"`
	Surfaced       bool   `json:"surfaced,omitempty"`
}

// IssueReport holds the collected and sanitized diagnostic report (#IssueReport).
type IssueReport struct {
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	NomosVersion    string                  `json:"nomos_version"`
	GoVersion       string                  `json:"go_version"`
	Platform        string                  `json:"platform"`
	GitVersion      string                  `json:"git_version"`
	ActivePhase     string                  `json:"active_phase"`
	BranchTopology  string                  `json:"branch_topology"`
	IsWorktree      bool                    `json:"is_worktree"`
	DatabaseStatus  string                  `json:"database_status"`
	RecentAnomalies []SubstrateAnomalyEvent `json:"recent_anomalies,omitempty"`
}

// NewIssueCmd constructs the canonical 'nomos issue' command.
func NewIssueCmd() *cobra.Command
```

---

## 4. Deterministic Topology, Path & Secret Scrubbing (`cmd_issue_scrub.go`)

Before writing `.tmp/nomos-diagnostic.md` or encoding URL parameters, the scrubber applies seven ordered redactions:
1. **Branch Topology Classification:** `classifyBranchTopology` maps git branches onto four classes (`trunk`, `task-worktree`, `detached`, or `custom-branch`). Raw enterprise or customer branch names never appear in output.
2. **Repository Root Redaction:** Replaces the active repository root path with `<repo-root>` before home-directory substitution.
3. **Home & User Path Redaction:** Replaces `os.UserHomeDir()` and `/Users/<name>/` or `/home/<name>/` prefixes with `~/`.
4. **GitHub Token Redaction:** Scrubs `gh[pousr]_*` and `github_pat_*` tokens to `[REDACTED_GH_TOKEN]`.
5. **Cloud & API Key Redaction:** Scrubs `sk-*`, `AIza*`, `AKIA*`, `ASIA*`, and `xox[baprs]-*` tokens to `[REDACTED_API_KEY]`.
6. **Bearer & Private Key Redaction:** Scrubs `Bearer <token>` headers to `Bearer [REDACTED]` and PEM private key blocks to `[REDACTED_PRIVATE_KEY]`.
7. **Multi-Scheme URI Credential Redaction:** Scrubs `<scheme>://<user>:<pass>@` across HTTP, PostgreSQL, MySQL, Redis, MongoDB, and AMQP URIs to `<scheme>://user:[REDACTED]@`.

---

## 5. Local Substrate Anomaly Ring Buffer & `nomos task sync` Surfacing

Nomos maintains a 32-entry anomaly ring buffer inside `<UserCacheDir>/nomos/diagnostics/` (`0700` permissions), keeping the repository worktree clean:
1. **Captured Anomaly Classes:**
   - `internal_panic`: Recovered runtime panics in CLI command dispatch (`Execute`).
   - `substrate_error`: Internal `ERR_CORE` or `ERR_SUBSTRATE` command failures.
   - `gate_slow`: Single DoD gate execution exceeding `5000ms` or returning an evaluation error.
   - `gate_thrash`: Identical DoD gate failing `4` or more times within one active task key.
2. **Deterministic `nomos task sync` Banner:**
   - When `0` unsurfaced anomalies exist for the synced task, `nomos task sync` emits zero extra lines.
   - When unsurfaced anomalies exist, `nomos task sync` prints a deterministic warning after release completion. The warning points operators to `nomos issue` and marks those events `Surfaced = true`.

---

## 6. CLI Command Syntax, Offline Default & Submission Channels

```text
nomos issue [title] [--output|-o <file>] [--dry-run] [--web|-w] [--yes|-y] [--body|-b <text>] [--clear]
```

| Flag | Shorthand | Default | Semantics |
| :--- | :---: | :---: | :--- |
| `--output` | `-o` | `".tmp/nomos-diagnostic.md"` | Writes sanitized Markdown report to a `0600` local file. |
| `--dry-run` | `-` | `false` | Prints sanitized Markdown to `stdout` without writing files. |
| `--web` | `-w` | `false` | Opens GitHub issue URL in browser when TTY is interactive. |
| `--yes` | `-y` | `false` | Skips `[y/N]` prompt when `--web` runs in a TTY. |
| `--body` | `-b` | `""` | Supplies reproduction notes scrubbed via `sanitizeDiagnosticText`. |
| `--clear` | `-` | `false` | Clears recorded entries from the local anomaly ring buffer. |

Every generated Markdown report lists two submission channels:
1. **Public Open-Source Tracker:** Pre-filled `https://github.com/mgantlett/nomos/issues/new` link.
2. **Private / Enterprise Channel:** Direct email instructions (`support@sophialabs.ca`) requiring zero GitHub identity.

---

## 7. Table-Driven Verification Vectors

| Vector ID | Scenario | Input Vector | Expected Verdict | Invariant Verified |
| :--- | :--- | :--- | :--- | :--- |
| `TC-ISS-01` | Offline Default | `nomos issue "Gate timeout"` | Writes `.tmp/nomos-diagnostic.md` offline with zero browser launch | Law 5 & Law 7 |
| `TC-ISS-02` | Branch Redaction | Branch `feature/acme-bank-wire-fix` | Emits `custom-branch` and omits `acme-bank-wire-fix` | Law 5 |
| `TC-ISS-03` | Multi-Scheme Scrub | `postgres://u:pw@db/app` and `<repoRoot>/a.go` | Emits `postgres://user:[REDACTED]@db/app` and `<repo-root>/a.go` | Law 5 & Law 7 |
| `TC-ISS-04` | Non-TTY Guard | `nomos issue --web` without TTY | Refuses browser spawn and prints sanitized URL | Law 7 |
| `TC-ISS-05` | URL Prefix Guard | `openBrowser("https://example.com/phish")` | Fails closed (`refusing to open non-GitHub URL`) | Law 7 |
| `TC-ISS-06` | Sync Anomaly Alert | `4` identical gate failures on `SUB-42` | Records `gate_thrash` and prints sync alert once | Law 1 & Law 2 |

---

## 8. Cross-Plane Traceability & DoD Verification

| Component | Substrate SSoT Path | Canonical 7x3 Cell |
| :--- | :--- | :--- |
| **Issue Diagnostic Collector** | `substrate/cli/cmd_issue.go` | `Cell 6.2 (zero_alias)`, `Cell 2.1 (dead_code)` |
| **Topology & Secret Scrubber** | `substrate/cli/cmd_issue_scrub.go` | `Cell 5.1 (forbidden_literals)`, `Cell 7.1 (secret)` |
| **Local Anomaly Ring Buffer** | `substrate/cli/diagnostics_anomaly.go` | `Cell 1.1 (phase_discipline)`, `Cell 2.1 (tdd)` |
| **Task Sync Anomaly Alert** | `substrate/cli/cmd_task_sync.go` | `Cell 1.3 (audit_trail)` |
| **Issue & Anomaly Test Suite** | `substrate/cli/cmd_issue_test.go` | `Cell 2.1 (tdd)`, `Cell 2.2 (coverage)` |

Deterministic verification executes via `nomos verify` and passes all 42 Definition of Done quality gates with exit code `0`.
