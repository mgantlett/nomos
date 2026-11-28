// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/taskui"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/spf13/cobra"
)

type taskChoreOpts struct {
	plane       string
	project     string
	paths       []string
	obligations []string
	jsonOut     bool
}

func newTaskChoreCmd() *cobra.Command {
	var opts taskChoreOpts
	cmd := &cobra.Command{
		Use:     "chore [title]",
		Short:   "Atomically provision a transient task worktree in EDIT phase (1-turn inception)",
		Long:    tui.HelpTaskChoreLong,
		Example: tui.HelpTaskChoreExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := "Maintenance chore"
			if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
				title = strings.TrimSpace(args[0])
			}
			return executeTaskChore(title, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.plane, tui.FlagNamePlane, "p", "substrate", "Target plane (intent|substrate)")
	cmd.Flags().StringVar(&opts.project, tui.FlagNameProject, "", "Repository project identifier (defaults to active repository)")
	cmd.Flags().StringSliceVar(&opts.paths, "paths", nil, "Target file/directory paths or globs for scope_paths")
	cmd.Flags().StringSliceVar(&opts.obligations, "obligations", nil, "Required TC-* compliance vectors bound to this chore")
	cmd.Flags().BoolVar(&opts.jsonOut, tui.FlagNameJSON, false, "Emit structured JSON response")
	return cmd
}

func runTaskChore(title string) error {
	return runTaskChoreWithOpts(title, "substrate", "")
}

func runTaskChoreWithPlane(title, rawPlane string) error {
	return runTaskChoreWithOpts(title, rawPlane, "")
}

func runTaskChoreWithOpts(title, rawPlane, rawProject string) error {
	return executeTaskChore(title, taskChoreOpts{plane: rawPlane, project: rawProject})
}

func executeTaskChore(title string, opts taskChoreOpts) error {
	t, store, err := createInitialChoreRecord(title, opts)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()
	wtPath, repoRoot, err := provisionChoreWorktreeAndLease(t.Key)
	if err != nil {
		return err
	}
	t.Status = task.StatusEdit
	if uErr := store.UpdateTask(*t); uErr != nil {
		return uErr
	}
	if opts.jsonOut {
		return emitStructuredJSON(os.Stdout, task.BuildInspectionRecord(*t, nil, nil, wtPath, "feature/"+t.Key))
	}
	tui.RenderTaskStartedBanner(t.Key, wtPath, "EDIT")
	renderTurnZeroExecutionBrief(os.Stdout, t)
	renderLearnedPitfallsBriefing(repoRoot, t.Title, t.ScopePaths)
	tui.RenderPhaseTransitionNextActions(os.Stdout, types.PhaseEdit, t.Key, wtPath, repoRoot)
	return nil
}

func createInitialChoreRecord(title string, opts taskChoreOpts) (*task.TaskRecord, *task.SQLiteTaskStore, error) {
	plane := tui.NormalizePlaneInput(opts.plane, "")
	if plane != "intent" && plane != "substrate" {
		return nil, nil, types.NewNomosError(types.ErrCodeValidationFailed, "invalid plane %q: must be intent or substrate", opts.plane)
	}
	paths := task.NormalizeScopePaths(opts.paths)
	obligations, oErr := task.NormalizeObligations(opts.obligations)
	if oErr != nil {
		return nil, nil, oErr
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return nil, nil, err
	}
	key, err := allocateNextKey(store, plane)
	if err != nil {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
		return nil, nil, err
	}
	t := task.TaskRecord{
		Key: key, Plane: plane, Project: taskui.ResolveActiveTaskProject(opts.project),
		SchemaType: task.SchemaType(plane), Title: title, Description: "Atomic 1-turn chore worktree",
		ScopePaths: paths, Obligations: obligations, Status: task.StatusTodo,
		Tier: task.Tier("T1"), Priority: 50, Burden: 1,
	}
	if cErr := store.CreateTask(t); cErr != nil {
		if clErr := store.Close(); clErr != nil {
			// closed
		}
		return nil, nil, cErr
	}
	return &t, store, nil
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
	allocateWorktreeMTPO(mgr.RepoRoot, key, wtPath)
	return wtPath, mgr.RepoRoot, nil
}

func allocateWorktreeMTPO(repoRoot, key, wtPath string) {
	baseSHA := readGitHeadSHA(repoRoot)
	if _, err := mtpo.AllocateSequence("", repoRoot, key, wtPath, baseSHA); err != nil {
		return
	}
}

func rebaseAndVerifySyncedTask(repoRoot, wtPath, branch, key string) (*verify.PipelineReport, error) {
	rebased, rErr := rebaseOntoTrunkIfDiverged(repoRoot, wtPath, branch)
	if rErr != nil {
		return nil, rErr
	}
	if rebased {
		invalidateWorktreeVerifyReceipt(repoRoot, wtPath)
	}
	report, gErr := executeGateCheckForTask(key)
	if gErr != nil {
		return nil, gErr
	}
	if uErr := mtpo.UpdateBaseCommit("", repoRoot, key, readGitHeadSHA(repoRoot)); uErr != nil {
		// best-effort ledger refresh after verified fast-forward ancestry on trunk
	}
	return report, nil
}

func mergeTrunkWithMTPOGuard(repoRoot, branch, key string) (string, error) {
	preMergeSHA := readGitHeadSHA(repoRoot)
	verdict, vErr := mtpo.VerifySyncSerializability("", repoRoot, key, preMergeSHA)
	if vErr == nil && verdict != nil && !verdict.Passed {
		return "", types.NewNomosError(types.ErrCodeInvariant, "MTPO cross-worktree AST serializability conflict for %s", key).
			WithRemedy(verdict.Remedy)
	}
	if mErr := fastForwardTrunkMerge(repoRoot, branch); mErr != nil {
		return "", mErr
	}
	return preMergeSHA, nil
}

func completeMTPOSync(repoRoot, key string) {
	mergedSHA := readGitHeadSHA(repoRoot)
	if err := mtpo.CompleteSequence("", repoRoot, key, mtpo.StatusCommitted, mergedSHA); err != nil {
		return
	}
}

func renderLearnedPitfallsBriefing(repoRoot, taskTitle string, scopePaths []string) {
	axioms := mem.RecallRepoAxiomsReadOnly(repoRoot, taskTitle, scopePaths, 2)
	for i, ax := range axioms {
		if i == 0 {
			fmt.Printf("  %s\n", tui.UiHeaderStyle.Render(fmt.Sprintf("Repo Axioms (%d active):", len(axioms))))
		}
		fmt.Printf("    🏛️  %s\n", tui.UiValueStyle.Render(ax))
	}
	pitfalls := mem.RecallRepoPitfallsReadOnly(repoRoot, scopePaths, 3)
	for i, p := range pitfalls {
		if i == 0 {
			fmt.Printf("  %s\n", tui.UiWarnStyle.Render(fmt.Sprintf("Learned Pitfalls (%d active):", len(pitfalls))))
		}
		fmt.Printf("    ⚡ %s\n", tui.UiValueStyle.Render(p))
	}
	patterns := mem.RecallRepoPatternsReadOnly(repoRoot, taskTitle, scopePaths, 2)
	for i, pat := range patterns {
		if i == 0 {
			fmt.Printf("  %s\n", tui.UiHeaderStyle.Render(fmt.Sprintf("Verified Patterns (%d active):", len(patterns))))
		}
		fmt.Printf("    🧩 %s\n", tui.UiMutedStyle.Render(pat))
	}
}

func recordFailedReportTraps(repoRoot, worktreePath, taskKey string, report *verify.PipelineReport) {
	if report == nil || report.Passed || strings.TrimSpace(taskKey) == "" {
		return
	}
	modified := impact.ListModifiedWorktreeFiles(worktreePath, taskKey)
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
