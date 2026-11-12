package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

func TestEvaluateMutationSensitivity(t *testing.T) {
	killed, total, err := EvaluateMutationSensitivity("", nil)
	if err != nil {
		t.Fatalf("unexpected mutation proof error: %v", err)
	}
	if killed != total {
		t.Errorf("expected 100%% mutants killed, got %d/%d", killed, total)
	}
}

func TestEvaluateMetamorphicSpecificity(t *testing.T) {
	passed, total, err := EvaluateMetamorphicSpecificity(nil)
	if err != nil {
		t.Fatalf("unexpected specificity proof error: %v", err)
	}
	if passed != total {
		t.Errorf("expected 100%% clean constructs to pass, got %d/%d", passed, total)
	}
}

func TestEvaluateNonVacuousGates(t *testing.T) {
	nonVac, total, err := EvaluateNonVacuousGates("")
	if err != nil {
		t.Fatalf("unexpected non-vacuous gates audit error: %v", err)
	}
	if total == 0 {
		t.Fatalf("expected to discover gate Evaluate functions, got 0")
	}
	if nonVac != total {
		t.Errorf("expected all gates to be non-vacuous, got %d/%d", nonVac, total)
	}
}

func TestEvaluateGateIntegrityAttestation(t *testing.T) {
	report, err := EvaluateGateIntegrityAttestation("")
	if err != nil {
		t.Fatalf("unexpected proof evaluation error: %v", err)
	}
	if !report.Passed {
		t.Errorf("expected gate integrity attestation to pass, got: %+v", report)
	}
	if report.MutationScore < 100.0 {
		t.Errorf("expected mutation score 100.0%%, got %.1f%%", report.MutationScore)
	}
	if report.FalsePositiveRate > 0.0 {
		t.Errorf("expected false positive rate 0.0%%, got %.1f%%", report.FalsePositiveRate)
	}
}

func TestGateIntegrityProofGate(t *testing.T) {
	gate := &GateIntegrityProofGate{}
	if gate.Name() != "Gate Integrity & Invariant Attestation (Law 2)" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}

	ctx := &vscope.VerifyContext{RepoRoot: ""}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected gate evaluation error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected gate to pass, got message: %s", res.Message)
	}
}

func TestSampleRealFunctions_MultiFileDiversity(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.go")
	f2 := filepath.Join(dir, "b.go")
	src1 := "package p\nfunc A1() int { return 1 }\nfunc A2() int { return 2 }\n"
	src2 := "package p\nfunc B1() int { return 3 }\nfunc B2() int { return 4 }\n"
	if err := os.WriteFile(f1, []byte(src1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte(src2), 0644); err != nil {
		t.Fatal(err)
	}

	got := sampleRealFunctions([]string{f1, f2}, 2)
	if len(got) != 2 || got[0].Name.Name != "A1" || got[1].Name.Name != "B1" {
		t.Fatalf("expected one sample per file [A1, B1], got %+v", got)
	}
}

func TestEvaluateASTGateDirectory_DetectsSubpackageVacuousGate(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "vscope")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	realGate := "package verify\ntype RealGate struct{}\nfunc (g *RealGate) Evaluate() (*vscope.GateResult, error) {\n\tx := 1\n\t_ = x\n\treturn &vscope.GateResult{Passed: true}, nil\n}\n"
	vacGate := "package vscope\ntype StubGate struct{}\nfunc (g *StubGate) Evaluate() (*GateResult, error) {\n\treturn &GateResult{Passed: true}, nil\n}\n"
	vacQualGate := "package verify\ntype QualStubGate struct{}\nfunc (g *QualStubGate) Evaluate() (*vscope.GateResult, error) {\n\treturn &vscope.GateResult{Passed: true}, nil\n}\n"
	if err := os.WriteFile(filepath.Join(root, "real.go"), []byte(realGate), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "qual_stub.go"), []byte(vacQualGate), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "stub.go"), []byte(vacGate), 0644); err != nil {
		t.Fatal(err)
	}

	nonVac, total, err := evaluateASTGateDirectory(root)
	if err != nil {
		t.Fatalf("evaluateASTGateDirectory error: %v", err)
	}
	if total != 3 || nonVac != 1 {
		t.Fatalf("expected 1/3 non-vacuous gates across root and subpackages, got %d/%d", nonVac, total)
	}
}
