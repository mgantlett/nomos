package impact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-OBL-012
func TestDiffSpecStructsBijective_MissingExtraAndTypeDrift(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "worktrees", "nomos-SUB-99")
	pkgDir := filepath.Join(tmp, "pkg", "orders")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	goSrc := "package orders\ntype OrderContract struct {\n\tID string\n\tTotalCents int64\n\tSecretExtra bool\n}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "order.go"), []byte(goSrc), 0o644); err != nil {
		t.Fatalf("write go: %v", err)
	}

	compiled := CollectCompiledStructIndex(tmp)
	specMD := "```go\npackage orders\ntype OrderContract struct {\n\tID string\n\tTotalCents string\n\tDeclaredOnly int\n}\n```\n"
	specIdx := ExtractSpecStructIndex(specMD)
	issues := DiffSpecStructsBijective("intent/architecture/specs/order_spec.md", specIdx, compiled, nil)
	joined := strings.Join(issues, " | ")
	if !strings.Contains(joined, "missing declared field DeclaredOnly") {
		t.Fatalf("expected missing declared field DeclaredOnly, got: %v", issues)
	}
	if !strings.Contains(joined, "field TotalCents type drift (spec=string, ast=int64)") {
		t.Fatalf("expected TotalCents type drift, got: %v", issues)
	}
	if !strings.Contains(joined, "undocumented compiled field SecretExtra") {
		t.Fatalf("expected undocumented compiled field SecretExtra, got: %v", issues)
	}
}

// @verifies TC-OBL-012
func TestLookupFields_PackageQualificationDisambiguatesCollisions(t *testing.T) {
	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "social")
	dirB := filepath.Join(tmp, "domaincalc")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatalf("mkdir A: %v", err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir B: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "a.go"), []byte("package social\ntype Violation struct {\n\tRule string\n\tDetail string\n}\n"), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "b.go"), []byte("package domaincalc\ntype Violation struct {\n\tCode string\n\tLine int\n}\n"), 0o644); err != nil {
		t.Fatalf("write b.go: %v", err)
	}

	compiled := CollectCompiledStructIndex(tmp)
	specMD := "```go\npackage social\ntype Violation struct {\n\tRule string\n\tDetail string\n}\n```\n"
	issues := DiffSpecStructsBijective("social_spec.md", ExtractSpecStructIndex(specMD), compiled, nil)
	if len(issues) != 0 {
		t.Fatalf("expected package-qualified social.Violation to pass cleanly without domaincalc collision, got: %v", issues)
	}

	unqualifiedMD := "```go\ntype Violation struct {\n\tRule string\n\tDetail string\n}\n```\n"
	unqualIssues := DiffSpecStructsBijective("social_spec.md", ExtractSpecStructIndex(unqualifiedMD), compiled, nil)
	if len(unqualIssues) != 0 {
		t.Fatalf("expected unqualified social.Violation to select nearest matching package struct without merging domaincalc fields, got: %v", unqualIssues)
	}
}
