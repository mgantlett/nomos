---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-05"
last_updated: "2026-11-05"
plane: intent
status: active
title: "Invisible Harness Asset Embedding & Ambient Hook Parity Specification"
type: spec
version: "4.0.0"
execution_frontier:
  tasks:
    - key: "INT-819"
      plane: "intent"
      summary: "Excise legacy .agents/workflows and codify zero-ceremony Invisible Harness ambient hooks"
    - key: "SUB-1381"
      plane: "substrate"
      summary: "Remove workflow templates, scaffold ambient SessionStart/PreToolUse hooks, and enforce Law 4/Law 6 parity"
ssot_repository: nomos
---

# Invisible Harness Asset Embedding & Ambient Hook Parity Specification

## 1. Executive Summary & Architectural Mandate

Under **Radical Minimalism** and **Law 1 (Dual-Core Inversion)**, developers preserve their natural engineering workflow without memorizing `/nomos-*` slash commands or skill walls. This zero-ceremony guarantee holds across every Tier 1 coding agent (`Antigravity`, `Claude Code`, `Cursor`, `OpenCode`, `Aider`, `Windsurf`).

Historically, Nomos distributed markdown workflow playbooks under `.agents/workflows/` (such as `/nomos-handshake` and `/nomos-idle`). Full Go phase enforcement and modern harness evolution rendered flat conversational workflow files redundant under **Law 6 (Canonical Singularity)**.

This specification codifies the **Zero-Ceremony Invisible Harness Asset Architecture**:
1. **Complete Excision of Workflows & Skill Walls:** Nomos permanently retires `.agents/workflows/` and scaffolds zero conversational `.agents/skills/`. Developers direct their Tier 1 agent in natural prose with zero workflow friction.
2. **Three-Layer Ambient Indirect Control:**
   - **Universal Always-On Protocol (`AGENTS.md` & `CLAUDE.md`):** Concise invariant card read natively at repository root by every Tier 1 coding agent.
   - **Ambient IDE Lifecycle Hooks (`.agents/hooks.json` & `.agents/scripts/`):** Pure 16-line launchers invoke `exec nomos hook {session-start,pre-tool,post-tool}` with zero external `jq` fallback logic.
   - **Git CapBAC Defense Hooks (`pre-commit`, `commit-msg`, `pre-push`):** Deterministic substrate backstop guaranteeing invariant enforcement even in agent harnesses that do not support IDE lifecycle hooks.
3. **Cryptographic Byte-Parity & Anti-Regression (`IntentParityGate`):** Go `//go:embed` embeds `AGENTS.md`, `.agents/hooks.json`, and `.agents/scripts/*` inside `substrate/cli/templates/`. Every `nomos verify` run asserts SHA-256 byte parity and rejects template drift or legacy `.agents/workflows/` directories.

---

## 2. Axiomatic Invariants & Laws Codified

| Law # | Law Name | Architectural Enforcement |
| :---: | :--- | :--- |
| **Law 1** | **Dual-Core Inversion** | Go core and ambient hooks govern session handshake, worktrees, and verification invisibly. |
| **Law 2** | **Non-Verbal Proof** | Byte-parity gates and vectors (`TC-EMBED-001` to `TC-EMBED-006`) prove scaffolded asset integrity. |
| **Law 4** | **Single Source of Truth** | Governance assets (`AGENTS.md`, `.agents/hooks.json`, `.agents/scripts/*`) match `substrate/cli/templates/` SHA-256 digests. |
| **Law 5** | **Zero-Hardcoded-Path** | Workspace paths resolve dynamically via `filepath.Join` and `config` discovery helpers. |
| **Law 6** | **Canonical Singularity** | `IntentParityGate` rejects `.agents/workflows/` directories and duplicate conversational skill files. |
| **Law 7** | **Fail-Closed Pre-Flight** | Any SHA-256 template mismatch triggers non-zero rejection in `nomos verify`. |

---

## 3. Canonical Directory Layout

Workspace governance and ambient hook assets maintain a minimal cross-harness layout at the repository root:

```text
nomos/
├── AGENTS.md                          # Universal Tier 1 agent protocol (AGY, Cursor, OpenCode, Aider)
├── CLAUDE.md                          # Claude Code protocol mirror (downstream scaffolding)
├── .agents/
│   ├── hooks.json                     # Ambient SessionStart, PreToolUse & PostToolUse manifest
│   └── scripts/
│       ├── session-start.sh           # Pure exec launcher -> nomos hook session-start
│       ├── worktree-guard.sh          # Pure exec launcher -> nomos hook pre-tool
│       ├── nomos-build-guard.sh       # Pure exec launcher -> nomos hook pre-tool
│       └── post-edit-radar.sh         # Pure exec launcher -> nomos hook post-tool
├── nomos.schema.json                  # Go-reflected Draft 2020-12 JSON Schema
├── intent/
│   └── config/
│       └── nomos.yaml                 # Declarative workspace governance manifest
└── substrate/
    ├── cli/
    │   └── templates/                 # Embedded protocol, config & hook templates (cmd_init.go)
    │       ├── AGENTS.md
    │       ├── nomos.yaml
    │       └── hooks/
    │           ├── hooks.json
    │           └── scripts/
    │               ├── session-start.sh
    │               ├── worktree-guard.sh
    │               ├── nomos-build-guard.sh
    │               └── post-edit-radar.sh
    └── src/
        └── nomos/
            └── modules/
                └── handshake/
                    └── directives/    # Embedded JIT phase directives
```

---

## 4. Substrate Embedding & Parity Contracts (`//go:embed`)

### 4.1 Formal Go DTO Contracts (`workspace/parity.go`)

All parity structures bind strictly to compiled Go types (`AssetType`, `EmbeddedAsset`, `ParityMismatch`, and `ParityAuditReport`) in `substrate/src/nomos/core/workspace/parity.go`:

```go
// AssetType defines the category of embedded governance asset.
type AssetType string

const (
	AssetTypeHook      AssetType = "hook"
	AssetTypeRule      AssetType = "rule"
	AssetTypeDirective AssetType = "directive"
	AssetTypeSchema    AssetType = "schema"
	AssetTypeConfig    AssetType = "config"
)

// EmbeddedAsset represents metadata for an embedded governance resource (#EmbeddedAsset).
type EmbeddedAsset struct {
	RelativePath string    `json:"relative_path"`
	AssetType    AssetType `json:"asset_type"`
	SHA256       string    `json:"sha256"`
	SizeBytes    int64     `json:"size_bytes"`
}
```

### 4.2 Downstream Upgrade Hygiene (`nomos init`)

When `nomos init` executes in a greenfield or existing downstream repository:
1. Scaffolds `nomos.yaml`, `AGENTS.md`, `CLAUDE.md`, `.agents/hooks.json`, and executable `.agents/scripts/*.sh` (`0755`).
2. Automatically prunes deprecated Nomos workflow files (`nomos-handshake.md`, `nomos-idle.md`, `nomos-review.md`) from `.agents/workflows/` and removes `.agents/workflows/` if empty, guaranteeing zero residual slash-command clutter.

---

## 5. Runtime Parity State Machine (`IntentParityGate`)

```mermaid
stateDiagram-v2
    [*] --> InitGate: nomos verify (IntentParityGate)
    InitGate --> CheckLegacyWorkflows: Assert .agents/workflows is absent (Law 6)
    CheckLegacyWorkflows --> AuditRules: Compute SHA-256 for AGENTS.md vs templates/AGENTS.md
    AuditRules --> AuditHooks: Compute SHA-256 for .agents/hooks.json & scripts/*.sh vs templates/hooks/
    AuditHooks --> CheckMismatches: Aggregate parity discrepancies

    state CheckMismatches <<choice>>
    CheckMismatches --> PassGate: len(drifts) == 0
    CheckMismatches --> FailClosed: len(drifts) > 0

    PassGate --> [*]: GateResult(Passed=true)
    FailClosed --> [*]: GateResult(Passed=false, Remedy, Exit 1)
```

---

## 6. Table-Driven Test Vectors

| Vector ID | Scenario | Input Target | Expected Outcome | Status |
| :--- | :--- | :--- | :--- | :--- |
| **TC-EMBED-001** | Ambient hook template lookup | `"templates/hooks/hooks.json"` | Non-empty `[]byte`, valid JSON manifest | PASS |
| **TC-EMBED-002** | Guard script permissions | `nomos init` on temp dir | `.agents/scripts/*.sh` created with `0755` mode | PASS |
| **TC-EMBED-003** | Rule asset byte parity | `AGENTS.md` vs `templates/AGENTS.md` | Identical SHA-256 digest | PASS |
| **TC-EMBED-004** | Hook asset byte parity | `.agents/hooks.json` & `scripts/*` | Identical SHA-256 digest with `templates/hooks/` | PASS |
| **TC-EMBED-005** | Legacy workflow tripwire | Residual `.agents/workflows/nomos-idle.md` | `IntentParityGate` fails closed (Law 6) | PASS |
| **TC-EMBED-006** | Downstream `nomos init` upgrade | Brownfield repo with `.agents/workflows/` | Legacy Nomos workflows pruned; hooks scaffolded | PASS |

---

## 7. Acceptance Criteria

1. **Zero Workflow or Skill Clutter:** Zero files exist under `.agents/workflows/` or `substrate/cli/templates/workflows/`, and Nomos requires zero custom slash-command skills.
2. **Ambient Session Orientation:** `.agents/hooks.json` registers a `SessionStart` handler invoking `nomos handshake` silently alongside `PreToolUse` boundary guards.
3. **Go SSoT Parity:** `substrate/src/nomos/core/workspace/parity.go` binds `AssetTypeHook` (`"hook"`) and `nomos.schema.json` cleanly with zero orphan contracts.
4. **Gate Suite Green:** `nomos verify` passes all 42 statutory Definition of Done gates with exit code 0.
