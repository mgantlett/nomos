// Package taskui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package taskui

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

type TaskListFilterOpts struct {
	Project     string
	AllProjects bool
	Status      string
	Plane       string
	Tier        string
	Pending     bool
	Limit       int
	JSONOut     bool
}

type TaskDAGFilterOpts struct {
	Project     string
	AllProjects bool
	Plane       string
	Tier        string
	Limit       int
	JSONOut     bool
}

// ResolveEffectiveProjectFilter executes deterministic taskui verification logic (@intent task_lifecycle_cli_spec.md).
func ResolveEffectiveProjectFilter(explicit string, allProjects bool) (string, bool) {
	if clean := strings.TrimSpace(explicit); clean != "" {
		return clean, false
	}
	if allProjects {
		return "", true
	}
	return ResolveActiveTaskProject(""), true
}

// ApplyTaskListFilters executes deterministic taskui verification logic (@intent task_lifecycle_cli_spec.md).
func ApplyTaskListFilters(all []task.TaskRecord, opts TaskListFilterOpts) ([]task.TaskRecord, error) {
	statusSet, err := parseStatusFilterSet(opts.Status)
	if err != nil {
		return nil, err
	}
	planeNorm, err := parseOptionalPlaneFilter(opts.Plane)
	if err != nil {
		return nil, err
	}
	tierNorm, err := parseOptionalTierFilter(opts.Tier)
	if err != nil {
		return nil, err
	}

	projFilter, allowUnassigned := ResolveEffectiveProjectFilter(opts.Project, opts.AllProjects)
	filtered := task.FilterTasksByProject(all, projFilter, allowUnassigned)
	out := make([]task.TaskRecord, 0, len(filtered))
	for _, t := range filtered {
		if !matchesTaskRecordFilters(t, opts.Pending, statusSet, planeNorm, tierNorm) {
			continue
		}
		out = append(out, t)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

func matchesTaskRecordFilters(t task.TaskRecord, pending bool, statusSet map[string]bool, planeNorm, tierNorm string) bool {
	st := strings.ToUpper(strings.TrimSpace(string(t.Status)))
	if pending && task.IsTerminalStatus(t.Status) {
		return false
	}
	if len(statusSet) > 0 && !statusSet[st] {
		return false
	}
	if planeNorm != "" && !strings.EqualFold(strings.TrimSpace(t.Plane), planeNorm) {
		return false
	}
	if tierNorm != "" && !strings.EqualFold(strings.TrimSpace(string(t.Tier)), tierNorm) {
		return false
	}
	return true
}

// ApplyWavefrontFilters executes deterministic taskui verification logic (@intent task_lifecycle_cli_spec.md).
func ApplyWavefrontFilters(wf []task.WavefrontCandidate, store task.TaskStore, opts TaskDAGFilterOpts) ([]task.WavefrontCandidate, error) {
	planeNorm, err := parseOptionalPlaneFilter(opts.Plane)
	if err != nil {
		return nil, err
	}
	tierNorm, err := parseOptionalTierFilter(opts.Tier)
	if err != nil {
		return nil, err
	}
	projClean, allowUnassigned := ResolveEffectiveProjectFilter(opts.Project, opts.AllProjects)

	var projIndex map[string]string
	if projClean != "" && store != nil {
		projIndex = buildTaskProjectIndex(store.ListTasks())
	}

	out := make([]task.WavefrontCandidate, 0, len(wf))
	for _, c := range wf {
		if !matchesWavefrontCandidate(c, projClean, allowUnassigned, projIndex, planeNorm, tierNorm) {
			continue
		}
		out = append(out, c)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

func matchesWavefrontCandidate(c task.WavefrontCandidate, projClean string, allowUnassigned bool, projIndex map[string]string, planeNorm, tierNorm string) bool {
	if projClean != "" {
		candProj := strings.TrimSpace(c.Project)
		if candProj == "" && projIndex != nil {
			candProj = projIndex[strings.ToUpper(strings.TrimSpace(c.Key))]
		}
		if !task.MatchesProject(candProj, projClean, allowUnassigned) {
			return false
		}
	}
	if planeNorm != "" && !strings.EqualFold(strings.TrimSpace(c.Plane), planeNorm) {
		return false
	}
	if tierNorm != "" && !strings.EqualFold(strings.TrimSpace(string(c.Tier)), tierNorm) {
		return false
	}
	return true
}

func buildTaskProjectIndex(tasks []task.TaskRecord) map[string]string {
	idx := make(map[string]string, len(tasks))
	for _, t := range tasks {
		idx[strings.ToUpper(strings.TrimSpace(t.Key))] = strings.TrimSpace(t.Project)
	}
	return idx
}

func parseStatusFilterSet(raw string) (map[string]bool, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return nil, nil
	}
	valid := map[string]bool{
		"TODO": true, "PLAN": true, "EDIT": true,
		"REVIEW": true, "SYNC": true, "DONE": true, "ABORTED": true,
	}
	out := make(map[string]bool)
	for _, part := range strings.Split(clean, ",") {
		tok := strings.ToUpper(strings.TrimSpace(part))
		if tok == "" {
			continue
		}
		if !valid[tok] {
			return nil, types.NewNomosError(types.ErrCodeValidationFailed,
				"invalid --status %q: allowed values are TODO, PLAN, EDIT, REVIEW, DONE, ABORTED", part).
				WithRemedy("pass one or more canonical statuses, e.g. 'nomos task list --status TODO,EDIT' or '--pending'")
		}
		out[tok] = true
	}
	return out, nil
}

func parseOptionalPlaneFilter(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", nil
	}
	norm := tui.NormalizePlaneInput(clean, "")
	if norm != "intent" && norm != "substrate" {
		return "", types.NewNomosError(types.ErrCodeValidationFailed,
			"invalid --plane %q: must be intent or substrate", raw).
			WithRemedy("pass '--plane intent' or '--plane substrate'")
	}
	return norm, nil
}

func parseOptionalTierFilter(raw string) (string, error) {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	if clean == "" {
		return "", nil
	}
	if clean != "T1" && clean != "T2" {
		return "", types.NewNomosError(types.ErrCodeValidationFailed,
			"invalid --tier %q: must be T1 or T2", raw).
			WithRemedy("pass '--tier T1' or '--tier T2'")
	}
	return clean, nil
}

// RenderTaskListNextActions executes deterministic taskui verification logic (@intent task_lifecycle_cli_spec.md).
func RenderTaskListNextActions(out io.Writer, tasks []task.TaskRecord, totalCount int) {
	fmt.Fprintf(out, "  %s\n", tui.UiFooterStyle.Render(fmt.Sprintf("Showing %d of %d registered task(s)", len(tasks), totalCount)))
	actions := []types.ActionAffordance{
		{Command: "nomos task dag", Summary: "Inspect unblocked causal DAG wavefront ready to start"},
	}
	for _, t := range tasks {
		if t.Status == task.StatusTodo {
			actions = append(actions,
				types.ActionAffordance{Command: fmt.Sprintf("nomos task show %s", t.Key), Summary: "Inspect full 3-section task card and twin linkage"},
				types.ActionAffordance{Command: fmt.Sprintf("nomos task start %s --edit", t.Key), Summary: "Provision isolated EDIT worktree for task"},
			)
			break
		}
	}
	tui.RenderNextActions(out, actions)
}

// RenderWavefrontNextActions executes deterministic taskui verification logic (@intent task_lifecycle_cli_spec.md).
func RenderWavefrontNextActions(out io.Writer, wf []task.WavefrontCandidate) {
	if len(wf) == 0 {
		tui.RenderNextActions(out, []types.ActionAffordance{
			{Command: "nomos task list --pending", Summary: "Inspect in-progress or blocked tasks"},
			{Command: "nomos task chore \"<title>\"", Summary: "Provision a 1-turn isolated EDIT worktree"},
		})
		return
	}
	top := wf[0].Key
	tui.RenderNextActions(out, []types.ActionAffordance{
		{Command: fmt.Sprintf("nomos task start %s --edit", top), Summary: "Start top unblocked wavefront task in isolated EDIT worktree"},
		{Command: fmt.Sprintf("nomos task show %s", top), Summary: "Inspect full 3-section task card before starting"},
	})
}
