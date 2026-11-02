// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

func newTaskPruneCmd() *cobra.Command {
	var dryRun, jsonOutput bool

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Sweep orphaned worktrees, merged feature branches, and stale walkthrough files",
		Long:    helpTaskPruneLong,
		Example: helpTaskPruneExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := pruneWorkspaceOrphans("", dryRun)
			if err != nil {
				return err
			}
			if jsonOutput {
				data, mErr := json.MarshalIndent(rep, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling prune report: %v", mErr)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			renderTaskPruneReport(cmd, rep)
			renderNextActions(cmd.OutOrStdout(), []types.ActionAffordance{
				{Command: "nomos status", Summary: "Inspect clean workspace state and active wavefront"},
			})
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Inspect orphaned worktrees and branches without deleting")
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit prune report as structured JSON")
	return cmd
}

func pruneWorkspaceOrphans(repoRoot string, dryRun bool) (*workspace.PruneReport, error) {
	mgr, err := workspace.NewManager(repoRoot)
	if err != nil {
		return nil, err
	}
	activeKeys := collectActiveTaskKeys(dryRun)
	return mgr.PruneOrphanWorktrees(dryRun, activeKeys)
}

func collectActiveTaskKeys(dryRun bool) map[string]bool {
	active := map[string]bool{}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return active
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	var completedKeys []string
	for _, t := range store.ListTasks() {
		key := strings.ToUpper(strings.TrimSpace(t.Key))
		switch t.Status {
		case task.StatusDone, task.StatusAborted:
			completedKeys = append(completedKeys, key)
		case task.StatusTodo:
			// Not yet started; no active worktree lease
		default:
			active[key] = true
		}
	}

	if !dryRun && len(completedKeys) > 0 {
		reconcileCompletedTaskLeases(completedKeys)
	}
	return active
}

func reconcileCompletedTaskLeases(completedKeys []string) {
	db, err := storage.OpenDB("")
	if err != nil {
		return
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			// closed
		}
	}()
	for _, key := range completedKeys {
		if rErr := storage.RevokeLease(db, key); rErr != nil {
			continue
		}
	}
}

func revokeActiveLease(taskKey string) {
	db, err := storage.OpenDB("")
	if err != nil {
		return
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			// closed
		}
	}()
	if rErr := storage.RevokeLease(db, taskKey); rErr != nil {
		return
	}
}

func renderTaskPruneReport(cmd *cobra.Command, rep *workspace.PruneReport) {
	out := cmd.OutOrStdout()
	mode := uiSuccessStyle.Render("Executed")
	if rep.DryRun {
		mode = uiWarnStyle.Render("Dry-Run Preview")
	}
	fmt.Fprintf(out, "%s (%s)\n", renderBrandTitle("🧹 Nomos Worktree Garbage Collector"), mode)
	fmt.Fprintln(out, uiRule(ruleStandard))
	fmt.Fprintln(out, kvLine("Orphan Worktrees Swept", 24, uiBoldValueStyle.Render(fmt.Sprintf("%d", len(rep.RemovedDirs)))))
	for _, d := range rep.RemovedDirs {
		fmt.Fprintf(out, "    - %s\n", uiMutedStyle.Render("worktrees/"+d))
	}
	fmt.Fprintln(out, kvLine("Merged Branches Pruned", 24, uiBoldValueStyle.Render(fmt.Sprintf("%d", len(rep.RemovedBranches)))))
	for _, b := range rep.RemovedBranches {
		fmt.Fprintf(out, "    - %s\n", uiMutedStyle.Render(b))
	}
	fmt.Fprintln(out, kvLine("Stale Walkthroughs Swept", 24, uiBoldValueStyle.Render(fmt.Sprintf("%d", len(rep.RemovedTempFiles)))))
	if len(rep.SkippedActive) > 0 {
		active := fmt.Sprintf("%s %s",
			uiBoldValueStyle.Render(fmt.Sprintf("%d", len(rep.SkippedActive))),
			uiMutedStyle.Render("("+strings.Join(rep.SkippedActive, ", ")+")"))
		fmt.Fprintln(out, kvLine("Active Worktrees Kept", 24, active))
	}
	fmt.Fprintln(out, uiRule(ruleStandard))
}
