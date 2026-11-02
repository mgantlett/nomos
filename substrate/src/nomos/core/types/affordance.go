// Package types provides foundational domain types, schema bindings, and
// invariant contracts synthesized directly from authoritative Intent CUE schemas.
package types

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ActionAffordance defines a deterministic HATEOAS next-step hypermedia command (#ActionAffordance).
type ActionAffordance struct {
	Command string `json:"command"`
	Summary string `json:"summary"`
	Phase   Phase  `json:"phase"`
}

// BuildPhaseNextActions synthesizes deterministic HATEOAS next-step commands for the active workspace state.
// @intent INT-840 #ActionAffordance
func BuildPhaseNextActions(phase Phase, taskKey, worktreePath string, directives, readyWavefront []string) []ActionAffordance {
	key := strings.TrimSpace(taskKey)
	if key == "" {
		key = "<KEY>"
	}
	wt := strings.TrimSpace(worktreePath)

	switch phase {
	case PhasePlan:
		return buildPlanAffordances(key, wt, directives)
	case PhaseEdit:
		return buildEditAffordances(key, wt)
	case PhaseReview:
		return buildReviewAffordances(key, wt)
	case PhaseSync, PhaseLearn:
		return buildSyncLearnAffordances()
	default:
		return buildIdleAffordances(directives, readyWavefront)
	}
}

func buildIdleAffordances(directives, readyWavefront []string) []ActionAffordance {
	actions := []ActionAffordance{
		{
			Command: `nomos task chore "<title>"`,
			Summary: "Provision an isolated task worktree in EDIT phase in 1 turn",
			Phase:   PhaseEdit,
		},
	}
	if len(readyWavefront) > 0 && strings.TrimSpace(readyWavefront[0]) != "" {
		actions = append(actions, ActionAffordance{
			Command: fmt.Sprintf("nomos task start %s --edit", strings.TrimSpace(readyWavefront[0])),
			Summary: "Start top unblocked wavefront task in isolated EDIT worktree",
			Phase:   PhaseEdit,
		})
	} else {
		actions = append(actions, ActionAffordance{
			Command: `nomos task create "<title>" --plane substrate`,
			Summary: "Allocate a planned task in the Living Ledger",
			Phase:   PhaseIdle,
		})
	}
	if len(directives) > 0 && strings.TrimSpace(directives[0]) != "" {
		actions = append(actions, ActionAffordance{
			Command: fmt.Sprintf("nomos directive %s", strings.TrimSpace(directives[0])),
			Summary: "Inspect active workspace orientation directive",
			Phase:   PhaseIdle,
		})
	}
	return actions
}

func buildPlanAffordances(key, wt string, directives []string) []ActionAffordance {
	var actions []ActionAffordance
	if wt != "" {
		actions = append(actions, ActionAffordance{
			Command: fmt.Sprintf("cd %s", wt),
			Summary: "Enter isolated task worktree before inspecting or planning",
			Phase:   PhasePlan,
		})
	}
	actions = append(actions, ActionAffordance{
		Command: fmt.Sprintf("nomos task edit %s", key),
		Summary: "Transition task lease from PLAN to EDIT to enable file mutations",
		Phase:   PhaseEdit,
	})
	if len(directives) > 0 && strings.TrimSpace(directives[0]) != "" {
		actions = append(actions, ActionAffordance{
			Command: fmt.Sprintf("nomos directive %s", strings.TrimSpace(directives[0])),
			Summary: "Inspect active planning directive",
			Phase:   PhasePlan,
		})
	}
	return actions
}

func buildEditAffordances(key, wt string) []ActionAffordance {
	var actions []ActionAffordance
	if wt != "" {
		actions = append(actions, ActionAffordance{
			Command: fmt.Sprintf("cd %s", wt),
			Summary: "Execute all file edits strictly inside isolated task worktree",
			Phase:   PhaseEdit,
		})
	}
	wtWalkthrough := walkthroughHintPath(key, wt)
	actions = append(actions,
		ActionAffordance{
			Command: fmt.Sprintf(`nomos task commit %s -m "<type(scope): summary>"`, key),
			Summary: "Stage and commit worktree changes (auto-hydrates Dual-Layer card/diff sections, or pass --rationale/--blast-radius/--mechanics)",
			Phase:   PhaseEdit,
		},
		ActionAffordance{
			Command: "nomos verify",
			Summary: "Evaluate all 42 Definition of Done quality gates",
			Phase:   PhaseReview,
		},
		ActionAffordance{
			Command: fmt.Sprintf("nomos task sync %s -F %s", key, wtWalkthrough),
			Summary: "Complete atomic 2PC merge to trunk and record release receipt",
			Phase:   PhaseSync,
		},
	)
	return actions
}

func buildReviewAffordances(key, wt string) []ActionAffordance {
	wtWalkthrough := walkthroughHintPath(key, wt)
	return []ActionAffordance{
		{
			Command: fmt.Sprintf("nomos task sync %s -F %s", key, wtWalkthrough),
			Summary: "Complete atomic 2PC merge to trunk with walkthrough receipt",
			Phase:   PhaseSync,
		},
		{
			Command: fmt.Sprintf("nomos task edit %s", key),
			Summary: "Return task to EDIT phase for additional modifications",
			Phase:   PhaseEdit,
		},
	}
}

func buildSyncLearnAffordances() []ActionAffordance {
	return []ActionAffordance{
		{
			Command: "nomos handshake",
			Summary: "Re-orient session on clean trunk baseline",
			Phase:   PhaseIdle,
		},
		{
			Command: "nomos task dag",
			Summary: "Inspect next unblocked candidate tasks on causal DAG wavefront",
			Phase:   PhaseIdle,
		},
	}
}

func walkthroughHintPath(key, wt string) string {
	fname := fmt.Sprintf("walkthrough-%s.md", key)
	if wt != "" {
		return filepath.ToSlash(filepath.Join(wt, ".tmp", fname))
	}
	return filepath.ToSlash(filepath.Join(".tmp", fname))
}

