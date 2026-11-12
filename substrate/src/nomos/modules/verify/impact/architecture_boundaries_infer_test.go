package impact

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// @verifies TC-OBL-011
func TestInferCleanArchitectureBoundaries_ZeroBaselineViolations(t *testing.T) {
	dir := t.TempDir()
	writeInferFixture(t, dir, "app/domain/order.py", "class Order:\n    pass\n")
	writeInferFixture(t, dir, "app/infra/repo.py", "from app.domain.order import Order\n")
	writeInferFixture(t, dir, "app/api/routes.py", "from app.infra.repo import Order\n")

	rules := InferCleanArchitectureBoundaries(dir)
	if len(rules) != 1 {
		t.Fatalf("expected 1 clean domain rule (pruning container app/), got: %+v", rules)
	}
	got := rules[0]
	if got.Name != "app-domain-purity" || got.From != "app/domain/**" {
		t.Fatalf("unexpected rule metadata: %+v", got)
	}
	if !slices.Contains(got.ForbidImports, "app/infra/**") || !slices.Contains(got.ForbidImports, "app/api/**") {
		t.Fatalf("expected forbid_imports for app/infra/** and app/api/**, got: %v", got.ForbidImports)
	}
	if v := EvaluateArchitectureBoundaries(dir, nil, rules); len(v) != 0 {
		t.Fatalf("expected 0 baseline violations on inferred rules, got: %v", v)
	}
}

func TestInferCleanArchitectureBoundaries_SkipsExistingLegacyCoupling(t *testing.T) {
	dir := t.TempDir()
	writeInferFixture(t, dir, "src/domain/order.ts", "import { query } from '../infra/db';\nexport const x = query;\n")
	writeInferFixture(t, dir, "src/infra/db.ts", "export const query = 1;\n")
	writeInferFixture(t, dir, "src/routes/page.ts", "import { x } from '../domain/order';\nexport const p = x;\n")

	rules := InferCleanArchitectureBoundaries(dir)
	if len(rules) != 1 {
		t.Fatalf("expected 1 selective rule, got: %+v", rules)
	}
	if slices.Contains(rules[0].ForbidImports, "src/infra/**") {
		t.Fatalf("expected existing legacy domain->infra coupling to be skipped, got: %v", rules[0].ForbidImports)
	}
	if !slices.Contains(rules[0].ForbidImports, "src/routes/**") {
		t.Fatalf("expected clean domain->routes boundary to be enforced, got: %v", rules[0].ForbidImports)
	}
	if v := EvaluateArchitectureBoundaries(dir, nil, rules); len(v) != 0 {
		t.Fatalf("expected 0 baseline violations, got: %v", v)
	}
}

func writeInferFixture(t *testing.T, root, relPath, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}
