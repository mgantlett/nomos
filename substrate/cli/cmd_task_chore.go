// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

func newTaskChoreCmd() *cobra.Command {
	var plane, project string
	cmd := &cobra.Command{
		Use:     "chore [title]",
		Short:   "Atomically provision a transient task worktree in EDIT phase (1-turn inception)",
		Long:    helpTaskChoreLong,
		Example: helpTaskChoreExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := "Maintenance chore"
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				title = strings.TrimSpace(args[0])
			}
			return runTaskChoreWithOpts(title, plane, project)
		},
	}
	cmd.Flags().StringVarP(&plane, flagNamePlane, "p", "substrate", "Target plane (intent|substrate)")
	cmd.Flags().StringVar(&project, flagNameProject, "", "Repository project identifier (defaults to active repository)")
	return cmd
}

func runTaskChore(title string) error {
	return runTaskChoreWithOpts(title, "substrate", "")
}

func runTaskChoreWithPlane(title, rawPlane string) error {
	return runTaskChoreWithOpts(title, rawPlane, "")
}

func runTaskChoreWithOpts(title, rawPlane, rawProject string) error {
	plane := normalizePlaneInput(rawPlane, "")
	if plane != "intent" && plane != "substrate" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid plane %q: must be intent or substrate", rawPlane)
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

	key, err := allocateNextKey(store, plane)
	if err != nil {
		return err
	}

	t := task.TaskRecord{
		Key:         key,
		Plane:       plane,
		Project:     resolveActiveTaskProject(rawProject),
		SchemaType:  task.SchemaType(plane),
		Title:       title,
		Description: "Atomic 1-turn chore worktree",
		Status:      task.StatusTodo,
		Tier:        task.Tier("T1"),
		Priority:    50,
		Burden:      1,
	}
	if cErr := store.CreateTask(t); cErr != nil {
		return cErr
	}

	wtPath, repoRoot, err := provisionChoreWorktreeAndLease(key)
	if err != nil {
		return err
	}

	t.Status = task.StatusEdit
	if uErr := store.UpdateTask(t); uErr != nil {
		return uErr
	}

	renderTaskStartedBanner(key, wtPath, "EDIT")
	renderLearnedPitfallsBriefing(repoRoot)
	renderPhaseTransitionNextActions(os.Stdout, types.PhaseEdit, key, wtPath, repoRoot)
	return nil
}

func provisionChoreWorktreeAndLease(key string) (string, string, error) {
	mgr, err := workspace.NewManager("")
	if err != nil {
		return "", "", err
	}
	wtPath, err := mgr.CreateWorktree(key)
	if err != nil {
		return "", "", err
	}
	tok, err := capbac.MintToken(types.PhaseEdit, key, wtPath, 100, 24*time.Hour)
	if err != nil {
		return "", "", err
	}
	if db, oErr := storage.OpenDB(""); oErr == nil {
		if saveErr := storage.SaveLease(db, tok.ID, key, "EDIT", 100, 0); saveErr != nil {
			_ = db.Close()
			return "", "", saveErr
		}
		_ = db.Close()
	}
	return wtPath, mgr.RepoRoot, nil
}

func renderLearnedPitfallsBriefing(repoRoot string) {
	pitfalls := mem.RecallRepoPitfalls(repoRoot, nil, 3)
	for i, p := range pitfalls {
		if i == 0 {
			fmt.Printf("  %s\n", uiWarnStyle.Render(fmt.Sprintf("Learned Pitfalls (%d active):", len(pitfalls))))
		}
		fmt.Printf("    ⚡ %s\n", uiValueStyle.Render(p))
	}
	patterns := mem.RecallRepoPatterns(repoRoot, "", nil, 2)
	for i, pat := range patterns {
		if i == 0 {
			fmt.Printf("  %s\n", uiHeaderStyle.Render(fmt.Sprintf("Verified Patterns (%d active):", len(patterns))))
		}
		fmt.Printf("    🧩 %s\n", uiMutedStyle.Render(pat))
	}
}

func recordFailedReportTraps(repoRoot, worktreePath, taskKey string, report *verify.PipelineReport) {
	if report == nil || report.Passed || strings.TrimSpace(taskKey) == "" {
		return
	}
	modified := verify.ListModifiedWorktreeFiles(worktreePath, taskKey)
	var traps []mem.GateTrapEntry
	for _, res := range report.GateResults {
		if !res.Passed {
			traps = append(traps, mem.GateTrapEntry{
				GateName: res.GateName,
				Message:  res.Message,
				Scope:    mem.DeriveTrapScope(worktreePath, res.Message, modified),
			})
		}
	}
	if rErr := mem.RecordGateTrap(repoRoot, taskKey, modified, traps); rErr != nil {
		// non-blocking episodic capture
	}
}
