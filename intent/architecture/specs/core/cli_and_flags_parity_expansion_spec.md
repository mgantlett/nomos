---
title: "CLI and Flag Surface Parity Contract Expansion Specification"
plane: intent
type: spec
status: active
version: "4.0.0"
classification: public
authors:
  - Mark Gantlett
  - Nomos Autonomous Architect
date: "2026-10-17"
last_updated: "2026-11-01"
ssot_repository: nomos
---

# CLI and Flag Surface Parity Contract Expansion Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Expand the Unified Nomos Parity Engine with two additional surface parity contracts—`contracts.cli` and `contracts.flags`—to detect and eliminate phantom commands, hallucinated CLI flags, and plane-crossing instructions across all Intent documentation.

---

## 0.0 Invariant Matrix

| Invariant Key | Invariant Name | Formal AST Invariant | Machine Enforcement |
| :--- | :--- | :--- | :--- |
| **`INV-PARITY-008`** | **Command Surface Invariance** | $\forall c \in \text{IntentCommands}, c \in \text{SubstrateCLI}$ | AST Regex Extractor $\leftrightarrow$ Cobra Command Registry |
| **`INV-PARITY-009`** | **Flag Surface Invariance** | $\forall (c, f) \in \text{IntentCommandFlags}, f \in \text{MergedFlags}(c)$ | Cobra Merged FlagSet (`Flags` + `PersistentFlags`) |
| **`INV-PARITY-010`** | **Hermetic Plane Boundary Invariance** | $\text{Scope}(c) \subseteq \text{RepoProfile}(\text{doc})$ | Open-Core / Sovereign Boundary Filter |
| **`INV-PARITY-011`** | **Cobra Help & Example Completeness** | $\forall c \in \text{StaticCLI}, \{Use, Short, Long, Example\} \ne \emptyset$ | `ScanCobraSurfaceCompletenessInDir` (`Cell 6.2` `ZeroAliasGate`) |

---

## 1.0 Problem Statement: The "Lamppost Effect"

The Sovereign Parity Engine (governed under gsi-sov) audits universal surface contracts:
1. `contracts.invariants`: Extracts `INV-*` tokens and matches them to `InvariantRegistry` Go enforcers.
2. `contracts.schemas`: Audits Intent `### <Name> Contract` headings against Substrate Go structs.
3. `contracts.reverse`: Verifies that Substrate Go struct types ending in `Contract` have Intent anchors.
4. `contracts.directives`: Audits active directives for obsolete gate numbers (`12 gates`, `38 gates`, `61 gates`).

When all four contracts match, the engine reports $\Delta(t) = 0.0$ (Parity Equilibrium). However, an empirical manual spot check across the repository revealed significant undetected drift:

- **Phantom Commands in Active Directives:** Intent directives cite unbuilt commands (counter-example: `nomos code watch` and `nomos code burndown` in open-core directives).
- **Aspirational Commands in Architectural Specs:** System specifications cite unbuilt subcommands (counter-example: `nomos physics serve --rate 5`).
- **Obsolete Lifecycle Commands in Handbooks:** Intent handbooks cite retired commands (counter-example: `nomos release ship`).
- **Namespace Divergence:** Documentation references non-existent command signatures (counter-example: `nomos intent compile` instead of `nomos verify`, and `nomos init --agent` instead of `nomos task template`).

Because the Parity Engine was not inspecting CLI command invocations or flag parameters, it suffered from the **Lamppost Effect**: searching only where its existing scanners were pointed.

---

## 2.0 Architectural Surface Parity Definitions

To eliminate this blind spot, the Parity Engine expands to evaluate six universal surface contracts ($N=6$):

### 2.1 Contract 5: `contracts.cli` (Command Surface Parity)
The engine extracts all command invocations matching the pattern:
```
`nomos <command> [<subcommand>] ...`
```
from markdown backticks, tables, and fenced code blocks (`bash`, `sh`, `console`).

#### Traversal & Disambiguation Grammar
Given an extracted invocation `nomos <token_1> <token_2> ... <token_k>`:
1. Start at `rootCmd`.
2. For each token:
   - If the previous token was a flag where `TakesValue == true` without an `=` assignment, consume this token as the flag's value.
   - If `currCmd.HasSubCommands()`:
     - Check if `token` matches any registered canonical subcommand name. Under Law 6 (Canonical Singularity), aliases are strictly prohibited; invoking an alias rather than the canonical command name is flagged as Canonical Drift.
     - If a matching canonical subcommand is found, advance `currCmd = matchedSubCmd` and continue.
     - If no matching subcommand is found:
       - Check if `currCmd.AcceptsArgs` is true.
       - If `currCmd.AcceptsArgs == false` (e.g. requires a subcommand), flag `token` as an **unknown subcommand** under `contracts.cli`.
       - If `currCmd.AcceptsArgs == true`, all remaining positional tokens are treated as arguments, not subcommands.
   - If `!currCmd.HasSubCommands()` (leaf command):
     - All subsequent positional tokens are arguments to `currCmd`.

$$\text{Discrepancy} = \text{fmt.Sprintf}(\text{"Doc \%s instructs running non-existent command '\%s'", docPath, cmdString})$$

---

### 2.2 Contract 6: `contracts.flags` (Flag Surface Parity)
For every command invocation, the engine tokenizes all option arguments:
1. **Flag Tokenization & Argument Consumption:**
   - Long flags: `--flag` and `--flag=value` (splits on `=`).
   - Short flags: `-f` and `-f=value`.
   - Space-separated flag arguments: If `flag.TakesValue == true` and no `=` is present, the subsequent token is consumed as the flag's argument.
2. **Merged FlagSet Resolution:**
   Every flag is resolved against the target command's **merged flag set**:
   $$\text{MergedFlags}(c) = c.\text{Flags}() \cup c.\text{PersistentFlags}() \cup c.\text{InheritedFlags}()$$
   This ensures standard persistent flags (`--help`, `-h`, `--json`, `--root`, `--quiet`) are universally recognized.
3. **Fail-Closed Drift Condition:**
   If a cited flag is missing from `MergedFlags(c)`, the discrepancy is recorded under `contracts.flags`:
   $$\text{Discrepancy} = \text{fmt.Sprintf}(\text{"Doc \%s cites invalid flag '\%s' for command '\%s'", docPath, flagName, cmdString})$$

---

### 2.3 Hermetic Plane & Profile Boundary (`INV-PARITY-010`)
Commands defined strictly in sovereign packages (sovereign only: `nomos code`, `nomos-plugin-ship`, `nomos cockpit`) must observe repository profiles:
- In open-core repositories (`nomos`), citations of sovereign commands in active directives or core specs are flagged as **Plane Contamination Drift**.
- In sovereign repositories (`gsi-sov`), citations of sovereign commands resolve against the sovereign CLI plugin registry.

---

## 3.0 Filtering & Exemption Grammar

To prevent false positives on historical records, counter-examples, and test fixtures, the scanner enforces deterministic filters:

1. **Negative Assertions & Counter-Examples:**
   Lines matching explicit negation or counter-example prefixes are exempt:
   - `do not run nomos ...`
   - `never run nomos ...`
   - `prohibited: nomos ...`
   - `invalid: nomos ...`
   - `phantom: nomos ...`
   - `counter-example: nomos ...`
   - `drift: nomos ...`
2. **Fenced Code Blocks:**
   Code blocks with language tags `anti-pattern`, `counter-example`, `drift-example`, or `test-vector` are exempt.
3. **Dynamic Lifecycle Status Filter (Law 5):**
   Any Intent document whose YAML frontmatter defines:
   $$\text{status} \in \{\text{"superseded"}, \text{"rejected"}, \text{"obsolete"}, \text{"deprecated"}, \text{"archived"}\}$$
   is universally exempt, resolving dynamically across all directories without hardcoded paths.
4. **Template Parameter Exemption:**
   Invocations containing unresolved template placeholders (e.g. `<command>`, `[flags]`, `$ARG`, `<task-key>`) are skipped.

---

## 4.0 Substrate Implementation Blueprint

### 4.1 CLI Descriptor Data Contracts (Law 4)

### CLIFlagDescriptor Contract
```go
// CLIFlagDescriptor defines the formal schema for a compiled CLI flag.
type CLIFlagDescriptor struct {
	Name       string `json:"name"`
	Shorthand  string `json:"shorthand,omitempty"`
	Usage      string `json:"usage"`
	Persistent bool   `json:"persistent"`
	TakesValue bool   `json:"takes_value"`
}
```

### CLICommandDescriptor Contract
```go
// CLICommandDescriptor defines the formal schema for a compiled CLI command.
type CLICommandDescriptor struct {
	Use         string                           `json:"use"`
	Path        string                           `json:"path"`
	Short       string                           `json:"short"`
	IsLeaf      bool                             `json:"is_leaf"`
	AcceptsArgs bool                             `json:"accepts_args"`
	Subcommands map[string]*CLICommandDescriptor `json:"subcommands,omitempty"`
	Flags       map[string]*CLIFlagDescriptor    `json:"flags"`
}
```

---

### 4.2 Fail-Closed Pre-Flight Gate (Law 7)

```go
// AuditCommandSurface audits Intent documentation against the compiled Cobra command registry.
func AuditCommandSurface(rootDir string, table *IntentSymbolTable, cliCmds map[string]*CLICommandDescriptor, repoProfile string) ([]string, error) {
	if cliCmds == nil || len(cliCmds) < 5 {
		return nil, fmt.Errorf("fail-closed pre-flight: CLI registry is empty or uninitialized (Law 7)")
	}
	if repoProfile == "" {
		repoProfile = "open-core" // Fail-closed to most restrictive boundary
	}
	// ... execution logic ...
	return discrepancies, nil
}
```

---

## 5.0 Verification Proofs & Test Vectors

| Vector ID | Test Vector Description | Expected Verification Outcome |
| :--- | :--- | :--- |
| **`VEC-CLI-01`** | Valid command invocation `nomos task create --desc "..."` | **PASS**: Command and flags recognized in Cobra registry. |
| **`VEC-CLI-02`** | Phantom command invocation `nomos phantomcmd run` | **DRIFT**: Flagged under `contracts.cli` (`unknown command "phantomcmd"`). |
| **`VEC-CLI-03`** | Valid command with invalid flag `nomos init --bogusflag` | **DRIFT**: Flagged under `contracts.flags` (`unknown flag "--bogusflag"`). |
| **`VEC-CLI-04`** | Sovereign command `nomos code watch` in open-core directive | **DRIFT**: Flagged under `contracts.cli` (Law 0.2 / `INV-PARITY-010`). |
| **`VEC-CLI-05`** | Negated command line `"never run nomos worktree directly"` | **PASS**: Filtered by negative assertion parser. |
| **`VEC-CLI-06`** | Global persistent flag invocation `nomos status --json` | **PASS**: Verified against `MergedFlags(c)`. |
| **`VEC-CLI-07`** | Static `&cobra.Command{}` missing `Long` or `Example` in `substrate/cli` | **FAIL CLOSED**: Rejected by `ScanCobraSurfaceCompletenessInDir` (`Cell 6.2` `ZeroAliasGate`, `INV-PARITY-011`). |
