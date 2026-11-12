package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

// @verifies TC-MEM-16 TC-MEM-17
func TestEvaluateFastFileRadar_HotScopeSensitizationAndCooling(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "hot-radar-repo")
	pkgDir := filepath.Join(repo, "substrate", "cli")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Function with complexity=10 (66% of max 15: silent at 80% baseline, warns at 65% hot-scope),
	// plus >8 parameters, blank-discarded error call, and an empty stub function.
	src := `package cli

func mayFail() error { return nil }

func EmptyShim() {}

func HotComplexHandler(a, b, c, d, e, f, g, h, i int) int {
	_ = mayFail()
	total := 0
	if a > 0 { total++ }
	if b > 0 { total++ }
	if c > 0 { total++ }
	if d > 0 { total++ }
	if e > 0 { total++ }
	if f > 0 { total++ }
	if g > 0 { total++ }
	if h > 0 { total++ }
	if i > 0 { total++ }
	return total
}
`
	filePath := filepath.Join(pkgDir, "handler.go")
	if err := os.WriteFile(filePath, []byte(src), 0644); err != nil {
		t.Fatalf("write handler.go: %v", err)
	}

	// Baseline (no hot-scope): 66% complexity is below 80% warning threshold, and hot-scope micro-probes do not fire.
	baseRes, err := EvaluateFastFileRadar(repo, filePath)
	if err != nil {
		t.Fatalf("EvaluateFastFileRadar baseline: %v", err)
	}
	if len(baseRes.Warnings) != 0 || len(baseRes.Breaches) != 0 {
		t.Fatalf("expected silent baseline radar at 66%% complexity, got warnings=%v breaches=%v", baseRes.Warnings, baseRes.Breaches)
	}

	// Seed 3 built-in AST recurrences in substrate/cli -> activates Hot-Scope (TC-MEM-16).
	touched := []string{"substrate/cli/handler.go"}
	for _, tk := range []string{"SUB-1", "SUB-2", "SUB-3"} {
		if rErr := mem.RecordGateTrap(repo, tk, touched, []mem.GateTrapEntry{{
			GateName: "Cognitive Complexity Invariant",
			Message:  "- substrate/cli/handler.go:7 (HotComplexHandler) cyclomatic complexity 16 exceeds max 15",
		}}); rErr != nil {
			t.Fatalf("RecordGateTrap %s: %v", tk, rErr)
		}
		if _, dErr := mem.DistillTaskLearnings(repo, tk, "- Extracted branch helper", touched); dErr != nil {
			t.Fatalf("DistillTaskLearnings %s: %v", tk, dErr)
		}
	}

	hotRes, err := EvaluateFastFileRadar(repo, filePath)
	if err != nil {
		t.Fatalf("EvaluateFastFileRadar hot-scope: %v", err)
	}
	joinedWarnings := strings.Join(hotRes.Warnings, " | ")
	if !strings.Contains(joinedWarnings, "hot-scope >=65% budget") {
		t.Fatalf("TC-MEM-16: expected 65%% hot-scope complexity warning, got %v", hotRes.Warnings)
	}
	joinedBreaches := strings.Join(hotRes.Breaches, " | ")
	for _, expected := range []string{"parameters", "discarded call result", "EmptyShim"} {
		if !strings.Contains(joinedBreaches, expected) {
			t.Fatalf("TC-MEM-16: expected hot-scope micro-probe breach containing %q, got %v", expected, hotRes.Breaches)
		}
	}

	// Record 5 consecutive clean task recalls -> cools hot-scope back to 80% baseline (TC-MEM-17).
	for _, tk := range []string{"SUB-10", "SUB-11", "SUB-12", "SUB-13", "SUB-14"} {
		_ = mem.RecallRepoPitfallsForTask(repo, tk, touched, 3)
	}
	cooledRes, err := EvaluateFastFileRadar(repo, filePath)
	if err != nil {
		t.Fatalf("EvaluateFastFileRadar cooled: %v", err)
	}
	if len(cooledRes.Warnings) != 0 || len(cooledRes.Breaches) != 0 {
		t.Fatalf("TC-MEM-17: expected hot-scope cooled back to silent baseline after 5 clean recalls, got warnings=%v breaches=%v", cooledRes.Warnings, cooledRes.Breaches)
	}
}

