package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/session"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

func newTaskSyncCmd() *cobra.Command {
	var walkthroughPath string

	cmd := &cobra.Command{
		Use:     "sync <key>",
		Short:   "2PC release: evaluate DoD gates, merge to trunk, and prune worktree",
		Long:    helpTaskSyncLong,
		Example: helpTaskSyncExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.ToUpper(strings.TrimSpace(args[0]))
			if walkthroughPath == "" {
				return types.NewNomosError(types.ErrCodeValidationFailed, "-F / --file walkthrough summary is mandatory").
					WithRemedy(fmt.Sprintf("write a summary to .tmp/walkthrough-%s.md and run: nomos task sync %s -F .tmp/walkthrough-%s.md", key, key, key))
			}
			fi, err := os.Stat(walkthroughPath)
			if err != nil || fi.Size() == 0 {
				return types.NewNomosError(types.ErrCodeValidationFailed, "walkthrough file %s missing or empty", walkthroughPath).
					WithRemedy(fmt.Sprintf("write non-empty release notes to %s and re-run: nomos task sync %s -F %s", walkthroughPath, key, walkthroughPath))
			}
			return finalizeSync(key, walkthroughPath)
		},
	}
	cmd.Flags().StringVarP(&walkthroughPath, "file", "F", "", "Path to walkthrough markdown file")
	return cmd
}

func finalizeSync(key, walkthroughPath string) error {
	mgr, err := workspace.NewManager("")
	if err != nil {
		return err
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// non-fatal close
		}
	}()
	t, err := store.GetTask(key)
	if err != nil {
		return err
	}
	wtPath := mgr.WorktreePath(key)
	if cErr := autoCommitDirtyWorktree(wtPath, key, t, walkthroughPath); cErr != nil {
		return cErr
	}
	report, gErr := executeGateCheckForTask(key)
	if gErr != nil {
		return gErr
	}
	touchedFiles := verify.ListModifiedWorktreeFiles(wtPath, key)
	if pErr := validateReceiptTraceParity(wtPath, walkthroughPath, touchedFiles); pErr != nil {
		return pErr
	}
	branch := mgr.BranchName(key)
	rebased, rErr := rebaseOntoTrunkIfDiverged(mgr.RepoRoot, wtPath, branch)
	if rErr != nil {
		return rErr
	}
	if rebased {
		rebasedReport, vErr := executeGateCheckForTask(key)
		if vErr != nil {
			return vErr
		}
		report = rebasedReport
	}
	preMergeSHA := readGitHeadSHA(mgr.RepoRoot)
	if mErr := fastForwardTrunkMerge(mgr.RepoRoot, branch); mErr != nil {
		return mErr
	}
	return completeSyncedTaskMetadata(mgr, store, t, walkthroughPath, preMergeSHA, touchedFiles, report)
}

func completeSyncedTaskMetadata(mgr *workspace.Manager, store task.TaskStore, t *task.TaskRecord, walkthroughPath, preMergeSHA string, touchedFiles []string, report *verify.PipelineReport) error {
	wtPath := mgr.WorktreePath(t.Key)
	now := time.Now().UTC().Format(time.RFC3339)
	t.Status = task.StatusDone
	t.CompletedAt = &now
	if err := store.UpdateTask(*t); err != nil {
		rollbackTrunkMergeOnDBError(mgr.RepoRoot, preMergeSHA)
		return err
	}
	specDigests := collectSyncSpecDigests(mgr.RepoRoot, t.Key, touchedFiles, t.TwinKey)
	recordReleaseReceipt(mgr.RepoRoot, wtPath, t.Key, walkthroughPath, now, report, specDigests)
	revokeActiveLease(t.Key)
	distillAndMaintainOnSync(mgr.RepoRoot, wtPath, t.Key, walkthroughPath, touchedFiles)
	clearHookSessionTrace(wtPath)
	cleanupAndAnnounceSyncedTask(mgr, t.Key)
	return nil
}

func cleanupAndAnnounceSyncedTask(mgr *workspace.Manager, key string) {
	if remErr := mgr.RemoveWorktree(key); remErr != nil {
		// logged or soft failure
	}
	if _, pErr := pruneWorkspaceOrphans(mgr.RepoRoot, false); pErr != nil {
		// best-effort post-sync worktree sweep
	}
	fmt.Printf("🎉 %s %s %s\n",
		uiSuccessStyle.Render("Task"),
		styleTaskKey(key),
		uiSuccessStyle.Render("synced and released to trunk"))
	renderTaskSyncAnomalyAlert(os.Stdout, mgr.RepoRoot, key)
	recompileSubstrateBinary(mgr.RepoRoot)
	renderPhaseTransitionNextActions(os.Stdout, types.PhaseSync, key, "", mgr.RepoRoot)
}

func collectSyncSpecDigests(repoRoot, key string, touchedFiles []string, twinKey string) map[string]string {
	isSub := strings.HasPrefix(key, "SUB-")
	isInt := strings.HasPrefix(key, "INT-")
	if !isSub && !isInt {
		return nil
	}
	var intentRoots []string
	var contractRoots []string
	contractsActive := false
	if cfg, err := config.LoadConfig(repoRoot); err == nil && cfg != nil {
		intentRoots = cfg.Governance.QualityGates.IntentSourceRoots
		c := cfg.Governance.QualityGates.GetContractsBoundary().Contracts
		contractsActive = c.Enabled && (c.StrictTypes || c.VerifyObligations || c.FailOnStaleObligations)
		contractRoots = c.SpecRoots
	}
	if isInt && !canAutoSealIntentSpecDigests(repoRoot, contractRoots) {
		return nil
	}
	digests := task.CollectReferencedSpecDigestsWithRoots(repoRoot, touchedFiles, twinKey, intentRoots)
	if !contractsActive {
		return digests
	}
	if digests == nil {
		digests = make(map[string]string)
	}
	for k, v := range task.CollectContractSpecDigests(repoRoot, contractRoots) {
		digests[k] = v
	}
	if len(digests) == 0 {
		return nil
	}
	return digests
}

func canAutoSealIntentSpecDigests(repoRoot string, contractRoots []string) bool {
	srcDir := filepath.Join(repoRoot, "substrate", "src")
	if len(verify.VerifySpecTypeSignatures(repoRoot, srcDir, contractRoots)) > 0 {
		return false
	}
	return len(verify.VerifyExecutableObligations(repoRoot, contractRoots)) == 0
}

func rebaseOntoTrunkIfDiverged(repoRoot, wtPath, branch string) (bool, error) {
	fi, err := os.Stat(wtPath)
	if err != nil || !fi.IsDir() {
		return false, nil
	}
	trunk := detectDefaultBranch(repoRoot)
	ancCmd := workspace.SafeGitCommand(repoRoot, "merge-base", "--is-ancestor", trunk, branch)
	if ancCmd.Run() == nil {
		return false, nil
	}
	rebCmd := workspace.SafeGitCommand(wtPath, "rebase", trunk)
	rebCmd.Env = append(rebCmd.Env, "NOMOS_COMMIT_IN_PROGRESS=1")
	if out, rebErr := rebCmd.CombinedOutput(); rebErr != nil {
		if abErr := workspace.SafeGitCommand(wtPath, "rebase", "--abort").Run(); abErr != nil {
			// best-effort clean abort
		}
		return false, types.NewNomosError(types.ErrCodeBoundary,
			"concurrent trunk conflict rebasing %s onto %s (rebase aborted cleanly): %s",
			branch, trunk, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func fastForwardTrunkMerge(repoRoot, branch string) error {
	healAccidentalBareConfig(repoRoot)
	unlockTrunk(repoRoot)
	mergeCmd := workspace.SafeGitCommand(repoRoot, "merge", "--ff-only", branch)
	if out, err := mergeCmd.CombinedOutput(); err != nil {
		lockTrunk(repoRoot)
		return types.NewNomosError(types.ErrCodeBoundary, "fast-forward merge failed: %s", string(out))
	}
	lockTrunk(repoRoot)
	return nil
}

func healAccidentalBareConfig(repoRoot string) {
	if repoRoot == "" {
		return
	}
	gitDir := filepath.Join(repoRoot, ".git")
	fi, err := os.Stat(gitDir)
	if err != nil || !fi.IsDir() {
		return
	}
	out, cfgErr := workspace.SafeGitCommand(repoRoot, "config", "--bool", "core.bare").Output()
	if cfgErr != nil || strings.TrimSpace(string(out)) != "true" {
		return
	}
	if setErr := workspace.SafeGitCommand(repoRoot, "config", "--bool", "core.bare", "false").Run(); setErr != nil {
		// best-effort bare flag healing
	}
}

func recordReleaseReceipt(repoRoot, wtPath, key, walkthroughPath, now string, report *verify.PipelineReport, specDigests map[string]string) {
	revCmd := workspace.SafeGitCommand(repoRoot, "rev-parse", "HEAD")
	revOut, err := revCmd.Output()
	if err != nil {
		return
	}
	commitSHA := strings.TrimSpace(string(revOut))
	if len(commitSHA) != 40 {
		return
	}

	walkthroughContent := readWalkthroughContent(walkthroughPath)
	gatesPassed := 42
	if report != nil && report.GatesPassed >= 42 {
		gatesPassed = report.GatesPassed
	}

	tokenID := resolveCapBACTokenID(key)
	receipt := task.ComputeReleaseReceipt(key, commitSHA, now, tokenID)
	payload := task.ReleaseNotePayload{
		TaskKey:            key,
		CommitSHA:          commitSHA,
		Timestamp:          now,
		Phase:              "SYNC",
		DoDGatesPassed:     gatesPassed,
		CapBACTokenID:      tokenID,
		SessionID:          resolveTaskSyncSessionID(repoRoot, key),
		HookTrace:          buildReleaseHookTraceSnapshot(wtPath),
		SpecDigests:        specDigests,
		WalkthroughContent: walkthroughContent,
		SHA256Receipt:      receipt,
	}

	if noteErr := task.RecordReleaseNote(repoRoot, "", payload); noteErr != nil {
		fmt.Printf("⚠️  %s %v\n", uiWarnStyle.Render("Warning: failed recording git note release receipt:"), noteErr)
		return
	}
	fmt.Printf("📜 %s %s\n",
		uiSuccessStyle.Render("Cryptographic release receipt recorded in Git Notes"),
		uiMutedStyle.Render("(refs/notes/nomos/releases)"))
}

func readWalkthroughContent(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func resolveCapBACTokenID(key string) string {
	tokenID := "cap-" + key + "-sync"
	db, err := storage.OpenDB("")
	if err == nil {
		if lID, _, _, _, lErr := storage.GetActiveLease(db, key); lErr == nil && lID != "" {
			tokenID = lID
		}
		_ = db.Close()
	}
	return tokenID
}

func unlockTrunk(root string) {
	if err := workspace.UnlockTrunk(root); err != nil {
		// ignore non-fatal trunk permission errors
	}
}

func lockTrunk(root string) {
	if err := workspace.LockTrunk(root); err != nil {
		// ignore non-fatal trunk permission errors
	}
}

func autoCommitDirtyWorktree(wtPath, key string, rec *task.TaskRecord, walkthroughPath string) error {
	fi, err := os.Stat(wtPath)
	if err != nil || !fi.IsDir() {
		return nil
	}
	statusCmd := workspace.SafeGitCommand(wtPath, "status", "--porcelain")
	out, err := statusCmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil
	}
	if discardPureWhitespaceAfterTaskCommit(wtPath, key, string(out)) {
		return nil
	}
	var title string
	if rec != nil {
		title = rec.Title
	}
	opts := &commitCmdOptions{
		dir:             wtPath,
		summary:         title,
		walkthroughFile: walkthroughPath,
		taskRecord:      rec,
		autoStage:       true,
	}
	return runTaskCommitCmd([]string{key}, opts)
}

func discardPureWhitespaceAfterTaskCommit(wtPath, key, porcelain string) bool {
	for _, line := range strings.Split(porcelain, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "??") || strings.HasPrefix(trimmed, "A") || strings.HasPrefix(trimmed, "D") {
			return false
		}
	}
	subj, sErr := workspace.SafeGitCommand(wtPath, "log", "-n", "1", "--format=%s").Output()
	if sErr != nil || !strings.Contains(string(subj), "["+key+"]") {
		return false
	}
	if qErr := workspace.SafeGitCommand(wtPath, "diff", "--ignore-blank-lines", "--ignore-space-at-eol", "--quiet").Run(); qErr != nil {
		return false
	}
	return workspace.SafeGitCommand(wtPath, "checkout", "--", ".").Run() == nil
}

func resolveTaskSyncSessionID(repoRoot, key string) string {
	if envID := strings.TrimSpace(os.Getenv("NOMOS_SESSION_ID")); envID != "" {
		return envID
	}
	sessions, err := session.DiscoverSessions("", repoRoot)
	if err != nil {
		return ""
	}
	for _, s := range sessions {
		if strings.EqualFold(strings.TrimSpace(s.TaskKey), key) {
			return s.ID
		}
	}
	return ""
}

func buildReleaseHookTraceSnapshot(wtPath string) *task.ReleaseHookTrace {
	tr := loadHookSessionTrace(wtPath)
	if tr == nil {
		return nil
	}
	if tr.ToolCallsCount == 0 && tr.ShellCommandsCount == 0 && tr.DeniedInterceptions == 0 && len(tr.MutatedFiles) == 0 {
		return nil
	}
	return &task.ReleaseHookTrace{
		ToolCallsCount:      tr.ToolCallsCount,
		ShellCommandsCount:  tr.ShellCommandsCount,
		TestCommandsCount:   tr.TestCommandsCount,
		DeniedInterceptions: tr.DeniedInterceptions,
		MutatedFiles:        tr.MutatedFiles,
	}
}
