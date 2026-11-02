// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
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
	var jsonOutput bool
	var projectFilter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered Living Ledger task records",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			tasks := filterTasksByProject(store.ListTasks(), projectFilter)
			if jsonOutput {
				data, err := json.MarshalIndent(tasks, "", "  ")
				if err != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling tasks JSON: %v", err)
				}
				fmt.Println(string(data))
				return nil
			}
			renderTaskList(tasks)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectFilter, "project", "", "Filter tasks by repository project identifier")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit task list as structured JSON")
	return cmd
}

func renderTaskList(tasks []task.TaskRecord) {
	fmt.Println(renderBrandTitle("📋 Nomos Living Ledger Tasks"))
	fmt.Println(uiRule(ruleStandard))
	fmt.Printf("  %s %s %s %s %s %s\n",
		padStyled("KEY", 10, uiHeaderStyle),
		padStyled("PLANE", 10, uiHeaderStyle),
		padStyled("STATUS", 10, uiHeaderStyle),
		padStyled("TIER", 5, uiHeaderStyle),
		padStyled("PRIO", 5, uiHeaderStyle),
		uiHeaderStyle.Render("TITLE"))
	fmt.Println(uiRule(ruleStandard))
	if len(tasks) == 0 {
		fmt.Printf("  %s\n", uiMutedStyle.Render("(No tasks registered in Living Ledger)"))
	}
	for _, t := range tasks {
		fmt.Printf("  %s %s %s %s %s %s\n",
			styleTaskKeyWidth(t.Key, 10),
			stylePlaneWidth(t.Plane, 10),
			styleSemanticBadgeWidth(string(t.Status), 10),
			padStyled(string(t.Tier), 5, uiMutedStyle),
			padStyled(fmt.Sprintf("%d", t.Priority), 5, uiLabelStyle),
			uiValueStyle.Render(t.Title))
	}
	fmt.Println(uiRule(ruleStandard))
}

func newTaskDAGCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "dag",
		Short: "Display causal DAG wavefront of unblocked candidate tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			wf, err := store.GetWavefront()
			if err != nil {
				return err
			}
			if jsonOutput {
				data, err := json.MarshalIndent(wf, "", "  ")
				if err != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling wavefront JSON: %v", err)
				}
				fmt.Println(string(data))
				return nil
			}
			renderWavefront(wf)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit wavefront as structured JSON")
	return cmd
}

func renderWavefront(wf []task.WavefrontCandidate) {
	fmt.Println(renderBrandTitle("🌊 Nomos Causal DAG Wavefront (Unblocked Ready Tasks)"))
	fmt.Println(uiRule(ruleStandard))
	if len(wf) == 0 {
		fmt.Printf("  %s\n", uiMutedStyle.Render("(No pending unblocked tasks in current wavefront)"))
	}
	for _, cand := range wf {
		dot := uiElectricCyanStyle.Render("◈")
		if strings.HasPrefix(strings.ToUpper(cand.Key), "INT-") {
			dot = uiIntentStyle.Render("◈")
		}
		fmt.Printf("  %s [%s] %s %s\n",
			dot,
			styleTaskKey(cand.Key),
			uiBoldValueStyle.Render(cand.Title),
			uiMutedStyle.Render(fmt.Sprintf("(%s, %s)", cand.Plane, cand.Tier)))
	}
	fmt.Println(uiRule(ruleStandard))
}

func newTaskStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Display active workspace phase and CapBAC token allocation",
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := ps.Validate(); err != nil {
				return err
			}
			fmt.Println(renderBrandTitle("📋 Nomos Workspace Phase State"))
			fmt.Println(uiRule(ruleNarrow))
			fmt.Println(kvLine("Active Phase", 15, styleSemanticBadge(string(ps.Phase))))
			if ps.TaskID != "" {
				fmt.Println(kvLine("Active Task", 15, styleTaskKey(ps.TaskID)))
			}
			fmt.Println(kvLine("Assignee", 15, uiValueStyle.Render(ps.Assignee)))
			fmt.Println(kvLine("Step Budget", 15, uiBoldValueStyle.Render(fmt.Sprintf("%d", ps.StepBudget))))
			fmt.Println(kvLine("Tokens Consumed", 15, uiValueStyle.Render(fmt.Sprintf("%d", ps.TokensUsed))))
			renderNextActions(cmd.OutOrStdout(), ps.NextActions)
			fmt.Println(uiRule(ruleNarrow))
			return nil
		},
	}
}

func newTaskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <key>",
		Short: "Inspect Living Ledger task card and twin status",
		Args:  cobra.ExactArgs(1),
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
			renderTaskShowCard(cmd.OutOrStdout(), rec)
			return nil
		},
	}
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
		Use:   "pair <title>",
		Short: "Atomically allocate twinned INT (Intent) and SUB (Substrate) tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.stdin = cmd.InOrStdin()
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			return executeTaskPair(store, args[0], opts)
		},
	}
	cmd.Flags().StringVarP(&opts.tier, "tier", "t", "T1", "Cognitive tier (T1|T2)")
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

	fmt.Println(renderBrandTitle("🔗 Nomos Atomic Twin Provisioning Complete"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Intent Task", 14, fmt.Sprintf("%s (%s)", styleTaskKey(intKey), uiValueStyle.Render(title))))
	fmt.Println(kvLine("Substrate Task", 14, fmt.Sprintf("%s (%s)", styleTaskKey(subKey), uiValueStyle.Render(title))))
	fmt.Println(kvLine("Bilateral Link", 14, fmt.Sprintf("%s %s %s",
		styleTaskKey(intKey), uiMutedStyle.Render("<─── twin ───>"), styleTaskKey(subKey))))
	fmt.Println(uiRule(ruleNarrow))
	return nil
}

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
