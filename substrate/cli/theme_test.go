package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/muesli/termenv"
)

func TestThemePadStyled_ANSISafeColumnWidth(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(origProfile)
	})

	colA := styleTaskKeyWidth("SUB-42", 10)
	colB := styleTaskKeyWidth("INT-1024", 10)
	pillCol := styleSemanticBadgeWidth("DONE", 10)

	if w := lipgloss.Width(colA); w != 10 {
		t.Fatalf("expected visible width 10 for SUB-42, got %d (%q)", w, colA)
	}
	if w := lipgloss.Width(colB); w != 10 {
		t.Fatalf("expected visible width 10 for INT-1024, got %d (%q)", w, colB)
	}
	if w := lipgloss.Width(pillCol); w != 10 {
		t.Fatalf("expected visible width 10 for DONE pill column, got %d (%q)", w, pillCol)
	}
	if !strings.Contains(colA, "\x1b[") || !strings.Contains(pillCol, "\x1b[") {
		t.Fatalf("expected ANSI escape sequence in TrueColor mode, got %q / %q", colA, pillCol)
	}
}

func TestBrandGradientAndDualCoreRule(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(origProfile)
	})

	lipgloss.SetColorProfile(termenv.Ascii)
	asciiTitle := renderBrandTitle("🛡️  Nomos Operational Status")
	if asciiTitle != "🛡️  Nomos Operational Status" {
		t.Fatalf("expected plain untouched title in Ascii mode, got %q", asciiTitle)
	}
	asciiRule := uiRule(20)
	if lipgloss.Width(asciiRule) != 20 || strings.Contains(asciiRule, "\x1b[") {
		t.Fatalf("expected plain 20-char rule in Ascii mode, got %q", asciiRule)
	}

	lipgloss.SetColorProfile(termenv.TrueColor)
	tcTitle := renderBrandTitle("🛡️  Nomos Operational Status")
	if !strings.HasPrefix(tcTitle, "🛡️  ") {
		t.Fatalf("expected leading emoji prefix preserved intact, got %q", tcTitle)
	}
	if !strings.Contains(tcTitle, "\x1b[") {
		t.Fatalf("expected TrueColor gradient escape codes in title, got %q", tcTitle)
	}
	tcRule := uiRule(24)
	if lipgloss.Width(tcRule) != 24 || !strings.Contains(tcRule, "\x1b[") {
		t.Fatalf("expected 24-cell TrueColor gradient rule, got width=%d (%q)", lipgloss.Width(tcRule), tcRule)
	}
}

func TestThemeSemanticAndPlaneBadges(t *testing.T) {
	origProfile := lipgloss.ColorProfile()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(origProfile)
	})

	for _, prof := range []termenv.Profile{termenv.Ascii, termenv.TrueColor} {
		lipgloss.SetColorProfile(prof)
		tokens := []string{
			"PASS", "DONE", "CONNECTED", "HEALTHY", "ACTIVE", "VERIFIED", "SYNC",
			"EDIT", "PLAN", "LEARN", "REVIEW", "DEGRADED", "WARN",
			"FAIL", "ABORTED", "UNHEALTHY", "IDLE", "TODO", "CUSTOM",
		}
		for _, tok := range tokens {
			rendered := styleSemanticBadge(tok)
			if !strings.Contains(rendered, tok) {
				t.Errorf("profile=%v: expected badge to preserve token %q, got %q", prof, tok, rendered)
			}
		}
	}

	if got := stylePlaneWidth("intent", 10); lipgloss.Width(got) != 10 {
		t.Errorf("expected width 10 for intent plane, got %d", lipgloss.Width(got))
	}
	if got := stylePlaneWidth("substrate", 10); lipgloss.Width(got) != 10 {
		t.Errorf("expected width 10 for substrate plane, got %d", lipgloss.Width(got))
	}
}

func TestFormatCLIError(t *testing.T) {
	if got := FormatCLIError(nil); got != "" {
		t.Fatalf("expected empty output for nil error, got %q", got)
	}

	nErr := types.NewNomosError(types.ErrCodePhaseViolation, "mutations forbidden on trunk").
		WithRemedy("nomos task chore \"Fix bug\"").
		WithExplainCommand("nomos capabilities cell_3_1")

	formatted := FormatCLIError(nErr)
	for _, want := range []string{
		"Error:",
		"ERR_PHASE_VIOLATION",
		"mutations forbidden on trunk",
		"Remedy :",
		"nomos task chore \"Fix bug\"",
		"Inspect:",
		"nomos capabilities cell_3_1",
	} {
		if !strings.Contains(formatted, want) {
			t.Errorf("expected %q in FormatCLIError output:\n%s", want, formatted)
		}
	}

	plain := FormatCLIError(errors.New("disk full"))
	if !strings.Contains(plain, "Error:") || !strings.Contains(plain, "disk full") {
		t.Errorf("unexpected plain error output: %q", plain)
	}
}

func TestRenderTaskShowCard(t *testing.T) {
	rec := task.TaskInspectionRecord{
		Key:          "SUB-1620",
		Title:        "Add Lipgloss CLI styling",
		Status:       task.StatusEdit,
		Plane:        "substrate",
		Project:      "nomos",
		Tier:         task.Tier("T1"),
		Priority:     50,
		TwinKey:      "INT-1620",
		BlockedBy:    []string{"INT-1620"},
		Blocks:       []string{"SUB-1621"},
		WorktreePath: "worktrees/nomos-SUB-1620",
		BranchName:   "feature/SUB-1620",
		Description:  "## Scope & Context\nCLI styling.\n\n## Acceptance Criteria\n- [x] Theme SSoT\n- [ ] Unit tests",
	}

	var buf bytes.Buffer
	renderTaskShowCard(&buf, rec)
	out := buf.String()

	for _, want := range []string{
		"SUB-1620",
		"Add Lipgloss CLI styling",
		"INT-1620",
		"SUB-1621",
		"## Scope & Context",
		"- [x] Theme SSoT",
		"- [ ] Unit tests",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in rendered task card:\n%s", want, out)
		}
	}
}
