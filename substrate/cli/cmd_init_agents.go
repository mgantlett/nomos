// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

const (
	agentClaude   = "claude"
	agentCursor   = "cursor"
	agentOpenCode = "opencode"
	agentGemini   = "gemini"
	agentAgy      = "agy"
	agentVSCode   = "vscode"
)

var canonicalInitAgents = []string{agentClaude, agentCursor, agentOpenCode, agentGemini, agentAgy, agentVSCode}

func resolveInitAgents(absDir, rawFlag string) ([]string, error) {
	if strings.TrimSpace(rawFlag) != "" {
		return parseSelectedAgents(rawFlag)
	}
	return detectInstalledAgents(absDir), nil
}

func parseSelectedAgents(raw string) ([]string, error) {
	seen := make(map[string]bool)
	var selected []string
	for _, part := range strings.Split(raw, ",") {
		tok := strings.ToLower(strings.TrimSpace(part))
		if tok == "" {
			continue
		}
		if tok == "all" {
			return append([]string(nil), canonicalInitAgents...), nil
		}
		if !slices.Contains(canonicalInitAgents, tok) {
			return nil, types.NewNomosError(types.ErrCodeValidationFailed, "unknown agent target %q", tok).
				WithRemedy("valid --agents targets: claude, cursor, opencode, gemini, agy, vscode, all")
		}
		if !seen[tok] {
			seen[tok] = true
			selected = append(selected, tok)
		}
	}
	if len(selected) == 0 {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "empty --agents filter").
			WithRemedy("pass comma-separated values from: claude, cursor, opencode, gemini, agy, vscode, all")
	}
	return selected, nil
}

func detectInstalledAgents(absDir string) []string {
	homeDir := ""
	if resolvedHome, err := os.UserHomeDir(); err == nil {
		homeDir = resolvedHome
	}
	var detected []string
	for _, agent := range canonicalInitAgents {
		if hasAgentDirSignal(absDir, agent) || hasAgentBinaryOnPath(agent) || hasAgentHomeSignal(homeDir, agent) {
			detected = append(detected, agent)
		}
	}
	return detected
}

func hasAgentHomeSignal(homeDir, agent string) bool {
	if strings.TrimSpace(homeDir) == "" {
		return false
	}
	switch agent {
	case agentClaude:
		return pathExists(filepath.Join(homeDir, claudeDirToken))
	case agentCursor:
		return pathExists(filepath.Join(homeDir, hookcmd.CursorDirToken))
	case agentOpenCode:
		return pathExists(filepath.Join(homeDir, ".config", "opencode")) || pathExists(filepath.Join(homeDir, openCodeDirToken))
	case agentGemini:
		return pathExists(filepath.Join(homeDir, geminiDirToken))
	case agentAgy:
		return pathExists(filepath.Join(homeDir, hookcmd.AgentsDirToken)) || pathExists(filepath.Join(homeDir, geminiDirToken, "antigravity"))
	case agentVSCode:
		return pathExists(filepath.Join(homeDir, tui.CopilotDirToken))
	default:
		return false
	}
}

func detectExistingHookAgents(absDir string) []string {
	var existing []string
	if fileExists(filepath.Join(absDir, claudeDirToken, settingsFileName)) {
		existing = append(existing, agentClaude)
	}
	if fileExists(filepath.Join(absDir, hookcmd.CursorDirToken, hookcmd.HooksFileName)) {
		existing = append(existing, agentCursor)
	}
	if fileExists(filepath.Join(absDir, openCodeDirToken, "plugins", "nomos.ts")) {
		existing = append(existing, agentOpenCode)
	}
	if fileExists(filepath.Join(absDir, geminiDirToken, settingsFileName)) {
		existing = append(existing, agentGemini)
	}
	if fileExists(filepath.Join(absDir, hookcmd.AgentsDirToken, hookcmd.HooksFileName)) {
		existing = append(existing, agentAgy)
	}
	if fileExists(tui.ResolveVSCodeManifestPath(false, absDir)) {
		existing = append(existing, agentVSCode)
	}
	return existing
}

func hasAgentDirSignal(absDir, agent string) bool {
	switch agent {
	case agentClaude:
		return pathExists(filepath.Join(absDir, claudeDirToken)) || fileExists(filepath.Join(absDir, "CLAUDE.md"))
	case agentCursor:
		return pathExists(filepath.Join(absDir, hookcmd.CursorDirToken))
	case agentOpenCode:
		return pathExists(filepath.Join(absDir, openCodeDirToken)) || fileExists(filepath.Join(absDir, "opencode.json"))
	case agentGemini:
		return pathExists(filepath.Join(absDir, geminiDirToken))
	case agentAgy:
		return pathExists(filepath.Join(absDir, hookcmd.AgentsDirToken))
	case agentVSCode:
		return pathExists(filepath.Join(absDir, ".vscode")) || pathExists(filepath.Join(absDir, tui.VSCodeHooksDirToken, tui.VSCodeHooksSubdirToken))
	default:
		return false
	}
}

func hasAgentBinaryOnPath(agent string) bool {
	_, err := exec.LookPath(agent)
	return err == nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func containsAgent(agents []string, target string) bool {
	return slices.Contains(agents, target)
}

func scaffoldAmbientHooks(dir string) error {
	existing := resolveExistingWorkspaceAgents(dir)
	if containsAgent(existing, agentAgy) || pathExists(filepath.Join(dir, hookcmd.AgentsDirToken, "workflows")) {
		pruneLegacyWorkflows(dir)
	}
	return scaffoldAmbientHooksForSelection(dir, existing)
}

func resolveExistingWorkspaceAgents(absDir string) []string {
	if existing := detectExistingHookAgents(absDir); len(existing) > 0 {
		return existing
	}
	var signaled []string
	for _, agent := range canonicalInitAgents {
		if hasAgentDirSignal(absDir, agent) {
			signaled = append(signaled, agent)
		}
	}
	if len(signaled) > 0 {
		return signaled
	}
	return canonicalInitAgents
}

func scaffoldAmbientHooksForSelection(dir string, agents []string) error {
	for _, agent := range agents {
		if err := scaffoldSingleAgentHooks(dir, agent); err != nil {
			return err
		}
	}
	return nil
}

func scaffoldSingleAgentHooks(dir, agent string) error {
	switch agent {
	case agentAgy:
		return writeAmbientHookBundle(filepath.Join(dir, hookcmd.AgentsDirToken))
	case agentClaude:
		return writeMergedJSONManifest(filepath.Join(dir, claudeDirToken, settingsFileName), tui.DefaultClaudeSettingsHooksJSON())
	case agentGemini:
		return writeMergedJSONManifest(filepath.Join(dir, geminiDirToken, settingsFileName), DefaultGeminiSettingsHooksJSON())
	case agentCursor:
		return writeMergedJSONManifest(filepath.Join(dir, hookcmd.CursorDirToken, hookcmd.HooksFileName), hookcmd.DefaultCursorHooksJSON())
	case agentOpenCode:
		return selfupdate.ScaffoldOpenCodePlugin(dir)
	case agentVSCode:
		return writeMergedJSONManifest(tui.ResolveVSCodeManifestPath(false, dir), tui.DefaultVSCodeHooksJSON())
	default:
		return nil
	}
}

func pruneLegacyWorkflows(dir string) {
	wfDir := filepath.Join(dir, hookcmd.AgentsDirToken, "workflows")
	legacy := []string{"nomos-handshake.md", "nomos-idle.md", "nomos-review.md"}
	for _, name := range legacy {
		if rErr := os.Remove(filepath.Join(wfDir, name)); rErr != nil && !os.IsNotExist(rErr) {
			// best-effort prune
		}
	}
	if entries, err := os.ReadDir(wfDir); err == nil && len(entries) == 0 {
		if rErr := os.Remove(wfDir); rErr != nil && !os.IsNotExist(rErr) {
			// best-effort directory prune
		}
	}
}

func writeAmbientHookBundle(rootDir string) error {
	scriptsDir := filepath.Join(rootDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}
	manifestBytes, rErr := initTemplates.ReadFile("templates/hooks/hooks.json")
	if rErr != nil {
		return rErr
	}
	manifestPath := filepath.Join(rootDir, "hooks.json")
	if rel, relErr := filepath.Rel(rootDir, manifestPath); relErr != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("unconfined hook manifest path: %s", manifestPath)
	}
	if mErr := scaffoldOrMergeHookManifestFile(manifestPath, manifestBytes); mErr != nil {
		return mErr
	}
	scripts := []string{"session-start.sh", "worktree-guard.sh", "nomos-build-guard.sh", "post-edit-radar.sh", "stop-guard.sh"}
	for _, name := range scripts {
		if wErr := writeHookBundleScript(scriptsDir, name); wErr != nil {
			return wErr
		}
	}
	return nil
}

func writeHookBundleScript(scriptsDir, name string) error {
	targetPath := filepath.Join(scriptsDir, name)
	tmplBytes, err := initTemplates.ReadFile("templates/hooks/scripts/" + name)
	if err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, tmplBytes, 0755); err != nil {
		return err
	}
	return os.Chmod(targetPath, 0755)
}

func scaffoldOrMergeHookManifestFile(targetPath string, templateBytes []byte) error {
	existingBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, templateBytes, 0644)
		}
		return err
	}
	mergedBytes, mErr := tui.MergeHookManifest(existingBytes, templateBytes)
	if mErr != nil {
		return mErr
	}
	return os.WriteFile(targetPath, mergedBytes, 0644)
}

func summaryWriter(opts InitOptions) io.Writer {
	if opts.Out != nil {
		return opts.Out
	}
	return os.Stdout
}

func printInitializationSummary(info initSummaryData) {
	out := summaryWriter(info.opts)
	check := tui.UiSuccessStyle.Render("✅")
	modeLabel := resolveInitModeLabel(info.opts)

	writeSummaryLine(out, "%s %s\n", tui.RenderBrandTitle("🚀 Initialized Nomos workspace in"), tui.UiBoldValueStyle.Render(info.absDir))
	writeSummaryLine(out, "%s\n", tui.UiRule(tui.RuleNarrow))
	writeSummaryLine(out, "  %s Governance mode: %s\n", check, tui.UiBoldValueStyle.Render(modeLabel))
	writeSummaryLine(out, "  %s Configuration & protocol: %s\n", check, tui.UiCommandStyle.Render("nomos.yaml, AGENTS.md"))
	printInitAgentHooksLine(out, check, info.selectedAgents)
	printInitRatchetAndBoundaryLines(out, check, info)
	if len(info.ciFiles) > 0 {
		writeSummaryLine(out, "  %s CI workflows: %s\n", check, tui.UiCommandStyle.Render(strings.Join(info.ciFiles, ", ")))
		writeSummaryLine(out, "  %s README badge: %s\n", check, tui.UiMutedStyle.Render(selfupdate.FormatReadmeBadgeSnippet(filepath.Base(info.absDir))))
	}
	if info.createdBaselineCommit {
		writeSummaryLine(out, "  %s Created baseline governance commit on %s\n", check, tui.UiBoldValueStyle.Render(info.defaultBranch))
	}
	printGitAndNextGuidance(out, check, info)
}

func printInitAgentHooksLine(out io.Writer, check string, agents []string) {
	if len(agents) > 0 {
		writeSummaryLine(out, "  %s Agent hooks: %s\n", check, tui.UiMutedStyle.Render(strings.Join(agents, ", ")))
		return
	}
	writeSummaryLine(out, "  %s Agent hooks: %s\n", check,
		tui.UiMutedStyle.Render("none auto-detected (wire anytime: nomos init --agents claude|cursor|opencode|gemini|agy|vscode)"))
}

func printInitRatchetAndBoundaryLines(out io.Writer, check string, info initSummaryData) {
	if info.opts.Strict {
		label := "Strict mode active (full structural & test ceilings enforced)"
		if info.promotedStrict {
			label = "Promoted existing nomos.yaml to strict_mode: true (historical baseline preserved)"
		}
		writeSummaryLine(out, "  %s %s\n", check, tui.UiBoldValueStyle.Render(label))
	} else if info.wroteBaseline && info.preInitXray != nil {
		hotspots := len(info.preInitXray.Hotspots)
		writeSummaryLine(out, "  %s Ratchet active: %s %s\n", check,
			tui.UiBoldValueStyle.Render(fmt.Sprintf("0 of %d existing hotspot(s) will fail; only net-new debt in touched files is gated", hotspots)),
			tui.UiCommandStyle.Render(fmt.Sprintf("(%s · score %d/100 · grade %s)", constants.FileNomosBaselineJSON, info.preInitXray.HangoverScore, info.preInitXray.Grade)))
	} else if !info.opts.Worktrees {
		writeSummaryLine(out, "  %s Ratchet active: %s\n", check,
			tui.UiMutedStyle.Render("anti-cheat & security gates live; pass --strict anytime for full ceilings"))
	}
	if info.inferredBoundaries > 0 {
		writeSummaryLine(out, "  %s Auto-locked boundaries: %s\n", check,
			tui.UiSuccessStyle.Render(fmt.Sprintf("%d clean architecture rule(s)", info.inferredBoundaries)))
	}
}

func resolveInitModeLabel(opts InitOptions) string {
	if opts.Worktrees && opts.Local {
		return "Hermetic Worktrees (Local Stealth)"
	}
	if opts.Worktrees {
		return "Hermetic Worktrees"
	}
	posture := "Ratchet"
	if opts.Strict {
		posture = "Strict"
	}
	if opts.Local {
		return fmt.Sprintf("Branch Guard (Local Stealth · %s)", posture)
	}
	return fmt.Sprintf("Branch Guard (%s)", posture)
}

func printGitAndNextGuidance(out io.Writer, check string, info initSummaryData) {
	if info.isGitRepo {
		writeSummaryLine(out, "  %s Installed git defense hooks %s\n", check, tui.UiMutedStyle.Render("(pre-commit, commit-msg, post-commit, pre-push)"))
	} else {
		writeSummaryLine(out, "  %s\n", tui.UiWarnStyle.Render("⚠️ Git defense hooks not installed (run 'git init' to enable)"))
	}
	if !info.opts.Worktrees {
		writeSummaryLine(out, "  %s Verified local memory connected\n", check)
	} else {
		writeSummaryLine(out, "  %s Living Ledger & Mneme storage connected\n", check)
	}
	writeSummaryLine(out, "%s\n", tui.UiRule(tui.RuleNarrow))
	if !info.opts.Worktrees {
		if info.currentBranch != "" && info.currentBranch != info.defaultBranch && !hookcmd.IsProtectedTrunkForConfig(info.currentBranch, nil) {
			writeSummaryLine(out, "  %s\n", tui.UiSuccessStyle.Render(fmt.Sprintf("✨ Ready on branch %s. Keep coding with your agent; run 'nomos status' anytime to view catches.", info.currentBranch)))
			return
		}
		writeSummaryLine(out, "  %s\n", tui.UiSuccessStyle.Render("✨ Ready on normal branches. Switch to a feature branch: git switch -c feat/<topic> (run 'nomos status' anytime to view catches)."))
		return
	}
	if info.createdBaselineCommit {
		writeSummaryLine(out, "  %s\n", tui.UiSuccessStyle.Render("✨ Workspace ready for Tier 1 agent indirect control."))
		return
	}
	writeSummaryLine(out, "  %s\n", tui.UiSuccessStyle.Render("✨ Workspace ready. Provision an isolated task via nomos task chore \"<title>\"."))
}

func writeSummaryLine(out io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		// non-fatal output write failure
	}
}
