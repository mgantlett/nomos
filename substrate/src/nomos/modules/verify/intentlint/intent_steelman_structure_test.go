package intentlint

import (
	"strings"
	"testing"
)

const canonicalStrictSpecFixture = `---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-01"
plane: intent
ssot_repository: nomos
status: active
title: "Sample Steel-Man Specification"
type: spec
---

# Sample Steel-Man Specification

---

## 1. Executive Summary & Problem Statement

The substrate enforces deterministic verification across all Intent documents.

---

## 2. Axiomatic Invariants & Governing Laws

Every Intent contract binds to one compiled Go Single Source of Truth.

---

## 3. Compiled Go SSoT Contracts

` + "```go" + `
package verify

// SampleContract defines a deterministic test structure.
type SampleContract struct {
	Enabled bool
}
` + "```" + `

---

## 4. Table-Driven Verification Vectors

| Vector ID | Scenario | Expected Verdict |
| :--- | :--- | :--- |
| TC-SMP-01 | Valid canonical specification | PASS |
| TC-SMP-02 | Missing frontmatter delimiter | FAIL |
| TC-SMP-03 | Unclosed Markdown code fence | FAIL |

---

## 5. Cross-Plane Traceability & DoD Verification

1. ` + "`substrate/src/nomos/modules/verify/intentlint/intent_steelman.go`" + ` enforces ` + "`Cell 4.1 (intent_steelman)`" + `.
`

func TestIntentSteelmanStrictBlueprintStructureVectors(t *testing.T) {
	cases := []struct {
		name      string
		relPath   string
		content   string
		wantError string
	}{
		{
			name:      "rejects_nested_core_subfolder_under_specs",
			relPath:   "architecture/specs/core/sample_spec.md",
			content:   canonicalStrictSpecFixture,
			wantError: "nested subdirectory under architecture/specs/ is prohibited",
		},
		{
			name:      "rejects_nested_substrate_subfolder_under_specs",
			relPath:   "architecture/specs/substrate/sample_spec.md",
			content:   canonicalStrictSpecFixture,
			wantError: "nested subdirectory under architecture/specs/ is prohibited",
		},
		{
			name:      "rejects_missing_spec_suffix",
			relPath:   "architecture/specs/polyglot_invariants.md",
			content:   canonicalStrictSpecFixture,
			wantError: "canonical '_spec.md' suffix",
		},
		{
			name:      "rejects_post_h1_role_preamble_before_section_1",
			relPath:   "architecture/specs/preamble_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "# Sample Steel-Man Specification\n\n---\n", "# Sample Steel-Man Specification\n\n- **Role:** Architect\n\n---\n", 1),
			wantError: "single '---' divider with zero preamble prose",
		},
		{
			name:      "rejects_non_contiguous_h2_numbering",
			relPath:   "architecture/specs/bad_nums_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "## 3. Compiled Go SSoT Contracts", "## 9. Compiled Go SSoT Contracts", 1),
			wantError: "non-contiguous H2 section number",
		},
		{
			name:      "rejects_missing_horizontal_rule_divider",
			relPath:   "architecture/specs/no_hr_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "---\n\n## 3. Compiled Go SSoT Contracts", "## 3. Compiled Go SSoT Contracts", 1),
			wantError: "missing '---' horizontal rule divider",
		},
		{
			name:      "rejects_missing_compiled_go_block",
			relPath:   "architecture/specs/no_go_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "```go", "```text", 1),
			wantError: "missing required fenced '```go' SSoT contract block",
		},
		{
			name:      "rejects_legacy_tv_vector_prefix",
			relPath:   "architecture/specs/legacy_vec_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "| TC-SMP-01 |", "| TV-SMP-01 |", 1),
			wantError: "legacy TV-/VEC-/CDV- prefix",
		},
		{
			name:      "rejects_missing_closing_traceability_section",
			relPath:   "architecture/specs/bad_close_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "## 5. Cross-Plane Traceability & DoD Verification", "## 5. Appendix Notes", 1),
			wantError: "Cross-Plane Traceability & DoD Verification",
		},
		{
			name:      "rejects_obsolete_flat_gate_number_prose",
			relPath:   "architecture/specs/flat_gate_spec.md",
			content:   canonicalStrictSpecFixture + "\nGate 19 blocks this module.\n",
			wantError: "obsolete flat 'Gate <N>' citation",
		},
		{
			name:      "rejects_dangling_adr_shorthand_prose",
			relPath:   "architecture/specs/dangling_adr_spec.md",
			content:   canonicalStrictSpecFixture + "\nEnforced under ADR-0031 boundary rules.\n",
			wantError: "dangling 'ADR-XXXX' shorthand",
		},
		{
			name:      "rejects_statediagram_semicolon_orphan_hazard",
			relPath:   "architecture/specs/mermaid_semi_spec.md",
			content:   canonicalStrictSpecFixture + "\n```mermaid\nstateDiagram-v2\n    FailClosed --> [*]: Reject release; emit report (Exit 1)\n```\n",
			wantError: "unescaped ';' in stateDiagram/sequenceDiagram",
		},
		{
			name:      "rejects_flowchart_unquoted_parentheses_node",
			relPath:   "architecture/specs/mermaid_node_spec.md",
			content:   canonicalStrictSpecFixture + "\n```mermaid\nflowchart TD\n    A --> B[Go Parser (go/ast)]\n```\n",
			wantError: "unquoted parentheses inside '[...]' node label",
		},
		{
			name:      "rejects_flowchart_unquoted_edge_parentheses",
			relPath:   "architecture/specs/mermaid_edge_spec.md",
			content:   canonicalStrictSpecFixture + "\n```mermaid\nflowchart TD\n    A -->|Events (MESSAGE_CREATE)| B\n```\n",
			wantError: "inside '|...|' edge label",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := tc.content
			if idx := strings.Index(body, "\n---\n"); idx >= 0 {
				body = strings.TrimLeft(body[idx+len("\n---\n"):], "\n")
			}
			var got string
			switch {
			case ValidateStrictSpecTopology(tc.relPath) != "":
				got = ValidateStrictSpecTopology(tc.relPath)
			case HasUnclosedCodeFence(body):
				got = "unclosed Markdown fenced code block detected"
			case ValidateMarkdownMermaidBlocks(body) != "":
				got = ValidateMarkdownMermaidBlocks(body)
			default:
				got = ValidateStrictSpecStructure(body)
			}
			if !strings.Contains(got, tc.wantError) {
				t.Fatalf("expected error containing %q, got: %q", tc.wantError, got)
			}
		})
	}
}
