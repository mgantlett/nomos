// Package diag implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package diag

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
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
	Phase            string                     `json:"phase"`
	Project          string                     `json:"project,omitempty"`
	RepoRoot         string                     `json:"repo_root"`
	ActiveBranch     string                     `json:"active_branch"`
	IsWorktree       bool                       `json:"is_worktree"`
	DatabasePath     string                     `json:"database_path"`
	DatabaseStatus   string                     `json:"database_status"`
	TotalTasks       int                        `json:"total_tasks"`
	ActiveTasks      int                        `json:"active_tasks"`
	PendingTasks     int                        `json:"pending_tasks"`
	WavefrontCount   int                        `json:"wavefront_count"`
	RemoteAheadCount int                        `json:"remote_ahead_count"`
	StaleObligations []task.StaleSpecObligation `json:"stale_obligations,omitempty"`
	NextActions      []types.ActionAffordance   `json:"next_actions,omitempty"`
}

// NewStatusCmd constructs the top-level 'nomos status' command.
func NewStatusCmd() *cobra.Command {
	var jsonOutput, allProjects bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Display unified operational status, phase state, and task wavefront",
		Long: `Emit a comprehensive status report covering active workspace lifecycle phase,
trunk branch, database connectivity, and repository-scoped Living Ledger task wavefront.`,
		Example: tui.HelpStatusExample,
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

	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit status as structured JSON")
	cmd.Flags().BoolVarP(&allProjects, tui.FlagNameAllProjects, "A", false, "Aggregate Living Ledger metrics across all workstation repositories")
	return cmd
}

// CollectStatusReport executes deterministic diag verification logic (@intent task_lifecycle_cli_spec.md).
func CollectStatusReport() *StatusReport {
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

	phase, nextActions := resolveStatusPhaseAndActions(treeRoot, isWorktree)
	remoteAhead := resolveRemoteAheadCount(repoRoot)
	staleObligations := ResolveStaleObligations(repoRoot)

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
		StaleObligations: staleObligations,
		NextActions:      nextActions,
	}
}

func resolveStatusPhaseAndActions(treeRoot string, isWorktree bool) (string, []types.ActionAffordance) {
	if ev, hErr := handshake.SynthesizeDynamicHandshake(treeRoot); hErr == nil && ev != nil {
		return string(ev.Phase), ev.NextActions
	}
	if isWorktree {
		return string(types.PhaseEdit), types.BuildPhaseNextActions(types.PhaseEdit, "", treeRoot, nil, nil)
	}
	return string(types.PhaseIdle), types.BuildPhaseNextActions(types.PhaseIdle, "", "", nil, nil)
}

// ResolveStaleObligations executes deterministic diag verification logic (@intent task_lifecycle_cli_spec.md).
func ResolveStaleObligations(repoRoot string) []task.StaleSpecObligation {
	computedStale, err := task.ComputeStaleSpecObligations(repoRoot, "")
	if err != nil {
		return nil
	}
	return computedStale
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
	fmt.Fprintln(out, tui.RenderBrandTitle("📋 Nomos Operational Status"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
	fmt.Fprintln(out, tui.KvLine("Active Phase", 15, tui.StyleSemanticBadge(r.Phase)))
	if r.Project != "" {
		fmt.Fprintln(out, tui.KvLine("Project Scope", 15, tui.UiBoldValueStyle.Render(r.Project)))
	}
	fmt.Fprintln(out, tui.KvLine("Active Branch", 15, tui.UiBoldValueStyle.Render(r.ActiveBranch)))
	fmt.Fprintln(out, tui.KvLine("Is Worktree", 15, formatWorktreeFlag(r.IsWorktree)))
	fmt.Fprintln(out, tui.KvLine("Database Status", 15, fmt.Sprintf("%s %s",
		tui.StyleSemanticBadge(r.DatabaseStatus),
		tui.UiMutedStyle.Render("("+r.DatabasePath+")"))))
	ledgerVal := fmt.Sprintf("%s Total %s %s In Progress %s %s Pending %s %s Wavefront Ready",
		tui.UiBoldValueStyle.Render(strconv.Itoa(r.TotalTasks)),
		tui.UiMutedStyle.Render("|"),
		tui.UiSubstrateStyle.Render(strconv.Itoa(r.ActiveTasks)),
		tui.UiMutedStyle.Render("|"),
		tui.UiValueStyle.Render(strconv.Itoa(r.PendingTasks)),
		tui.UiMutedStyle.Render("|"),
		tui.UiSuccessStyle.Render(strconv.Itoa(r.WavefrontCount)))
	fmt.Fprintln(out, tui.KvLine("Living Ledger", 15, ledgerVal))
	if r.RemoteAheadCount > 0 {
		syncWarn := fmt.Sprintf("%s  %s",
			tui.UiWarnStyle.Render(fmt.Sprintf("⚠️  %d unpushed commit(s) ahead of origin", r.RemoteAheadCount)),
			tui.UiMutedStyle.Render("(run 'git push origin main refs/notes/nomos/releases')"))
		fmt.Fprintln(out, tui.KvLine("Remote Sync", 15, syncWarn))
	} else {
		fmt.Fprintln(out, tui.KvLine("Remote Sync", 15, tui.UiSuccessStyle.Render("✅ Synced with origin")))
	}
	renderStaleObligationsLine(out, r.StaleObligations)
	tui.RenderNextActions(out, r.NextActions)
	fmt.Fprintln(out, tui.UiRule(tui.RuleNarrow))
}

func renderStaleObligationsLine(out io.Writer, stale []task.StaleSpecObligation) {
	if len(stale) == 0 {
		return
	}
	warn := fmt.Sprintf("%s  %s",
		tui.UiWarnStyle.Render(fmt.Sprintf("⚠️  %d spec(s) updated since last SUB-* receipt", len(stale))),
		tui.UiMutedStyle.Render("("+stale[0].SpecPath+")"))
	fmt.Fprintln(out, tui.KvLine("Obligations", 15, warn))
}

func formatWorktreeFlag(isWT bool) string {
	if isWT {
		return tui.UiSubstrateStyle.Render("true")
	}
	return tui.UiMutedStyle.Render("false")
}
