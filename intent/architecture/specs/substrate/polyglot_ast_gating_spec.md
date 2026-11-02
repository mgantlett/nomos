---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-04"
last_updated: "2026-11-01"
plane: intent
ssot_repository: "nomos"
status: active
tags:
    - architecture
    - substrate
    - polyglot
    - validation
    - gates
title: Polyglot AST Validation & Language-Agnostic Gating Architecture
type: spec
version: "4.1.0"
execution_frontier:
  tasks:
    - key: "INT-321"
      plane: "intent"
      summary: "Specify Polyglot AST Validation and Language-Agnostic Gating with Tree-sitter Integration"
    - key: "SUB-110"
      plane: "substrate"
      summary: "Implement Polyglot AST Scanner for Svelte/TS and Bind Gates 21 & 22"
    - key: "INT-862"
      plane: "intent"
      summary: "Specify Real Wazero Tree-sitter WASI AST Engine, S-Expression Query Binding, and JVM/C# Polyglot Toolchain Parity"
    - key: "SUB-1543"
      plane: "substrate"
      summary: "Implement Real Wazero Tree-sitter WASI Runtime, .scm AST Evaluators, and Polyglot Toolchain/Coverage Gates"
---

# Polyglot AST Validation & Language-Agnostic Gating Architecture

## 1. Context & Dual-Core Motivation

The Nomos Substrate enforces mathematical Definition of Done (DoD) quality gates via Go-native Abstract Syntax Trees (`go/ast`). However, as Nomos orchestrates Tier 1 agentic frameworks, Membrane UIs (SvelteKit/TypeScript), and polyglot customer fixtures (Customer Zero Lab), the Substrate must audit polyglot assets with the same deterministic rigor.

This specification formalizes the **Polyglot AST Validation Engine**:
1. **Dynamic Dialect Dispatching:** Routes Go source files to `go/ast` (zero CGO overhead, 100% type-aware) and TypeScript, JavaScript, and Svelte assets to `go-tree-sitter`.
2. **DoD Gate Integration:** Enforces Gate 21 (`ip_leak`), Gate 22 (`magic_strings`), and Gate 23 (`magic_string_duplication`) across all polyglot files.
3. **CUE Contract Parity:** Validates findings against `intent/contracts/cue/polyglot.cue`.
4. **Universal Topology:** References strictly adhere to `nomos` (Open-Core Public Foundation) and `gsi-sov` (Sovereign Proprietary Extensions).

---

## 2. Axiomatic Invariants & Laws Codified

| Law # | Law Name | Architectural Enforcement |
| :---: | :--- | :--- |
| **Law 1** | **Dual-Core Inversion** | Substrate Go core executes deterministic AST parsing; Tier 1 agents cannot disable polyglot verification. |
| **Law 2** | **Non-Verbal Proof** | AST node inspection replaces fragile regex scanning, proven by table-driven test vectors (`TC-POLY-001` through `TC-POLY-006`). |
| **Law 4** | **Single Source of Truth** | Scanners unify findings against `intent/contracts/cue/polyglot.cue` and compiled Go DTOs. |
| **Law 5** | **Zero-Hardcoded-Path** | Scanned roots and exclusions resolve dynamically via `nomos.yaml` (`quality_gates.substrate_source_roots`). |
| **Law 6** | **Canonical Singularity** | Single canonical scanner pipeline (`substrate/src/nomos/modules/verify/gates_architecture.go`) auditing all frontend and service syntaxes. |
| **Law 7** | **Fail-Closed Pre-Flight** | Any unparseable AST node or Tree-sitter error fails closed immediately with exit code 1. |

---

## 3. DoD Pipeline Routing Architecture

```mermaid
flowchart TD
    A[nomos verify] --> B{Detect File Extension}
    B -- .go --> C[Go Token FileSet Parser (go/ast)]
    B -- .ts, .tsx, .js, .jsx, .svelte --> D[Tree-sitter Polyglot Parser]
    
    C --> E[Go AST Quality Gates (Coverage, Anti-Shim, CDS)]
    D --> F[Gate 22: Magic Strings & Ports (Tree-sitter)]
    D --> G[Gate 21: Sovereign IP Leak Scanner]
    D --> H[Gate 23: Magic String Duplication]
    
    E --> I{Evaluate DoD Quality Gates}
    F --> I
    G --> I
    H --> I
    
    I -- All DoD Gates Pass --> J[Exit 0: Verification Approved]
    I -- Any Gate Fails --> K[Exit 1: Rollback & Block Commit]
```

### 3.1 Gated Invariants Across Polyglot Files
1. **Zero Hardcoded Ports & Endpoints (Law 5):** Rejects raw port literals (e.g., `":8089"`, `port: 5173`) and `localhost` URLs inside Svelte/TS source files. Service ports must derive from `nomos.yaml` topology.
2. **Zero Hardcoded Repository Maps:** Rejects inline dictionaries mapping bare workspace slugs.
3. **Zero Sovereign IP Bleed:** Rejects strings containing banned Sovereign prefixes (e.g. `github.com/mgantlett/gsi-sov`) from public open-core code.
4. **Wazero WASI Linear-Memory Source Scanner & 0700 User Cache Isolation (`INT-862` / `SUB-1543` / `INT-871` / `SUB-1577`):** The pure-Go `wazero` runtime binds a persistent `wazero.CompilationCache` inside a user-private `0700` cache directory (`os.UserCacheDir()/nomos/wazero` verified via `os.Lstat` to reject symlinks, never un-namespaced world-writable `/tmp`) and executes compiled WebAssembly linear-memory source scanning (`scan_source_bytes(ptr, len) -> line_count`) on every `ParseAndEvaluate` pass alongside `verify_probe` under `CGO_ENABLED=0`.
5. **Supply-Chain & Runtime Boundary Hardening (`INT-871` / `SUB-1577`):**
   - `install.sh` enforces `set -euo pipefail`, downloads and extracts inside an isolated `mktemp -d` directory with `trap` cleanup, and verifies the archive's SHA-256 digest against the release `checksums.txt` (`sha256sum` / `shasum -a 256`) before installing the binary.
   - `nomos hook` loop circuit-breaker state files reside in a user-private `0700` directory (`os.UserCacheDir()/nomos/hooks`) verified via `os.Lstat` (`ModeSymlink == 0`).
   - `discoverPlugins` rejects symlinked or world-writable (`mode.Perm()&0002 != 0`) plugin binaries, and `openBrowser` validates `https://github.com/` URL prefixes before invoking the OS browser opener.
6. **Embedded `.scm` S-Expression Query Binding:** Embedded `.scm` query files (`go.scm`, `typescript.scm`, `svelte.scm`, `python.scm`, `rust.scm`, `c.scm`, `cpp.scm`, `java.scm`, `kotlin.scm`, `csharp.scm`, `html.scm`, `css.scm`) are parsed into structured S-expression AST query rules (`ParseSCMQuery`) driving function extraction, branch counting, anti-shim stub detection (`todo!()`, `unimplemented!()`, `pass`, `raise NotImplementedError`, `throw new UnsupportedOperationException`, `throw new NotImplementedException`), and test-skip reward-hacking detection (`it.skip`, `test.skip`, `describe.skip`, `pytest.mark.skip`, `@unittest.skip`, `#[ignore]`, `@Disabled`, `[Fact(Skip=`).
7. **Literal & Multiline Template Stripping:** Before evaluating structural braces, branches, or function headers, the polyglot lexer strips multiline backtick template literals (`` `...` ``), raw string literals (`r#"..."#`, `"""..."""`, `'''...'''`, `@"..."`), and comments so keywords inside strings never trigger false positives.
8. **Per-`source_root` Polyglot Toolchain & Coverage Ratchet Parity:** `gates_polyglot_toolchain.go` iterates across root and configured `source_roots` for TypeScript (`tsc --noEmit`), Python (`python3 -m py_compile` + `pytest`), Rust (`cargo check`), C/C++ (`cmake` / `clang -fsyntax-only`), Java/Kotlin (`gradle check` / `mvn test-compile`), and C# (`dotnet build`), and parses `lcov.info` and `cobertura.xml` coverage reports against `min_coverage`.

---

## 4. Formal Schema & Go Contracts

The contracts reside in `substrate/src/nomos/modules/verify/` and mirror `polyglot.cue`:

```go
package verify

// ComplexityFinding represents an AST violation in a polyglot source file.
type ComplexityFinding struct {
	File    string `json:"File"`
	Line    int    `json:"Line"`
	Func    string `json:"Func"`
	Value   int    `json:"-"`
	IsError bool   `json:"-"`
}

// PolyglotScanner defines the interface for polyglot AST inspection.
type PolyglotScanner interface {
	ScanFile(absPath, relPath string) ([]ComplexityFinding, error)
	EvaluateGate(ctx *VerifyContext) (*GateResult, error)
}
```

---

## 5. Table-Driven Verification Vectors

The following deterministic test vectors validate polyglot AST verification:

| Vector ID | Target File | Content Under Test | Expected Scanner Output | Invariant Enforced |
| :--- | :--- | :--- | :--- | :--- |
| **TC-POLY-001** | `src/routes/app.svelte` | `fetch("http://localhost:8089/api")` | REJECT: Hardcoded localhost URL (Gate 22) | Law 5 (Zero-hardcoded-path) |
| **TC-POLY-002** | `src/lib/client.ts` | `const port = 5173;` | REJECT: Numeric port assignment (Gate 22) | Law 5 (Port abstraction) |
| **TC-POLY-003** | `vite.config.ts` | `server: { port: 5173 }` | PASS: Build config file exempt | Config exemption |
| **TC-POLY-004** | `src/lib/auth.ts` | `import "github.com/mgantlett/gsi-sov"` | REJECT: Sovereign IP leak (Gate 21) | Law 7 (IP boundary) |
| **TC-POLY-005** | `src/lib/util.ts` | Clean code using dynamic config | PASS: Zero findings | Clean-room pass |
| **TC-POLY-006** | `src/corrupt.ts` | Unclosed template string / syntax error | REJECT: Tree-sitter parse failure | Law 7 (Fail-closed parser) |

---

## 6. Acceptance Criteria & Rigor Boundary

1. **Tree-Sitter Structural Precision:** Polyglot analysis inspects AST property and string nodes; whitespace and comments are completely ignored.
2. **Exemption Integrity:** Tooling configs (`vite.config.ts`, `package.json`) are legitimately exempt; application source files are strictly enforced.
3. **CUE Contract Parity:** Validated against `intent/contracts/cue/polyglot.cue`.
4. **42-Gate DoD Verification:** Verification passes all 42 quality gates (`nomos verify`) with exit code 0.
5. **No Broken Links:** All markdown documentation links resolve cleanly.
