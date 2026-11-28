// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	localStealthStartMarker = "# Nomos Local Stealth Trial Start"
	localStealthEndMarker   = "# Nomos Local Stealth Trial End"
)

func localStealthPatterns(includeWorktrees bool) []string {
	patterns := []string{
		"/" + constants.FileNomosYAML,
		"/" + constants.FileNomosBaselineJSON,
		"/AGENTS.md",
		"/CLAUDE.md",
		"/" + constants.DirNomosHome + "/",
		"/" + constants.DirTmp + "/",
		"/" + hookcmd.AgentsDirToken + "/",
		"/" + claudeDirToken + "/",
		"/" + hookcmd.CursorDirToken + "/",
		"/" + geminiDirToken + "/",
		"/" + openCodeDirToken + "/",
		"/" + filepath.ToSlash(filepath.Join(tui.VSCodeHooksDirToken, tui.VSCodeHooksSubdirToken, tui.VSCodeHooksFileName)),
		"/go.work",
		"/go.work.sum",
		"*.tmp",
		"*.bak",
		"*.orig",
		"*.swp",
	}
	if includeWorktrees {
		patterns = append(patterns, "/"+constants.DirWorktrees+"/")
	}
	return patterns
}

func resolveGitExcludeFile(absDir string) (string, error) {
	gitDir := filepath.Join(absDir, constants.DirGit)
	if out, err := workspace.SafeGitCommand(absDir, "rev-parse", "--git-common-dir").Output(); err == nil {
		raw := strings.TrimSpace(string(out))
		if raw != "" {
			if filepath.IsAbs(raw) {
				gitDir = raw
			} else {
				gitDir = filepath.Join(absDir, raw)
			}
		}
	}
	if _, err := os.Stat(gitDir); err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "--local stealth mode requires an initialized Git repository").
			WithRemedy("run 'git init' first or omit --local")
	}
	infoDir := filepath.Join(gitDir, "info")
	if err := os.MkdirAll(infoDir, 0755); err != nil {
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed creating git info directory: %v", err)
	}
	return filepath.Join(infoDir, "exclude"), nil
}

func stripStealthExcludeBlock(existing string) string {
	lines := strings.Split(existing, "\n")
	out := make([]string, 0, len(lines))
	inside := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == localStealthStartMarker {
			inside = true
			continue
		}
		if trimmed == localStealthEndMarker {
			inside = false
			continue
		}
		if !inside {
			out = append(out, line)
		}
	}
	cleaned := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if cleaned != "" {
		cleaned += "\n"
	}
	return cleaned
}

func formatStealthExcludeBlock(includeWorktrees bool) string {
	var sb strings.Builder
	sb.WriteString(localStealthStartMarker)
	sb.WriteString("\n")
	for _, pat := range localStealthPatterns(includeWorktrees) {
		sb.WriteString(pat)
		sb.WriteString("\n")
	}
	sb.WriteString(localStealthEndMarker)
	return sb.String()
}

func extractStealthExcludeBlock(existing string) (string, bool) {
	start := strings.Index(existing, localStealthStartMarker)
	end := strings.Index(existing, localStealthEndMarker)
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	return existing[start : end+len(localStealthEndMarker)], true
}

func renderLocalStealthExcludeContent(existing string, includeWorktrees bool) string {
	cleaned := stripStealthExcludeBlock(existing)
	var sb strings.Builder
	sb.WriteString(cleaned)
	if cleaned != "" && !strings.HasSuffix(cleaned, "\n\n") {
		sb.WriteString("\n")
	}
	sb.WriteString(formatStealthExcludeBlock(includeWorktrees))
	sb.WriteString("\n")
	return sb.String()
}

func syncLocalStealthExcludes(absDir string, includeWorktrees bool) error {
	excludePath, err := resolveGitExcludeFile(absDir)
	if err != nil {
		return err
	}
	raw, rErr := os.ReadFile(excludePath)
	if rErr != nil && !os.IsNotExist(rErr) {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed reading %s: %v", excludePath, rErr)
	}
	rendered := renderLocalStealthExcludeContent(string(raw), includeWorktrees)
	if wErr := os.WriteFile(excludePath, []byte(rendered), 0644); wErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed writing %s: %v", excludePath, wErr)
	}
	return nil
}

func isWorktreeIsolationRepo(absDir string) bool {
	cfg, err := config.LoadConfig(absDir)
	return err == nil && cfg != nil && cfg.Governance.Boundary.EffectiveIsolationMode() == config.IsolationModeWorktree
}

func hasLocalStealthExcludeDrift(absDir string) bool {
	if _, err := os.Stat(filepath.Join(absDir, constants.DirGit)); err != nil {
		return false
	}
	excludePath, err := resolveGitExcludeFile(absDir)
	if err != nil {
		return false
	}
	raw, rErr := os.ReadFile(excludePath)
	if rErr != nil {
		return false
	}
	block, ok := extractStealthExcludeBlock(string(raw))
	if !ok {
		return false
	}
	return strings.TrimSpace(block) != strings.TrimSpace(formatStealthExcludeBlock(isWorktreeIsolationRepo(absDir)))
}

func refreshLocalStealthIfPresent(absDir string) error {
	if _, err := os.Stat(filepath.Join(absDir, constants.DirGit)); err != nil {
		return nil
	}
	excludePath, err := resolveGitExcludeFile(absDir)
	if err != nil {
		return nil
	}
	raw, rErr := os.ReadFile(excludePath)
	if rErr != nil || !strings.Contains(string(raw), localStealthStartMarker) {
		return nil
	}
	return syncLocalStealthExcludes(absDir, isWorktreeIsolationRepo(absDir))
}

func clearLocalStealthExcludes(absDir string) error {
	gitDir := filepath.Join(absDir, constants.DirGit)
	if _, err := os.Stat(gitDir); err != nil {
		return nil
	}
	excludePath, err := resolveGitExcludeFile(absDir)
	if err != nil {
		return nil
	}
	raw, rErr := os.ReadFile(excludePath)
	if rErr != nil || !strings.Contains(string(raw), localStealthStartMarker) {
		return nil
	}
	cleaned := stripStealthExcludeBlock(string(raw))
	if wErr := os.WriteFile(excludePath, []byte(cleaned), 0644); wErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed cleaning %s: %v", excludePath, wErr)
	}
	return nil
}

func collectGovernanceStagePaths(absDir string) []string {
	candidates := []string{
		".gitignore",
		constants.FileNomosYAML,
		constants.FileNomosBaselineJSON,
		"AGENTS.md",
		"CLAUDE.md",
		hookcmd.AgentsDirToken,
		filepath.Join(claudeDirToken, settingsFileName),
		filepath.Join(geminiDirToken, settingsFileName),
		filepath.Join(hookcmd.CursorDirToken, hookcmd.HooksFileName),
		filepath.Join(openCodeDirToken, "plugins", "nomos.ts"),
		filepath.Join(tui.VSCodeHooksDirToken, tui.VSCodeHooksSubdirToken, tui.VSCodeHooksFileName),
	}
	existing := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(absDir, rel)); err == nil {
			existing = append(existing, rel)
		}
	}
	return existing
}

func hasExistingReleaseProvenance(absDir string) bool {
	return workspace.SafeGitCommand(absDir, "notes", "--ref=nomos/releases", "show", "HEAD").Run() == nil
}

func isProtectedToolchainTarget(absDir string) bool {
	return os.Getenv("NOMOS_IN_TOOLCHAIN_GATE") == "1" && hasExistingReleaseProvenance(absDir)
}

func stageAndCommitGovernanceOnly(absDir string) (bool, error) {
	if hasExistingReleaseProvenance(absDir) {
		return false, nil
	}
	govPaths := collectGovernanceStagePaths(absDir)
	if len(govPaths) == 0 {
		return false, nil
	}
	addArgs := append([]string{"add", "--"}, govPaths...)
	if err := workspace.SafeGitCommand(absDir, addArgs...).Run(); err != nil {
		return false, err
	}
	diffArgs := append([]string{"diff", "--cached", "--quiet", "--"}, govPaths...)
	if err := workspace.SafeGitCommand(absDir, diffArgs...).Run(); err == nil {
		return false, nil
	}
	commitArgs := append([]string{"commit", "--only", "-m", "chore: initialize Nomos deterministic governance", "--"}, govPaths...)
	commitCmd := workspace.SafeGitCommand(absDir, commitArgs...)
	commitCmd.Env = append(commitCmd.Env, "NOMOS_INIT_IN_PROGRESS=1")
	if err := commitCmd.Run(); err != nil {
		return false, err
	}
	return true, nil
}
