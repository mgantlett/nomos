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
plane: intent
ssot_repository: nomos
status: active
title: "Sample Steel-Man Specification"
type: spec
version: "4.0.0"
---

# Sample Steel-Man Specification

## 1. Executive Summary

The substrate enforces deterministic verification across all Intent documents.

## 2. Table-Driven Test Vectors

| Vector ID | Scenario | Expected Verdict |
| :--- | :--- | :--- |
| TV-01 | Valid specification | PASS |
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

func TestIntentSteelmanStrictStandardVectors(t *testing.T) {
	cases := []struct {
		name      string
		relPath   string
		content   string
		wantError string
	}{
		{
			name:    "valid_canonical_spec_passes",
			relPath: "architecture/specs/core/sample_spec.md",
			content: canonicalStrictSpecFixture,
		},
		{
			name:    "valid_handbook_without_test_vectors_passes",
			relPath: "architecture/handbook/01-guide.md",
			content: strings.Replace(canonicalStrictSpecFixture, "type: spec", "type: handbook", 1),
		},
		{
			name:      "rejects_non_spec_filename_in_specs_dir",
			relPath:   "architecture/specs/substrate/polyglot_invariants.md",
			content:   canonicalStrictSpecFixture,
			wantError: "canonical '_spec.md' suffix",
		},
		{
			name:      "rejects_missing_test_vectors_in_spec",
			relPath:   "architecture/specs/core/no_vectors_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "## 2. Table-Driven Test Vectors", "## 2. Notes", 1),
			wantError: "Table-Driven Test Vectors/Matrix",
		},
		{
			name:      "rejects_unicode_em_dash",
			relPath:   "architecture/specs/core/dash_spec.md",
			content:   canonicalStrictSpecFixture + "\nThis line contains an em-dash \u2014 in prose.\n",
			wantError: "Unicode em-dash or en-dash",
		},
		{
			name:      "rejects_missing_ssot_repository",
			relPath:   "architecture/specs/core/no_repo_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "ssot_repository: nomos\n", "", 1),
			wantError: "ssot_repository",
		},
		{
			name:      "rejects_invalid_version_semver",
			relPath:   "architecture/specs/core/bad_ver_spec.md",
			content:   strings.Replace(canonicalStrictSpecFixture, "version: \"4.0.0\"", "version: \"v4\"", 1),
			wantError: "invalid frontmatter version",
		},
		{
			name:      "rejects_ncte_ambiguous_modal",
			relPath:   "architecture/specs/core/modal_spec.md",
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
	specDir := filepath.Join(tempDir, "intent", "architecture", "specs", "core")
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
