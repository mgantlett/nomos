package tui

import (
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/prproof"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

func TestRenderXrayAndPRProofLipgloss(t *testing.T) {
	xrayOut := RenderXrayTerminal(&xray.XrayReport{
		ProjectName:        "demo-svc",
		DurationMillis:     14,
		FilesScanned:       12,
		TotalLOC:           1420,
		Languages:          map[string]int{"go": 10, "ts": 2},
		HangoverScore:      22,
		Grade:              "B",
		CloneGroups:        1,
		MonolithFiles:      1,
		InferredBoundaries: 3,
		Hotspots: []xray.XrayHotspot{
			{File: "pkg/core/engine.go", Language: "go", LOC: 412, CDS: 64, MaxComplexity: 14, Reasons: []string{"monolith >400 LOC"}},
		},
		NextCommand: "nomos init --baseline",
	})
	for _, want := range []string{"Nomos Brownfield X-Ray", "demo-svc", "22/100", "pkg/core/engine.go", "nomos init --baseline"} {
		if !strings.Contains(xrayOut, want) {
			t.Fatalf("expected %q in RenderXrayTerminal output:\n%s", want, xrayOut)
		}
	}

	mutRep := &prproof.DiffMutationReport{
		BaseRef:         "HEAD~1",
		MutantsTotal:    8,
		MutantsKilled:   7,
		MutantsSurvived: 1,
		KillScorePct:    87.5,
		Passed:          false,
		DurationMillis:  45,
		Survivors: []prproof.DiffMutant{
			{ID: "MUT-01", File: "engine.go", Line: 42, Function: "Check", OriginalOp: "==", MutatedOp: "!="},
		},
	}
	proofOut := RenderPRProofTerminal(&prproof.PRProofReceipt{
		ProjectName:    "demo-svc",
		BaseRef:        "HEAD~1",
		HeadCommit:     "abc12345",
		GatesPassed:    42,
		GatesTotal:     42,
		PlaneSummary:   "substrate",
		DebtDelta:      prproof.PRDebtDelta{BaseScore: 24, HeadScore: 22, DeltaScore: -2, HeadGrade: "B"},
		MutationReport: mutRep,
		Passed:         false,
		ReceiptSHA256:  strings.Repeat("a", 64),
	})
	for _, want := range []string{"PR Proof Receipt", "42/42", "87.5%", "MUT-01", "engine.go:42"} {
		if !strings.Contains(proofOut, want) {
			t.Fatalf("expected %q in RenderPRProofTerminal output:\n%s", want, proofOut)
		}
	}

	diffOut := RenderDiffMutationTerminal(mutRep)
	if !strings.Contains(diffOut, "Diff Micro-Mutation Oracle") || !strings.Contains(diffOut, "MUT-01") {
		t.Fatalf("unexpected RenderDiffMutationTerminal output:\n%s", diffOut)
	}

	mtpoOut := RenderMTPOAdvisories([]mtpo.MTPOConflict{
		{
			EarlierSeq:         8,
			EarlierTaskKey:     "SUB-2410",
			LaterSeq:           9,
			LaterTaskKey:       "SUB-2411",
			HazardKind:         "WRITE_WRITE",
			OverlappingFiles:   []string{"server.go"},
			OverlappingSymbols: []string{"NewServer"},
		},
	})
	if !strings.Contains(mtpoOut, "SUB-2411") || !strings.Contains(mtpoOut, "seq #9") {
		t.Fatalf("unexpected RenderMTPOAdvisories output:\n%s", mtpoOut)
	}
}
