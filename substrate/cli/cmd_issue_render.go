// Package cli implements deterministic Cobra command dispatch, software physics telemetry,
// session management, and terminal UI contracts for Nomos.
package cli

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// formatIssueBodyMarkdown renders the sanitized diagnostic Markdown body suitable for GitHub URLs.
func formatIssueBodyMarkdown(report *IssueReport) string {
	var sb strings.Builder

	sb.WriteString("### Description\n")
	if report.Description != "" {
		sb.WriteString(report.Description)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("Encountered unexpected behavior or substrate invariant anomaly.\n\n")
	}

	sb.WriteString("### Diagnostic Telemetry (Scrubbed)\n")
	sb.WriteString(fmt.Sprintf("- **Nomos Version:** `%s`\n", report.NomosVersion))
	sb.WriteString(fmt.Sprintf("- **Platform:** `%s`\n", report.Platform))
	sb.WriteString(fmt.Sprintf("- **Go Version:** `%s`\n", report.GoVersion))
	sb.WriteString(fmt.Sprintf("- **Git Version:** `%s`\n", report.GitVersion))
	sb.WriteString(fmt.Sprintf("- **Active Phase:** `%s`\n", report.ActivePhase))
	sb.WriteString(fmt.Sprintf("- **Branch Topology:** `%s`\n", report.BranchTopology))
	sb.WriteString(fmt.Sprintf("- **Is Worktree:** `%v`\n", report.IsWorktree))
	sb.WriteString(fmt.Sprintf("- **Database Status:** `%s`\n\n", report.DatabaseStatus))

	appendRecentAnomaliesMarkdown(&sb, report.RecentAnomalies)
	return sb.String()
}

// formatIssueMarkdown renders the complete local Markdown file including dual submission instructions.
func formatIssueMarkdown(report *IssueReport, issueURL string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", report.Title))
	sb.WriteString("> Generated locally by `nomos issue` with zero network requests. Inspect before sharing.\n\n")
	sb.WriteString(formatIssueBodyMarkdown(report))
	sb.WriteString("### Submission Options (Optional)\n")
	sb.WriteString(fmt.Sprintf("1. **Public GitHub Issue (Open-Source):** [Open pre-filled GitHub Issue](%s)\n", issueURL))
	sb.WriteString(fmt.Sprintf("2. **Private / Anonymous Email (Enterprise & Regulated):** Email this Markdown file to `%s` (zero GitHub account required).\n", privateDiagnosticEmail))
	return sb.String()
}

func appendRecentAnomaliesMarkdown(sb *strings.Builder, anomalies []SubstrateAnomalyEvent) {
	if len(anomalies) == 0 {
		return
	}
	sb.WriteString("### Recent Local Substrate Anomalies\n")
	start := 0
	if len(anomalies) > 8 {
		start = len(anomalies) - 8
	}
	for _, ev := range anomalies[start:] {
		line := fmt.Sprintf("- `%s` · **%s** (`%s`, `%dx`)", ev.Timestamp, ev.Kind, ev.Component, ev.Occurrences)
		if ev.DurationMillis > 0 {
			line += fmt.Sprintf(" · `%dms`", ev.DurationMillis)
		}
		if ev.Summary != "" {
			line += ": " + ev.Summary
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString("\n")
}

func generateGitHubIssueURL(report *IssueReport) string {
	params := url.Values{}
	params.Set("title", report.Title)
	params.Set("body", formatIssueBodyMarkdown(report))
	params.Set("labels", "bug,triage")
	return fmt.Sprintf("%s?%s", publicGitHubNewIssueURL, params.Encode())
}

func renderIssueSummary(out io.Writer, report *IssueReport) {
	fmt.Fprintln(out, "🩺  Nomos Sanitized Diagnostic Report (Offline-First)")
	fmt.Fprintln(out, "────────────────────────────────────────────────────────────")
	fmt.Fprintf(out, "  Title           : %s\n", report.Title)
	fmt.Fprintf(out, "  Nomos Version   : %s (%s · %s)\n", report.NomosVersion, report.Platform, report.GoVersion)
	fmt.Fprintf(out, "  Active Phase    : %s (%s)\n", report.ActivePhase, report.BranchTopology)
	fmt.Fprintf(out, "  Local Anomalies : %d recorded\n", len(report.RecentAnomalies))
	fmt.Fprintln(out, "  Redactions      : <repo-root>, ~, branch names, tokens, URIs, cloud keys")
	fmt.Fprintln(out, "────────────────────────────────────────────────────────────")
}

func renderIssueOfflineReceipt(out io.Writer, report *IssueReport, writtenPath, issueURL string) {
	renderIssueSummary(out, report)
	fmt.Fprintf(out, "📄 Saved local report : %s (0600 · inspect before sharing)\n", writtenPath)
	fmt.Fprintln(out, "\n👉 How to submit (choose one):")
	fmt.Fprintf(out, "  1. Public GitHub  : %s\n", issueURL)
	fmt.Fprintf(out, "     (or run: nomos issue %q --web)\n", report.Title)
	fmt.Fprintf(out, "  2. Private Email  : Send %s to %s (no GitHub login required)\n", writtenPath, privateDiagnosticEmail)
}

func renderIssueDryRun(out io.Writer, report *IssueReport, issueURL string) {
	renderIssueSummary(out, report)
	fmt.Fprintln(out, "\n📋 Sanitized Markdown Payload:")
	fmt.Fprintln(out, formatIssueMarkdown(report, issueURL))
}
