// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

// RenderTaskShowCard executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func RenderTaskShowCard(out io.Writer, rec task.TaskInspectionRecord) {
	fmt.Fprintf(out, "%s %s\n", RenderBrandTitle("🔎 Living Ledger Task Card ·"), StyleTaskKey(rec.Key))
	fmt.Fprintln(out, UiRule(RuleStandard))
	fmt.Fprintln(out, KvLine("Title", 12, UiBoldValueStyle.Render(rec.Title)))
	if rec.Project != "" {
		fmt.Fprintln(out, KvLine("Project", 12, UiValueStyle.Render(rec.Project)))
	}
	meta := fmt.Sprintf("%s %s %s %s %s %s %s",
		StylePlane(rec.Plane), UiMutedStyle.Render("|"),
		UiBoldValueStyle.Render(string(rec.Tier)), UiMutedStyle.Render("|"),
		StyleSemanticBadge(string(rec.Status)), UiMutedStyle.Render("|"),
		UiLabelStyle.Render(fmt.Sprintf("Prio %d", rec.Priority)))
	fmt.Fprintln(out, KvLine("Metadata", 12, meta))
	if rec.TwinKey != "" {
		fmt.Fprintln(out, KvLine("Twin Key", 12, StyleTaskKey(rec.TwinKey)))
	}
	if len(rec.BlockedBy) > 0 {
		fmt.Fprintln(out, KvLine("Blocked By", 12, UiWarnStyle.Render("["+strings.Join(rec.BlockedBy, ", ")+"]")))
	}
	if len(rec.Blocks) > 0 {
		fmt.Fprintln(out, KvLine("Blocks", 12, UiSubstrateStyle.Render("["+strings.Join(rec.Blocks, ", ")+"]")))
	}
	if rec.WorktreePath != "" {
		fmt.Fprintln(out, KvLine("Worktree", 12, fmt.Sprintf("%s %s",
			UiCommandStyle.Render(rec.WorktreePath), UiMutedStyle.Render("(branch: "+rec.BranchName+")"))))
	}
	fmt.Fprintln(out, UiRule(RuleStandard))
	renderStyledCardBody(out, rec.Description)
	fmt.Fprintln(out, UiRule(RuleStandard))
	renderTaskShowNextActions(out, rec)
}

func renderTaskShowNextActions(out io.Writer, rec task.TaskInspectionRecord) {
	var actions []types.ActionAffordance
	switch rec.Status {
	case task.StatusTodo:
		actions = append(actions, types.ActionAffordance{
			Command: fmt.Sprintf("nomos task start %s --edit", rec.Key),
			Summary: "Provision isolated EDIT worktree for this task",
		})
	case task.StatusPlan, task.StatusEdit, task.StatusReview:
		actions = append(actions, types.ActionAffordance{
			Command: fmt.Sprintf("nomos task review %s", rec.Key),
			Summary: "Evaluate 42 Definition of Done quality gates on task worktree",
		})
	}
	if rec.TwinKey != "" {
		actions = append(actions, types.ActionAffordance{
			Command: fmt.Sprintf("nomos task show %s", rec.TwinKey),
			Summary: "Inspect bilateral twin task card",
		})
	}
	RenderNextActions(out, actions)
}

func renderStyledCardBody(out io.Writer, body string) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		fmt.Fprintf(out, "  %s\n", UiMutedStyle.Render("(No description)"))
		return
	}
	for _, line := range strings.Split(trimmed, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			fmt.Fprintf(out, "  %s\n", UiHeaderStyle.Render(line))
		case strings.HasPrefix(strings.TrimSpace(line), "- [x] "), strings.HasPrefix(strings.TrimSpace(line), "- [X] "):
			fmt.Fprintf(out, "    %s\n", UiSuccessStyle.Render(strings.TrimSpace(line)))
		case strings.HasPrefix(strings.TrimSpace(line), "- [ ] "):
			fmt.Fprintf(out, "    %s\n", UiValueStyle.Render(strings.TrimSpace(line)))
		default:
			fmt.Fprintf(out, "    %s\n", UiValueStyle.Render(line))
		}
	}
}
