package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"testing"
)

type mockGate struct {
	name   string
	probe  string
	passed bool
	msg    string
}

func (m *mockGate) Name() string { return m.name }
func (m *mockGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	return &vscope.GateResult{
		GateName: m.name,
		ProbeID:  m.probe,
		Passed:   m.passed,
		Message:  m.msg,
	}, nil
}

func TestPipeline_Execute(t *testing.T) {
	gates := []vscope.Gate{
		&mockGate{name: "Anti-Shim AST Invariant", probe: "anti_shim", passed: true, msg: "clean"},
		&mockGate{name: "Complexity Invariant", probe: "complexity", passed: true, msg: "clean"},
	}

	pipe := NewPipeline(gates)
	ctx := &vscope.VerifyContext{TaskID: "SUB-1195", Phase: "REVIEW"}

	rep, err := pipe.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error executing pipeline: %v", err)
	}

	if !rep.Passed {
		t.Errorf("expected pipeline to pass")
	}
	if rep.GatesTotal != 2 {
		t.Errorf("expected 2 total gates, got %d", rep.GatesTotal)
	}
	if rep.GatesPassed != 2 {
		t.Errorf("expected 2 passed gates, got %d", rep.GatesPassed)
	}
	if len(rep.LawReports) != 7 {
		t.Errorf("expected 7 law reports, got %d", len(rep.LawReports))
	}
	if len(rep.CellReports) != 21 {
		t.Errorf("expected 21 cell reports, got %d", len(rep.CellReports))
	}
}

func TestPipeline_Execute_Failure(t *testing.T) {
	gates := []vscope.Gate{
		&mockGate{name: "Anti-Shim AST Invariant", probe: "anti_shim", passed: true, msg: "clean"},
		&mockGate{name: "Failing Gate", probe: "complexity", passed: false, msg: "exceeded"},
	}

	pipe := NewPipeline(gates)
	ctx := &vscope.VerifyContext{TaskID: "SUB-1195", Phase: "REVIEW"}

	rep, err := pipe.Execute(ctx)
	if err != nil {
		t.Fatalf("unexpected error executing pipeline: %v", err)
	}

	if rep.Passed {
		t.Errorf("expected pipeline to fail when a gate fails")
	}
	if rep.GatesPassed != 1 {
		t.Errorf("expected 1 passed gate, got %d", rep.GatesPassed)
	}
}
