// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"os"
	"path/filepath"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func collectGovernanceStagePaths(absDir string) []string {
	candidates := []string{
		".gitignore",
		constants.FileNomosYAML,
		constants.FileNomosBaselineJSON,
		"AGENTS.md",
		"CLAUDE.md",
		agentsDirToken,
		filepath.Join(claudeDirToken, settingsFileName),
		filepath.Join(geminiDirToken, settingsFileName),
		filepath.Join(cursorDirToken, hooksFileName),
		filepath.Join(openCodeDirToken, "plugins", "nomos.ts"),
	}
	existing := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(absDir, rel)); err == nil {
			existing = append(existing, rel)
		}
	}
	return existing
}

func stageAndCommitGovernanceOnly(absDir string) (bool, error) {
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
		return false, nil
	}
	return true, nil
}
