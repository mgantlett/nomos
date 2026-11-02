// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

type commitCmdOptions struct {
	summary         string
	rationale       string
	blastRadius     string
	mechanics       string
	walkthroughFile string
	dir             string
	autoStage       bool
	taskRecord      *task.TaskRecord
}

func newTaskCommitCmd() *cobra.Command {
	opts := &commitCmdOptions{}

	cmd := &cobra.Command{
		Use:   "commit [TASK_KEY]",
		Short: "Execute context-hydrated Dual-Layer Hybrid AI commit adhering to commit defense hook",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTaskCommitCmd(args, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.summary, "message", "m", "", "Commit summary line (normalized to Conventional Commit format)")
	cmd.Flags().StringVar(&opts.rationale, "rationale", "", "Architectural Rationale (defaults to Living Ledger Scope & Context)")
	cmd.Flags().StringVar(&opts.blastRadius, "blast-radius", "", "Blast Radius & Contracts (defaults to staged file manifest)")
	cmd.Flags().StringVar(&opts.mechanics, "mechanics", "", "Resolution Mechanics (defaults to walkthrough or Acceptance Criteria)")
	cmd.Flags().StringVarP(&opts.walkthroughFile, "file", "F", "", "Optional walkthrough markdown file to hydrate commit sections")
	cmd.Flags().StringVarP(&opts.dir, "dir", "d", "", "Working directory")
	cmd.Flags().BoolVarP(&opts.autoStage, "all", "a", true, "Automatically stage all modified files")

	return cmd
}

func runTaskCommitCmd(args []string, opts *commitCmdOptions) error {
	taskKey, err := resolveTaskKeyForCommit(args, opts)
	if err != nil {
		return err
	}

	dir, err := resolveCommitWorkingDir(taskKey, opts)
	if err != nil {
		return err
	}

	if opts.autoStage {
		stageCmd := workspace.SafeGitCommand(dir, "add", "-A")
		if out, sErr := stageCmd.CombinedOutput(); sErr != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed staging files: %s", string(out))
		}
	}

	ctx := buildCommitHydrationContext(taskKey, dir, opts)
	msg := hydrateCommitMessage(ctx, opts)

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
		uiSuccessStyle.Render("Dual-Layer Hybrid Commit Successful for"),
		styleTaskKey(taskKey))
	fmt.Println(uiMutedStyle.Render(strings.TrimSpace(string(out))))
	renderPhaseTransitionNextActions(os.Stdout, types.PhaseReview, taskKey, dir, "")
	return nil
}

func resolveCommitWorkingDir(taskKey string, opts *commitCmdOptions) (string, error) {
	if opts != nil && opts.dir != "" {
		return opts.dir, nil
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

func resolveTaskKeyForCommit(args []string, opts *commitCmdOptions) (string, error) {
	if len(args) > 0 {
		key := strings.ToUpper(strings.TrimSpace(args[0]))
		if err := task.ValidateTaskKey(key); err != nil {
			return "", err
		}
		return key, nil
	}

	dir := ""
	if opts != nil {
		dir = opts.dir
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

func formatCommitMessage(taskKey string, opts *commitCmdOptions) string {
	var dir string
	if opts != nil {
		dir = opts.dir
	}
	ctx := buildCommitHydrationContext(taskKey, dir, opts)
	return hydrateCommitMessage(ctx, opts)
}
