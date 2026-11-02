// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type rgbStop struct {
	r, g, b int
}

// sophialabs.ca hero H1 multi-stop gradient (#bae6fd -> #38bdf8 -> #818cf8 -> #c084fc -> #f472b6).
var (
	darkHeroStops = []rgbStop{
		{0xba, 0xe6, 0xfd}, // Ice Sky (#bae6fd)
		{0x38, 0xbd, 0xf8}, // Substrate Sky Cyan (#38bdf8)
		{0x81, 0x8c, 0xf8}, // Cosmic Indigo (#818cf8)
		{0xc0, 0x84, 0xfc}, // Celestial Purple (#c084fc)
		{0xf4, 0x72, 0xb6}, // Aurora Pink (#f472b6)
	}
	lightHeroStops = []rgbStop{
		{0x02, 0x84, 0xc7}, // Deep Sky (#0284c7)
		{0x4f, 0x46, 0xe5}, // Indigo (#4f46e5)
		{0x93, 0x33, 0xea}, // Purple (#9333ea)
		{0xdb, 0x27, 0x77}, // Rose (#db2777)
	}

	// sophialabs.ca Dual-Core horizon rule fade (#38bdf8 -> #6366f1 -> #a855f7 -> #1e293b).
	darkRuleStops = []rgbStop{
		{0x38, 0xbd, 0xf8}, // Substrate Cyan (#38bdf8)
		{0x63, 0x66, 0xf1}, // Indigo Bridge (#6366f1)
		{0xa8, 0x55, 0xf7}, // Intent Purple (#a855f7)
		{0x1e, 0x29, 0x3b}, // Deep Navy Hairline (#1e293b)
	}
	lightRuleStops = []rgbStop{
		{0x02, 0x84, 0xc7},
		{0x63, 0x66, 0xf1},
		{0x93, 0x33, 0xea},
		{0xcb, 0xd5, 0xe1},
	}
)

// renderBrandTitle renders CLI headers with the sophialabs.ca Cyan -> Purple -> Aurora gradient
// on 256-color/TrueColor terminals while preserving exact plain substrings in non-TTY/Ascii mode.
func renderBrandTitle(title string) string {
	profile := lipgloss.ColorProfile()
	if profile == termenv.Ascii {
		return title
	}
	if profile == termenv.ANSI {
		return uiTitleStyle.Render(title)
	}
	prefix, body := splitLeadingIconPrefix(title)
	stops := darkHeroStops
	if !lipgloss.HasDarkBackground() {
		stops = lightHeroStops
	}
	return prefix + renderGradientRunes(body, stops, true)
}

// uiRule renders a sophialabs.ca Dual-Core Cyan -> Indigo -> Purple -> Navy divider rule.
func uiRule(width int) string {
	if width <= 0 {
		width = ruleNarrow
	}
	line := strings.Repeat("─", width)
	profile := lipgloss.ColorProfile()
	if profile == termenv.Ascii {
		return line
	}
	if profile == termenv.ANSI {
		return uiRuleStyle.Render(line)
	}
	stops := darkRuleStops
	if !lipgloss.HasDarkBackground() {
		stops = lightRuleStops
	}
	return renderGradientRunes(line, stops, false)
}

func splitLeadingIconPrefix(s string) (string, string) {
	first, size := utf8.DecodeRuneInString(s)
	if size == 0 || first == utf8.RuneError || first <= 127 {
		return "", s
	}
	idx := strings.IndexByte(s, ' ')
	if idx < 0 {
		return "", s
	}
	for idx < len(s) && s[idx] == ' ' {
		idx++
	}
	return s[:idx], s[idx:]
}

func renderGradientRunes(text string, stops []rgbStop, bold bool) string {
	runes := []rune(text)
	if len(runes) == 0 || len(stops) == 0 {
		return text
	}
	var sb strings.Builder
	total := len(runes)
	for i, r := range runes {
		if r == ' ' {
			sb.WriteRune(' ')
			continue
		}
		t := 0.0
		if total > 1 {
			t = float64(i) / float64(total-1)
		}
		hex := interpolateHexStop(stops, t)
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		if bold {
			st = st.Bold(true)
		}
		sb.WriteString(st.Render(string(r)))
	}
	return sb.String()
}

func interpolateHexStop(stops []rgbStop, t float64) string {
	if len(stops) == 1 || t <= 0 {
		return fmt.Sprintf("#%02x%02x%02x", stops[0].r, stops[0].g, stops[0].b)
	}
	if t >= 1 {
		last := stops[len(stops)-1]
		return fmt.Sprintf("#%02x%02x%02x", last.r, last.g, last.b)
	}
	scaled := t * float64(len(stops)-1)
	idx := int(scaled)
	if idx >= len(stops)-1 {
		idx = len(stops) - 2
	}
	frac := scaled - float64(idx)
	a, b := stops[idx], stops[idx+1]
	r := int(float64(a.r) + frac*float64(b.r-a.r))
	g := int(float64(a.g) + frac*float64(b.g-a.g))
	bl := int(float64(a.b) + frac*float64(b.b-a.b))
	return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
}
