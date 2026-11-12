package impact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-OBL-013
func TestScanOrphanedGoTestObligations_IgnoresFixtureStringsAndCatchesRealOrphans(t *testing.T) {
	tmp := t.TempDir()
	pkgDir := filepath.Join(tmp, "pkg", "sample")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	testSrc := `package sample
import "testing"

// @verifies TC-DECL-01
// @verifies TC-GHOST-99
func TestObligations(t *testing.T) {
	fixture := "# Sample\n| TC-FIXTURE-77 | Synthetic fixture |\n# @verifies TC-FIXTURE-77\n"
	cases := []struct{ name string }{
		{name: "TC-DECL-02 declared table case"},
		{name: "TC-ORPHAN-42 undeclared table case"},
	}
	if fixture == "" || len(cases) == 0 {
		t.Fatal("empty fixture")
	}
}
`
	if err := os.WriteFile(filepath.Join(pkgDir, "sample_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	declared := map[string]bool{"TC-DECL-01": true, "TC-DECL-02": true}
	orphans := ScanOrphanedGoTestObligations(tmp, declared)
	joined := strings.Join(orphans, " | ")
	if !strings.Contains(joined, "TC-GHOST-99") || !strings.Contains(joined, "TC-ORPHAN-42") {
		t.Fatalf("expected TC-GHOST-99 and TC-ORPHAN-42 to be flagged, got: %v", orphans)
	}
	if strings.Contains(joined, "TC-FIXTURE-77") || strings.Contains(joined, "TC-DECL-01") {
		t.Fatalf("unexpected false positive on fixture or declared TC: %v", orphans)
	}
}

// @verifies TC-OBL-014
func TestVerifyRepoNormativeSurjection_FlagsUndocumentedSurfaceEntries(t *testing.T) {
	tmp := t.TempDir()
	mustWrite := func(rel, body string) {
		full := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("substrate/src/nomos/modules/verify/g.go", `package verify
func init() { RegisterGate("phantom_probe", nil) }
const ErrPhantom = "ERR_PHANTOM_BREACH"
`)
	mustWrite("substrate/src/nomos/core/config/c.go", "package config\ntype PhantomConfig struct { Enabled bool }\n")
	mustWrite("substrate/cli/cmd.go", `package cli
func bind(cmd any) { cmd.Flags().Bool("phantom-flag", false, "desc") }
`)

	issues := VerifyRepoNormativeSurjection(tmp, "# Incomplete Spec\n")
	if len(issues) != 4 {
		t.Fatalf("expected 4 normative surjection issues, got %d: %v", len(issues), issues)
	}

	completeSpec := "Probe `phantom_probe`, flag `--phantom-flag`, error `ERR_PHANTOM_BREACH`, and:\n```go\npackage config\ntype PhantomConfig struct { Enabled bool }\n```\n"
	if clean := VerifyRepoNormativeSurjection(tmp, completeSpec); len(clean) != 0 {
		t.Fatalf("expected 0 issues when all normative items documented, got: %v", clean)
	}
}
