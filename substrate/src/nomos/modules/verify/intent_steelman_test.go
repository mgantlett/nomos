package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canonicalStrictSpecFixture = `---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-01"
last_updated: "2026-11-05"
lineage:
  - INT-932
  - SUB-1797
plane: intent
ssot_repository: nomos
status: active
title: "Sample Steel-Man Specification"
type: spec
version: "4.8.0"
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

1. ` + "`substrate/src/nomos/modules/verify/intent_steelman.go`" + ` enforces ` + "`Cell 4.1 (intent_steelman)`" + `.
`

const strictNomosConfigFixture = `project: test-strict
default_branch: main
governance:
  quality_gates:
    strict_mode: true
    contracts:
      contracts:
        enabled: true
`

func TestIntentSteelmanStrictFrontmatterAndNCTE(t *testing.T) {
	cases := []struct {
		name      string
		relPath   string
		content   string
		wantError string
	}{
		{
			name:    "valid_flat_canonical_spec_passes",
			relPath: "architecture/specs/sample_spec.md",
			content: canonicalStrictSpecFixture,
		},
		{
			name:    "valid_root_intent_manifest_passes",
			relPath: "intent.md",
			content: canonicalStrictSpecFixture,
		},
		{
			name:      "rejects_handbook_document_in_open_core",
			relPath:   "architecture/handbook/01-guide.md",
			content:   canonicalStrictSpecFixture,
			wantError: "handbook",
		},
		{
			name:      "rejects_disallowed_frontmatter_trigger_and_tags",
			relPath:   "architecture/specs/extra_keys_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "plane: intent\n", "plane: intent\ntrigger: always_on\ntags:\n  - legacy\n", 1),
			wantError: "disallowed YAML frontmatter key(s)",
		},
		{
			name:      "rejects_non_spec_doc_type_in_strict_mode",
			relPath:   "architecture/specs/wrong_type_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "type: spec", "type: research", 1),
			wantError: "requires 'type: spec'",
		},
		{
			name:      "rejects_invalid_lineage_task_key",
			relPath:   "architecture/specs/bad_lineage_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "  - INT-932", "  - CUST-99", 1),
			wantError: "invalid frontmatter lineage task key",
		},
		{
			name:      "rejects_unicode_em_dash",
			relPath:   "architecture/specs/dash_spec.md",
			content:   canonicalStrictSpecFixture + "\nThis line contains an em-dash \u2014 in prose.\n",
			wantError: "Unicode em-dash or en-dash",
		},
		{
			name:      "rejects_missing_ssot_repository",
			relPath:   "architecture/specs/no_repo_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "ssot_repository: nomos\n", "", 1),
			wantError: "ssot_repository",
		},
		{
			name:      "rejects_invalid_version_semver",
			relPath:   "architecture/specs/bad_ver_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "version: \"4.8.0\"", "version: \"v4\"", 1),
			wantError: "invalid frontmatter version",
		},
		{
			name:      "rejects_ncte_ambiguous_modal",
			relPath:   "architecture/specs/modal_spec.md",
			content:   canonicalStrictSpecFixture + "\nOperators should configure thresholds carefully.\n",
			wantError: "ERR_NCTE_AMBIGUOUS_MODAL",
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
			if tc.wantError == "" && len(issues) > 0 {
				t.Fatalf("expected clean pass, got issues: %v", issues)
			}
			if tc.wantError != "" {
				joined := strings.Join(issues, "; ")
				if !strings.Contains(joined, tc.wantError) {
					t.Fatalf("expected error containing %q, got: %v", tc.wantError, issues)
				}
			}
		})
	}
}

func TestIntentSteelmanGateEndToEndStrictMode(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(strictNomosConfigFixture), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	specDir := filepath.Join(tempDir, "intent", "architecture", "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	specPath := filepath.Join(specDir, "sample_spec.md")
	if err := os.WriteFile(specPath, []byte(canonicalStrictSpecFixture), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	g := &IntentSteelmanGate{}
	ctx := &VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir}
	resPass, err := g.Evaluate(ctx)
	if err != nil || !resPass.Passed {
		t.Fatalf("expected strict IntentSteelmanGate pass, got res=%+v err=%v", resPass, err)
	}

	violating := canonicalStrictSpecFixture + "\nThe record was updated by the worker.\n"
	if err := os.WriteFile(specPath, []byte(violating), 0644); err != nil {
		t.Fatalf("write violating spec: %v", err)
	}
	resFail, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resFail.Passed || !strings.Contains(resFail.Message, "ERR_NCTE_PASSIVE_VOICE") {
		t.Fatalf("expected strict IntentSteelmanGate failure on passive voice, got: %+v", resFail)
	}
}

func TestRepositoryIntentCorpusPassesStrictSteelman(t *testing.T) {
	repoIntentDir := filepath.Join("..", "..", "..", "..", "..", "intent")
	if _, err := os.Stat(repoIntentDir); os.IsNotExist(err) {
		t.Skip("repository intent directory not present at relative path")
	}
	opts := intentSteelmanOptions{RequireFrontmatter: true, StrictStandard: true}
	count, issues := validateIntentDocumentsWithOptions(repoIntentDir, opts)
	if count == 0 {
		t.Fatalf("expected non-zero Intent documents in repository")
	}
	if len(issues) > 0 {
		t.Fatalf("expected 0 strict Steel-Man/NCTE-100 issues across %d Intent docs, got:\n%s",
			count, strings.Join(issues, "\n"))
	}
}
