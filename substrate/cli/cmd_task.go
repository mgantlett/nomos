// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

// NewTaskCmd constructs the canonical 'nomos task' command hierarchy.
func NewTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage Living Ledger tasks and execution lifecycles",
		Long: `Living Ledger task management. Workspace mutability is procedurally locked
across lifecycle phases: IDLE -> PLAN -> EDIT -> REVIEW -> SYNC -> LEARN.`,
		Example: helpTaskExample,
	}

	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskPairCmd())
	cmd.AddCommand(newTaskChoreCmd())
	cmd.AddCommand(newTaskProjectCmd())
	cmd.AddCommand(newTaskStartCmd())
	cmd.AddCommand(newTaskEditCmd())
	cmd.AddCommand(newTaskCommitCmd())
	cmd.AddCommand(newTaskReviewCmd())
	cmd.AddCommand(newTaskSyncCmd())
	cmd.AddCommand(newTaskAbortCmd())
	cmd.AddCommand(newTaskPruneCmd())
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskDAGCmd())
	cmd.AddCommand(newTaskStatusCmd())
	cmd.AddCommand(newTaskShowCmd())
	return cmd
}

func newTaskListCmd() *cobra.Command {
	var opts taskListFilterOpts

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List and filter registered Living Ledger task records",
		Long:    helpTaskListLong,
		Example: helpTaskListExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			all := store.ListTasks()
			tasks, fErr := applyTaskListFilters(all, opts)
			if fErr != nil {
				return fErr
			}
			out := cmd.OutOrStdout()
			if opts.jsonOut {
				data, mErr := json.MarshalIndent(tasks, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling tasks JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderTaskList(out, tasks)
			renderTaskListNextActions(out, tasks, len(all))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.project, flagNameProject, "", "Filter tasks by repository project identifier")
	cmd.Flags().StringVarP(&opts.status, flagNameStatus, "s", "", "Filter by status (TODO, PLAN, EDIT, REVIEW, DONE, ABORTED; comma-separated)")
	cmd.Flags().BoolVar(&opts.pending, flagNamePending, false, "Show only unfinished tasks (excludes DONE and ABORTED)")
	cmd.Flags().StringVarP(&opts.plane, flagNamePlane, "p", "", "Filter tasks by plane (intent or substrate)")
	cmd.Flags().StringVarP(&opts.tier, flagNameTier, "t", "", "Filter tasks by cognitive tier (T1 or T2)")
	cmd.Flags().IntVarP(&opts.limit, flagNameLimit, "n", 0, "Maximum number of tasks to return (0 = all)")
	cmd.Flags().BoolVar(&opts.jsonOut, flagNameJSON, false, "Emit task list as structured JSON")
	return cmd
}

func renderTaskList(out io.Writer, tasks []task.TaskRecord) {
	fmt.Fprintln(out, renderBrandTitle("📋 Nomos Living Ledger Tasks"))
	fmt.Fprintln(out, uiRule(ruleStandard))
	fmt.Fprintf(out, "  %s %s %s %s %s %s\n",
		padStyled("KEY", 10, uiHeaderStyle),
		padStyled("PLANE", 10, uiHeaderStyle),
		padStyled("STATUS", 10, uiHeaderStyle),
		padStyled("TIER", 5, uiHeaderStyle),
		padStyled("PRIO", 5, uiHeaderStyle),
		uiHeaderStyle.Render("TITLE"))
	fmt.Fprintln(out, uiRule(ruleStandard))
	if len(tasks) == 0 {
		fmt.Fprintf(out, "  %s\n", uiMutedStyle.Render("(No matching tasks in Living Ledger)"))
	}
	for _, t := range tasks {
		fmt.Fprintf(out, "  %s %s %s %s %s %s\n",
			styleTaskKeyWidth(t.Key, 10),
			stylePlaneWidth(t.Plane, 10),
			styleSemanticBadgeWidth(string(t.Status), 10),
			padStyled(string(t.Tier), 5, uiMutedStyle),
			padStyled(fmt.Sprintf("%d", t.Priority), 5, uiLabelStyle),
			uiValueStyle.Render(t.Title))
	}
	fmt.Fprintln(out, uiRule(ruleStandard))
}

func newTaskDAGCmd() *cobra.Command {
	var opts taskDAGFilterOpts

	cmd := &cobra.Command{
		Use:     "dag",
		Short:   "Display causal DAG wavefront of unblocked candidate tasks",
		Long:    helpTaskDAGLong,
		Example: helpTaskDAGExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			wf, err := store.GetWavefront()
			if err != nil {
				return err
			}
			filtered, fErr := applyWavefrontFilters(wf, store, opts)
			if fErr != nil {
				return fErr
			}
			out := cmd.OutOrStdout()
			if opts.jsonOut {
				data, mErr := json.MarshalIndent(filtered, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling wavefront JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderWavefront(out, filtered)
			renderWavefrontNextActions(out, filtered)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.project, flagNameProject, "", "Filter wavefront candidates by repository project identifier")
	cmd.Flags().StringVarP(&opts.plane, flagNamePlane, "p", "", "Filter wavefront candidates by plane (intent or substrate)")
	cmd.Flags().StringVarP(&opts.tier, flagNameTier, "t", "", "Filter wavefront candidates by cognitive tier (T1 or T2)")
	cmd.Flags().IntVarP(&opts.limit, flagNameLimit, "n", 0, "Maximum wavefront candidates to display (0 = all)")
	cmd.Flags().BoolVar(&opts.jsonOut, flagNameJSON, false, "Emit wavefront as structured JSON")
	return cmd
}

func renderWavefront(out io.Writer, wf []task.WavefrontCandidate) {
	fmt.Fprintln(out, renderBrandTitle("🌊 Nomos Causal DAG Wavefront (Unblocked Ready Tasks)"))
	fmt.Fprintln(out, uiRule(ruleStandard))
	if len(wf) == 0 {
		fmt.Fprintf(out, "  %s\n", uiMutedStyle.Render("(No pending unblocked tasks in current wavefront)"))
	}
	for _, cand := range wf {
		dot := uiElectricCyanStyle.Render("◈")
		if strings.HasPrefix(strings.ToUpper(cand.Key), "INT-") {
			dot = uiIntentStyle.Render("◈")
		}
		fmt.Fprintf(out, "  %s [%s] %s %s\n",
			dot,
			styleTaskKey(cand.Key),
			uiBoldValueStyle.Render(cand.Title),
			uiMutedStyle.Render(fmt.Sprintf("(%s, %s)", cand.Plane, cand.Tier)))
	}
	fmt.Fprintln(out, uiRule(ruleStandard))
}

func newTaskStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Display active workspace phase and CapBAC token allocation",
		Long:    helpTaskStatusLong,
		Example: helpTaskStatusExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			ps := collectPhaseStateSnapshot()
			if err := ps.Validate(); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if jsonOutput {
				data, mErr := json.MarshalIndent(ps, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling phase state JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			fmt.Fprintln(out, renderBrandTitle("📋 Nomos Workspace Phase State"))
			fmt.Fprintln(out, uiRule(ruleNarrow))
			fmt.Fprintln(out, kvLine("Active Phase", 15, styleSemanticBadge(string(ps.Phase))))
			if ps.TaskID != "" {
				fmt.Fprintln(out, kvLine("Active Task", 15, styleTaskKey(ps.TaskID)))
			}
			fmt.Fprintln(out, kvLine("Assignee", 15, uiValueStyle.Render(ps.Assignee)))
			fmt.Fprintln(out, kvLine("Step Budget", 15, uiBoldValueStyle.Render(fmt.Sprintf("%d", ps.StepBudget))))
			fmt.Fprintln(out, kvLine("Tokens Consumed", 15, uiValueStyle.Render(fmt.Sprintf("%d", ps.TokensUsed))))
			renderNextActions(out, ps.NextActions)
			fmt.Fprintln(out, uiRule(ruleNarrow))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit phase state as structured JSON")
	return cmd
}

func collectPhaseStateSnapshot() *types.PhaseState {
	ps := types.NewDefaultPhaseState()
	if ev, hErr := handshake.SynthesizeDynamicHandshake(""); hErr == nil && ev != nil {
		if ev.Seat != "" {
			ps.Assignee = ev.Seat
		}
		if ev.ActiveTask != "" && ev.TokenID != "" {
			ps.Phase = ev.Phase
			ps.TaskID = ev.ActiveTask
			ps.Token = ev.TokenID
			ps.StepBudget = ev.StepBudget
			ps.TokensUsed = ev.TokensUsed
		}
		ps.NextActions = ev.NextActions
	}
	if len(ps.NextActions) == 0 {
		ps.NextActions = types.BuildPhaseNextActions(ps.Phase, ps.TaskID, "", nil, nil)
	}
	return ps
}

func newTaskShowCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "show <key>",
		Short:   "Inspect Living Ledger task card and twin status",
		Long:    helpTaskShowLong,
		Example: helpTaskShowExample,
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
			rec := task.BuildInspectionRecord(*t, t.Dependencies, nil, "", "")
			out := cmd.OutOrStdout()
			if jsonOutput {
				data, mErr := json.MarshalIndent(rec, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling task inspection JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderTaskShowCard(out, rec)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit task inspection card as structured JSON")
	return cmd
}

type taskPairOpts struct {
	tier     string
	priority int
	card     taskCardFlags
	stdin    io.Reader
}

func newTaskPairCmd() *cobra.Command {
	var opts taskPairOpts

	cmd := &cobra.Command{
		Use:     "pair <title>",
		Short:   "Atomically allocate twinned INT (Intent) and SUB (Substrate) tasks",
		Long:    helpTaskPairLong,
		Example: helpTaskPairExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.stdin = cmd.InOrStdin()
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			return executeTaskPair(store, args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.tier, flagNameTier, "t", "T1", "Cognitive tier (T1|T2)")
	cmd.Flags().IntVarP(&opts.priority, "priority", "r", 50, "Priority (0-100)")
	bindTaskCardFlags(cmd, &opts.card, true)
	return cmd
}

func executeTaskPair(store task.TaskStore, title string, opts taskPairOpts) error {
	if err := materializeCardStdin(opts.stdin, &opts.card); err != nil {
		return attachCardRemedy(err, "pair", title, "substrate")
	}
	intBody, intBurden, err := resolveCardBodyAndBurden(nil, "pair", title, "intent", opts.card, opts.card.intScope)
	if err != nil {
		return err
	}
	subBody, subBurden, err := resolveCardBodyAndBurden(nil, "pair", title, "substrate", opts.card, opts.card.subScope)
	if err != nil {
		return err
	}
	intKey, err := allocateNextKey(store, "intent")
	if err != nil {
		return err
	}
	subKey, err := allocateNextKey(store, "substrate")
	if err != nil {
		return err
	}
	deps, err := resolveValidatedDependencies(store, opts.card.dependsOn, intKey, subKey)
	if err != nil {
		return err
	}

	project := resolveActiveTaskProject(opts.card.project)
	intTask := task.TaskRecord{
		Key: intKey, Plane: "intent", Project: project, SchemaType: task.SchemaIntent,
		Title: title, Description: intBody, Status: task.StatusTodo,
		Tier: task.Tier(opts.tier), Priority: opts.priority, TwinKey: subKey,
		Dependencies: deps, Burden: intBurden,
	}
	subTask := task.TaskRecord{
		Key: subKey, Plane: "substrate", Project: project, SchemaType: task.SchemaSubstrate,
		Title: title, Description: subBody, Status: task.StatusTodo,
		Tier: task.Tier(opts.tier), Priority: opts.priority, TwinKey: intKey,
		Dependencies: deps, Burden: subBurden,
	}

	if err := store.CreateTask(intTask); err != nil {
		return fmt.Errorf("failed creating intent task %s: %w", intKey, err)
	}
	if err := store.CreateTask(subTask); err != nil {
		return fmt.Errorf("failed creating substrate task %s: %w", subKey, err)
	}
	if err := store.BindTwin(intKey, subKey); err != nil {
		return fmt.Errorf("failed binding twins %s <-> %s: %w", intKey, subKey, err)
	}
	renderTaskPairSuccess(intKey, subKey, title)
	return nil
}

func renderTaskPairSuccess(intKey, subKey, title string) {
	fmt.Println(renderBrandTitle("🔗 Nomos Atomic Twin Provisioning Complete"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Intent Task", 14, fmt.Sprintf("%s (%s)", styleTaskKey(intKey), uiValueStyle.Render(title))))
	fmt.Println(kvLine("Substrate Task", 14, fmt.Sprintf("%s (%s)", styleTaskKey(subKey), uiValueStyle.Render(title))))
	fmt.Println(kvLine("Bilateral Link", 14, fmt.Sprintf("%s %s %s",
		styleTaskKey(intKey), uiMutedStyle.Render("<─── twin ───>"), styleTaskKey(subKey))))
	fmt.Println(uiRule(ruleNarrow))
	renderNextActions(os.Stdout, []types.ActionAffordance{
		{Command: fmt.Sprintf("nomos task start %s --edit", intKey), Summary: "Start Intent twin in isolated EDIT worktree"},
		{Command: fmt.Sprintf("nomos task start %s --edit", subKey), Summary: "Start Substrate twin in isolated EDIT worktree"},
	})
}
