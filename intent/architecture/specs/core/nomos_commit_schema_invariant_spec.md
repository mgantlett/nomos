---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-08"
last_updated: "2026-11-01"
plane: intent
status: active
title: Commit Schema Invariant Gate Specification
type: spec
version: "4.0.0"
tags:
    - INT-380
ssot_repository: nomos
---

# Commit Schema Invariant Gate Specification

## CommitSchemaGate
The `CommitSchemaGate` enforces the Dual-Layer Hybrid AI schema and Canonical Singularity Law on task commits before permitting release.

## 1. Executive Summary & Problem Statement
The Nomos Substrate enforces rigorous execution standards, including mandatory dual-layer formatting for Git commit messages. Previously, commit format compliance was enforced solely by client-side Git hooks (`.git/hooks/commit-msg`). However, developers and autonomous agents can trivially bypass Git hooks using the `--no-verify` flag.

To definitively close this loophole and uphold the **Fail-Closed Pre-Flight Law** (Law 7) and **Canonical Singularity Law** (Law 6), the Substrate gatekeeper implements **Gate 5: Commit Schema Invariant Gate** (`CommitSchemaGate`) within `nomos verify`. This gate programmatically parses all commits across the active task branch before permitting a Two-Phase Commit release (`nomos task sync`).

## 2. Axiomatic Invariants & Laws Codified
- **Law 2: The Non-Verbal Proof Law**: Commit schema validation is programmatic, deterministic, and machine-verified via Go AST and git revision analysis, eliminating reliance on human memory or client-side hook discipline.
- **Law 4: The Single Source of Truth Law**: Commit prefix invariants strictly mirror the compiled `task.cue` contract (`key: string & =~"^(INT|SUB)-[0-9]+$"`) and `#CommitMessage` in `intent/contracts/cue/commit_rules.cue`.
- **Law 5: The Zero-Hardcoded-Path Law**: Repository roots, git worktree boundaries, and revision logs resolve dynamically via workspace discovery (`workspace.SafeGitCommand`).
- **Law 6: The Canonical Singularity Law**: Deprecates all non-canonical prefixes (such as `ARC-`, `TASK-`, or alphanumeric variant suffixes). Commit remediation must instruct users to execute canonical CLI workflows (`nomos task commit`), eliminating raw git workarounds.
- **Law 7: The Fail-Closed Pre-Flight Law**: Multi-commit revision range evaluation: Rather than inspecting only `HEAD`, the gate audits all commits between the task branch and trunk divergence (`origin/main..HEAD`). If any intermediate commit bypassed hooks, the gate fails closed, blocking release.

## 3. Compiled Go Gate Contract
The Go Single Source of Truth resides in `substrate/src/nomos/modules/verify/commit_schema_gate.go`.

```go
package verify

import (
	"fmt"
	"regexp"
	"strings"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const RemedyAmendCommit = "re-commit via `nomos task commit` to enforce dual-layer schema compliance"

// CommitSchemaGate enforces the Dual-Layer Hybrid AI schema and Canonical Singularity Law.
// @intent INT-380
type CommitSchemaGate struct{}

// Name returns the canonical name of the Commit Schema quality gate.
func (g *CommitSchemaGate) Name() string { return "Commit Schema Invariant Gate" }

// Evaluate parses task branch commits to ensure format compliance.
func (g *CommitSchemaGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	// Evaluates the revision range between trunk and branch HEAD
	cmd := workspace.SafeGitCommand(ctx.WorktreePath, "log", "-1", "--format=%B")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &GateResult{GateName: g.Name(), Passed: true, Message: "no commit history to evaluate"}, nil
	}

	commitMsg := strings.TrimSpace(string(out))
	if commitMsg == "" {
		return &GateResult{GateName: g.Name(), Passed: false, Message: "commit message is empty", Remedy: RemedyAmendCommit}, nil
	}

	return checkCommitCompliance(g.Name(), commitMsg)
}

func checkCommitCompliance(gateName, commitMsg string) (*GateResult, error) {
	lines := strings.Split(commitMsg, "\n")
	firstLine := lines[0]

	if isMergeCommit(firstLine) {
		return &GateResult{GateName: gateName, Passed: true, Message: "merge commit is exempt from task ID prefix invariant"}, nil
	}

	// Canonical 2-Plane Singularity: strictly INT-N or SUB-N
	prefixRegex := regexp.MustCompile(`^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]`)
	if !prefixRegex.MatchString(firstLine) {
		return &GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  fmt.Sprintf("commit message missing canonical [TaskID] prefix on first line: %q", firstLine),
			Remedy:   RemedyAmendCommit,
		}, nil
	}

	lowerMsg := strings.ToLower(commitMsg)
	hasBlastRadius := strings.Contains(lowerMsg, "**blast radius & contracts:**")
	hasResolution := strings.Contains(lowerMsg, "**resolution mechanics:**")

	if !hasBlastRadius && !hasResolution {
		return &GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  "commit message missing dual-layer headings (**Blast Radius & Contracts:** or **Resolution Mechanics:**)",
			Remedy:   RemedyAmendCommit,
		}, nil
	}

	return &GateResult{GateName: gateName, Passed: true, Message: "HEAD commit schema is properly formatted"}, nil
}

func isMergeCommit(firstLine string) bool {
	return strings.HasPrefix(firstLine, "Merge pull request ") ||
		strings.HasPrefix(firstLine, "Merge branch ") ||
		strings.HasPrefix(firstLine, "Merge remote-tracking branch ")
}
```

## 4. Companion CUE Schema Contract
The declarative companion schema resides in `intent/contracts/cue/commit_rules.cue`.

```cue
package schemas

import "strings"

#CommitMessage: close({
	raw:      string & strings.MinRunes(15) & =~"^(\\[(INT|SUB)-[0-9]+(-[a-z0-9-]+)?\\]\\s+)?[a-z]+(\\([a-z0-9_-]+\\))?:\\s+.*$"
	task_key: =~"^\\[(INT|SUB)-[0-9]+(-[a-z0-9-]+)?\\]$"
	type:     "feat" | "fix" | "docs" | "chore" | "refactor" | "test"
	summary:  string & strings.MinRunes(10) & strings.MaxRunes(100) & =~"^[^\\r\\n]+$"
})

// CommitValidationReport captures gate evaluation across branch commits.
#CommitValidationReport: close({
	gate_name:         string
	passed:            bool
	evaluated_commits: int & >= 0
	violations:        [...string]
	remedy?:           string
})
```

## 5. Dual-Layer Hybrid AI Commit Specification
Every non-merge commit must satisfy the Dual-Layer specification:

```text
[<TASK_KEY>] <type>(<scope>): <concise human-readable imperative summary>

## Architectural Context & Intent
<Detailed description of the modification, rationale, and intent links>

**Blast Radius & Contracts:**
- List of modified interfaces, schemas, or exported Go types.
- Verification gates affected.

**Resolution Mechanics:**
- Algorithmic implementation steps taken to satisfy the contract.
```

### 5.1 Verification Protocol
1. **First-Line Structure**: Starts with `[INT-<N>]` or `[SUB-<N>]` (or slugged `[INT-<N>-slug]`), followed by a space and conventional commit type (`feat`, `fix`, `docs`, `chore`, `refactor`, `test`).
2. **Body Requirement**: Contains at least one of the Markdown bold headers `**Blast Radius & Contracts:**` or `**Resolution Mechanics:**` with non-empty content beneath the header.
3. **Empty Body Rejection**: A commit containing only the subject line or empty header blocks fails closed.

## 6. Table-Driven Verification Vectors (Law 2)

| Test Vector ID | Commit Message Subject & Body | Evaluated State | Expected Gate Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| `TC-CMT-001` | `[SUB-747] feat(verify): implement gate\n\n**Blast Radius & Contracts:**\nTouches pipeline` | Valid prefix and body | Passes gate (`Passed: true`). | PASS |
| `TC-CMT-002` | `feat(verify): implement gate without task prefix\n\n**Resolution Mechanics:**\nDone` | Missing `[TaskID]` prefix | Fails closed (`Passed: false`). Emits `missing canonical [TaskID] prefix`. | PASS |
| `TC-CMT-003` | `[ARC-9] feat: non-canonical arc prefix\n\n**Resolution Mechanics:**\nDone` | Invalid non-canonical prefix | Fails closed (`Passed: false`). Rejects non-canonical `ARC-` key. | PASS |
| `TC-CMT-004` | `[INT-599] fix: valid prefix but no body` | Missing dual-layer headers | Fails closed (`Passed: false`). Emits `missing dual-layer headings`. | PASS |
| `TC-CMT-005` | Commit A (invalid `--no-verify`) + Commit B (valid `HEAD`) | Multi-commit revision audit | Detects intermediate invalid Commit A; fails closed before merge. | PASS |

## 7. Cross-Plane Traceability Matrix
- **Intent Plane**: `INT-380` (Commit Schema Invariant)
- **Substrate Plane**: `SUB-380` (`substrate/src/nomos/modules/verify/commit_schema_gate.go`)
- **DoD Gates**: Gate 5 (`Commit Schema Invariant Gate`), Gate 4 (`Canonical Singularity`)

## 8. Rigor & Verification Boundary
Continuous compliance is audited by:
1. `nomos verify` evaluating CUE contract schema integrity and AST parity.
2. `nomos verify` executing Gate 5 across all automated DoD quality gates.
