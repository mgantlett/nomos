---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-03"
last_updated: "2026-11-04"
lineage:
  - INT-279
  - INT-593
  - INT-863
  - INT-868
  - INT-907
  - INT-914
  - INT-925
  - SUB-86
  - SUB-1546
  - SUB-1560
  - SUB-1708
  - SUB-1720
  - SUB-1764
plane: intent
ssot_repository: nomos
status: active
title: "Agent Tiers & Indirect Control: The Open-Core Charter Architectural Specification"
type: spec
version: "4.2.0"
---

# Agent Tiers & Indirect Control: The Open-Core Charter Architectural Specification

---

## 1. Executive Summary & Problem Statement

Unbounded AI coding agents generate non-deterministic file modifications, hallucinate architectural contracts, and bypass compilation gates.
Open-core Nomos operates strictly as a deterministic control harness under Law 1 (`SPEC-AGENT-TIERS-0031`).
The substrate does not generate source code and does not invoke LLMs for code synthesis.
The substrate governs external Tier 1 coding agents through machine-enforced indirect control across Laws `1` through `7`.
This specification codifies the Open-Core Indirect-Control Boundary separating public open-core Nomos from proprietary sovereign extensions.

---

## 2. Axiomatic Invariants & Governing Laws

Open-core Nomos enforces indirect control through six deterministic levers.

| Indirect Control Lever | Substrate Enforcement Mechanism | Fail-Closed Invariant |
| :--- | :--- | :--- |
| **Phase Discipline** | Immutable 6-phase state machine (`IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN`) | `ERR_PHASE_VIOLATION` (Exit 1) |
| **Trunk Immutability** | Read-only trunk branch `main` enforced via filesystem permissions | `ERR_HERMETIC_BOUNDARY` (Exit 1) |
| **Worktree Sandboxing** | Ephemeral task worktrees provisioned dynamically under `config.Paths.WorktreesDir` | `ERR_WORKTREE_ESCAPE` (Exit 1) |
| **Non-Verbal Proof** | 42 Go-native Definition of Done quality gates (`nomos verify`) | `ERR_DOD_VIOLATION` (Exit 1) |
| **Cognitive Scoping** | Causal Directed Acyclic Graph (DAG) and unblocked execution wavefront | `ERR_DAG_BLOCKED` (Exit 1) |
| **Capability Leases** | Cryptographic CapBAC tokens with automatic expiration | `ERR_CAPBAC_EXPIRED` (Exit 1) |

---

## 3. Agent Tiers (Canonical Singularity Vocabulary)

The term **Tier** applies exclusively to the classification of the coding agent governed by Nomos.
The substrate prohibits the use of Tier to describe human users, customer profiles, or licensing agreements.

```mermaid
flowchart LR
    subgraph OpenCore["Open-Core Engine (nomos)"]
        Harness["Deterministic Control Harness"]
    end

    subgraph Tier1["Tier 1 Coding Agent (External)"]
        Agent["External Agent: AGY-IDE / Claude Code / Cursor / opencode"]
    end

    subgraph Sovereign["Sovereign Substrate (gsi-sov)"]
        Tier2["Tier 2 Coding Agent: nomos code"]
        Swarm["Autonomous Swarm & Multi-Seat Mesh"]
        Cockpit["Cockpit UI & Telemetry Server"]
        Critic["Argon / Grok Adversarial Critic"]
    end

    Harness -->|"Indirect Control (CapBAC + Gates)"| Agent
    Agent -->|"Diff Proposals"| Worktree["Isolated Task Worktree"]
    Sovereign -.->|"Overlays & Extensions"| OpenCore
```

- **Tier 1 Coding Agent (Indirect Control)**: An external agent selected by the operator. Nomos establishes bounds; the external agent performs cognition. Open-core distribution.
- **Tier 2 Coding Agent (Direct Control)**: Autonomous headless worker fleets and swarm dispatchers (sovereign only: `nomos code`). Sovereign distribution only.

---

## 4. Dual-Core Interface Contracts and SSoT Data Structures

The boundary enforcement gate (`Cell 1.1 (boundary)`: `BoundaryGate`) verifies module isolation using compiled Go types in `substrate/src/nomos/core/config/config.go`.

```go
package config

// BoundaryConfig declares architectural package and CLI command boundary protections.
type BoundaryConfig struct {
	Enforce            bool     `yaml:"enforce" json:"enforce"`
	ProhibitedPackages []string `yaml:"prohibited_packages,omitempty" json:"prohibited_packages,omitempty"`
	ProhibitedCommands []string `yaml:"prohibited_commands,omitempty" json:"prohibited_commands,omitempty"`
	LockDependencies   bool     `yaml:"lock_dependencies,omitempty" json:"lock_dependencies,omitempty"`
	AllowedMCPServers  []string `yaml:"allowed_mcp_servers,omitempty" json:"allowed_mcp_servers,omitempty"`
	BlockedMCPServers  []string `yaml:"blocked_mcp_servers,omitempty" json:"blocked_mcp_servers,omitempty"`
}

// AgentTier specifies the canonical classification of a coding agent.
type AgentTier string

const (
	Tier1Agent AgentTier = "TIER_1_INDIRECT"
	Tier2Agent AgentTier = "TIER_2_DIRECT"
)

// AdoptionProfileSpec defines the configuration schema for downstream consumer profiles.
type AdoptionProfileSpec struct {
	ProfileName   string    `json:"profile_name"`
	AgentTier     AgentTier `json:"agent_tier"`
	StrictMode    bool      `json:"strict_mode"`
	AuditRequired bool      `json:"audit_required"`
}

// SovereignOverlayExtension defines the dynamic registration interface for sovereign capabilities.
type SovereignOverlayExtension interface {
	RegisterOverlayCommands(registry CommandRegistry) error
	RegisterOverlayGates(gatekeeper GatekeeperRegistry) error
	OverlayIdentifier() string
}

// CommandRegistry allows dynamic command injection without modifying open-core entrypoints.
type CommandRegistry interface {
	RegisterCommand(name string, handler interface{}) error
}

// GatekeeperRegistry allows dynamic gate injection into the verification pipeline.
type GatekeeperRegistry interface {
	RegisterGate(name string, gateEvaluator interface{}) error
}
```

Reflected Draft 2020-12 JSON Schema counterpart (`nomos.schema.json`):

```json
{
  "boundary": {
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "enforce": { "type": "boolean", "default": true },
      "prohibited_packages": { "type": "array", "items": { "type": "string" } },
      "prohibited_commands": { "type": "array", "items": { "type": "string" } },
      "lock_dependencies": { "type": "boolean" }
    }
  }
}
```

---

## 5. Open-Core Whitelist and Sovereign Deny-List

The boundary engine enforces clean-room separation between public open-core and proprietary sovereign modules.

### Open-Core Whitelist (Public Substrate)
- Phase discipline state machine and session handshake.
- Native compiled `nomos hook` lifecycle interceptors (`session-start`, `pre-tool`, `post-tool`) with zero external shell/jq/python dependencies.
- Hermetic worktree management, `--hermetic-dir` and `--container-sandbox` container modes, and Git hygiene interceptors.
- Definition of Done gatekeeper (`nomos verify`) and cognitive debt profilers.
- Two-Phase Commit release synchronization (`nomos task sync`) and Git Notes receipts.
- Living Ledger task engine, causal DAG topology, and scaffold wizards.
- Intent plane parser, byte parity validator, and NCTE-100 AST linter.
- Single-session CapBAC token authority and basic vector memory (`nomos mem`).

### Native Compiled Hook Subcommands (`nomos hook`)

Tier 1 coding agents integrate with the Nomos harness through three compiled Go subcommands under `nomos hook`.
These compiled subcommands replace fragile external shell, `jq`, and Python hook scripts with deterministic binary execution.

| Subcommand | Lifecycle Event | Core Responsibility | Output Contract |
| :--- | :--- | :--- | :--- |
| `nomos hook session-start` | `SessionStart` | Executes `nomos handshake` in the target workspace and formats the orientation payload. | JSON `injectSteps` or `hookSpecificOutput` on `stdout` |
| `nomos hook pre-tool` | `PreToolUse` / `BeforeTool` | Enforces Law 3 boundaries, `go install` guards, MCP IFC, and `CapabilityFlowGraph`. | Claude `hookSpecificOutput` (`permissionDecision`) or Gemini/generic `{"decision": "allow" \| "deny", "reason": "..."}` |
| `nomos hook post-tool` | `PostToolUse` / `AfterTool` / `PostInvocation` | Runs in-memory AST, JSON, and JSONL syntax verification on mutated files. | Silent `{}` on valid edits; emits `additionalContext` or `ephemeralMessage` on violations |

Every `nomos hook` subcommand accepts an optional `--hermetic-dir <path>` or `--container-sandbox` flag for containerized execution environments.
Active `--hermetic-dir <path>` mode confines file mutations and shell paths to `<path>` without requiring a `worktrees/` subdirectory.
Active `--container-sandbox` mode permits container OS paths (`/etc`, `/tmp`, `/var`, `sqlite3`) while blocking verifier roots (`/logs/verifier`, `/solution`, `/tests`), wildcard `rm`, and 3-strike loops.
In `--container-sandbox` mode, `nomos hook post-tool` validates `.go`, `.py`, `.json`, and `.jsonl` edits in memory and runs `EvaluateFastFileRadar` when `nomos.yaml` exists.
Omitting both flags in a standard Nomos repository enforces the Law 3 hermetic worktree boundary.

Both `nomos init` and `nomos hook` preserve and chain pre-existing `.claude/settings.json` hook entries.
Furthermore, `nomos hook pre-tool` enforces a 3-strike circuit-breaker keyed on `(tool_name, canonical_args_hash, workspace_mtime_digest)` to halt non-progressing tool loops.
Across all modes, `nomos hook pre-tool` blocks `Read` and `read_file` on binary, archive, media, or database files and directs shell metadata queries.
Headless `--container-sandbox` and `--hermetic-dir` modes block Gemini CLI media `read_file` crashes and unbounded interactive commands (`tail -f`, `watch`, `vim`, `nano`, `less`, `top`).
Finally, `--budget-sec <seconds>` (or `NOMOS_BUDGET_SECONDS`) configures `nomos hook post-tool` to emit deduplicated advisories at `75%` and `90%` elapsed budget.

### Sovereign Deny-List (Prohibited in Open-Core)

| Prohibited Capability | Denial Identifier | Architectural Justification |
| :--- | :--- | :--- |
| sovereign only: `nomos code` Worker Fleet | `CAP_TIER2_WORKER` | Direct-control autonomous execution is proprietary commercial core. |
| Swarm Orchestration Engine | `CAP_SWARM_ORCHESTRATOR` | Multi-agent swarm crawler reserved for enterprise sovereign seats. |
| Multi-Seat Mesh Telemetry | `CAP_MULTISEAT_MESH` | Distributed peer heartbeat mesh reserved for sovereign infrastructure. |
| Glass Cockpit UI & Daemon | `CAP_COCKPIT_MEMBRANE` | Membrane visual cockpit and WebSocket telemetry server are sovereign. |
| Adversarial Critic Client | `CAP_ADVERSARIAL_CRITIC` | External LLM red-team audit integration is sovereign review extension. |
| PM2 Daemon Supervisor | `CAP_PM2_DAEMON` | Infrastructure daemon lifecycle manager is proprietary operations harness. |
| Public Portal Ship Harness | `CAP_PORTAL_SHIP` | Static site synthesis and Cloudflare DNS deployment are proprietary. |

---

## 6. Fail-Closed Boundary Pre-Flight Predicates (Law 7)

The gatekeeper evaluates the open-core boundary prior to task verification and release synchronization via **`Cell 1.1 (boundary)` (`BoundaryGate`)**.
Any detection of prohibited packages, commands, or unconfigured boundary policies terminates with exit code 1.

### Boundary Verification Predicates
1. **Config Existence Predicate ($P_{\text{cfg}}$)**:
   $$P_{\text{cfg}} = \neg \text{policy.Enforce} \lor (\text{len}(\text{policy.ProhibitedPackages}) > 0 \lor \text{len}(\text{policy.ProhibitedCommands}) > 0)$$
2. **AST Package Boundary Predicate ($P_{\text{pkg}}$)**:
   $$\forall p \in \text{InternalPackages}, \quad p \notin \text{policy.ProhibitedPackages}$$
3. **Command Registration Boundary Predicate ($P_{\text{cmd}}$)**:
   $$\forall c \in \text{CobraCommands}, \quad c \notin \text{policy.ProhibitedCommands}$$

### Pre-Flight Evaluation Rules
- If $P_{\text{cfg}} = \text{false}$, abort execution with error `ERR_BOUNDARY_CONFIG_MISSING`.
- If $P_{\text{pkg}} = \text{false}$, abort compilation with error `ERR_OPEN_CORE_PACKAGE_LEAK`.
- If $P_{\text{cmd}} = \text{false}$, abort verification with error `ERR_OPEN_CORE_COMMAND_LEAK`.

---

## 7. Dynamic Configuration Discovery (Law 5)

All filesystem paths resolve dynamically through workspace configuration discovery under Law 5.
Specifications reject hardcoded path literals.

| Symbolic Configuration Key | Runtime Target | Resolution Method |
| :--- | :--- | :--- |
| `config.Paths.IntentDir` | Declarative intent root | Dynamic discovery via `config.ResolveIntentRoot()` |
| `config.Paths.RulesDir` | Agent operating rules | Dynamic discovery via `config.ResolveRulesDir()` |
| `config.Paths.HooksDir` | Ambient lifecycle hooks | Dynamic discovery via `config.ResolveHooksDir()` |
| `config.Paths.WorktreesDir` | Transient task worktrees | Dynamic discovery via `config.ResolveWorktreeDir()` |
| `config.Paths.SchemasDir` | Reflected JSON Schema contracts | Dynamic discovery via `config.LocateConfigPath()` |

---

## 8. Adoption Profiles (Downstream Customer Classification)

Downstream customers configure Nomos using named Adoption Profiles.
Every profile deploys its own external Tier 1 coding agent under indirect control.

| Profile Name | Target Environment | Architectural Focus | Security & Enforcement Bounds |
| :--- | :--- | :--- | :--- |
| **Solo** | Individual developer workstation | Zero-configuration onboarding, fast phase transitions | Single-seat CapBAC, local SQLite ledger |
| **Team** | Multi-developer product teams | Shared Intent SSoT, branch protection, CI gate integration | Distributed trunk locks, CI `nomos verify` enforcement |
| **Regulated** | Compliance-bound enterprise | Cryptographic audit trail, air-gapped execution | Strict CapBAC, Git Notes release receipt verification |

The **Foundry** profile governs internal SophiaLabs operations and remains strictly outside downstream distribution.

---

## 9. Table-Driven Verification Vectors

The table below defines deterministic verification vectors for the Open-Core Boundary Invariant Gate.

| Vector ID | Target Substrate Input | Evaluated Invariant | Expected Error Code | Expected Verdict |
| :--- | :--- | :--- | :--- | :--- |
| **TC-TIER-01** | Open-core module importing `substrate/src/nomos/modules/verify` | Permitted Public Package | `NONE` | **PASS (true)** |
| **TC-TIER-02** | Open-core module importing `pkg/sovereign/cockpit` | Prohibited Sovereign Package | `ERR_OPEN_CORE_PACKAGE_LEAK` | **FAIL (false)** |
| **TC-TIER-03** | Open-core CLI registering `nomos code` command | Prohibited Sovereign Command | `ERR_OPEN_CORE_COMMAND_LEAK` | **FAIL (false)** |
| **TC-TIER-04** | Missing `BoundaryConfig` definition when enabled | Fail-Closed Configuration (Law 7) | `ERR_BOUNDARY_CONFIG_MISSING` | **FAIL (false)** |
| **TC-TIER-05** | Source file containing a hardcoded host user-home directory literal | Zero-Hardcoded-Path Law (Law 5) | `ERR_FORBIDDEN_LITERAL` | **FAIL (false)** |
| **TC-TIER-06** | Task key formatted with invalid tier prefix `TIER1-01` | Canonical Term Singularity (Law 6) | `ERR_CANONICAL_VIOLATION` | **FAIL (false)** |

---

## 10. Cross-Plane Traceability & DoD Verification

1. **Boundary Gate (`Cell 1.1 (boundary)`)**: `BoundaryGate` in `substrate/src/nomos/modules/verify/gates_architecture.go` verifies the open-core tree against `BoundaryConfig` (`substrate/src/nomos/core/config/config.go`).
2. **Native Hook Subcommands (`substrate/cli/cmd_hook.go`)**: Compiled `nomos hook` interceptors enforce indirect control across `session-start`, `pre-tool`, and `post-tool`.
3. **Markdown Integrity (`Cell 4.1 (broken_wire)`)**: `BrokenWireGate` verifies all relative Markdown links deterministically without broken wires.
4. **Forbidden Literals (`Cell 5.1 (forbidden_literals)`)**: `ForbiddenLiteralsGate` rejects workstation path literals across source trees.
5. **Verification Command**:
   ```bash
   nomos verify
   ```
   Must pass all 42 Definition of Done quality gates with zero regressions.
