// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func init() {
	RegisterGate("gate_integrity_proof", func() Gate { return &GateIntegrityProofGate{} })
}

// GateIntegrityProofGate enforces the Three Invariant Attestation Vectors of Gate Integrity (Cell 2.3, Law 2).
type GateIntegrityProofGate struct{}

// Name returns the canonical display name of the gate.
func (g *GateIntegrityProofGate) Name() string {
	return "Gate Integrity & Invariant Attestation (Law 2)"
}

// Evaluate executes the Three Invariant Attestation Vectors against the target repository.
func (g *GateIntegrityProofGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := ctx.RepoRoot
	if root == "" {
		root = ctx.WorktreePath
	}
	report, err := EvaluateGateIntegrityAttestation(root)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeInvariant, "gate integrity attestation execution failed: %v", err)
	}
	if !report.Passed {
		return &GateResult{
			GateName: g.Name(),
			ProbeID:  "gate_integrity_proof",
			Law:      2,
			Ring:     RingCryptographic,
			Passed:   false,
			Message: fmt.Sprintf("gate integrity attestation failed: MS=%.1f%% (%d/%d killed), FPR=%.1f%%, NonVacuous=%d/%d",
				report.MutationScore, report.MutationKilled, report.MutationTotal,
				report.FalsePositiveRate, report.NonVacuousGates, report.TotalGatesEvaluated),
			Remedy: "Definition of Done gates must kill 100% of mutants, produce 0% false alarms, and have non-vacuous AST logic",
		}, nil
	}

	return &GateResult{
		GateName: g.Name(),
		ProbeID:  "gate_integrity_proof",
		Law:      2,
		Ring:     RingCryptographic,
		Passed:   true,
		Message: fmt.Sprintf("mathematical proof satisfied: MS=%.1f%% (%d/%d killed), FPR=%.1f%% (%d/%d passed), %d non-vacuous gates (%.2f ms)",
			report.MutationScore, report.MutationKilled, report.MutationTotal,
			report.FalsePositiveRate, report.ReferenceGatesPass, report.ReferenceGatesTotal,
			report.NonVacuousGates, float64(report.DurationTotalMicros)/1000.0),
	}, nil
}
