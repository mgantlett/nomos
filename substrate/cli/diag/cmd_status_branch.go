// Package diag implements the Branch-Guard-native status dashboard, Shield Scorecard, and 4-rung graduation ladder.
// @intent task_lifecycle_cli_spec.md
package diag

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

const (
	stealthMarkerToken   = "# Nomos Local Stealth Trial Start"
	ciNudgeMinCommits    = 3
	rungStealthTrial     = 1
	rungBranchGuard      = 2
	rungCIPRProofs       = 3
	rungStrictOrWorktree = 4
)

func enrichBranchGuardStatus(r *StatusReport, repoRoot string, isWorktree bool) {
	cfg, err := config.LoadConfig(repoRoot)
	if err != nil || cfg == nil {
		return
	}
	mode := cfg.Governance.Boundary.EffectiveIsolationMode()
	r.IsolationMode = mode
	r.StrictMode = cfg.Governance.QualityGates.StrictMode
	if mode != config.IsolationModeBranch || isWorktree {
		return
	}

	r.StealthLocal = detectLocalStealthExclude(repoRoot)
	r.HooksInstalled = detectGitHooksActive(repoRoot)
	r.WiredAgents = detectStatusWiredAgents(repoRoot)
	r.CIEnabled = detectCIWorkflowPresent(repoRoot)
	loadBaselineAnchorIntoStatus(repoRoot, r)
	if sum, sErr := mem.QueryShieldSummary(repoRoot); sErr == nil && sum != nil {
		r.Shield = sum
	}
	r.GraduationRung, r.GraduationName, r.NextActions = computeBranchGraduation(r, cfg.DefaultBranch)
}

func detectLocalStealthExclude(repoRoot string) bool {
	excludePath := filepath.Join(repoRoot, constants.DirGit, "info", "exclude")
	raw, err := os.ReadFile(excludePath)
	return err == nil && strings.Contains(string(raw), stealthMarkerToken)
}

func detectGitHooksActive(repoRoot string) bool {
	hookPath := filepath.Join(repoRoot, constants.DirGit, "hooks", "pre-commit")
	raw, err := os.ReadFile(hookPath)
	return err == nil && strings.Contains(string(raw), "Nomos")
}

func detectStatusWiredAgents(repoRoot string) []string {
	checks := []struct {
		name string
		rel  string
	}{
		{"claude", filepath.Join(".claude", "settings.json")},
		{"cursor", filepath.Join(".cursor", "hooks.json")},
		{"opencode", filepath.Join(".opencode", "plugins", "nomos.ts")},
		{"gemini", filepath.Join(".gemini", "settings.json")},
		{"agy", filepath.Join(".agents", "hooks.json")},
		{"vscode", filepath.Join(tui.VSCodeHooksDirToken, tui.VSCodeHooksSubdirToken, tui.VSCodeHooksFileName)},
	}
	var wired []string
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(repoRoot, c.rel)); err == nil {
			wired = append(wired, c.name)
		}
	}
	return wired
}

func detectCIWorkflowPresent(repoRoot string) bool {
	ghPath := filepath.Join(repoRoot, ".github", "workflows", "nomos-verify.yml")
	if _, err := os.Stat(ghPath); err == nil {
		return true
	}
	glRaw, err := os.ReadFile(filepath.Join(repoRoot, ".gitlab-ci.yml"))
	return err == nil && strings.Contains(string(glRaw), "nomos verify")
}

func loadBaselineAnchorIntoStatus(repoRoot string, r *StatusReport) {
	manifest, err := xray.LoadBaselineManifest(repoRoot)
	if err != nil || manifest == nil {
		return
	}
	r.HasBaseline = true
	r.BaselineScore = manifest.HangoverScore
	r.BaselineGrade = manifest.Grade
}

func isBranchStatusProtectedTrunk(branch, defaultBranch string) bool {
	clean := strings.TrimSpace(branch)
	if clean == "" || clean == "HEAD" || workspace.IsProtectedBranch(clean) {
		return true
	}
	return defaultBranch != "" && strings.EqualFold(clean, strings.TrimSpace(defaultBranch))
}

func computeBranchGraduation(r *StatusReport, defaultBranch string) (int, string, []types.ActionAffordance) {
	verifiedCommits := 0
	if r.Shield != nil {
		verifiedCommits = r.Shield.VerifiedCommits
	}
	onTrunk := isBranchStatusProtectedTrunk(r.ActiveBranch, defaultBranch)

	if r.StrictMode {
		return rungStrictOrWorktree, "Strict", buildTrunkWithFollowUp(onTrunk,
			"Create a feature branch off protected trunk before editing",
			"nomos init --worktrees",
			"Optional: scale up to parallel isolated agent worktrees when running concurrent agents")
	}
	if r.StealthLocal {
		return rungStealthTrial, "Stealth Trial", buildTrunkWithFollowUp(onTrunk,
			"Switch off protected trunk so your agent can edit freely under Branch Guard",
			"nomos init",
			"Promote from local stealth trial to shared repository governance when ready")
	}
	if r.CIEnabled {
		return rungCIPRProofs, "CI PR Proofs", buildRung3Actions(r, onTrunk)
	}
	return rungBranchGuard, "Branch Guard", buildRung2Actions(verifiedCommits, onTrunk)
}

func buildTrunkWithFollowUp(onTrunk bool, trunkSummary, cmd, summary string) []types.ActionAffordance {
	var actions []types.ActionAffordance
	if onTrunk {
		actions = append(actions, types.ActionAffordance{
			Command: "git switch -c feat/<topic>",
			Summary: trunkSummary,
		})
	}
	actions = append(actions, types.ActionAffordance{
		Command: cmd,
		Summary: summary,
	})
	return actions
}

func buildRung2Actions(verifiedCommits int, onTrunk bool) []types.ActionAffordance {
	var actions []types.ActionAffordance
	if onTrunk {
		actions = append(actions, types.ActionAffordance{
			Command: "git switch -c feat/<topic>",
			Summary: "Create a feature branch off protected trunk before editing",
		})
	}
	if verifiedCommits >= ciNudgeMinCommits {
		actions = append(actions, types.ActionAffordance{
			Command: "nomos init --ci",
			Summary: fmt.Sprintf("%d verified commits complete - graduate to zero-trust CI PR proofs & SARIF", verifiedCommits),
		})
		return actions
	}
	actions = append(actions, types.ActionAffordance{
		Command: "nomos verify",
		Summary: "Verify touched files on your active branch (0 LLM tokens)",
	})
	return actions
}

func buildRung3Actions(r *StatusReport, onTrunk bool) []types.ActionAffordance {
	var actions []types.ActionAffordance
	if onTrunk {
		actions = append(actions, types.ActionAffordance{
			Command: "git switch -c feat/<topic>",
			Summary: "Create a feature branch off protected trunk before editing",
		})
	}
	gradeA := !r.HasBaseline || r.BaselineScore == 0 || strings.HasPrefix(strings.ToUpper(r.BaselineGrade), "A")
	if gradeA {
		actions = append(actions, types.ActionAffordance{
			Command: "nomos init --strict",
			Summary: "Grade A achieved - promote nomos.yaml to full strict mode",
		})
	} else {
		actions = append(actions, types.ActionAffordance{
			Command: "nomos xray .",
			Summary: "Track structural debt progress toward Grade A before enabling --strict",
		})
	}
	actions = append(actions, types.ActionAffordance{
		Command: "nomos init --worktrees",
		Summary: "Optional: graduate to isolated parallel multi-agent git worktrees",
	})
	return actions
}

func renderBranchGuardStatusReport(out io.Writer, r *StatusReport) {
	fmt.Fprintln(out, tui.RenderBrandTitle("🛡️  Nomos Branch Guard Status"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	fmt.Fprintln(out, tui.KvLine("Governance Mode", 16, tui.UiBoldValueStyle.Render(formatBranchGovernanceLabel(r))))
	if r.Project != "" {
		fmt.Fprintln(out, tui.KvLine("Project Scope", 16, tui.UiBoldValueStyle.Render(r.Project)))
	}
	fmt.Fprintln(out, tui.KvLine("Active Branch", 16, formatBranchGuardBranchState(r)))
	fmt.Fprintln(out, tui.KvLine("Defense Hooks", 16, formatHooksAndAgentsLine(r)))
	fmt.Fprintln(out, tui.KvLine("Shield Score", 16, formatShieldScoreLine(r.Shield)))
	if r.HasBaseline {
		baseText := fmt.Sprintf("%d/100 %s %s", r.BaselineScore, tui.RenderXrayGradeBadge(r.BaselineGrade),
			tui.UiMutedStyle.Render("(Day-0 anchor - 0 grandfathered files fail)"))
		fmt.Fprintln(out, tui.KvLine("Day-0 Baseline", 16, baseText))
	}
	fmt.Fprintln(out, tui.KvLine("Graduation", 16, formatGraduationLadder(r.GraduationRung)))
	tui.RenderNextActions(out, r.NextActions)
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
}

func formatBranchGovernanceLabel(r *StatusReport) string {
	posture := "Ratchet"
	if r.StrictMode {
		posture = "Strict"
	}
	if r.StealthLocal {
		return fmt.Sprintf("Branch Guard (Local Stealth · %s)", posture)
	}
	return fmt.Sprintf("Branch Guard (%s)", posture)
}

func formatBranchGuardBranchState(r *StatusReport) string {
	if workspace.IsProtectedBranch(r.ActiveBranch) {
		return fmt.Sprintf("%s %s", tui.UiWarnStyle.Render(r.ActiveBranch),
			tui.UiMutedStyle.Render("(protected trunk - switch to feat/* to edit)"))
	}
	return fmt.Sprintf("%s %s", tui.UiSuccessStyle.Render(r.ActiveBranch),
		tui.UiMutedStyle.Render("(feature branch ready)"))
}

func formatHooksAndAgentsLine(r *StatusReport) string {
	hookBadge := tui.UiWarnStyle.Render("MISSING")
	if r.HooksInstalled {
		hookBadge = tui.UiSuccessStyle.Render("ACTIVE")
	}
	agents := "none (pass --agents to wire)"
	if len(r.WiredAgents) > 0 {
		agents = strings.Join(r.WiredAgents, ", ")
	}
	return fmt.Sprintf("%s · %s %s", hookBadge, tui.UiMutedStyle.Render("Agents:"), tui.UiValueStyle.Render(agents))
}

func formatShieldScoreLine(sum *mem.ShieldSummary) string {
	if sum == nil {
		return tui.UiMutedStyle.Render("0 shortcuts blocked · 0 verified commits · 0 lessons")
	}
	catDetail := formatShieldCategorySummary(sum.CategoryCounts)
	blocked := fmt.Sprintf("%s shortcut(s) blocked", tui.UiBoldValueStyle.Render(fmt.Sprintf("%d", sum.ShortcutsBlocked)))
	if catDetail != "" {
		blocked += " " + tui.UiMutedStyle.Render("("+catDetail+")")
	}
	return fmt.Sprintf("%s · %s verified commit(s) · %s lesson(s)",
		blocked,
		tui.UiSuccessStyle.Render(fmt.Sprintf("%d", sum.VerifiedCommits)),
		tui.UiSubstrateStyle.Render(fmt.Sprintf("%d", sum.DistilledLessons)))
}

func formatShieldCategorySummary(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	order := []struct {
		key   string
		label string
	}{
		{mem.ShieldCategoryAntiCheat, "anti-cheat"},
		{mem.ShieldCategorySecurityAirlock, "security"},
		{mem.ShieldCategoryTrunkGuard, "trunk"},
		{mem.ShieldCategoryStructuralRatchet, "ratchet"},
		{mem.ShieldCategoryStopGuard, "stop-guard"},
	}
	var parts []string
	for _, item := range order {
		if n := counts[item.key]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, item.label))
		}
	}
	return strings.Join(parts, ", ")
}

func formatGraduationLadder(currentRung int) string {
	rungs := []struct {
		num   int
		label string
	}{
		{rungStealthTrial, "[1] Stealth"},
		{rungBranchGuard, "[2] Branch Guard"},
		{rungCIPRProofs, "[3] CI PR Proofs"},
		{rungStrictOrWorktree, "[4] Strict/Worktrees"},
	}
	parts := make([]string, 0, len(rungs))
	for _, r := range rungs {
		if r.num == currentRung {
			parts = append(parts, tui.UiSuccessStyle.Render(r.label+"*"))
		} else if r.num < currentRung {
			parts = append(parts, tui.UiValueStyle.Render(r.label))
		} else {
			parts = append(parts, tui.UiMutedStyle.Render(r.label))
		}
	}
	return strings.Join(parts, tui.UiMutedStyle.Render(" -> "))
}
