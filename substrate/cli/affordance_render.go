// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func renderNextActions(out io.Writer, actions []types.ActionAffordance) {
	if out == nil || len(actions) == 0 {
		return
	}
	fmt.Fprintf(out, "  %s %s\n", uiIntentStyle.Render("◈"), uiHeaderStyle.Render("Next Actions:"))
	arrow := uiElectricCyanStyle.Render("→")
	for _, act := range actions {
		fmt.Fprintf(out, "    %s %s  %s\n",
			arrow,
			uiCommandStyle.Render(act.Command),
			uiMutedStyle.Render("("+act.Summary+")"))
	}
}

func renderPhaseTransitionNextActions(out io.Writer, phase types.Phase, taskKey, worktreePath, repoRoot string) {
	wt := worktreePath
	if wt == "" && taskKey != "" {
		wt = resolveExistingTaskWorktree(taskKey)
	}
	directives := handshake.LoadPhaseDirectives(repoRoot, phase)
	actions := types.BuildPhaseNextActions(phase, taskKey, wt, directives, nil)
	renderNextActions(out, actions)
}

func resolveExistingTaskWorktree(taskKey string) string {
	mgr, err := workspace.NewManager("")
	if err != nil {
		return ""
	}
	wt := mgr.WorktreePath(taskKey)
	if fi, sErr := os.Stat(wt); sErr == nil && fi.IsDir() {
		return wt
	}
	return ""
}

