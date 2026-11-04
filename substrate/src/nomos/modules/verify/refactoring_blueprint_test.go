package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrescribeGoFunctionSeams(t *testing.T) {
	goSrc := `package sample

func monolithicProcess(items []string, cfg map[string]int) int {
	total := 0
	for _, item := range items {
		if item == "alpha" || item == "beta" {
			total += cfg[item]
		} else if item == "gamma" && total > 0 {
			total -= 1
		}
		switch item {
		case "delta":
			total += 2
		case "epsilon":
			total += 3
		}
	}
	if total > 10 && total < 100 || total == 42 && len(items) > 2 || cfg["mode"] == 1 {
		total++
	}
	return total
}
`
	seams := PrescribeFunctionSeams("sample.go", []byte(goSrc), "monolithicProcess", 3)
	if len(seams) == 0 {
		t.Fatalf("expected AST seams to be prescribed for monolithicProcess, got 0")
	}
	joined := strings.Join(seams, "\n")
	if !strings.Contains(joined, "range loop") {
		t.Errorf("expected range loop seam prescription, got:\n%s", joined)
	}
	if !strings.Contains(joined, "inputs(") {
		t.Errorf("expected variable closure inputs(...) in seam prescription, got:\n%s", joined)
	}
	if !strings.Contains(joined, "compound boolean branches") {
		t.Errorf("expected boolean branch density seam prescription, got:\n%s", joined)
	}
}

func TestPrescribePolyglotFunctionSeams(t *testing.T) {
	tsSrc := `export function normalizeScene(raw: any) {
  const a = raw.a || 1;
  const b = raw.b ?? 2;
  const c = raw.c || 3;
  const d = raw.d ?? 4;
  const e = raw.e ? raw.e : 5;
  const f = raw.f || 6;
  return { a, b, c, d, e, f };
}
`
	seams := PrescribeFunctionSeams("scene.ts", []byte(tsSrc), "normalizeScene", 1)
	if len(seams) == 0 {
		t.Fatalf("expected polyglot seams for fallback-heavy TS function, got 0")
	}
	if !strings.Contains(seams[0], "Object.assign") {
		t.Errorf("expected Object.assign prescription for inline fallback chain, got: %s", seams[0])
	}
}

func TestEvaluateParameterArityViolations(t *testing.T) {
	goClean := `package p
func okFn(a, b, c, d int) int { return a + b + c + d }
`
	if v := EvaluateParameterArityViolations("clean.go", []byte(goClean), MaxFunctionParams); len(v) != 0 {
		t.Fatalf("expected 0 arity violations for 4 params, got: %v", v)
	}

	goTooMany := `package p
func confettiHelper(a, b, c, d, e, f, g, h, i int) int {
	return a + b + c + d + e + f + g + h + i
}
`
	vGo := EvaluateParameterArityViolations("confetti.go", []byte(goTooMany), MaxFunctionParams)
	if len(vGo) != 1 || !strings.Contains(vGo[0], "parameter count 9 exceeds max 8") {
		t.Fatalf("expected Go parameter arity violation for 9 params, got: %v", vGo)
	}

	tsTooMany := `export function renderWidget(a: number, b: number, c: number, d: number, e: number, f: number, g: number, h: number, i: number) {
  return a + b;
}
`
	vTS := EvaluateParameterArityViolations("widget.ts", []byte(tsTooMany), MaxFunctionParams)
	if len(vTS) != 1 || !strings.Contains(vTS[0], "parameter count 9 exceeds max 8") {
		t.Fatalf("expected TS parameter arity violation for 9 params, got: %v", vTS)
	}
}

func TestDetectStructuralClones_GoType2(t *testing.T) {
	fileA := `package alpha

func computeAlphaMetrics(records []int, scale int) int {
	acc := 0
	for idx, val := range records {
		if val > 10 {
			acc += val * scale
		} else if val < 0 {
			acc -= idx
		}
		switch val {
		case 1:
			acc += 5
		case 2:
			acc += 10
		default:
			acc += 1
		}
	}
	return acc
}
`
	// Type-2 clone in fileB: different package, function name, and local variable identifiers, identical AST structure
	fileB := `package beta

func evaluateBetaScores(entries []int, factor int) int {
	sum := 0
	for pos, item := range entries {
		if item > 10 {
			sum += item * factor
		} else if item < 0 {
			sum -= pos
		}
		switch item {
		case 1:
			sum += 5
		case 2:
			sum += 10
		default:
			sum += 1
		}
	}
	return sum
}
`
	files := map[string][]byte{
		"pkg/alpha/alpha.go": []byte(fileA),
		"pkg/beta/beta.go":   []byte(fileB),
	}

	clones := DetectStructuralClones(files, 18)
	if len(clones) != 1 {
		t.Fatalf("expected 1 structural clone group across alpha.go and beta.go, got %d", len(clones))
	}
	if len(clones[0].Occurrences) != 2 {
		t.Fatalf("expected 2 clone occurrences, got %d", len(clones[0].Occurrences))
	}

	formatted := FormatCloneViolations(clones)
	if len(formatted) != 1 || !strings.Contains(formatted[0], "structural AST clone") {
		t.Fatalf("expected formatted Law 6 clone violation, got: %v", formatted)
	}
}

func TestPrescribeFileDecomposition_And_Enrichment(t *testing.T) {
	tempDir := t.TempDir()
	src := `package big

func FirstHeavy(x int) int {
	if x > 1 && x < 10 || x == 20 {
		for i := 0; i < x; i++ {
			if i%2 == 0 {
				x += i
			}
		}
	}
	return x
}

func SecondHelper(y int) int {
	if y > 0 {
		return y + 1
	}
	return 0
}
`
	relPath := "big.go"
	fullPath := filepath.Join(tempDir, relPath)
	if err := os.WriteFile(fullPath, []byte(src), 0644); err != nil {
		t.Fatalf("failed writing temp file: %v", err)
	}

	decomp := PrescribeFileDecomposition(tempDir, relPath, []byte(src), 450, 60, 15, 60)
	if !strings.Contains(decomp, "EXTRACT 1: FirstHeavy()") {
		t.Fatalf("expected FirstHeavy() ranked as EXTRACT 1, got: %s", decomp)
	}

	locViolations := EnrichFileLOCViolations(tempDir, []FileMetric{{Name: relPath, LOC: 450}}, DefaultThresholds())
	if len(locViolations) != 1 || !strings.Contains(locViolations[0], "EXTRACT 1: FirstHeavy()") {
		t.Fatalf("expected enriched File LOC violation to include extraction candidates, got: %v", locViolations)
	}
}

func TestCognitiveDebtAndComplexityGates_EndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	// Simulate worktree context via TaskID
	ctx := &VerifyContext{
		WorktreePath: tempDir,
		RepoRoot:     tempDir,
		TaskID:       "SUB-999",
	}

	cloneBody := `
func cloneWorker(items []int, mult int) int {
	res := 0
	for i, v := range items {
		if v > 100 {
			res += v * mult
		} else if v < 0 {
			res -= i
		}
		switch v {
		case 10:
			res += 1
		case 20:
			res += 2
		default:
			res += 3
		}
	}
	return res
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "one.go"), []byte("package p\n"+cloneBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "two.go"), []byte("package p\n"+strings.ReplaceAll(cloneBody, "cloneWorker", "duplicateWorker")), 0644); err != nil {
		t.Fatal(err)
	}

	cdsGate := &CognitiveDebtGate{}
	cdsRes, err := cdsGate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error evaluating CognitiveDebtGate: %v", err)
	}
	if cdsRes.Passed || !strings.Contains(cdsRes.Message, "structural AST clone") {
		t.Fatalf("expected CognitiveDebtGate to fail closed on structural AST clone, got: %+v", cdsRes)
	}
}

func TestIntentTraceabilityGate_GoTypeAndSpecAnchorResolution(t *testing.T) {
	tempDir := t.TempDir()
	specDir := filepath.Join(tempDir, "intent", "architecture", "specs", "core")
	srcDir := filepath.Join(tempDir, "substrate", "src")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(specDir, "sample_spec.md"), []byte("---\ntitle: Sample\nplane: intent\nstatus: active\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Valid Go type and spec anchor (#DefinedContract + sample_spec.md) passes
	validGo := "package p\n// DefinedContract models the compiled Go contract.\ntype DefinedContract struct{ ID string }\n// ValidFunc is anchored.\n// @intent sample_spec.md #DefinedContract\nfunc ValidFunc() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(srcDir, "valid.go"), []byte(validGo), 0644); err != nil {
		t.Fatal(err)
	}

	gate := &IntentTraceabilityGate{}
	ctx := &VerifyContext{RepoRoot: tempDir, WorktreePath: tempDir, IsBaseline: true}
	resOk, err := gate.Evaluate(ctx)
	if err != nil || !resOk.Passed {
		t.Fatalf("expected valid @intent Go type and spec anchor to pass, got: %+v, err=%v", resOk, err)
	}

	// 2. Unregistered Go type (#GhostSchema) fails closed
	invalidGo := "package p\n// GhostFunc references a missing Go type.\n// @intent: #GhostSchema\nfunc GhostFunc() int { return 2 }\n"
	if err := os.WriteFile(filepath.Join(srcDir, "ghost.go"), []byte(invalidGo), 0644); err != nil {
		t.Fatal(err)
	}

	resFail, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resFail.Passed || !strings.Contains(resFail.Message, "#GhostSchema") {
		t.Fatalf("expected IntentTraceabilityGate to fail closed on #GhostSchema, got: %+v", resFail)
	}
}
