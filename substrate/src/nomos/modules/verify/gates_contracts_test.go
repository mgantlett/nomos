package verify

import (
	"strings"
	"testing"
)

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.MinCoverage != 55.0 {
		t.Errorf("expected 55.0 min coverage, got %f", th.MinCoverage)
	}
	if th.MaxCDS != 45 {
		t.Errorf("expected 45 max CDS, got %d", th.MaxCDS)
	}
	if th.MaxComplexity != 15 {
		t.Errorf("expected 15 max complexity, got %d", th.MaxComplexity)
	}
	if th.MaxFuncLOC != 60 {
		t.Errorf("expected 60 max func loc, got %d", th.MaxFuncLOC)
	}
	if th.MaxFileLOC != 400 {
		t.Errorf("expected 400 max file loc, got %d", th.MaxFileLOC)
	}
}

func TestEvaluations(t *testing.T) {
	// Coverage
	vCov := EvaluateCoverageViolations(50.0, 55.0)
	if len(vCov) != 1 {
		t.Errorf("expected coverage violation")
	}
	vCovPass := EvaluateCoverageViolations(80.0, 55.0)
	if len(vCovPass) != 0 {
		t.Errorf("expected zero coverage violations for 80%%")
	}

	// CDS
	files := []FileMetric{
		{Name: "clean.go", CDS: 20},
		{Name: "debt.go", CDS: 60},
	}
	vCDS := EvaluateCDSViolations(files, 45)
	if len(vCDS) != 1 {
		t.Fatalf("expected 1 CDS violation, got %d", len(vCDS))
	}

	// Docstrings
	vDocs := EvaluateDocstringViolations(5, 10, 1, 20, []string{"Foo", "Bar"})
	if len(vDocs) != 1 {
		t.Fatalf("expected 1 docstring violation")
	}

	// Complexity
	funcs := []FunctionMetric{
		{File: "a.go", Line: 10, Name: "BigFn", Complexity: 20, LOC: 80},
	}
	vComp := EvaluateComplexityViolations(funcs, 15, 60)
	if len(vComp) != 2 {
		t.Fatalf("expected 2 complexity/loc violations, got %d", len(vComp))
	}

	// File LOC
	fileMetrics := []FileMetric{{Name: "giant.go", LOC: 500}}
	vFile := EvaluateFileLOCViolations(fileMetrics, 400)
	if len(vFile) != 1 {
		t.Fatalf("expected 1 file loc violation, got %d", len(vFile))
	}

	// Symlinks
	vSym := EvaluateZeroFilesystemAliasViolations([]string{"alias.txt"})
	if len(vSym) != 1 {
		t.Fatalf("expected 1 symlink violation, got %d", len(vSym))
	}
	if len(EvaluateZeroFilesystemAliasViolations(nil)) != 0 {
		t.Fatalf("expected 0 symlink violations for nil")
	}

	// Plane Violations
	vPlaneSub := EvaluatePlaneViolations("SUB-100", []string{"intent/rule.md", "substrate/main.go"})
	if len(vPlaneSub) != 1 {
		t.Fatalf("expected 1 plane violation for SUB modifying intent, got %d", len(vPlaneSub))
	}
	vPlaneInt := EvaluatePlaneViolations("INT-200", []string{"substrate/main.go", "intent/rule.md"})
	if len(vPlaneInt) != 1 {
		t.Fatalf("expected 1 plane violation for INT modifying substrate, got %d", len(vPlaneInt))
	}
	vPlaneOk := EvaluatePlaneViolations("SUB-100", []string{"substrate/main.go", "README.md"})
	if len(vPlaneOk) != 0 {
		t.Fatalf("expected 0 plane violations for legal SUB files, got %d", len(vPlaneOk))
	}

	remSub := PlaneSeparationRemedy("SUB-100")
	if !strings.Contains(remSub, "intent") {
		t.Fatalf("expected SUB remedy to reference intent twin task: %s", remSub)
	}
	remInt := PlaneSeparationRemedy("INT-200")
	if !strings.Contains(remInt, "substrate") {
		t.Fatalf("expected INT remedy to reference substrate twin task: %s", remInt)
	}
}

func TestEvaluatePlaneViolationsWithRoots(t *testing.T) {
	intentRoots := []string{"docs", "specs/architecture"}
	substrateRoots := []string{"src", "backend/cmd"}

	// SUB task cannot touch custom intent roots
	vSub := EvaluatePlaneViolationsWithRoots("SUB-10", []string{"docs/architecture.md", "src/main.go"}, intentRoots, substrateRoots)
	if len(vSub) != 1 {
		t.Fatalf("expected 1 violation for SUB modifying custom intent root docs/, got %d", len(vSub))
	}

	vSubNested := EvaluatePlaneViolationsWithRoots("SUB-10", []string{"specs/architecture/api_spec.md"}, intentRoots, substrateRoots)
	if len(vSubNested) != 1 {
		t.Fatalf("expected 1 violation for SUB modifying nested intent root specs/architecture/, got %d", len(vSubNested))
	}

	// INT task cannot touch custom substrate roots
	vInt := EvaluatePlaneViolationsWithRoots("INT-10", []string{"src/main.go", "docs/architecture.md"}, intentRoots, substrateRoots)
	if len(vInt) != 1 {
		t.Fatalf("expected 1 violation for INT modifying custom substrate root src/, got %d", len(vInt))
	}

	vIntNested := EvaluatePlaneViolationsWithRoots("INT-10", []string{"backend/cmd/server.go"}, intentRoots, substrateRoots)
	if len(vIntNested) != 1 {
		t.Fatalf("expected 1 violation for INT modifying nested substrate root backend/cmd/, got %d", len(vIntNested))
	}

	// Legal modifications in respective planes
	vSubOk := EvaluatePlaneViolationsWithRoots("SUB-10", []string{"src/main.go", "backend/cmd/server.go", "README.md"}, intentRoots, substrateRoots)
	if len(vSubOk) != 0 {
		t.Fatalf("expected 0 violations for valid SUB modifications, got %d", len(vSubOk))
	}

	vIntOk := EvaluatePlaneViolationsWithRoots("INT-10", []string{"docs/architecture.md", "specs/architecture/api_spec.md", "README.md"}, intentRoots, substrateRoots)
	if len(vIntOk) != 0 {
		t.Fatalf("expected 0 violations for valid INT modifications, got %d", len(vIntOk))
	}
}
