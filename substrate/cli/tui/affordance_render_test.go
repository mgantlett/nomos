package tui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func TestRenderNextActions(t *testing.T) {
	var buf bytes.Buffer
	actions := types.BuildPhaseNextActions(types.PhaseEdit, "SUB-42", "worktrees/demo-SUB-42", nil, nil)
	RenderNextActions(&buf, actions)

	out := buf.String()
	for _, want := range []string{
		"Next Actions:",
		"cd worktrees/demo-SUB-42",
		"nomos task commit SUB-42",
		"nomos verify",
		"nomos task sync SUB-42 -F worktrees/demo-SUB-42/.tmp/walkthrough-SUB-42.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in rendered actions:\n%s", want, out)
		}
	}
}
