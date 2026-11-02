// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/muesli/termenv"
)

const (
	ruleNarrow   = 60
	ruleStandard = 76
	ruleWide     = 92
	ruleSession  = 108
)

// Canonical SophiaLabs (sophialabs.ca) adaptive design tokens:
// Cosmic Void (#05070f / #060911) & Deep Navy (#0f172a) surfaces paired with
// Alabaster (#f8fafc), Substrate Sky Cyan (#38bdf8 / #22d3ee / #7dd3fc),
// Celestial Purple (#c084fc / #a855f7), Verification Mint (#34d399),
// Solar Amber (#fbbf24), and Fail-Closed Coral Rose (#f87171 / #f43f5e).
var (
	uiTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"})

	uiHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"})

	uiLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94a3b8"})

	uiValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"})

	uiBoldValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#09090b", Dark: "#f8fafc"})

	uiMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#64748b"})

	uiFooterStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	uiRuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#1e293b"})

	// Matches sophialabs.ca .quick-chip (#7dd3fc on subtle #0b192c cyan glass tint).
	uiCommandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#7dd3fc"})

	uiChipStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#bae6fd"}).
			Background(lipgloss.AdaptiveColor{Light: "#e0f2fe", Dark: "#0c1e33"})

	// Dual-Core Hemisphere Accents: Yang Celestial Purple (#c084fc) & Yin Substrate Cyan (#38bdf8).
	uiIntentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#c084fc"})

	uiSubstrateStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"})

	uiElectricCyanStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#22d3ee"})

	// Semantic telemetry foreground styles (Verification Mint, Solar Amber, Fail-Closed Coral).
	uiSuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"})

	uiWarnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"})

	uiDangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#e11d48", Dark: "#f87171"})

	// Tinted glassmorphic pill styles matching sophialabs.ca .badge.{success,cyan,purple,warning,danger}.
	pillSuccessStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34d399"}).
				Background(lipgloss.AdaptiveColor{Light: "#d1fae5", Dark: "#06281e"})

	pillCyanStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#38bdf8"}).
			Background(lipgloss.AdaptiveColor{Light: "#e0f2fe", Dark: "#082438"})

	pillPurpleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#6d28d9", Dark: "#c084fc"}).
			Background(lipgloss.AdaptiveColor{Light: "#f3e8ff", Dark: "#1f1033"})

	pillWarnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fbbf24"}).
			Background(lipgloss.AdaptiveColor{Light: "#fef3c7", Dark: "#291d07"})

	pillDangerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#be123c", Dark: "#f87171"}).
			Background(lipgloss.AdaptiveColor{Light: "#ffe4e6", Dark: "#2c0b14"})

	pillNeutralStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94a3b8"}).
				Background(lipgloss.AdaptiveColor{Light: "#e2e8f0", Dark: "#0f172a"})
)

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

// styleTaskKey colors INT-* keys in Celestial Purple and SUB-* keys in Substrate Sky Cyan.
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

// styleSemanticBadge renders a sophialabs.ca tinted glass pill in TTY mode and compact text in Ascii mode.
func styleSemanticBadge(token string) string {
	if lipgloss.ColorProfile() == termenv.Ascii {
		return token
	}
	return semanticPillStyleFor(token).Render(" " + token + " ")
}

// styleSemanticBadgeWidth renders a tinted pill badge padded cleanly to exact visible column width.
func styleSemanticBadgeWidth(token string, width int) string {
	style := semanticPillStyleFor(token)
	if lipgloss.ColorProfile() == termenv.Ascii {
		return padRight(token, width)
	}
	pillText := " " + token + " "
	pillVis := lipgloss.Width(pillText)
	if pillVis > width {
		return padStyled(token, width, style)
	}
	return style.Render(pillText) + strings.Repeat(" ", width-pillVis)
}

func semanticPillStyleFor(token string) lipgloss.Style {
	switch strings.ToUpper(strings.TrimSpace(token)) {
	case "PASS", "DONE", "CONNECTED", "HEALTHY", "ACTIVE", "VERIFIED", "SYNC":
		return pillSuccessStyle
	case "EDIT":
		return pillCyanStyle
	case "PLAN", "LEARN":
		return pillPurpleStyle
	case "REVIEW", "DEGRADED", "DISABLED", "UNGROUNDED", "WARN", "WARNING":
		return pillWarnStyle
	case "FAIL", "FAILED", "ABORTED", "UNHEALTHY", "DENY":
		return pillDangerStyle
	case "IDLE", "TODO":
		return pillNeutralStyle
	default:
		return uiValueStyle
	}
}

// kvLine formats an aligned "  Label : Value" telemetry line.
func kvLine(label string, labelWidth int, value string) string {
	return fmt.Sprintf("  %s %s %s",
		padStyled(label, labelWidth, uiLabelStyle),
		uiMutedStyle.Render(":"),
		value)
}

// renderTaskStartedBanner prints the canonical task worktree inception banner.
func renderTaskStartedBanner(key, wtPath, phase string) {
	fmt.Printf("%s %s\n  %s %s\n  %s %s\n",
		renderBrandTitle("🚀 Started task:"), styleTaskKey(key),
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
		codeChip := pillDangerStyle.Render("[" + nErr.Code + "]")
		if lipgloss.ColorProfile() != termenv.Ascii {
			codeChip = pillDangerStyle.Render(" " + nErr.Code + " ")
		}
		sb.WriteString(fmt.Sprintf("%s %s %s\n",
			uiDangerStyle.Render("✗ Error:"),
			codeChip,
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
