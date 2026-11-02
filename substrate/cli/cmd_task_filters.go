// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

type taskListFilterOpts struct {
	project string
	status  string
	plane   string
	tier    string
	pending bool
	limit   int
	jsonOut bool
}

type taskDAGFilterOpts struct {
	project string
	plane   string
	tier    string
	limit   int
	jsonOut bool
}

func applyTaskListFilters(all []task.TaskRecord, opts taskListFilterOpts) ([]task.TaskRecord, error) {
	statusSet, err := parseStatusFilterSet(opts.status)
	if err != nil {
		return nil, err
	}
	planeNorm, err := parseOptionalPlaneFilter(opts.plane)
	if err != nil {
		return nil, err
	}
	tierNorm, err := parseOptionalTierFilter(opts.tier)
	if err != nil {
		return nil, err
	}

	filtered := filterTasksByProject(all, opts.project)
	out := make([]task.TaskRecord, 0, len(filtered))
	for _, t := range filtered {
		if !matchesTaskRecordFilters(t, opts.pending, statusSet, planeNorm, tierNorm) {
			continue
		}
		out = append(out, t)
		if opts.limit > 0 && len(out) >= opts.limit {
			break
		}
	}
	return out, nil
}

func matchesTaskRecordFilters(t task.TaskRecord, pending bool, statusSet map[string]bool, planeNorm, tierNorm string) bool {
	st := strings.ToUpper(strings.TrimSpace(string(t.Status)))
	if pending && (st == string(task.StatusDone) || st == string(task.StatusAborted)) {
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

func applyWavefrontFilters(wf []task.WavefrontCandidate, store task.TaskStore, opts taskDAGFilterOpts) ([]task.WavefrontCandidate, error) {
	planeNorm, err := parseOptionalPlaneFilter(opts.plane)
	if err != nil {
		return nil, err
	}
	tierNorm, err := parseOptionalTierFilter(opts.tier)
	if err != nil {
		return nil, err
	}
	projClean := strings.TrimSpace(opts.project)

	var projIndex map[string]string
	if projClean != "" && store != nil {
		projIndex = buildTaskProjectIndex(store.ListTasks())
	}

	out := make([]task.WavefrontCandidate, 0, len(wf))
	for _, c := range wf {
		if projClean != "" && !strings.EqualFold(projIndex[strings.ToUpper(strings.TrimSpace(c.Key))], projClean) {
			continue
		}
		if planeNorm != "" && !strings.EqualFold(strings.TrimSpace(c.Plane), planeNorm) {
			continue
		}
		if tierNorm != "" && !strings.EqualFold(strings.TrimSpace(string(c.Tier)), tierNorm) {
			continue
		}
		out = append(out, c)
		if opts.limit > 0 && len(out) >= opts.limit {
			break
		}
	}
	return out, nil
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
	norm := normalizePlaneInput(clean, "")
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

func renderTaskListNextActions(out io.Writer, tasks []task.TaskRecord, totalCount int) {
	fmt.Fprintf(out, "  %s\n", uiFooterStyle.Render(fmt.Sprintf("Showing %d of %d registered task(s)", len(tasks), totalCount)))
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
	renderNextActions(out, actions)
}

func renderWavefrontNextActions(out io.Writer, wf []task.WavefrontCandidate) {
	if len(wf) == 0 {
		renderNextActions(out, []types.ActionAffordance{
			{Command: "nomos task list --pending", Summary: "Inspect in-progress or blocked tasks"},
			{Command: "nomos task chore \"<title>\"", Summary: "Provision a 1-turn isolated EDIT worktree"},
		})
		return
	}
	top := wf[0].Key
	renderNextActions(out, []types.ActionAffordance{
		{Command: fmt.Sprintf("nomos task start %s --edit", top), Summary: "Start top unblocked wavefront task in isolated EDIT worktree"},
		{Command: fmt.Sprintf("nomos task show %s", top), Summary: "Inspect full 3-section task card before starting"},
	})
}
