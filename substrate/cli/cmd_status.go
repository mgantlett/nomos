// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

// StatusReport models the unified status snapshot for machine and human consumption.
type StatusReport struct {
	Phase            string                   `json:"phase"`
	Project          string                   `json:"project,omitempty"`
	RepoRoot         string                   `json:"repo_root"`
	ActiveBranch     string                   `json:"active_branch"`
	IsWorktree       bool                     `json:"is_worktree"`
	DatabasePath     string                   `json:"database_path"`
	DatabaseStatus   string                   `json:"database_status"`
	TotalTasks       int                      `json:"total_tasks"`
	ActiveTasks      int                      `json:"active_tasks"`
	PendingTasks     int                      `json:"pending_tasks"`
	WavefrontCount   int                      `json:"wavefront_count"`
	RemoteAheadCount int                      `json:"remote_ahead_count"`
	NextActions      []types.ActionAffordance `json:"next_actions,omitempty"`
}

// NewStatusCmd constructs the top-level 'nomos status' command.
func NewStatusCmd() *cobra.Command {
	var jsonOutput, allProjects bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display unified operational status, phase state, and task wavefront",
		Long: `Emit a comprehensive status report covering active workspace lifecycle phase,
trunk branch, database connectivity, and repository-scoped Living Ledger task wavefront.`,
		Example: helpStatusExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := collectStatusReportWithScope(allProjects)
			out := cmd.OutOrStdout()
			if jsonOutput {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling status JSON: %v", err)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderStatusReport(out, report)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit status as structured JSON")
	cmd.Flags().BoolVarP(&allProjects, flagNameAllProjects, "A", false, "Aggregate Living Ledger metrics across all workstation repositories")
	return cmd
}

func collectStatusReport() *StatusReport {
	return collectStatusReportWithScope(false)
}

func collectStatusReportWithScope(allProjects bool) *StatusReport {
	repoRoot, err := workspace.FindRepoRoot("")
	if err != nil {
		repoRoot = "."
	}

	treeRoot, tErr := workspace.FindWorkingTreeRoot("")
	if tErr != nil {
		treeRoot = repoRoot
	}

	activeBranch := resolveBranchName(treeRoot)
	isWorktree := strings.Contains(treeRoot, "worktrees")
	project := task.ResolveRepoProject(repoRoot)
	if allProjects {
		project = "all"
	}

	dbPath := config.ResolveGlobalDBPath()
	dbStatus := "CONNECTED"
	total, active, pending, wavefront := queryTaskMetrics(dbPath, project, allProjects)
	if total == -1 {
		dbStatus = "DEGRADED"
		total, active, pending, wavefront = 0, 0, 0, 0
	}

	phase := string(types.PhaseIdle)
	var nextActions []types.ActionAffordance
	if ev, hErr := handshake.SynthesizeDynamicHandshake(treeRoot); hErr == nil && ev != nil {
		phase = string(ev.Phase)
		nextActions = ev.NextActions
	} else if isWorktree {
		phase = string(types.PhaseEdit)
		nextActions = types.BuildPhaseNextActions(types.PhaseEdit, "", treeRoot, nil, nil)
	} else {
		nextActions = types.BuildPhaseNextActions(types.PhaseIdle, "", "", nil, nil)
	}

	remoteAhead := resolveRemoteAheadCount(repoRoot)

	return &StatusReport{
		Phase:            phase,
		Project:          project,
		RepoRoot:         repoRoot,
		ActiveBranch:     activeBranch,
		IsWorktree:       isWorktree,
		DatabasePath:     dbPath,
		DatabaseStatus:   dbStatus,
		TotalTasks:       total,
		ActiveTasks:      active,
		PendingTasks:     pending,
		WavefrontCount:   wavefront,
		RemoteAheadCount: remoteAhead,
		NextActions:      nextActions,
	}
}

func resolveBranchName(repoRoot string) string {
	cmd := workspace.SafeGitCommand(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func resolveRemoteAheadCount(repoRoot string) int {
	cmd := workspace.SafeGitCommand(repoRoot, "rev-list", "@{u}..HEAD", "--count")
	if out, err := cmd.Output(); err == nil {
		if count, cErr := strconv.Atoi(strings.TrimSpace(string(out))); cErr == nil {
			return count
		}
	}
	for _, ref := range []string{"origin/main..HEAD", "origin/master..HEAD"} {
		c := workspace.SafeGitCommand(repoRoot, "rev-list", ref, "--count")
		if o, oErr := c.Output(); oErr == nil {
			if count, cErr := strconv.Atoi(strings.TrimSpace(string(o))); cErr == nil {
				return count
			}
		}
	}
	return 0
}

func queryTaskMetrics(dbPath, project string, allProjects bool) (int, int, int, int) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return -1, -1, -1, -1
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			// ignore on query completion
		}
	}()

	store := task.NewSQLiteTaskStore(db)
	tasks := store.ListTasks()
	if !allProjects {
		tasks = task.FilterTasksByProject(tasks, project, true)
	}
	active, pending := 0, 0
	for _, t := range tasks {
		if task.IsInProgressStatus(t.Status) {
			active++
		}
		if !task.IsTerminalStatus(t.Status) {
			pending++
		}
	}

	wf, err := store.GetWavefront()
	wfCount := 0
	if err == nil {
		for _, cand := range wf {
			if task.IsInProgressStatus(cand.Status) {
				continue
			}
			if !allProjects && !task.MatchesProject(cand.Project, project, true) {
				continue
			}
			wfCount++
		}
	}
	return len(tasks), active, pending, wfCount
}

func renderStatusReport(out io.Writer, r *StatusReport) {
	fmt.Fprintln(out, renderBrandTitle("📋 Nomos Operational Status"))
	fmt.Fprintln(out, uiRule(ruleNarrow))
	fmt.Fprintln(out, kvLine("Active Phase", 15, styleSemanticBadge(r.Phase)))
	if r.Project != "" {
		fmt.Fprintln(out, kvLine("Project Scope", 15, uiBoldValueStyle.Render(r.Project)))
	}
	fmt.Fprintln(out, kvLine("Active Branch", 15, uiBoldValueStyle.Render(r.ActiveBranch)))
	fmt.Fprintln(out, kvLine("Is Worktree", 15, formatWorktreeFlag(r.IsWorktree)))
	fmt.Fprintln(out, kvLine("Database Status", 15, fmt.Sprintf("%s %s",
		styleSemanticBadge(r.DatabaseStatus),
		uiMutedStyle.Render("("+r.DatabasePath+")"))))
	ledgerVal := fmt.Sprintf("%s Total %s %s In Progress %s %s Pending %s %s Wavefront Ready",
		uiBoldValueStyle.Render(strconv.Itoa(r.TotalTasks)),
		uiMutedStyle.Render("|"),
		uiSubstrateStyle.Render(strconv.Itoa(r.ActiveTasks)),
		uiMutedStyle.Render("|"),
		uiValueStyle.Render(strconv.Itoa(r.PendingTasks)),
		uiMutedStyle.Render("|"),
		uiSuccessStyle.Render(strconv.Itoa(r.WavefrontCount)))
	fmt.Fprintln(out, kvLine("Living Ledger", 15, ledgerVal))
	if r.RemoteAheadCount > 0 {
		syncWarn := fmt.Sprintf("%s  %s",
			uiWarnStyle.Render(fmt.Sprintf("⚠️  %d unpushed commit(s) ahead of origin", r.RemoteAheadCount)),
			uiMutedStyle.Render("(run 'git push origin main refs/notes/nomos/releases')"))
		fmt.Fprintln(out, kvLine("Remote Sync", 15, syncWarn))
	} else {
		fmt.Fprintln(out, kvLine("Remote Sync", 15, uiSuccessStyle.Render("✅ Synced with origin")))
	}
	renderNextActions(out, r.NextActions)
	fmt.Fprintln(out, uiRule(ruleNarrow))
}

func formatWorktreeFlag(isWT bool) string {
	if isWT {
		return uiSubstrateStyle.Render("true")
	}
	return uiMutedStyle.Render("false")
}
