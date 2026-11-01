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
		Use:   "prune",
		Short: "Sweep orphaned worktrees, merged feature branches, and stale walkthrough files",
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
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Inspect orphaned worktrees and branches without deleting")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit prune report as structured JSON")
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
	mode := "Executed"
	if rep.DryRun {
		mode = "Dry-Run Preview"
	}
	fmt.Fprintf(out, "🧹 Nomos Worktree Garbage Collector (%s)\n", mode)
	fmt.Fprintln(out, "────────────────────────────────────────────────────────────────────────────")
	fmt.Fprintf(out, "  Orphan Worktrees Swept   : %d\n", len(rep.RemovedDirs))
	for _, d := range rep.RemovedDirs {
		fmt.Fprintf(out, "    - worktrees/%s\n", d)
	}
	fmt.Fprintf(out, "  Merged Branches Pruned   : %d\n", len(rep.RemovedBranches))
	for _, b := range rep.RemovedBranches {
		fmt.Fprintf(out, "    - %s\n", b)
	}
	fmt.Fprintf(out, "  Stale Walkthroughs Swept : %d\n", len(rep.RemovedTempFiles))
	if len(rep.SkippedActive) > 0 {
		fmt.Fprintf(out, "  Active Worktrees Kept    : %d (%s)\n", len(rep.SkippedActive), strings.Join(rep.SkippedActive, ", "))
	}
	fmt.Fprintln(out, "────────────────────────────────────────────────────────────────────────────")
}
