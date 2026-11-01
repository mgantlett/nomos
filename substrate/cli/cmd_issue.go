// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/spf13/cobra"
)

// IssueReport holds the collected and sanitized diagnostic bundle.
type IssueReport struct {
	Title          string
	Description    string
	NomosVersion   string
	GoVersion      string
	Platform       string
	GitVersion     string
	ActivePhase    string
	ActiveBranch   string
	IsWorktree     bool
	DatabaseStatus string
}

var (
	tokenRe   = regexp.MustCompile(`(ghp_[a-zA-Z0-9]{20,}|github_pat_[a-zA-Z0-9_]{20,})`)
	apiKeyRe  = regexp.MustCompile(`(sk-[a-zA-Z0-9_\-]{20,})`)
	bearerRe  = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.]+`)
	urlAuthRe = regexp.MustCompile(`(https?://)([^:]+):([^@]+)@`)
)

type issueOptions struct {
	dryRun  bool
	web     bool
	autoYes bool
	body    string
}

// NewIssueCmd constructs the canonical 'nomos issue' command.
func NewIssueCmd() *cobra.Command {
	opts := &issueOptions{}
	cmd := &cobra.Command{
		Use:   "issue [title]",
		Short: "Generate sanitized diagnostic bundle and open a GitHub issue",
		Long: `Collects system telemetry, environment parameters, and git state, scrubs all
personal workstation paths and credentials, and formats an authenticated bug report for GitHub.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIssueCommand(opts, args)
		},
	}

	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview sanitized diagnostic bundle without opening browser")
	cmd.Flags().BoolVarP(&opts.web, "web", "w", true, "Open pre-filled issue in default browser")
	cmd.Flags().BoolVarP(&opts.autoYes, "yes", "y", false, "Confirm issue opening non-interactively")
	cmd.Flags().StringVarP(&opts.body, "body", "b", "", "Additional issue description or error details")

	return cmd
}

func runIssueCommand(opts *issueOptions, args []string) error {
	title := "Bug: Substrate diagnostic report"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		title = strings.TrimSpace(args[0])
	}

	report := collectIssueReport(title, opts.body)

	if opts.dryRun {
		renderIssueDryRun(report)
		return nil
	}

	renderIssueSummary(report)
	issueURL := generateGitHubIssueURL(report)

	if !opts.autoYes && isInteractive() {
		confirmed, err := promptBrowserOpen()
		if err != nil || !confirmed {
			fmt.Println("\nAborted. You can manually submit this report at:")
			fmt.Println(issueURL)
			return nil
		}
	}

	dispatchBrowser(opts, issueURL)
	return nil
}

func promptBrowserOpen() (bool, error) {
	fmt.Print("👉 Open pre-filled issue in browser? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func dispatchBrowser(opts *issueOptions, issueURL string) {
	if opts.web || opts.autoYes || isInteractive() {
		if err := openBrowser(issueURL); err != nil {
			fmt.Printf("\n⚠️  Could not automatically launch browser: %v\n", err)
			fmt.Println("👉 Open this URL in your browser to submit:")
			fmt.Println(issueURL)
		} else {
			fmt.Println("🚀 Opened issue submission in your browser.")
		}
	} else {
		fmt.Println("👉 Submit issue at:")
		fmt.Println(issueURL)
	}
}

func sanitizeText(text string) string {
	if text == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		text = strings.ReplaceAll(text, home, "~")
	}
	text = tokenRe.ReplaceAllString(text, "[REDACTED_GH_TOKEN]")
	text = apiKeyRe.ReplaceAllString(text, "[REDACTED_API_KEY]")
	text = bearerRe.ReplaceAllString(text, "Bearer [REDACTED]")
	text = urlAuthRe.ReplaceAllString(text, "${1}user:[REDACTED]@")

	return text
}

func collectIssueReport(title, userBody string) *IssueReport {
	status := collectStatusReport()
	gitVer := getGitVersion(status.RepoRoot)

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	goVer := runtime.Version()

	return &IssueReport{
		Title:          sanitizeText(title),
		Description:    sanitizeText(userBody),
		NomosVersion:   RootCmdVersion,
		GoVersion:      goVer,
		Platform:       platform,
		GitVersion:     gitVer,
		ActivePhase:    status.Phase,
		ActiveBranch:   status.ActiveBranch,
		IsWorktree:     status.IsWorktree,
		DatabaseStatus: status.DatabaseStatus,
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

func formatIssueMarkdown(report *IssueReport) string {
	var sb strings.Builder

	sb.WriteString("### Description\n")
	if report.Description != "" {
		sb.WriteString(report.Description)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("Encountered unexpected behavior or substrate invariant failure.\n\n")
	}

	sb.WriteString("### Diagnostic Telemetry\n")
	sb.WriteString(fmt.Sprintf("- **Nomos Version:** `%s`\n", report.NomosVersion))
	sb.WriteString(fmt.Sprintf("- **Platform:** `%s`\n", report.Platform))
	sb.WriteString(fmt.Sprintf("- **Go Version:** `%s`\n", report.GoVersion))
	sb.WriteString(fmt.Sprintf("- **Git Version:** `%s`\n", report.GitVersion))
	sb.WriteString(fmt.Sprintf("- **Active Phase:** `%s`\n", report.ActivePhase))
	sb.WriteString(fmt.Sprintf("- **Active Branch:** `%s`\n", report.ActiveBranch))
	sb.WriteString(fmt.Sprintf("- **Is Worktree:** `%v`\n", report.IsWorktree))
	sb.WriteString(fmt.Sprintf("- **Database Status:** `%s`\n\n", report.DatabaseStatus))

	sb.WriteString("### Terminal Proof / Logs\n```text\n")
	sb.WriteString(fmt.Sprintf("nomos version: %s\nphase: %s\nbranch: %s\n", report.NomosVersion, report.ActivePhase, report.ActiveBranch))
	sb.WriteString("```\n")

	return sb.String()
}

func generateGitHubIssueURL(report *IssueReport) string {
	baseURL := "https://github.com/mgantlett/nomos/issues/new"
	bodyContent := formatIssueMarkdown(report)

	params := url.Values{}
	params.Set("title", report.Title)
	params.Set("body", bodyContent)
	params.Set("labels", "bug,triage")

	return fmt.Sprintf("%s?%s", baseURL, params.Encode())
}

func renderIssueSummary(report *IssueReport) {
	fmt.Println("🩺  Nomos Sanitized Diagnostic Bundle")
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Printf("  Title          : %s\n", report.Title)
	fmt.Printf("  Nomos Version  : %s\n", report.NomosVersion)
	fmt.Printf("  Platform       : %s\n", report.Platform)
	fmt.Printf("  Go Version     : %s\n", report.GoVersion)
	fmt.Printf("  Active Phase   : %s\n", report.ActivePhase)
	fmt.Printf("  Sanitization   : Local paths (~), tokens redacted\n")
	fmt.Println("────────────────────────────────────────────────────────────")
}

func renderIssueDryRun(report *IssueReport) {
	renderIssueSummary(report)
	fmt.Println("\n📋 Formatted Issue Payload:")
	fmt.Println(formatIssueMarkdown(report))
	fmt.Println("🔗 Generated GitHub URL:")
	fmt.Println(generateGitHubIssueURL(report))
}

func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}
