package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-OBL-001
// @verifies TC-OBL-002
func TestVerifyExecutableObligations(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "intent", "architecture", "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	specContent := "---\ntitle: Auth Spec\n---\n\n| Vector ID | Scenario |\n| :--- | :--- |\n| `TC-AUTH-01` | Valid token |\n| `TC-AUTH-99` | Expired token |\n"
	if err := os.WriteFile(filepath.Join(specDir, "auth_spec.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	testDir := filepath.Join(tmp, "pkg", "auth")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("mkdir testDir: %v", err)
	}
	testContent := "package auth\n// @verifies TC-AUTH-01\nfunc TestValidToken() {}\n"
	if err := os.WriteFile(filepath.Join(testDir, "auth_test.go"), []byte(testContent), 0644); err != nil {
		t.Fatalf("write go test: %v", err)
	}

	issues := VerifyExecutableObligations(tmp, nil)
	if len(issues) != 1 || !strings.Contains(issues[0], "TC-AUTH-99") {
		t.Fatalf("expected exactly TC-AUTH-99 unbound, got: %v", issues)
	}

	pyTest := "# @verifies TC-AUTH-99\ndef test_expired():\n    assert True\n"
	if err := os.WriteFile(filepath.Join(testDir, "test_auth.py"), []byte(pyTest), 0644); err != nil {
		t.Fatalf("write py test: %v", err)
	}
	if clean := VerifyExecutableObligations(tmp, nil); len(clean) != 0 {
		t.Fatalf("expected 0 unbound obligations after polyglot test binding, got: %v", clean)
	}
}

// @verifies TC-OBL-003
func TestVerifySpecTypeSignaturesAndGate(t *testing.T) {
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "intent", "architecture", "specs")
	subDir := filepath.Join(tmp, "substrate", "pkg", "sample")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir specDir: %v", err)
	}
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir subDir: %v", err)
	}

	specBody := "# Sample Spec\n\n```go\npackage sample\ntype WidgetContract struct {\n\tID string\n\tStrict bool\n}\n```\n\n| Vector | Desc |\n| :--- | :--- |\n| `TC-WID-01` | Check |\n"
	if err := os.WriteFile(filepath.Join(specDir, "widget_spec.md"), []byte(specBody), 0644); err != nil {
		t.Fatalf("write widget_spec.md: %v", err)
	}
	driftedGo := "package sample\n// WidgetContract models a widget (@intent widget_spec.md #WidgetContract).\ntype WidgetContract struct {\n\tID string\n}\n// NewWidget creates a widget (@intent widget_spec.md #WidgetContract).\nfunc NewWidget() WidgetContract { return WidgetContract{} }\n"
	if err := os.WriteFile(filepath.Join(subDir, "widget.go"), []byte(driftedGo), 0644); err != nil {
		t.Fatalf("write widget.go: %v", err)
	}

	sigIssues := VerifySpecTypeSignatures(tmp, subDir, nil)
	if len(sigIssues) != 1 || !strings.Contains(sigIssues[0], "missing declared field Strict") {
		t.Fatalf("expected missing field Strict drift, got: %v", sigIssues)
	}

	nomosYAML := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    contracts:\n      contracts:\n        enabled: true\n        strict_types: true\n        verify_obligations: true\n    thresholds:\n      max_cognitive_debt_score: 45\n      max_cyclomatic_complexity: 15\n      max_file_loc: 400\n      max_func_loc: 60\n      min_comment_density_pct: 10\n      min_test_coverage_pct: 55\n"
	if err := os.WriteFile(filepath.Join(tmp, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	gate := &IntentTraceabilityGate{}
	ctx := &VerifyContext{RepoRoot: tmp, WorktreePath: tmp}
	res, err := gate.Evaluate(ctx)
	if err != nil || res == nil || res.Passed || !strings.Contains(res.Message, "Strict") {
		t.Fatalf("expected IntentTraceabilityGate to reject struct field drift, got: %+v, err=%v", res, err)
	}

	alignedGo := "package sample\n// WidgetContract models a widget (@intent widget_spec.md #WidgetContract).\ntype WidgetContract struct {\n\tID string\n\tStrict bool\n}\n// NewWidget creates a widget (@intent widget_spec.md #WidgetContract).\nfunc NewWidget() WidgetContract { return WidgetContract{} }\n"
	if err := os.WriteFile(filepath.Join(subDir, "widget.go"), []byte(alignedGo), 0644); err != nil {
		t.Fatalf("write aligned widget.go: %v", err)
	}
	res2, err2 := gate.Evaluate(ctx)
	if err2 != nil || res2 == nil || res2.Passed || !strings.Contains(res2.Message, "TC-WID-01") {
		t.Fatalf("expected IntentTraceabilityGate to reject unbound TC-WID-01, got: %+v, err=%v", res2, err2)
	}

	testGo := "package sample\nimport \"testing\"\nfunc TestTC_WID_01(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(subDir, "widget_test.go"), []byte(testGo), 0644); err != nil {
		t.Fatalf("write widget_test.go: %v", err)
	}
	res3, err3 := gate.Evaluate(ctx)
	if err3 != nil || res3 == nil || !res3.Passed {
		t.Fatalf("expected IntentTraceabilityGate to pass once struct and TC-WID-01 are aligned, got: %+v, err=%v", res3, err3)
	}
}

func TestSiblingTwinSpecIsolationHelpers(t *testing.T) {
	if got := extractWorktreeSubTaskKey("/repo/trunk"); got != "" {
		t.Fatalf("expected empty key for trunk root, got %q", got)
	}
	if got := extractWorktreeSubTaskKey("/repo/worktrees/nomos-INT-1951"); got != "" {
		t.Fatalf("expected empty key for INT worktree, got %q", got)
	}
	if got := extractWorktreeSubTaskKey("/repo/worktrees/nomos-SUB-2188"); got != "SUB-2188" {
		t.Fatalf("expected SUB-2188 for isolated SUB worktree, got %q", got)
	}

	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "intent", "architecture", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	specA := filepath.Join(specDir, "alpha_spec.md")
	specB := filepath.Join(specDir, "beta_spec.md")
	if err := os.WriteFile(specA, []byte("# Alpha\n"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.WriteFile(specB, []byte("# Beta\n"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	roots := []string{"intent/architecture/specs"}
	if trunkRoots := ResolveActiveWorktreeSpecRoots(tmp, roots); len(trunkRoots) != 1 || trunkRoots[0] != roots[0] {
		t.Fatalf("expected trunk root to keep original specRoots, got %v", trunkRoots)
	}
	kept, skipped := filterDeferredSpecRoots(tmp, roots, map[string]bool{"intent/architecture/specs/beta_spec.md": true})
	if !skipped || len(kept) != 1 || filepath.Base(kept[0]) != "alpha_spec.md" {
		t.Fatalf("expected only alpha_spec.md after deferring beta_spec.md, got kept=%v skipped=%v", kept, skipped)
	}
	unchanged, skippedNone := filterDeferredSpecRoots(tmp, roots, map[string]bool{"intent/architecture/specs/other.md": true})
	if skippedNone || len(unchanged) != 1 {
		t.Fatalf("expected unchanged roots when deferred path does not match, got %v skipped=%v", unchanged, skippedNone)
	}
}
