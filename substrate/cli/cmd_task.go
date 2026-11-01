// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
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

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered Living Ledger task records",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			tasks := store.ListTasks()
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
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit task list as structured JSON")
	return cmd
}

func renderTaskList(tasks []task.TaskRecord) {
	fmt.Println("📋 Nomos Living Ledger Tasks")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("  %-10s %-10s %-10s %-5s %-5s %s\n", "KEY", "PLANE", "STATUS", "TIER", "PRIO", "TITLE")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	if len(tasks) == 0 {
		fmt.Println("  (No tasks registered in Living Ledger)")
	}
	for _, t := range tasks {
		fmt.Printf("  %-10s %-10s %-10s %-5s %-5d %s\n",
			t.Key, t.Plane, t.Status, t.Tier, t.Priority, t.Title)
	}
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
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
	fmt.Println("🌊 Nomos Causal DAG Wavefront (Unblocked Ready Tasks)")
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
	if len(wf) == 0 {
		fmt.Println("  (No pending unblocked tasks in current wavefront)")
	}
	for _, cand := range wf {
		fmt.Printf("  • [%s] %s (%s, %s)\n", cand.Key, cand.Title, cand.Plane, cand.Tier)
	}
	fmt.Println("────────────────────────────────────────────────────────────────────────────")
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
			fmt.Println("📋 Nomos Workspace Phase State")
			fmt.Println("────────────────────────────────────────────────────────────")
			fmt.Printf("  Active Phase    : %s\n", ps.Phase)
			fmt.Printf("  Assignee        : %s\n", ps.Assignee)
			fmt.Printf("  Step Budget     : %d\n", ps.StepBudget)
			fmt.Printf("  Tokens Consumed : %d\n", ps.TokensUsed)
			renderNextActions(cmd.OutOrStdout(), ps.NextActions)
			fmt.Println("────────────────────────────────────────────────────────────")
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
			fmt.Print(task.FormatInspectionCard(rec))
			return nil
		},
	}
}

func newTaskPairCmd() *cobra.Command {
	var tier, desc string
	var priority int

	cmd := &cobra.Command{
		Use:   "pair <title>",
		Short: "Atomically allocate twinned INT (Intent) and SUB (Substrate) tasks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := task.NewDefaultSQLiteTaskStore()
			if err != nil {
				return err
			}
			return executeTaskPair(store, args[0], desc, tier, priority)
		},
	}
	cmd.Flags().StringVarP(&tier, "tier", "t", "T1", "Cognitive tier (T1|T2)")
	cmd.Flags().IntVarP(&priority, "priority", "r", 50, "Priority (0-100)")
	cmd.Flags().StringVarP(&desc, "desc", "d", "", "Detailed task description")
	return cmd
}

func executeTaskPair(store task.TaskStore, title, desc, tier string, priority int) error {
	intKey, err := allocateNextKey(store, "intent")
	if err != nil {
		return err
	}
	subKey, err := allocateNextKey(store, "substrate")
	if err != nil {
		return err
	}

	intTask := task.TaskRecord{
		Key: intKey, Plane: "intent", SchemaType: task.SchemaIntent,
		Title: title, Description: desc, Status: task.StatusTodo,
		Tier: task.Tier(tier), Priority: priority, TwinKey: subKey,
	}
	subTask := task.TaskRecord{
		Key: subKey, Plane: "substrate", SchemaType: task.SchemaSubstrate,
		Title: title, Description: desc, Status: task.StatusTodo,
		Tier: task.Tier(tier), Priority: priority, TwinKey: intKey,
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

	fmt.Println("🔗 Nomos Atomic Twin Provisioning Complete")
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Printf("  Intent Task    : %s (%s)\n", intKey, title)
	fmt.Printf("  Substrate Task : %s (%s)\n", subKey, title)
	fmt.Printf("  Bilateral Link : %s <─── twin ───> %s\n", intKey, subKey)
	fmt.Println("────────────────────────────────────────────────────────────")
	return nil
}
