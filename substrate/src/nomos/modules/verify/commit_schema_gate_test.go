package verify

import (
	"testing"
)

func TestCommitSchemaGate_Name(t *testing.T) {
	gate := &CommitSchemaGate{}
	if gate.Name() != "Commit Schema Invariant Gate" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}
}

func TestCheckCommitCompliance(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		wantPassed bool
	}{
		{
			name: "valid full dual-layer",
			msg: `[SUB-101] feat(core): implement feature

**Architectural Rationale:**
Rationale details

**Blast Radius & Contracts:**
Blast radius details

**Resolution Mechanics:**
Mechanics details`,
			wantPassed: true,
		},
		{
			name: "missing prefix",
			msg: `feat(core): implement feature

**Architectural Rationale:**
Rationale details`,
			wantPassed: false,
		},
		{
			name: "missing headings",
			msg:  `[SUB-101] feat(core): implement feature without headings`,
			wantPassed: false,
		},
		{
			name:       "merge commit exempt",
			msg:        `Merge branch 'feature/SUB-101' into main`,
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := checkCommitCompliance("Commit Schema Invariant Gate", tt.msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Passed != tt.wantPassed {
				t.Errorf("checkCommitCompliance() passed = %v, want %v (msg: %s)", res.Passed, tt.wantPassed, res.Message)
			}
		})
	}
}
