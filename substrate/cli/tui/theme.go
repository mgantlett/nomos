// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/muesli/termenv"
)

const (
	RuleNarrow   = 60
	RuleStandard = 76
	RuleWide     = 92
	RuleSession  = 108
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

	UiHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#f8fafc"})

	UiLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#475569", Dark: "#94a3b8"})

	UiValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#0f172a", Dark: "#e2e8f0"})

	UiBoldValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#09090b", Dark: "#f8fafc"})

	UiMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#64748b"})

	UiFooterStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#64748b", Dark: "#94a3b8"})

	UiRuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#cbd5e1", Dark: "#1e293b"})

	// Matches sophialabs.ca .quick-chip (#7dd3fc on subtle #0b192c cyan glass tint).
	UiCommandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#7dd3fc"})

	UiChipStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#bae6fd"}).
			Background(lipgloss.AdaptiveColor{Light: "#e0f2fe", Dark: "#0c1e33"})

	// Dual-Core Hemisphere Accents: Yang Celestial Purple (#c084fc) & Yin Substrate Cyan (#38bdf8).
	UiIntentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#c084fc"})

	UiSubstrateStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0284c7", Dark: "#38bdf8"})

	UiElectricCyanStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#0891b2", Dark: "#22d3ee"})

	// Semantic telemetry foreground styles (Verification Mint, Solar Amber, Fail-Closed Coral).
	UiSuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34d399"})

	UiWarnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"})

	UiDangerStyle = lipgloss.NewStyle().
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
func PadStyled(plain string, width int, style lipgloss.Style) string {
	return style.Render(padRight(plain, width))
}

// styleTaskKey colors INT-* keys in Celestial Purple and SUB-* keys in Substrate Sky Cyan.
func StyleTaskKey(key string) string {
	return TaskKeyStyleFor(key).Render(key)
}

// styleTaskKeyWidth renders a fixed-width, plane-colored task key.
func StyleTaskKeyWidth(key string, width int) string {
	return PadStyled(key, width, TaskKeyStyleFor(key))
}

// TaskKeyStyleFor executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func TaskKeyStyleFor(key string) lipgloss.Style {
	upper := strings.ToUpper(strings.TrimSpace(key))
	switch {
	case strings.HasPrefix(upper, "INT-"):
		return UiIntentStyle
	case strings.HasPrefix(upper, "SUB-"):
		return UiSubstrateStyle
	default:
		return UiBoldValueStyle
	}
}

// stylePlane renders "intent" or "substrate" in its canonical hemisphere color.
func StylePlane(plane string) string {
	return planeStyleFor(plane).Render(plane)
}

// stylePlaneWidth renders a fixed-width, hemisphere-colored plane label.
func StylePlaneWidth(plane string, width int) string {
	return PadStyled(plane, width, planeStyleFor(plane))
}

func planeStyleFor(plane string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(plane)) {
	case "intent", "int":
		return UiIntentStyle
	case "substrate", "sub":
		return UiSubstrateStyle
	default:
		return UiValueStyle
	}
}

// styleSemanticBadge renders a sophialabs.ca tinted glass pill in TTY mode and compact text in Ascii mode.
func StyleSemanticBadge(token string) string {
	if lipgloss.ColorProfile() == termenv.Ascii {
		return token
	}
	return semanticPillStyleFor(token).Render(" " + token + " ")
}

// styleSemanticBadgeWidth renders a tinted pill badge padded cleanly to exact visible column width.
func StyleSemanticBadgeWidth(token string, width int) string {
	style := semanticPillStyleFor(token)
	if lipgloss.ColorProfile() == termenv.Ascii {
		return padRight(token, width)
	}
	pillText := " " + token + " "
	pillVis := lipgloss.Width(pillText)
	if pillVis > width {
		return PadStyled(token, width, style)
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
		return UiValueStyle
	}
}

// kvLine formats an aligned "  Label : Value" telemetry line.
func KvLine(label string, labelWidth int, value string) string {
	return fmt.Sprintf("  %s %s %s",
		PadStyled(label, labelWidth, UiLabelStyle),
		UiMutedStyle.Render(":"),
		value)
}

// renderTaskStartedBanner prints the canonical task worktree inception banner.
func RenderTaskStartedBanner(key, wtPath, phase string) {
	fmt.Printf("%s %s\n  %s %s\n  %s %s\n",
		RenderBrandTitle("🚀 Started task:"), StyleTaskKey(key),
		UiLabelStyle.Render("Worktree:"), UiCommandStyle.Render(wtPath),
		UiLabelStyle.Render("Phase:"), StyleSemanticBadge(phase))
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
			UiDangerStyle.Render("✗ Error:"),
			codeChip,
			UiValueStyle.Render(nErr.Description)))
		if nErr.Remedy != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				UiWarnStyle.Render("Remedy :"),
				UiCommandStyle.Render(nErr.Remedy)))
		}
		if nErr.ExplainCommand != "" {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				UiLabelStyle.Render("Inspect:"),
				UiCommandStyle.Render(nErr.ExplainCommand)))
		}
		return sb.String()
	}
	return fmt.Sprintf("%s %s\n", UiDangerStyle.Render("✗ Error:"), UiValueStyle.Render(err.Error()))
}

// ResolveAppVersion returns the statutory Nomos release version wired by root CLI startup (@intent task_lifecycle_cli_spec.md).
var ResolveAppVersion = func() string { return "4.16.2" }

// TemplatesFS stores the embedded workspace governance templates wired by root CLI startup (@intent task_lifecycle_cli_spec.md).
var TemplatesFS fs.FS

// ScaffoldAmbientHooksFn stores the workspace ambient-hook scaffolding callback wired by root CLI startup (@intent task_lifecycle_cli_spec.md).
var ScaffoldAmbientHooksFn func(repoRoot string) error

// ReadEmbeddedTemplate reads a template file from TemplatesFS (@intent task_lifecycle_cli_spec.md).
func ReadEmbeddedTemplate(relPath string) ([]byte, error) {
	if TemplatesFS == nil {
		return nil, errors.New("embedded templates filesystem not initialized")
	}
	return fs.ReadFile(TemplatesFS, relPath)
}

// SortedMapKeys returns deterministic sorted keys from a string set (@intent software_physics_engine_spec.md).
func SortedMapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
