// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/commitmsg"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

func newTaskCommitCmd() *cobra.Command {
	opts := &commitmsg.CommitCmdOptions{}

	cmd := &cobra.Command{
		Use:     "commit [TASK_KEY]",
		Short:   "Record a Golden Conventional Commit with hydrated rationale and Task trailer",
		Long:    tui.HelpTaskCommitLong,
		Example: tui.HelpTaskCommitExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskCommitCmd(args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Summary, "message", "m", "", "Commit summary line (normalized to <=72-rune Conventional Commit subject)")
	cmd.Flags().StringVar(&opts.Rationale, "rationale", "", "Problem context and architectural rationale (defaults to Living Ledger Scope & Context)")
	cmd.Flags().StringVar(&opts.Mechanics, "mechanics", "", "Optional key design mechanics (defaults to deduplicated walkthrough or Acceptance Criteria)")
	cmd.Flags().StringVarP(&opts.WalkthroughFile, "file", "F", "", "Optional walkthrough markdown file to hydrate commit prose")
	cmd.Flags().StringVarP(&opts.Dir, "dir", "d", "", "Working directory")
	cmd.Flags().BoolVarP(&opts.AutoStage, "all", "a", true, "Automatically stage all modified files")

	return cmd
}

func runTaskCommitCmd(args []string, opts *commitmsg.CommitCmdOptions) error {
	taskKey, err := resolveTaskKeyForCommit(args, opts)
	if err != nil {
		return err
	}

	dir, err := resolveCommitWorkingDir(taskKey, opts)
	if err != nil {
		return err
	}

	if opts.AutoStage {
		stageCmd := workspace.SafeGitCommand(dir, "add", "-A")
		if out, sErr := stageCmd.CombinedOutput(); sErr != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed staging files: %s", string(out))
		}
	}

	ctx := commitmsg.BuildCommitHydrationContext(taskKey, dir, opts)
	msg := commitmsg.HydrateCommitMessage(ctx, opts)
	if twinErr := enforcePreCommitIntentTwinMode(dir, taskKey, msg); twinErr != nil {
		return twinErr
	}

	commitCmd := workspace.SafeGitCommand(dir, "commit", "-m", msg)
	commitCmd.Env = append(commitCmd.Env, "NOMOS_COMMIT_IN_PROGRESS=1")

	out, err := commitCmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "Plane Separation Invariant") || strings.Contains(outStr, "Law 7 violation") {
			return types.NewNomosError(types.ErrCodeCrossPlaneMutation, "cross-plane mutation prohibited (Law 7):\n%s", strings.TrimSpace(outStr))
		}
		return types.NewNomosError(types.ErrCodeBoundary, "commit failed: %s", outStr)
	}

	fmt.Printf("✅ %s %s\n",
		tui.UiSuccessStyle.Render("Golden Commit recorded for"),
		tui.StyleTaskKey(taskKey))
	fmt.Println(tui.UiMutedStyle.Render(strings.TrimSpace(string(out))))
	tui.RenderPhaseTransitionNextActions(os.Stdout, types.PhaseReview, taskKey, dir, "")
	return nil
}

func enforcePreCommitIntentTwinMode(dir, taskKey, commitMsg string) error {
	cfgTarget := dir
	if pr, err := workspace.FindRepoRoot(dir); err == nil && pr != "" {
		cfgTarget = pr
	}
	cfg, err := config.LoadConfig(cfgTarget)
	if err != nil || cfg == nil {
		return nil
	}
	mode := cfg.Governance.QualityGates.EffectiveIntentTwinMode()
	if violation := verify.ValidateWorktreeIntentTwin(dir, commitMsg, mode); violation != "" {
		return types.NewNomosError(
			types.ErrCodeBoundary,
			"%s\n  Remedy: %s",
			violation,
			verify.IntentTwinModeRemedy(taskKey),
		)
	}
	return nil
}

func resolveCommitWorkingDir(taskKey string, opts *commitmsg.CommitCmdOptions) (string, error) {
	if opts != nil && opts.Dir != "" {
		return opts.Dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	if !strings.Contains(filepath.ToSlash(dir), "/worktrees/") && taskKey != "" {
		if mgr, mErr := workspace.NewManager(dir); mErr == nil {
			wt := mgr.WorktreePath(taskKey)
			if fi, sErr := os.Stat(wt); sErr == nil && fi.IsDir() {
				dir = wt
			}
		}
	}
	return dir, nil
}

func resolveTaskKeyForCommit(args []string, opts *commitmsg.CommitCmdOptions) (string, error) {
	if len(args) > 0 {
		key := strings.ToUpper(strings.TrimSpace(args[0]))
		if err := task.ValidateTaskKey(key); err != nil {
			return "", err
		}
		return key, nil
	}

	dir := ""
	if opts != nil {
		dir = opts.Dir
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = cwd
	}

	if key := extractTaskKeyFromDirName(filepath.Base(dir)); key != "" {
		return key, nil
	}

	branchCmd := workspace.SafeGitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if out, err := branchCmd.Output(); err == nil {
		branch := strings.TrimSpace(string(out))
		if strings.HasPrefix(branch, "feature/") {
			candKey := strings.TrimPrefix(branch, "feature/")
			if err := task.ValidateTaskKey(candKey); err == nil {
				return candKey, nil
			}
		}
	}

	return "", types.NewNomosError(types.ErrCodeValidationFailed, "cannot resolve active task key; pass TASK_KEY explicitly or run inside a task worktree")
}

func extractTaskKeyFromDirName(base string) string {
	parts := strings.Split(base, "-")
	if len(parts) < 2 {
		return ""
	}
	candKey := strings.ToUpper(parts[len(parts)-2] + "-" + parts[len(parts)-1])
	if err := task.ValidateTaskKey(candKey); err == nil {
		return candKey
	}
	candKeySingle := strings.ToUpper(parts[len(parts)-1])
	if err := task.ValidateTaskKey(candKeySingle); err == nil {
		return candKeySingle
	}
	return ""
}

func formatCommitMessage(taskKey string, opts *commitmsg.CommitCmdOptions) string {
	var dir string
	if opts != nil {
		dir = opts.Dir
	}
	ctx := commitmsg.BuildCommitHydrationContext(taskKey, dir, opts)
	return commitmsg.HydrateCommitMessage(ctx, opts)
}
