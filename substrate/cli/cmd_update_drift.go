// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
// @intent task_lifecycle_cli_spec.md
package cli

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

var canonicalHookBundleScripts = []string{
	"session-start.sh",
	"worktree-guard.sh",
	"nomos-build-guard.sh",
	"post-edit-radar.sh",
	"stop-guard.sh",
}

func detectAdapterAndStealthDrift(repoRoot string) []string {
	var drifted []string
	drifted = append(drifted, detectJSONProviderManifestDrift(repoRoot)...)
	if hasOpenCodePluginDrift(repoRoot) {
		drifted = append(drifted, filepath.ToSlash(filepath.Join(openCodeDirToken, "plugins", "nomos.ts")))
	}
	drifted = append(drifted, detectAntigravityBundleDrift(repoRoot)...)
	if hasLocalStealthExcludeDrift(repoRoot) {
		drifted = append(drifted, filepath.ToSlash(filepath.Join(constants.DirGit, "info", "exclude")))
	}
	return drifted
}

func detectJSONProviderManifestDrift(repoRoot string) []string {
	specs := []struct {
		dir, file string
		tmpl      []byte
	}{
		{claudeDirToken, settingsFileName, tui.DefaultClaudeSettingsHooksJSON()},
		{hookcmd.CursorDirToken, hookcmd.HooksFileName, hookcmd.DefaultCursorHooksJSON()},
		{geminiDirToken, settingsFileName, DefaultGeminiSettingsHooksJSON()},
		{filepath.Join(tui.VSCodeHooksDirToken, tui.VSCodeHooksSubdirToken), tui.VSCodeHooksFileName, tui.DefaultVSCodeHooksJSON()},
	}
	var drifted []string
	for _, sp := range specs {
		target := filepath.Join(repoRoot, sp.dir, sp.file)
		if hasJSONManifestDrift(target, sp.tmpl) {
			drifted = append(drifted, filepath.ToSlash(filepath.Join(sp.dir, sp.file)))
		}
	}
	return drifted
}

func hasJSONManifestDrift(targetPath string, templateBytes []byte) bool {
	existing, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	normExisting, nErr := tui.MergeHookManifest(existing, []byte("{}"))
	if nErr != nil {
		return true
	}
	merged, mErr := tui.MergeHookManifest(existing, templateBytes)
	if mErr != nil {
		return false
	}
	return !bytes.Equal(bytes.TrimSpace(normExisting), bytes.TrimSpace(merged))
}

func hasOpenCodePluginDrift(repoRoot string) bool {
	target := filepath.Join(repoRoot, openCodeDirToken, "plugins", "nomos.ts")
	existing, err := os.ReadFile(target)
	if err != nil {
		return false
	}
	tmpl, tErr := selfupdate.DefaultOpenCodePluginTS()
	if tErr != nil {
		return false
	}
	merged := selfupdate.MergeOpenCodePluginContent(existing, tmpl)
	return !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(merged))
}

func detectAntigravityBundleDrift(repoRoot string) []string {
	manifestPath := filepath.Join(repoRoot, hookcmd.AgentsDirToken, hookcmd.HooksFileName)
	if !fileExists(manifestPath) {
		return nil
	}
	var drifted []string
	if tmpl, tErr := initTemplates.ReadFile("templates/hooks/hooks.json"); tErr == nil {
		if hasJSONManifestDrift(manifestPath, tmpl) {
			drifted = append(drifted, filepath.ToSlash(filepath.Join(hookcmd.AgentsDirToken, hookcmd.HooksFileName)))
		}
	}
	scriptsDir := filepath.Join(repoRoot, hookcmd.AgentsDirToken, "scripts")
	for _, name := range canonicalHookBundleScripts {
		expected, tErr := initTemplates.ReadFile("templates/hooks/scripts/" + name)
		if tErr != nil {
			continue
		}
		actual, rErr := os.ReadFile(filepath.Join(scriptsDir, name))
		if rErr != nil || !bytes.Equal(actual, expected) {
			drifted = append(drifted, filepath.ToSlash(filepath.Join(hookcmd.AgentsDirToken, "scripts")))
			break
		}
	}
	return drifted
}
