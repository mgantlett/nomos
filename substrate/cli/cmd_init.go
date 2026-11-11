// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/spf13/cobra"
)

//go:embed templates/* templates/hooks/* templates/hooks/scripts/*
var initTemplates embed.FS

// NewInitCmd creates the workspace initialization command.
func NewInitCmd() *cobra.Command {
	var targetDir string

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a new Nomos workspace with operational boundaries, tracking DB, and git defense hooks",
		Long:    helpInitLong,
		Example: helpInitExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := targetDir
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed resolving current working directory: %w", err)
				}
				dir = cwd
			}
			return executeInit(dir)
		},
	}

	cmd.Flags().StringVarP(&targetDir, "dir", "d", "", "Target directory to initialize (defaults to current directory)")
	return cmd
}

func executeInit(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed resolving absolute path for %s: %v", dir, err)
	}

	if err := scaffoldAllGovernanceFiles(absDir); err != nil {
		return err
	}

	isGitRepo, createdBaselineCommit := initializeGitGovernance(absDir)

	if dbErr := verifyDatabaseConnectivity(); dbErr != nil {
		return dbErr
	}

	defaultBranch := detectDefaultBranch(absDir)
	printInitializationSummary(absDir, isGitRepo, createdBaselineCommit, defaultBranch)
	return nil
}

func scaffoldAllGovernanceFiles(absDir string) error {
	steps := []struct {
		label string
		fn    func(string) error
	}{
		{"workspace boundaries", scaffoldNomosBoundaries},
		{"nomos.yaml", scaffoldNomosConfig},
		{"AGENTS.md", scaffoldAgentsConfig},
		{"CLAUDE.md", scaffoldClaudeConfig},
	}
	for _, s := range steps {
		if err := s.fn(absDir); err != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed scaffolding %s: %v", s.label, err)
		}
	}
	pruneLegacyWorkflows(absDir)
	if err := scaffoldAmbientHooks(absDir); err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed scaffolding ambient lifecycle hooks: %v", err)
	}
	return nil
}

func initializeGitGovernance(absDir string) (bool, bool) {
	gitDir := filepath.Join(absDir, ".git")
	fi, err := os.Stat(gitDir)
	if err != nil || !fi.IsDir() {
		fmt.Printf("⚠️ %s is not a git repository. Run 'git init' to enable Git defense hooks and worktree isolation.\n", absDir)
		return false, false
	}

	createdCommit, stageErr := stageAndCommitGovernanceOnly(absDir)
	if stageErr != nil {
		fmt.Printf("⚠️ failed staging governance files: %v\n", stageErr)
	}

	if hookErr := workspace.InstallHooks(absDir); hookErr != nil {
		fmt.Printf("⚠️ failed installing git defense hooks: %v\n", hookErr)
	}
	return true, createdCommit
}

func verifyDatabaseConnectivity() error {
	db, err := storage.OpenDB("")
	if err != nil {
		return types.NewNomosError(types.ErrCodeCore, "failed connecting to Living Ledger database: %v", err)
	}
	if cErr := db.Close(); cErr != nil {
		// safe to continue
	}
	return nil
}

func detectDefaultBranch(dir string) string {
	cmd := workspace.SafeGitCommand(dir, "branch", "--show-current")
	if out, err := cmd.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" && b != "HEAD" {
			return b
		}
	}
	symCmd := workspace.SafeGitCommand(dir, "symbolic-ref", "--short", "HEAD")
	if out, err := symCmd.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" {
			return b
		}
	}
	cfgCmd := workspace.SafeGitCommand(dir, "config", "--get", "init.defaultBranch")
	if out, err := cfgCmd.Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" {
			return b
		}
	}
	return constants.ValMain
}

func printInitializationSummary(absDir string, isGitRepo, createdBaselineCommit bool, defaultBranch string) {
	fmt.Printf("%s %s\n", renderBrandTitle("🚀 Initialized Nomos workspace in"), uiBoldValueStyle.Render(absDir))
	fmt.Println(uiRule(ruleNarrow))
	check := uiSuccessStyle.Render("✅")
	fmt.Printf("  %s Scaffolded operational boundaries %s\n", check, uiMutedStyle.Render("(worktrees/, .tmp/)"))
	fmt.Printf("  %s Scaffolded downstream configuration %s\n", check, uiCommandStyle.Render("(nomos.yaml)"))
	fmt.Printf("  %s Scaffolded universal agent protocol %s\n", check, uiCommandStyle.Render("(AGENTS.md)"))
	fmt.Printf("  %s Scaffolded Tier 1 agent protocol %s\n", check, uiCommandStyle.Render("(CLAUDE.md)"))
	fmt.Printf("  %s Scaffolded ambient session & defense hooks %s\n", check, uiMutedStyle.Render("(Claude Code, Gemini CLI, OpenCode, Antigravity)"))
	if createdBaselineCommit {
		fmt.Printf("  %s Created initial baseline commit on %s\n", check, uiBoldValueStyle.Render(defaultBranch))
	}
	if isGitRepo {
		fmt.Printf("  %s Installed git defense hooks %s\n", check, uiMutedStyle.Render("(pre-commit, commit-msg, pre-push)"))
	} else {
		fmt.Printf("  %s\n", uiWarnStyle.Render("⚠️ Git defense hooks not installed (run 'git init' to enable)"))
	}
	fmt.Printf("  %s Living Ledger storage connected\n", check)
	fmt.Println(uiRule(ruleNarrow))
	if createdBaselineCommit {
		fmt.Println(uiSuccessStyle.Render("✨ Workspace ready for Tier 1 agent indirect control."))
	} else if isGitRepo {
		fmt.Println(uiSuccessStyle.Render("✨ Workspace ready. Run 'git add . && git commit -m \"chore: initialize Nomos governance\"' to establish your baseline."))
	} else {
		fmt.Println(uiWarnStyle.Render("✨ Workspace ready. Run 'git init' followed by initial commit to enable full governance."))
	}
	fmt.Printf("  %s\n", uiMutedStyle.Render(fmt.Sprintf("💡 Live agent edits run inside isolated worktrees/%s-<task> sandboxes and merge into %s on completion.", filepath.Base(absDir), defaultBranch)))
}

func scaffoldNomosConfig(dir string) error {
	configPath := filepath.Join(dir, "nomos.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}
	tmplBytes, err := initTemplates.ReadFile("templates/nomos.yaml")
	if err != nil {
		return err
	}
	projectName := filepath.Base(dir)
	branch := detectDefaultBranch(dir)
	content := renderScaffoldedNomosYAML(dir, string(tmplBytes), projectName, branch)
	return os.WriteFile(configPath, []byte(content), 0644)
}

const (
	protocolStartMarker = "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->"
	protocolEndMarker   = "<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->"
)

func scaffoldClaudeConfig(dir string) error {
	claudePath := filepath.Join(dir, "CLAUDE.md")
	return scaffoldOrMergeProtocolFile(claudePath)
}

func scaffoldAgentsConfig(dir string) error {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	return scaffoldOrMergeProtocolFile(agentsPath)
}

func scaffoldOrMergeProtocolFile(targetPath string) error {
	tmplBytes, err := initTemplates.ReadFile("templates/AGENTS.md")
	if err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, tmplBytes, 0644)
		}
		return err
	}

	mergedBytes := mergeProtocolContent(existingBytes, tmplBytes)
	return os.WriteFile(targetPath, mergedBytes, 0644)
}

func mergeProtocolContent(existingBytes, templateBytes []byte) []byte {
	existing := string(existingBytes)
	template := strings.TrimSpace(string(templateBytes))

	if strings.TrimSpace(existing) == "" {
		return []byte(template + "\n")
	}

	startIdx := strings.Index(existing, protocolStartMarker)
	endIdx := strings.Index(existing, protocolEndMarker)

	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		endIdx += len(protocolEndMarker)
		var sb strings.Builder
		sb.WriteString(existing[:startIdx])
		sb.WriteString(template)
		sb.WriteString(existing[endIdx:])
		return []byte(sb.String())
	}

	var sb strings.Builder
	sb.WriteString(template)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString(strings.TrimSpace(existing))
	sb.WriteString("\n")
	return []byte(sb.String())
}

func pruneLegacyWorkflows(dir string) {
	wfDir := filepath.Join(dir, ".agents", "workflows")
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

func scaffoldAmbientHooks(dir string) error {
	agentsDir := filepath.Join(dir, ".agents")
	if err := writeAmbientHookBundle(agentsDir); err != nil {
		return err
	}
	claudeSettingsPath := filepath.Join(dir, claudeDirToken, settingsFileName)
	if cErr := writeMergedJSONManifest(claudeSettingsPath, DefaultClaudeSettingsHooksJSON()); cErr != nil {
		return cErr
	}
	geminiSettingsPath := filepath.Join(dir, geminiDirToken, settingsFileName)
	if gErr := writeMergedJSONManifest(geminiSettingsPath, DefaultGeminiSettingsHooksJSON()); gErr != nil {
		return gErr
	}
	return scaffoldOpenCodePlugin(dir)
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
	if mErr := scaffoldOrMergeHookManifestFile(filepath.Join(rootDir, "hooks.json"), manifestBytes); mErr != nil {
		return mErr
	}
	scripts := []string{"session-start.sh", "worktree-guard.sh", "nomos-build-guard.sh", "post-edit-radar.sh", "stop-guard.sh"}
	for _, name := range scripts {
		targetPath := filepath.Join(scriptsDir, name)
		tmplBytes, err := initTemplates.ReadFile("templates/hooks/scripts/" + name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, tmplBytes, 0755); err != nil {
			return err
		}
		if cErr := os.Chmod(targetPath, 0755); cErr != nil {
			return cErr
		}
	}
	return nil
}

func scaffoldOrMergeHookManifestFile(targetPath string, templateBytes []byte) error {
	existingBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, templateBytes, 0644)
		}
		return err
	}
	mergedBytes, mErr := MergeHookManifest(existingBytes, templateBytes)
	if mErr != nil {
		return mErr
	}
	return os.WriteFile(targetPath, mergedBytes, 0644)
}


