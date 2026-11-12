// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/taskui"
	"github.com/mgantlett/nomos/substrate/cli/tui"
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
		Example: tui.HelpTaskExample,
	}

	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskPairCmd())
	cmd.AddCommand(newTaskChoreCmd())
	cmd.AddCommand(taskui.NewTaskProjectCmd())
	cmd.AddCommand(newTaskStartCmd())
	cmd.AddCommand(newTaskEditCmd())
	cmd.AddCommand(newTaskCommitCmd())
	cmd.AddCommand(newTaskReviewCmd())
	cmd.AddCommand(newTaskSyncCmd())
	cmd.AddCommand(newTaskAbortCmd())
	cmd.AddCommand(taskui.NewTaskPruneCmd())
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskDAGCmd())
	cmd.AddCommand(newTaskStatusCmd())
	cmd.AddCommand(newTaskShowCmd())
	return cmd
}

func newTaskListCmd() *cobra.Command {
	var opts taskui.TaskListFilterOpts

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List and filter registered Living Ledger task records",
		Long:    tui.HelpTaskListLong,
		Example: tui.HelpTaskListExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			all := store.ListTasks()
			tasks, fErr := taskui.ApplyTaskListFilters(all, opts)
			if fErr != nil {
				return fErr
			}
			out := cmd.OutOrStdout()
			if opts.JSONOut {
				data, mErr := json.MarshalIndent(tasks, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling tasks JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderTaskList(out, tasks)
			projFilter, allowUnassigned := taskui.ResolveEffectiveProjectFilter(opts.Project, opts.AllProjects)
			scopedPool := task.FilterTasksByProject(all, projFilter, allowUnassigned)
			taskui.RenderTaskListNextActions(out, tasks, len(scopedPool))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Project, tui.FlagNameProject, "", "Filter tasks by repository project identifier (defaults to active repo)")
	cmd.Flags().BoolVarP(&opts.AllProjects, tui.FlagNameAllProjects, "A", false, "Include tasks across all repositories on workstation")
	cmd.Flags().StringVarP(&opts.Status, tui.FlagNameStatus, "s", "", "Filter by status (TODO, PLAN, EDIT, REVIEW, DONE, ABORTED; comma-separated)")
	cmd.Flags().BoolVar(&opts.Pending, tui.FlagNamePending, false, "Show only unfinished tasks (excludes DONE and ABORTED)")
	cmd.Flags().StringVarP(&opts.Plane, tui.FlagNamePlane, "p", "", "Filter tasks by plane (intent or substrate)")
	cmd.Flags().StringVarP(&opts.Tier, tui.FlagNameTier, "t", "", "Filter tasks by cognitive tier (T1 or T2)")
	cmd.Flags().IntVarP(&opts.Limit, tui.FlagNameLimit, "n", 0, "Maximum number of tasks to return (0 = all)")
	cmd.Flags().BoolVar(&opts.JSONOut, tui.FlagNameJSON, false, "Emit task list as structured JSON")
	return cmd
}

func renderTaskList(out io.Writer, tasks []task.TaskRecord) {
	fmt.Fprintln(out, tui.RenderBrandTitle("📋 Nomos Living Ledger Tasks"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	fmt.Fprintf(out, "  %s %s %s %s %s %s\n",
		tui.PadStyled("KEY", 10, tui.UiHeaderStyle),
		tui.PadStyled("PLANE", 10, tui.UiHeaderStyle),
		tui.PadStyled("STATUS", 10, tui.UiHeaderStyle),
		tui.PadStyled("TIER", 5, tui.UiHeaderStyle),
		tui.PadStyled("PRIO", 5, tui.UiHeaderStyle),
		tui.UiHeaderStyle.Render("TITLE"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	if len(tasks) == 0 {
		fmt.Fprintf(out, "  %s\n", tui.UiMutedStyle.Render("(No matching tasks in Living Ledger)"))
	}
	for _, t := range tasks {
		fmt.Fprintf(out, "  %s %s %s %s %s %s\n",
			tui.StyleTaskKeyWidth(t.Key, 10),
			tui.StylePlaneWidth(t.Plane, 10),
			tui.StyleSemanticBadgeWidth(string(t.Status), 10),
			tui.PadStyled(string(t.Tier), 5, tui.UiMutedStyle),
			tui.PadStyled(fmt.Sprintf("%d", t.Priority), 5, tui.UiLabelStyle),
			tui.UiValueStyle.Render(t.Title))
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
}

func newTaskDAGCmd() *cobra.Command {
	var opts taskui.TaskDAGFilterOpts

	cmd := &cobra.Command{
		Use:     "dag",
		Short:   "Display causal DAG wavefront of unblocked candidate tasks",
		Long:    tui.HelpTaskDAGLong,
		Example: tui.HelpTaskDAGExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			wf, err := store.GetWavefront()
			if err != nil {
				return err
			}
			filtered, fErr := taskui.ApplyWavefrontFilters(wf, store, opts)
			if fErr != nil {
				return fErr
			}
			out := cmd.OutOrStdout()
			if opts.JSONOut {
				data, mErr := json.MarshalIndent(filtered, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling wavefront JSON: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderWavefront(out, filtered)
			taskui.RenderWavefrontNextActions(out, filtered)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Project, tui.FlagNameProject, "", "Filter wavefront candidates by repository project identifier (defaults to active repo)")
	cmd.Flags().BoolVarP(&opts.AllProjects, tui.FlagNameAllProjects, "A", false, "Include wavefront candidates across all repositories on workstation")
	cmd.Flags().StringVarP(&opts.Plane, tui.FlagNamePlane, "p", "", "Filter wavefront candidates by plane (intent or substrate)")
	cmd.Flags().StringVarP(&opts.Tier, tui.FlagNameTier, "t", "", "Filter wavefront candidates by cognitive tier (T1 or T2)")
	cmd.Flags().IntVarP(&opts.Limit, tui.FlagNameLimit, "n", 0, "Maximum wavefront candidates to display (0 = all)")
	cmd.Flags().BoolVar(&opts.JSONOut, tui.FlagNameJSON, false, "Emit wavefront as structured JSON")
	return cmd
}

func renderWavefront(out io.Writer, wf []task.WavefrontCandidate) {
	fmt.Fprintln(out, tui.RenderBrandTitle("🌊 Nomos Causal DAG Wavefront (Unblocked Ready Tasks)"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	if len(wf) == 0 {
		fmt.Fprintf(out, "  %s\n", tui.UiMutedStyle.Render("(No pending unblocked tasks in current wavefront)"))
	}
	for _, cand := range wf {
		dot := tui.UiElectricCyanStyle.Render("◈")
		if strings.HasPrefix(strings.ToUpper(cand.Key), "INT-") {
			dot = tui.UiIntentStyle.Render("◈")
		}
		fmt.Fprintf(out, "  %s [%s] %s %s\n",
			dot,
			tui.StyleTaskKey(cand.Key),
			tui.UiBoldValueStyle.Render(cand.Title),
			tui.UiMutedStyle.Render(fmt.Sprintf("(%s, %s)", cand.Plane, cand.Tier)))
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
}

func newTaskStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Display active workspace phase and CapBAC token allocation",
		Long:    tui.HelpTaskStatusLong,
		Example: tui.HelpTaskStatusExample,
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
			fmt.Fprintln(out, tui.RenderBrandTitle("📋 Nomos Workspace Phase State"))
			fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
			fmt.Fprintln(out, tui.KvLine("Active Phase", 15, tui.StyleSemanticBadge(string(ps.Phase))))
			if ps.TaskID != "" {
				fmt.Fprintln(out, tui.KvLine("Active Task", 15, tui.StyleTaskKey(ps.TaskID)))
			}
			fmt.Fprintln(out, tui.KvLine("Assignee", 15, tui.UiValueStyle.Render(ps.Assignee)))
			fmt.Fprintln(out, tui.KvLine("Step Budget", 15, tui.UiBoldValueStyle.Render(fmt.Sprintf("%d", ps.StepBudget))))
			fmt.Fprintln(out, tui.KvLine("Tokens Consumed", 15, tui.UiValueStyle.Render(fmt.Sprintf("%d", ps.TokensUsed))))
			tui.RenderNextActions(out, ps.NextActions)
			fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit phase state as structured JSON")
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
		Long:    tui.HelpTaskShowLong,
		Example: tui.HelpTaskShowExample,
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
			tui.RenderTaskShowCard(out, rec)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit task inspection card as structured JSON")
	return cmd
}

type taskPairOpts struct {
	tier     string
	priority int
	card     tui.TaskCardFlags
	stdin    io.Reader
}

func newTaskPairCmd() *cobra.Command {
	var opts taskPairOpts

	cmd := &cobra.Command{
		Use:     "pair <title>",
		Short:   "Atomically allocate twinned INT (Intent) and SUB (Substrate) tasks",
		Long:    tui.HelpTaskPairLong,
		Example: tui.HelpTaskPairExample,
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
	cmd.Flags().StringVarP(&opts.tier, tui.FlagNameTier, "t", "T1", "Cognitive tier (T1|T2)")
	cmd.Flags().IntVarP(&opts.priority, "priority", "r", 50, "Priority (0-100)")
	tui.BindTaskCardFlags(cmd, &opts.card, true)
	return cmd
}

func executeTaskPair(store task.TaskStore, title string, opts taskPairOpts) error {
	if err := tui.MaterializeCardStdin(opts.stdin, &opts.card); err != nil {
		return tui.AttachCardRemedy(err, "pair", title, "substrate")
	}
	intBody, intBurden, err := tui.ResolveCardBodyAndBurden(nil, "pair", title, "intent", opts.card, opts.card.IntScope)
	if err != nil {
		return err
	}
	subBody, subBurden, err := tui.ResolveCardBodyAndBurden(nil, "pair", title, "substrate", opts.card, opts.card.SubScope)
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
	deps, err := tui.ResolveValidatedDependencies(store, opts.card.DependsOn, intKey, subKey)
	if err != nil {
		return err
	}

	project := taskui.ResolveActiveTaskProject(opts.card.Project)
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
	fmt.Println(tui.RenderBrandTitle("🔗 Nomos Atomic Twin Provisioning Complete"))
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	fmt.Println(tui.KvLine("Intent Task", 14, fmt.Sprintf("%s (%s)", tui.StyleTaskKey(intKey), tui.UiValueStyle.Render(title))))
	fmt.Println(tui.KvLine("Substrate Task", 14, fmt.Sprintf("%s (%s)", tui.StyleTaskKey(subKey), tui.UiValueStyle.Render(title))))
	fmt.Println(tui.KvLine("Bilateral Link", 14, fmt.Sprintf("%s %s %s",
		tui.StyleTaskKey(intKey), tui.UiMutedStyle.Render("<─── twin ───>"), tui.StyleTaskKey(subKey))))
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	tui.RenderNextActions(os.Stdout, []types.ActionAffordance{
		{Command: fmt.Sprintf("nomos task start %s --edit", intKey), Summary: "Start Intent twin in isolated EDIT worktree"},
		{Command: fmt.Sprintf("nomos task start %s --edit", subKey), Summary: "Start Substrate twin in isolated EDIT worktree"},
	})
}
