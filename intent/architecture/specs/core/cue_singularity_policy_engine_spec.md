---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-12"
last_updated: "2026-11-01"
plane: intent
status: active
title: Nomos CUE Singularity Policy Engine Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos CUE Singularity Policy Engine Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Define the unified declarative policy, schema, commit rule, and gate verification architecture using pure Cuelang (CUE), formally deprecating and removing OPA Rego from the Nomos ecosystem.

---

## 1. Problem Statement & Architectural Motivation
In the Gen 3 AST Engine (ADR-0054), Nomos split declarative evaluation across two external runtime systems:
1. **CUE Runtime** (`cuelang.org/go/cue`): Responsible for structural configuration and state schemas.
2. **OPA Rego Runtime** (`github.com/open-policy-agent/opa/v1/rego`): Responsible for quality gate policies, CapBAC assertions, and authorization rules.

This bifurcation violates two primary laws of the Nomos Open-Core Protocol:
- **Law 4 (The Single Source of Truth Law)**: Having two declarative constraint engines fragments policy specifications across two disparate DSL ecosystems.
- **Law 6 (The Canonical Singularity Law)**: Zero aliases or duplicate abstraction layers are tolerated. The OPA dependency bloats the compiled Go binary with redundant query machinery.

The **CUE Singularity** unifies all schema definition, commit message syntax, task contracts, and quality gate evaluations into a single, Go-native CUE constraint lattice.

---

## 2. Core Axioms & Law Enforcements

| Law # | Law Name | CUE Enforcement Mechanism |
| :---: | :--- | :--- |
| **Law 2** | **The Non-Verbal Proof Law** | Quality gate metrics must unify with CUE schemas without concrete evaluation errors (`Validate(cue.Concrete(true)) == nil`). |
| **Law 4** | **The Single Source of Truth Law** | Exactly one file defines each schema domain under `intent/contracts/cue/*.cue`. |
| **Law 6** | **The Canonical Singularity Law** | Exactly one policy engine runtime (`CUEEngine`) is linked into the Nomos Go substrate. |

---

## 3. Unified CUE Schema Architecture

All schemas reside under `intent/contracts/cue/`:

### 3.1 Gate Policies (`intent/contracts/cue/gates.cue`)
Replaces `gates.rego`, `ast.rego`, and `extended.rego`:
```cue
package schemas

#GateThresholds: {
	min_coverage:               float | *80.0
	max_cds:                    int | *50
	min_docstring_density:      int | *80
	min_traceability_density:   int | *80
	max_complexity:             int | *15
	max_file_loc:               int | *500
	max_func_loc:               int | *80
}

#ASTMetrics: {
	functions: [...{
		name:       string
		file:       string
		line:       int
		complexity: int
		loc:        int
	}]
	files: [...{
		name: string
		loc:  int
		cds:  int
	coverage: float
	docstring_density: int
	traceability_density: int

	// Relational Unification Evaluation (Law 2 Non-Verbal Proof)
	pass: bool
	pass: (coverage >= #GateThresholds.min_coverage) &
	      (docstring_density >= #GateThresholds.min_docstring_density) &
	      (traceability_density >= #GateThresholds.min_traceability_density)
}
```

Dynamic schema paths resolve strictly via `config.ResolveIntentPath("runtime/schemas")` to satisfy **Law 5 (Zero-Hardcoded-Path)**.


### 3.2 Commit Message Contract (`intent/contracts/cue/commit_rules.cue`)
Replaces bash regex in `hooks.go`:
```cue
package schemas

import "strings"

#CommitMessage: {
	raw: string
	task_key: =~"^\\[(INT|SUB)-[0-9]+(-[a-z0-9-]+)?\\]"
	type: "feat" | "fix" | "docs" | "chore" | "refactor" | "test"
	summary: string & strings.MinRunes(10) & strings.MaxRunes(100)
}
```

### 3.3 Task Taxonomies (`intent/contracts/cue/task.cue`)
Elevates task contracts into compile-time typed CUE definitions.

---

## 4. Cross-Plane Transduction Matrix

| Intent Artifact | Target CUE Schema | Substrate Consumer |
| :--- | :--- | :--- |
| Gate Policies | `intent/contracts/cue/gates.cue` | `core/policy.CUEEngine` & `modules/verify/*.go` |
| Commit Rules | `intent/contracts/cue/commit_rules.cue` | `core/workspace/hooks.go` & `cmd/nomos task commit` |
| Task Schemas | `intent/contracts/cue/task.cue` | `modules/task/task.go` |
| Phase States | `intent/contracts/cue/phase_state.cue` | `core/types/phase.go` |
| CapBAC Leases | `intent/contracts/cue/capbac_token.cue` | `core/capbac/token.go` |

---

## 5. Table-Driven Verification Vectors

| Vector ID | Input Artifact / Condition | Governing Law | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-CUE-01` | Valid gate metric report unified against `gates.cue` | Law 2 & Law 4 | `PASS` (Unification succeeds) |
| `TC-CUE-02` | Commit summary shorter than 10 runes or missing `[INT-\|SUB-]` key | Law 6 (`commit_rules.cue`) | `FAIL` (`ERR_COMMIT_SCHEMA_VIOLATION`) |
| `TC-CUE-03` | Unknown top-level field injected into closed CUE policy struct | Law 7 (Fail-Closed) | `FAIL` (`ERR_SCHEMA_UNIFICATION_FAILED`) |
| `TC-CUE-04` | Hardcoded filesystem path literal in schema loader | Law 5 (Zero-Hardcoded-Path) | `FAIL` (`ERR_FORBIDDEN_LITERAL`) |

---

## 6. Verification & Rigor Boundary
- **Intent Boundary**: This specification, ADR-0056, and canonical schema contracts in `intent/contracts/cue/`.
- **Substrate Boundary**: Pure-Go `cuelang.org/go/cue` evaluation in `core/policy` and `modules/verify` with zero OPA dependencies.
