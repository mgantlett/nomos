package cli

import (
	"bytes"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

func TestNewVerifyCmd_Registration(t *testing.T) {
	cmd := NewVerifyCmd()
	if cmd.Use != "verify [path]" {
		t.Errorf("unexpected command use: %s", cmd.Use)
	}
	proofCmd, _, err := cmd.Find([]string{"proof"})
	if err != nil || proofCmd == nil {
		t.Fatalf("expected proof subcommand to be registered, got error: %v", err)
	}
}

func TestRenderGateIntegrityAttestationSummary(t *testing.T) {
	reportPass := &verify.ProofReport{
		TargetDir:            "/workspace",
		IsCodebaseTargeted:   true,
		CodebaseFilesScanned: 15,
		MutationScore:        100.0,
		MutationKilled:       6,
		MutationTotal:        6,
		FalsePositiveRate:    0.0,
		ReferenceGatesPass:   42,
		ReferenceGatesTotal:  42,
		NonVacuousGates:      42,
		TotalGatesEvaluated:  42,
		Passed:               true,
	}

	renderGateIntegrityAttestationSummary(reportPass)

	reportFail := &verify.ProofReport{
		MutationScore:       75.0,
		MutationKilled:      3,
		MutationTotal:       4,
		FalsePositiveRate:   2.4,
		ReferenceGatesPass:  40,
		ReferenceGatesTotal: 42,
		NonVacuousGates:     40,
		TotalGatesEvaluated: 42,
		Passed:              false,
	}
	renderGateIntegrityAttestationSummary(reportFail)
}

func TestVerifyProofCmd_Execution(t *testing.T) {
	cmd := newVerifyProofCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected proof execution to succeed, got: %v", err)
	}
}
