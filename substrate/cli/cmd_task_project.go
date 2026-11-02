// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

func resolveActiveTaskProject(explicit string) string {
	if clean := strings.TrimSpace(explicit); clean != "" {
		return clean
	}
	if repoRoot, err := workspace.FindRepoRoot(""); err == nil && repoRoot != "" {
		return resolveRepoProjectName(repoRoot)
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		return filepath.Base(filepath.Clean(cwd))
	}
	return constants.AppName
}

func resolveRepoProjectName(repoRoot string) string {
	cleanRoot := filepath.Clean(repoRoot)
	repoBase := filepath.Base(cleanRoot)
	if cfg, err := config.LoadConfig(cleanRoot); err == nil && cfg != nil {
		cfgProj := strings.TrimSpace(cfg.Project)
		if cfgProj != "" && (cfgProj != constants.AppName || repoBase == constants.AppName) {
			return cfgProj
		}
	}
	if repoBase != "" && repoBase != "." && repoBase != string(filepath.Separator) {
		return repoBase
	}
	return constants.AppName
}

func filterTasksByProject(tasks []task.TaskRecord, project string) []task.TaskRecord {
	clean := strings.TrimSpace(project)
	if clean == "" {
		return tasks
	}
	out := make([]task.TaskRecord, 0, len(tasks))
	for _, t := range tasks {
		if strings.EqualFold(strings.TrimSpace(t.Project), clean) {
			out = append(out, t)
		}
	}
	return out
}

func newTaskProjectCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "project <project> <key> [key...]",
		Short:   "Assign or reconcile repository project identifier on Living Ledger tasks",
		Long:    helpTaskProjectLong,
		Example: helpTaskProjectExample,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project := strings.TrimSpace(args[0])
			if project == "" {
				return types.NewNomosError(types.ErrCodeValidationFailed, "project identifier cannot be empty")
			}
			keys, err := parseProjectTargetKeys(args[1:])
			if err != nil {
				return err
			}
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			defer func() {
				if cErr := store.Close(); cErr != nil {
					// closed
				}
			}()
			if err := assignProjectToTasks(store, project, keys); err != nil {
				return err
			}
			styledKeys := make([]string, 0, len(keys))
			for _, k := range keys {
				styledKeys = append(styledKeys, styleTaskKey(k))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "🏷️  %s %s to %d task(s): %s\n",
				uiSuccessStyle.Render("Assigned project"),
				uiBoldValueStyle.Render(fmt.Sprintf("%q", project)),
				len(keys),
				strings.Join(styledKeys, ", "))
			return nil
		},
	}
}

func parseProjectTargetKeys(rawArgs []string) ([]string, error) {
	seen := make(map[string]bool, len(rawArgs))
	var keys []string
	for _, arg := range rawArgs {
		for _, part := range strings.Split(arg, ",") {
			k := strings.ToUpper(strings.TrimSpace(part))
			if k == "" || seen[k] {
				continue
			}
			if err := task.ValidateTaskKey(k); err != nil {
				return nil, err
			}
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "at least one valid task key is required")
	}
	return keys, nil
}

func assignProjectToTasks(store task.TaskStore, project string, keys []string) error {
	records := make([]*task.TaskRecord, 0, len(keys))
	for _, k := range keys {
		rec, err := store.GetTask(k)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	for _, rec := range records {
		rec.Project = project
		if err := store.UpdateTask(*rec); err != nil {
			return err
		}
	}
	return nil
}
