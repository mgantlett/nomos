// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
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

type taskCreateOpts struct {
	plane     string
	tier      string
	customKey string
	twinOf    string
	priority  int
	card      taskCardFlags
	stdin     io.Reader
}

func newTaskCreateCmd() *cobra.Command {
	var opts taskCreateOpts

	cmd := &cobra.Command{
		Use:     "create <title>",
		Short:   "Allocate and persist a new Living Ledger task record",
		Long:    helpTaskCreateLong,
		Example: helpTaskCreateExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.stdin = cmd.InOrStdin()
			return executeTaskCreate(args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.plane, flagNamePlane, "p", "substrate", "Target plane (intent|substrate)")
	cmd.Flags().StringVarP(&opts.tier, flagNameTier, "t", "T1", "Cognitive tier (T1|T2)")
	cmd.Flags().IntVarP(&opts.priority, "priority", "r", 50, "Priority (0-100)")
	cmd.Flags().StringVarP(&opts.customKey, "key", "k", "", "Explicit task key (e.g. SUB-1212)")
	cmd.Flags().StringVar(&opts.twinOf, "twin-of", "", "Twin task key in opposite plane to link with")
	bindTaskCardFlags(cmd, &opts.card, false)
	return cmd
}

func normalizePlaneInput(plane, customKey string) string {
	switch strings.ToLower(strings.TrimSpace(plane)) {
	case "sub", "substrate":
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(customKey)), "INT-") {
			return "intent"
		}
		return "substrate"
	case "int", "intent":
		return "intent"
	default:
		return strings.ToLower(strings.TrimSpace(plane))
	}
}

func executeTaskCreate(title string, opts taskCreateOpts) error {
	opts.plane = normalizePlaneInput(opts.plane, opts.customKey)
	body, burden, err := resolveCardBodyAndBurden(opts.stdin, "create", title, opts.plane, opts.card, "")
	if err != nil {
		return err
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return err
	}
	key, err := resolveOrAllocateCreateKey(store, opts.plane, opts.customKey)
	if err != nil {
		return err
	}
	deps, err := resolveValidatedDependencies(store, opts.card.dependsOn, key)
	if err != nil {
		return err
	}
	twinOf := strings.ToUpper(strings.TrimSpace(opts.twinOf))
	if twinOf != "" {
		if err := task.ValidateTaskKey(twinOf); err != nil {
			return fmt.Errorf("invalid twin task key: %w", err)
		}
	}

	t := task.TaskRecord{
		Key: key, Plane: opts.plane, Project: resolveActiveTaskProject(opts.card.project),
		SchemaType: task.SchemaType(opts.plane), Title: title, Description: body,
		Status: task.StatusTodo, Tier: task.Tier(opts.tier), Priority: opts.priority,
		TwinKey: twinOf, Dependencies: deps, Burden: burden,
	}
	if err := store.CreateTask(t); err != nil {
		return err
	}
	if twinOf != "" {
		if err := store.BindTwin(key, twinOf); err != nil {
			return fmt.Errorf("failed linking twin %s: %w", twinOf, err)
		}
		fmt.Printf("✨ %s %s (%s) - %s (twinned with %s)\n",
			uiSuccessStyle.Render("Created task:"), styleTaskKey(key), stylePlane(opts.plane),
			uiBoldValueStyle.Render(title), styleTaskKey(twinOf))
	} else {
		fmt.Printf("✨ %s %s (%s) - %s\n",
			uiSuccessStyle.Render("Created task:"), styleTaskKey(key), stylePlane(opts.plane),
			uiBoldValueStyle.Render(title))
	}
	if len(deps) > 0 {
		fmt.Printf("  %s %s %s\n", uiWarnStyle.Render("Blocked By:"),
			strings.Join(deps, ", "), uiMutedStyle.Render("(auto-promotes to wavefront once DONE)"))
	} else {
		fmt.Printf("  %s %s\n", uiLabelStyle.Render("Next:"),
			uiCommandStyle.Render(fmt.Sprintf("nomos task start %s --edit", key)))
	}
	return nil
}

func resolveOrAllocateCreateKey(store task.TaskStore, plane, customKey string) (string, error) {
	if customKey == "" {
		return allocateNextKey(store, plane)
	}
	key := strings.ToUpper(strings.TrimSpace(customKey))
	if err := task.ValidateTaskKey(key); err != nil {
		return "", err
	}
	return key, nil
}

func newTaskStartCmd() *cobra.Command {
	var autoEdit, forceProject bool
	cmd := &cobra.Command{
		Use:     "start <key>",
		Short:   "Provision transient worktree and CapBAC lease for task",
		Long:    helpTaskStartLong,
		Example: helpTaskStartExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskStartWithOpts(args[0], autoEdit, forceProject)
		},
	}
	cmd.Flags().BoolVarP(&autoEdit, "edit", "e", false, "Immediately transition worktree into EDIT phase")
	cmd.Flags().BoolVar(&forceProject, "force-project", false, "Allow starting a task stamped with a different repository project")
	return cmd
}

func runTaskStart(rawKey string, autoEdit bool) error {
	return runTaskStartWithOpts(rawKey, autoEdit, false)
}

func runTaskStartWithOpts(rawKey string, autoEdit, forceProject bool) error {
	key := strings.ToUpper(strings.TrimSpace(rawKey))
	if err := task.ValidateTaskKey(key); err != nil {
		return err
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return err
	}
	t, err := store.GetTask(key)
	if err != nil {
		return err
	}
	mgr, err := workspace.NewManager("")
	if err != nil {
		return err
	}
	activeProj := resolveRepoProjectName(mgr.RepoRoot)
	if !forceProject && !task.MatchesProject(t.Project, activeProj, true) {
		return types.NewNomosError(types.ErrCodeValidationFailed,
			"task %s belongs to project %q, cannot start inside repository %q", key, t.Project, activeProj).
			WithRemedy(fmt.Sprintf("switch to the %q repository, run 'nomos task dag' for %q tasks, or run 'nomos task project %s %s' to reassign",
				t.Project, activeProj, activeProj, key))
	}
	wtPath, err := mgr.CreateWorktree(key)
	if err != nil {
		return err
	}
	targetPhase, targetStatus := types.PhasePlan, task.StatusPlan
	if autoEdit {
		targetPhase, targetStatus = types.PhaseEdit, task.StatusEdit
	}
	if err := persistStartedLease(targetPhase, key, wtPath); err != nil {
		return err
	}
	t.Status = targetStatus
	if err := store.UpdateTask(*t); err != nil {
		return err
	}
	renderTaskStartedBanner(key, wtPath, string(targetPhase))
	renderLearnedPitfallsBriefing(mgr.RepoRoot)
	renderPhaseTransitionNextActions(os.Stdout, targetPhase, key, wtPath, mgr.RepoRoot)
	return nil
}

func persistStartedLease(phase types.Phase, key, wtPath string) error {
	tok, err := capbac.MintToken(phase, key, wtPath, 100, 24*time.Hour)
	if err != nil {
		return err
	}
	db, err := storage.OpenDB("")
	if err != nil {
		return nil
	}
	defer func() { _ = db.Close() }()
	return storage.SaveLease(db, tok.ID, key, string(phase), 100, 0)
}

func newTaskEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "edit <key>",
		Short:   "Transition task to EDIT phase and enable code mutations",
		Long:    helpTaskEditLong,
		Example: helpTaskEditExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToUpper(strings.TrimSpace(args[0]))
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			t, err := store.GetTask(key)
			if err != nil {
				return err
			}
			t.Status = task.StatusEdit
			if err := store.UpdateTask(*t); err != nil {
				return err
			}
			updateActiveLeasePhase(key, "EDIT")
			fmt.Printf("✏️  Task %s transitioned to %s phase\n", styleTaskKey(key), styleSemanticBadge("EDIT"))
			renderPhaseTransitionNextActions(os.Stdout, types.PhaseEdit, key, "", "")
			return nil
		},
	}
}

func newTaskReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "review <key>",
		Short:   "Transition task to REVIEW phase and evaluate DoD quality gates",
		Long:    helpTaskReviewLong,
		Example: helpTaskReviewExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToUpper(strings.TrimSpace(args[0]))
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			t, err := store.GetTask(key)
			if err != nil {
				return err
			}
			t.Status = task.StatusReview
			if err := store.UpdateTask(*t); err != nil {
				return err
			}
			updateActiveLeasePhase(key, "REVIEW")
			_, gateErr := executeGateCheckForTask(key)
			if gateErr == nil {
				renderPhaseTransitionNextActions(os.Stdout, types.PhaseReview, key, "", "")
			}
			return gateErr
		},
	}
}

func updateActiveLeasePhase(taskKey, phase string) {
	db, err := storage.OpenDB("")
	if err != nil {
		return
	}
	defer func() { _ = db.Close() }()
	if leaseID, _, budget, used, lErr := storage.GetActiveLease(db, taskKey); lErr == nil {
		if sErr := storage.SaveLease(db, leaseID, taskKey, phase, budget, used); sErr != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to update lease phase: %v\n", sErr)
		}
	}
}

func executeGateCheckForTask(taskKey string) (*verify.PipelineReport, error) {
	targetDir, err := os.Getwd()
	if err != nil {
		targetDir = "."
	}
	repoRoot, err := workspace.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	worktreePath := targetDir

	if taskKey != "" {
		if mgr, mErr := workspace.NewManager(repoRoot); mErr == nil {
			wt := mgr.WorktreePath(taskKey)
			if fi, sErr := os.Stat(wt); sErr == nil && fi.IsDir() {
				worktreePath = wt
			}
		}
	}

	ctx := &verify.VerifyContext{
		RepoRoot:     repoRoot,
		WorktreePath: worktreePath,
		TaskID:       taskKey,
		Phase:        "REVIEW",
	}
	pipeline := verify.NewPipeline(verify.DefaultBaselineGates())
	report, err := pipeline.Execute(ctx)
	if err != nil {
		return nil, err
	}
	if !report.Passed {
		recordFailedReportTraps(repoRoot, worktreePath, taskKey, report)
		var failedMsgs []string
		for _, res := range report.GateResults {
			if !res.Passed {
				if res.GateName == "Plane Separation Invariant" {
					nErr := types.NewNomosError(types.ErrCodeCrossPlaneMutation, "%s", res.Message)
					nErr.Remedy = res.Remedy
					return nil, nErr
				}
				failedMsgs = append(failedMsgs, fmt.Sprintf("%s: %s", res.GateName, res.Message))
			}
		}
		return nil, types.NewNomosError(types.ErrCodeInvariant, "DoD verification failed: %d/%d gates passed\n  - %s",
			report.GatesPassed, report.GatesTotal, strings.Join(failedMsgs, "\n  - "))
	}
	fmt.Printf("🔍 %s %s\n",
		uiSuccessStyle.Render("DoD Verification Succeeded:"),
		uiBoldValueStyle.Render(fmt.Sprintf("%d/%d gates passed", report.GatesPassed, report.GatesTotal)))
	return report, nil
}

func newTaskAbortCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "abort <key>",
		Short:   "Abort task execution, revoke capability lease, and remove worktree",
		Long:    helpTaskAbortLong,
		Example: helpTaskAbortExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToUpper(strings.TrimSpace(args[0]))
			mgr, err := workspace.NewManager("")
			if err != nil {
				return err
			}
			if remErr := mgr.RemoveWorktree(key); remErr != nil {
				// non-fatal if worktree already removed
			}
			revokeActiveLease(key)
			mem.RemoveTaskTrapBuffer(mgr.RepoRoot, key)
			store, err := task.NewDefaultSQLiteTaskStore()
			if err == nil {
				if t, getErr := store.GetTask(key); getErr == nil {
					t.Status = task.StatusAborted
					if updErr := store.UpdateTask(*t); updErr != nil {
						return updErr
					}
				}
			}
			fmt.Printf("🛑 Task %s %s\n", styleTaskKey(key), uiWarnStyle.Render("aborted and pruned"))
			renderPhaseTransitionNextActions(os.Stdout, types.PhaseIdle, "", "", mgr.RepoRoot)
			return nil
		},
	}
}


func allocateNextKey(store task.TaskStore, plane string) (string, error) {
	prefix := "SUB"
	if strings.ToLower(plane) == "intent" {
		prefix = "INT"
	}
	tasks := store.ListTasks()
	maxNum := 0
	for _, t := range tasks {
		if strings.HasPrefix(t.Key, prefix+"-") {
			parts := strings.Split(t.Key, "-")
			if len(parts) >= 2 {
				if num, err := strconv.Atoi(parts[1]); err == nil && num > maxNum {
					maxNum = num
				}
			}
		}
	}
	return task.FormatTaskKey(prefix, maxNum+1)
}
