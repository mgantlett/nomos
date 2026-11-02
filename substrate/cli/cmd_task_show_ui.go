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

func renderTaskShowCard(out io.Writer, rec task.TaskInspectionRecord) {
	fmt.Fprintf(out, "%s %s\n", renderBrandTitle("🔎 Living Ledger Task Card ·"), styleTaskKey(rec.Key))
	fmt.Fprintln(out, uiRule(ruleStandard))
	fmt.Fprintln(out, kvLine("Title", 12, uiBoldValueStyle.Render(rec.Title)))
	if rec.Project != "" {
		fmt.Fprintln(out, kvLine("Project", 12, uiValueStyle.Render(rec.Project)))
	}
	meta := fmt.Sprintf("%s %s %s %s %s %s %s",
		stylePlane(rec.Plane), uiMutedStyle.Render("|"),
		uiBoldValueStyle.Render(string(rec.Tier)), uiMutedStyle.Render("|"),
		styleSemanticBadge(string(rec.Status)), uiMutedStyle.Render("|"),
		uiLabelStyle.Render(fmt.Sprintf("Prio %d", rec.Priority)))
	fmt.Fprintln(out, kvLine("Metadata", 12, meta))
	if rec.TwinKey != "" {
		fmt.Fprintln(out, kvLine("Twin Key", 12, styleTaskKey(rec.TwinKey)))
	}
	if len(rec.BlockedBy) > 0 {
		fmt.Fprintln(out, kvLine("Blocked By", 12, uiWarnStyle.Render("["+strings.Join(rec.BlockedBy, ", ")+"]")))
	}
	if len(rec.Blocks) > 0 {
		fmt.Fprintln(out, kvLine("Blocks", 12, uiSubstrateStyle.Render("["+strings.Join(rec.Blocks, ", ")+"]")))
	}
	if rec.WorktreePath != "" {
		fmt.Fprintln(out, kvLine("Worktree", 12, fmt.Sprintf("%s %s",
			uiCommandStyle.Render(rec.WorktreePath), uiMutedStyle.Render("(branch: "+rec.BranchName+")"))))
	}
	fmt.Fprintln(out, uiRule(ruleStandard))
	renderStyledCardBody(out, rec.Description)
	fmt.Fprintln(out, uiRule(ruleStandard))
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
	renderNextActions(out, actions)
}

func renderStyledCardBody(out io.Writer, body string) {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		fmt.Fprintf(out, "  %s\n", uiMutedStyle.Render("(No description)"))
		return
	}
	for _, line := range strings.Split(trimmed, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			fmt.Fprintf(out, "  %s\n", uiHeaderStyle.Render(line))
		case strings.HasPrefix(strings.TrimSpace(line), "- [x] "), strings.HasPrefix(strings.TrimSpace(line), "- [X] "):
			fmt.Fprintf(out, "    %s\n", uiSuccessStyle.Render(strings.TrimSpace(line)))
		case strings.HasPrefix(strings.TrimSpace(line), "- [ ] "):
			fmt.Fprintf(out, "    %s\n", uiValueStyle.Render(strings.TrimSpace(line)))
		default:
			fmt.Fprintf(out, "    %s\n", uiValueStyle.Render(line))
		}
	}
}
