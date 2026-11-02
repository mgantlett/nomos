// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

const (
	ruleNarrow   = 60
	ruleStandard = 76
	ruleWide     = 92
	ruleSession  = 108
)

var (
	uiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#38bdf8"})

	uiHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f1f5f9"})

	uiLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94a3b8"})

	uiValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"})

	uiBoldValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"})

	uiMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#64748b"})

	uiFooterStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#64748b"})

	uiRuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#475569"})

	uiCommandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#7dd3fc"})

	uiIntentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#6d28d9", Dark: "#a78bfa"})

	uiSubstrateStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"})

	uiSuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#10b981"})

	uiWarnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#f59e0b"})

	uiDangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#ef4444"})
)

// uiRule renders a styled horizontal divider of exact rune width.
func uiRule(width int) string {
	if width <= 0 {
		width = ruleNarrow
	}
	return uiRuleStyle.Render(strings.Repeat("─", width))
}

// padRight pads plain text to width using visible terminal cell width before ANSI styling.
func padRight(plain string, width int) string {
	vis := lipgloss.Width(plain)
	if vis >= width {
		return plain
	}
	return plain + strings.Repeat(" ", width-vis)
}

// padStyled pads plain text to width first and then applies Lipgloss styling so ANSI codes never skew columns.
func padStyled(plain string, width int, style lipgloss.Style) string {
	return style.Render(padRight(plain, width))
}

// styleTaskKey colors INT-* keys in Intent violet and SUB-* keys in Substrate sky blue.
func styleTaskKey(key string) string {
	return taskKeyStyleFor(key).Render(key)
}

// styleTaskKeyWidth renders a fixed-width, plane-colored task key.
func styleTaskKeyWidth(key string, width int) string {
	return padStyled(key, width, taskKeyStyleFor(key))
}

func taskKeyStyleFor(key string) lipgloss.Style {
	upper := strings.ToUpper(strings.TrimSpace(key))
	switch {
	case strings.HasPrefix(upper, "INT-"):
		return uiIntentStyle
	case strings.HasPrefix(upper, "SUB-"):
		return uiSubstrateStyle
	default:
		return uiBoldValueStyle
	}
}

// stylePlane renders "intent" or "substrate" in its canonical hemisphere color.
func stylePlane(plane string) string {
	return planeStyleFor(plane).Render(plane)
}

// stylePlaneWidth renders a fixed-width, hemisphere-colored plane label.
func stylePlaneWidth(plane string, width int) string {
	return padStyled(plane, width, planeStyleFor(plane))
}

func planeStyleFor(plane string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(plane)) {
	case "intent", "int":
		return uiIntentStyle
	case "substrate", "sub":
		return uiSubstrateStyle
	default:
		return uiValueStyle
	}
}

// styleSemanticBadge applies semantic lifecycle/health color to phases, statuses, and gate verdicts.
func styleSemanticBadge(token string) string {
	return semanticStyleFor(token).Render(token)
}

// styleSemanticBadgeWidth renders a fixed-width semantic lifecycle/health token.
func styleSemanticBadgeWidth(token string, width int) string {
	return padStyled(token, width, semanticStyleFor(token))
}

func semanticStyleFor(token string) lipgloss.Style {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "PASS", "DONE", "CONNECTED", "HEALTHY", "ACTIVE", "VERIFIED", "SYNC":
		return uiSuccessStyle
	case "EDIT":
		return uiSubstrateStyle
	case "PLAN", "LEARN":
		return uiIntentStyle
	case "REVIEW", "DEGRADED", "DISABLED", "UNGROUNDED", "WARN", "WARNING":
		return uiWarnStyle
	case "FAIL", "FAILED", "ABORTED", "UNHEALTHY", "DENY":
		return uiDangerStyle
	case "IDLE", "TODO":
		return uiMutedStyle
	default:
		return uiValueStyle
	}
}

// kvLine formats an aligned "  Label : Value" telemetry line.
func kvLine(label string, labelWidth int, value string) string {
	return fmt.Sprintf("  %s : %s", padStyled(label, labelWidth, uiLabelStyle), value)
}

// renderTaskStartedBanner prints the canonical task worktree inception banner.
func renderTaskStartedBanner(key, wtPath, phase string) {
	fmt.Printf("🚀 %s %s\n  %s %s\n  %s %s\n",
		uiTitleStyle.Render("Started task:"), styleTaskKey(key),
		uiLabelStyle.Render("Worktree:"), uiCommandStyle.Render(wtPath),
		uiLabelStyle.Render("Phase:"), styleSemanticBadge(phase))
}

// FormatCLIError renders top-level CLI errors with structured Lipgloss emphasis for human operators.
func FormatCLIError(err error) string {
	if err == nil {
		return ""
	}
	var nErr *types.NomosError
	if errors.As(err, &nErr) && nErr != nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s %s %s\n",
			uiDangerStyle.Render("✗ Error:"),
			uiDangerStyle.Render("["+nErr.Code+"]"),
			uiValueStyle.Render(nErr.Description)))
		if nErr.Remedy != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				uiWarnStyle.Render("Remedy :"),
				uiCommandStyle.Render(nErr.Remedy)))
		}
		if nErr.ExplainCommand != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				uiLabelStyle.Render("Inspect:"),
				uiCommandStyle.Render(nErr.ExplainCommand)))
		}
		return sb.String()
	}
	return fmt.Sprintf("%s %s\n", uiDangerStyle.Render("✗ Error:"), uiValueStyle.Render(err.Error()))
}
