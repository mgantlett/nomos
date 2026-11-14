package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func TestRenderScaffoldedNomosYAML_InfersCleanArchitectureBoundaries(t *testing.T) {
	dir := t.TempDir()
	domainFile := filepath.Join(dir, "src", "domain", "order.py")
	infraFile := filepath.Join(dir, "src", "infra", "db.py")
	if err := os.MkdirAll(filepath.Dir(domainFile), 0755); err != nil {
		t.Fatalf("mkdir domain: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(infraFile), 0755); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	if err := os.WriteFile(domainFile, []byte("class Order:\n    pass\n"), 0644); err != nil {
		t.Fatalf("write domain: %v", err)
	}
	if err := os.WriteFile(infraFile, []byte("from src.domain.order import Order\n"), 0644); err != nil {
		t.Fatalf("write infra: %v", err)
	}

	rawTmpl := "project: \"{{.ProjectName}}\"\ndefault_branch: \"{{.DefaultBranch}}\"\ngovernance:\n    boundary:\n        enforce: true\n    quality_gates:\n        strict_mode: true\n        thresholds:\n            max_file_loc: 400\n            max_func_loc: 60\n            max_cyclomatic_complexity: 15\n            max_cognitive_debt_score: 45\n            min_comment_density_pct: 10\n            min_test_coverage_pct: 55\n"
	rendered := RenderScaffoldedNomosYAML(dir, rawTmpl, "hex-app", "main")
	if !strings.Contains(rendered, "strict_mode: false") {
		t.Fatalf("expected strict_mode: false, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "architecture_boundaries:") || !strings.Contains(rendered, "src-domain-purity") {
		t.Fatalf("expected inferred architecture_boundaries in rendered YAML, got:\n%s", rendered)
	}

	cfgPath := filepath.Join(dir, "nomos.yaml")
	if err := os.WriteFile(cfgPath, []byte(rendered), 0644); err != nil {
		t.Fatalf("write rendered nomos.yaml: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig failed on rendered nomos.yaml: %v", err)
	}
	if len(cfg.Governance.QualityGates.ArchitectureBoundaries) != 1 {
		t.Fatalf("expected 1 parsed ArchitectureBoundaryRule, got: %+v", cfg.Governance.QualityGates.ArchitectureBoundaries)
	}

	emptyDir := t.TempDir()
	forced := RenderScaffoldedNomosYAMLWithOptions(emptyDir, rawTmpl, "forced-bf", "main", true)
	if !strings.Contains(forced, "strict_mode: false") {
		t.Fatalf("expected forceBrownfield=true to emit strict_mode: false, got:\n%s", forced)
	}
}
