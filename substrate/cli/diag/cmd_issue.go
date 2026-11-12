// Package diag implements deterministic Cobra command dispatch, software physics telemetry,
// session management, and terminal UI contracts for Nomos.
package diag

import (
	"bufio"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/spf13/cobra"
)

const (
	defaultDiagnosticOutputPath = ".tmp/nomos-diagnostic.md"
	publicGitHubNewIssueURL     = "https://github.com/mgantlett/nomos/issues/new"
	PrivateDiagnosticEmail      = "support@sophialabs.ca"
)

// IssueReport holds the collected and sanitized diagnostic report (#IssueReport).
type IssueReport struct {
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	NomosVersion    string                  `json:"nomos_version"`
	GoVersion       string                  `json:"go_version"`
	Platform        string                  `json:"platform"`
	GitVersion      string                  `json:"git_version"`
	ActivePhase     string                  `json:"active_phase"`
	BranchTopology  string                  `json:"branch_topology"`
	IsWorktree      bool                    `json:"is_worktree"`
	DatabaseStatus  string                  `json:"database_status"`
	RecentAnomalies []SubstrateAnomalyEvent `json:"recent_anomalies,omitempty"`
}

type issueOptions struct {
	outputPath   string
	dryRun       bool
	web          bool
	autoYes      bool
	clearHistory bool
	body         string
}

// NewIssueCmd constructs the canonical 'nomos issue' command.
func NewIssueCmd() *cobra.Command {
	opts := &issueOptions{}
	cmd := &cobra.Command{
		Use:   "issue [title]",
		Short: "Export a scrubbed, offline-first diagnostic Markdown report",
		Long: `Collects runtime telemetry, git topology, and local substrate anomalies, scrubs all
repository paths, private branch names, and credentials, and writes an auditable local
Markdown report (.tmp/nomos-diagnostic.md) with public and private submission instructions.`,
		Example: tui.HelpIssueExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssueCommand(cmd.OutOrStdout(), opts, args)
		},
	}

	cmd.Flags().StringVarP(&opts.outputPath, "output", "o", defaultDiagnosticOutputPath, "Path to write scrubbed local Markdown report")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Print sanitized Markdown payload to stdout without writing files")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", false, "Open pre-filled GitHub issue in default browser (interactive TTY only)")
	cmd.Flags().BoolVarP(&opts.autoYes, "yes", "y", false, "Skip [y/N] confirmation when --web is used in an interactive TTY")
	cmd.Flags().StringVarP(&opts.body, "body", "b", "", "Additional reproduction details or error notes (scrubbed automatically)")
	cmd.Flags().BoolVar(&opts.clearHistory, "clear", false, "Clear recorded entries from the local substrate anomaly ring buffer")

	return cmd
}

func runIssueCommand(out io.Writer, opts *issueOptions, args []string) error {
	status := CollectStatusReport()
	if opts.clearHistory && len(args) == 0 && strings.TrimSpace(opts.body) == "" && !opts.dryRun && !opts.web {
		if err := ClearSubstrateAnomalies(status.RepoRoot); err != nil {
			return types.NewNomosError(types.ErrCodeBoundary, "failed clearing local anomaly ring buffer: %v", err)
		}
		fmt.Fprintln(out, "🧹 Cleared local substrate anomaly ring buffer.")
		return nil
	}

	title := "Bug: Substrate diagnostic report"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		title = strings.TrimSpace(args[0])
	}

	report := BuildIssueReportFromStatus(status, title, opts.body)
	issueURL := GenerateGitHubIssueURL(report)

	if opts.dryRun {
		renderIssueDryRun(out, report, issueURL)
		return clearIfRequested(out, opts.clearHistory, status.RepoRoot)
	}

	writtenPath, err := writeSanitizedDiagnosticFile(opts.outputPath, FormatIssueMarkdown(report, issueURL))
	if err != nil {
		return err
	}
	renderIssueOfflineReceipt(out, report, writtenPath, issueURL)

	if opts.web {
		dispatchInteractiveBrowser(out, opts, issueURL)
	}
	return clearIfRequested(out, opts.clearHistory, status.RepoRoot)
}

func clearIfRequested(out io.Writer, clear bool, repoRoot string) error {
	if !clear {
		return nil
	}
	if err := ClearSubstrateAnomalies(repoRoot); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed clearing local anomaly ring buffer: %v", err)
	}
	fmt.Fprintln(out, "🧹 Cleared local substrate anomaly ring buffer.")
	return nil
}

func writeSanitizedDiagnosticFile(outputPath, content string) (string, error) {
	target := strings.TrimSpace(outputPath)
	if target == "" {
		target = defaultDiagnosticOutputPath
	}
	parent := filepath.Dir(target)
	if parent != "" && parent != "." {
		if mkErr := os.MkdirAll(parent, 0700); mkErr != nil {
			return "", types.NewNomosError(types.ErrCodeBoundary, "failed creating diagnostic directory %s: %v", parent, mkErr)
		}
	}
	if fi, statErr := os.Lstat(target); statErr == nil && !fi.Mode().IsRegular() {
		return "", types.NewNomosError(types.ErrCodeBoundary, "refusing to overwrite non-regular diagnostic path: %s", target)
	}
	if wErr := os.WriteFile(target, []byte(content), 0600); wErr != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed writing diagnostic report %s: %v", target, wErr)
	}
	return target, nil
}

func dispatchInteractiveBrowser(out io.Writer, opts *issueOptions, issueURL string) {
	if !isInteractive() {
		fmt.Fprintln(out, "\n🔒 Non-interactive shell detected; skipping automatic browser launch (Law 7).")
		fmt.Fprintf(out, "   Open manually if desired: %s\n", issueURL)
		return
	}
	if !opts.autoYes {
		confirmed, err := promptBrowserOpen(out)
		if err != nil || !confirmed {
			fmt.Fprintf(out, "\nSkipped browser launch. Submit manually if desired:\n  %s\n", issueURL)
			return
		}
	}
	if err := OpenBrowser(issueURL); err != nil {
		fmt.Fprintf(out, "\n⚠️  Could not launch browser (%v). Open manually:\n  %s\n", err, issueURL)
		return
	}
	fmt.Fprintln(out, "🚀 Opened pre-filled GitHub issue in your browser.")
}

func promptBrowserOpen(out io.Writer) (bool, error) {
	fmt.Fprint(out, "👉 Open pre-filled GitHub issue in browser? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	ans := strings.TrimSpace(strings.ToLower(input))
	return ans == "y" || ans == "yes", nil
}

// BuildIssueReportFromStatus executes deterministic diag verification logic (@intent task_lifecycle_cli_spec.md).
func BuildIssueReportFromStatus(status *StatusReport, title, userBody string) *IssueReport {
	repoRoot := "."
	activePhase := "IDLE"
	activeBranch := ""
	isWT := false
	dbStatus := "UNKNOWN"
	if status != nil {
		repoRoot = status.RepoRoot
		activePhase = status.Phase
		activeBranch = status.ActiveBranch
		isWT = status.IsWorktree
		dbStatus = status.DatabaseStatus
	}
	return &IssueReport{
		Title:           SanitizeDiagnosticText(title, repoRoot),
		Description:     SanitizeDiagnosticText(userBody, repoRoot),
		NomosVersion:    tui.ResolveAppVersion(),
		GoVersion:       runtime.Version(),
		Platform:        fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		GitVersion:      getGitVersion(repoRoot),
		ActivePhase:     activePhase,
		BranchTopology:  ClassifyBranchTopology(activeBranch, isWT),
		IsWorktree:      isWT,
		DatabaseStatus:  dbStatus,
		RecentAnomalies: LoadRecentAnomalies(repoRoot),
	}
}

func getGitVersion(root string) string {
	cmd := workspace.SafeGitCommand(root, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "git unknown"
	}
	return strings.TrimSpace(string(out))
}

func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// OpenBrowser executes deterministic diag verification logic (@intent task_lifecycle_cli_spec.md).
func OpenBrowser(targetURL string) error {
	if !strings.HasPrefix(targetURL, "https://github.com/") {
		return fmt.Errorf("refusing to open non-GitHub URL: %s", targetURL)
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}
