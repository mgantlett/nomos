package impact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func TestEvaluateArchitectureBoundaries_DottedModulePrefixCandidate(t *testing.T) {
	dir := t.TempDir()
	domainFile := filepath.Join(dir, "src", "domain", "order.py")
	infraFile := filepath.Join(dir, "src", "infra", "db.py")
	if err := os.MkdirAll(filepath.Dir(domainFile), 0755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(infraFile), 0755); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	if err := os.WriteFile(domainFile, []byte("from mypkg.src.infra.db import save\n"), 0644); err != nil {
		t.Fatalf("write domain file: %v", err)
	}
	if err := os.WriteFile(infraFile, []byte("def save():\n    return True\n"), 0644); err != nil {
		t.Fatalf("write infra file: %v", err)
	}

	rules := []config.ArchitectureBoundaryRule{{
		Name:          "domain-purity",
		From:          "src/domain/**",
		ForbidImports: []string{"src/infra/**"},
	}}
	violations := EvaluateArchitectureBoundaries(dir, nil, rules)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation for dotted module prefix import, got: %v", violations)
	}
}
