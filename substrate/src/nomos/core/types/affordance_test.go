package types_test

import (
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func TestBuildPhaseNextActions_AllPhases(t *testing.T) {
	wtPath := "worktrees/acme-SUB-7"
	cases := []struct {
		name           string
		phase          types.Phase
		taskKey        string
		wt             string
		directives     []string
		wavefront      []string
		wantSubstrings []string
	}{
		{
			name:           "IDLE with wavefront and directive",
			phase:          types.PhaseIdle,
			directives:     []string{"00_tier1_agency.md"},
			wavefront:      []string{"SUB-7"},
			wantSubstrings: []string{"nomos task chore", "nomos task start SUB-7 --edit", "nomos directive 00_tier1_agency.md"},
		},
		{
			name:           "IDLE empty wavefront",
			phase:          types.PhaseIdle,
			wantSubstrings: []string{"nomos task chore", "nomos task create"},
		},
		{
			name:           "PLAN with worktree",
			phase:          types.PhasePlan,
			taskKey:        "SUB-7",
			wt:             wtPath,
			directives:     []string{"plan_01_sizing_and_dag.md"},
			wantSubstrings: []string{"cd " + wtPath, "nomos task edit SUB-7", "nomos directive plan_01_sizing_and_dag.md"},
		},
		{
			name:           "EDIT with worktree",
			phase:          types.PhaseEdit,
			taskKey:        "SUB-7",
			wt:             wtPath,
			wantSubstrings: []string{"cd " + wtPath, `nomos task sync SUB-7 -m "<summary>"`, "nomos task commit SUB-7", "nomos verify", "nomos task sync SUB-7 -F worktrees/acme-SUB-7/.tmp/walkthrough-SUB-7.md"},
		},
		{
			name:           "REVIEW without worktree",
			phase:          types.PhaseReview,
			taskKey:        "SUB-7",
			wantSubstrings: []string{`nomos task sync SUB-7 -m "<summary>"`, "nomos task sync SUB-7 -F .tmp/walkthrough-SUB-7.md", "nomos task edit SUB-7"},
		},
		{
			name:           "SYNC transition",
			phase:          types.PhaseSync,
			wantSubstrings: []string{"nomos handshake", "nomos task dag"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := types.BuildPhaseNextActions(tc.phase, tc.taskKey, tc.wt, tc.directives, tc.wavefront)
			if len(got) == 0 {
				t.Fatalf("expected non-empty affordances for %s", tc.phase)
			}
			var combined []string
			for _, a := range got {
				if strings.TrimSpace(a.Command) == "" || strings.TrimSpace(a.Summary) == "" {
					t.Fatalf("affordance has empty command or summary: %+v", a)
				}
				combined = append(combined, a.Command)
			}
			joined := strings.Join(combined, "\n")
			for _, sub := range tc.wantSubstrings {
				if !strings.Contains(joined, sub) {
					t.Errorf("expected %q in affordances:\n%s", sub, joined)
				}
			}
		})
	}
}

func TestBuildHandshakeNextActions_LeanChoreModeOmitsCreateCeremony(t *testing.T) {
	got := types.BuildHandshakeNextActions(types.PhaseIdle, "", "", []string{"00_tier1_agency.md"}, nil, true)
	var cmds []string
	for _, a := range got {
		cmds = append(cmds, a.Command)
	}
	joined := strings.Join(cmds, "\n")
	if !strings.Contains(joined, "nomos task chore") {
		t.Fatalf("expected lean chore command in IDLE, got:\n%s", joined)
	}
	if strings.Contains(joined, "nomos task create") {
		t.Fatalf("expected lean chore mode to omit task create ceremony when wavefront is empty, got:\n%s", joined)
	}
}

