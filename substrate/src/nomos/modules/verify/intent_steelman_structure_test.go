package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	}

	opts := intentSteelmanOptions{RequireFrontmatter: true, StrictStandard: true}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			full := filepath.Join(root, "intent", filepath.FromSlash(tc.relPath))
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			_, issues := validateIntentDocumentsWithOptions(filepath.Join(root, "intent"), opts)
			joined := strings.Join(issues, "; ")
			if !strings.Contains(joined, tc.wantError) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantError, issues)
			}
		})
	}
}
