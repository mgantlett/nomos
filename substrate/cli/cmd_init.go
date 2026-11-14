// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"embed"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
	"github.com/spf13/cobra"
)

//go:embed templates/* templates/hooks/* templates/hooks/scripts/*
var initTemplates embed.FS

// NewInitCmd creates the workspace initialization command.
func NewInitCmd() *cobra.Command {
	var targetDir string
	var baselineLock bool

	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a new Nomos workspace with operational boundaries, tracking DB, and git defense hooks",
		Long:    tui.HelpInitLong,
		Example: tui.HelpInitExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := targetDir
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed resolving current working directory: %w", err)
				}
				dir = cwd
			}
			return executeInitWithOptions(dir, baselineLock)
		},
	}

	cmd.Flags().StringVarP(&targetDir, "dir", "d", "", "Target directory to initialize (defaults to current directory)")
	cmd.Flags().BoolVar(&baselineLock, "baseline", false, "Snapshot Day-0 brownfield structural debt bounds into nomos.baseline.json and enforce strict_mode: false")
	return cmd
}

func executeInit(dir string) error {
	return executeInitWithOptions(dir, false)
}

func executeInitWithOptions(dir string, baselineLock bool) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed resolving absolute path for %s: %v", dir, err)
	}

	var preInitXray *xray.XrayReport
	if baselineLock {
		rep, scanErr := xray.ScanRepository(absDir, xray.DefaultScanOptions())
		if scanErr != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed scanning Day-0 baseline: %v", scanErr)
		}
		preInitXray = rep
	}

	forceBrownfield := baselineLock && preInitXray != nil && preInitXray.HangoverScore > 0
	if err := scaffoldAllGovernanceFilesWithMode(absDir, forceBrownfield); err != nil {
		return err
	}
	if baselineLock {
		if _, wErr := xray.WriteBaselineManifest(absDir, preInitXray); wErr != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed writing %s: %v", constants.FileNomosBaselineJSON, wErr)
		}
	}

	isGitRepo, createdBaselineCommit := initializeGitGovernance(absDir)

	if dbErr := verifyDatabaseConnectivity(); dbErr != nil {
		return dbErr
	}
	seedDayZeroBrownfieldMemory(absDir)

	defaultBranch := detectDefaultBranch(absDir)
	printInitializationSummary(absDir, isGitRepo, createdBaselineCommit, defaultBranch)
	if baselineLock && preInitXray != nil {
		fmt.Printf("  %s Locked Day-0 brownfield baseline %s\n",
			tui.UiSuccessStyle.Render("✅"),
			tui.UiCommandStyle.Render(fmt.Sprintf("(%s · score %d/100 · grade %s)", constants.FileNomosBaselineJSON, preInitXray.HangoverScore, preInitXray.Grade)))
	}
	return nil
}

func scaffoldAllGovernanceFiles(absDir string) error {
	return scaffoldAllGovernanceFilesWithMode(absDir, false)
}

func scaffoldAllGovernanceFilesWithMode(absDir string, forceBrownfield bool) error {
	steps := []struct {
		label string
		fn    func(string) error
	}{
		{"workspace boundaries", selfupdate.ScaffoldNomosBoundaries},
		{"nomos.yaml", func(d string) error { return scaffoldNomosConfigWithMode(d, forceBrownfield) }},
		{"AGENTS.md", selfupdate.ScaffoldAgentsConfig},
		{"CLAUDE.md", selfupdate.ScaffoldClaudeConfig},
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

// seedDayZeroBrownfieldMemory persists a Day-0 Mneme architecture baseline for brownfield repositories (@intent task_lifecycle_cli_spec.md).
func seedDayZeroBrownfieldMemory(absDir string) {
	cfg, err := config.LoadConfig(absDir)
	if err != nil || cfg == nil || cfg.Governance.QualityGates.StrictMode {
		return
	}
	bf := handshake.SynthesizeBrownfieldTopology(absDir, cfg)
	summary := handshake.FormatBrownfieldMemorySummary(filepath.Base(absDir), bf)
	if summary == "" {
		return
	}
	if sErr := mem.SeedRepositoryArchitectureBaseline(absDir, summary); sErr != nil {
		// best-effort cold-start memory warm-up
	}
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
	fmt.Printf("%s %s\n", tui.RenderBrandTitle("🚀 Initialized Nomos workspace in"), tui.UiBoldValueStyle.Render(absDir))
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	check := tui.UiSuccessStyle.Render("✅")
	fmt.Printf("  %s Scaffolded operational boundaries %s\n", check, tui.UiMutedStyle.Render("(worktrees/, .tmp/)"))
	fmt.Printf("  %s Scaffolded downstream configuration %s\n", check, tui.UiCommandStyle.Render("(nomos.yaml)"))
	fmt.Printf("  %s Scaffolded universal agent protocol %s\n", check, tui.UiCommandStyle.Render("(AGENTS.md)"))
	fmt.Printf("  %s Scaffolded Tier 1 agent import pointer %s\n", check, tui.UiCommandStyle.Render("(CLAUDE.md -> @AGENTS.md)"))
	fmt.Printf("  %s Scaffolded ambient session & defense hooks %s\n", check, tui.UiMutedStyle.Render("(Claude Code, Gemini CLI, OpenCode, Cursor, Antigravity)"))
	if createdBaselineCommit {
		fmt.Printf("  %s Created initial baseline commit on %s\n", check, tui.UiBoldValueStyle.Render(defaultBranch))
	}
	if isGitRepo {
		fmt.Printf("  %s Installed git defense hooks %s\n", check, tui.UiMutedStyle.Render("(pre-commit, commit-msg, pre-push)"))
	} else {
		fmt.Printf("  %s\n", tui.UiWarnStyle.Render("⚠️ Git defense hooks not installed (run 'git init' to enable)"))
	}
	fmt.Printf("  %s Living Ledger storage connected\n", check)
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	if createdBaselineCommit {
		fmt.Println(tui.UiSuccessStyle.Render("✨ Workspace ready for Tier 1 agent indirect control."))
	} else if isGitRepo {
		fmt.Println(tui.UiSuccessStyle.Render("✨ Workspace ready. Run 'git add . && git commit -m \"chore: initialize Nomos governance\"' to establish your baseline."))
	} else {
		fmt.Println(tui.UiWarnStyle.Render("✨ Workspace ready. Run 'git init' followed by initial commit to enable full governance."))
	}
	fmt.Printf("  %s\n", tui.UiMutedStyle.Render(fmt.Sprintf("💡 Live agent edits run inside isolated worktrees/%s-<task> sandboxes and merge into %s on completion.", filepath.Base(absDir), defaultBranch)))
}

func scaffoldNomosConfig(dir string) error {
	return scaffoldNomosConfigWithMode(dir, false)
}

func scaffoldNomosConfigWithMode(dir string, forceBrownfield bool) error {
	configPath := filepath.Join(dir, constants.FileNomosYAML)
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}
	tmplBytes, err := initTemplates.ReadFile("templates/nomos.yaml")
	if err != nil {
		return err
	}
	projectName := filepath.Base(dir)
	branch := detectDefaultBranch(dir)
	content := selfupdate.RenderScaffoldedNomosYAMLWithOptions(dir, string(tmplBytes), projectName, branch, forceBrownfield)
	return os.WriteFile(configPath, []byte(content), 0644)
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
	agentsDir := filepath.Join(dir, agentsDirToken)
	if err := writeAmbientHookBundle(agentsDir); err != nil {
		return err
	}
	claudeSettingsPath := filepath.Join(dir, claudeDirToken, settingsFileName)
	if cErr := writeMergedJSONManifest(claudeSettingsPath, tui.DefaultClaudeSettingsHooksJSON()); cErr != nil {
		return cErr
	}
	geminiSettingsPath := filepath.Join(dir, geminiDirToken, settingsFileName)
	if gErr := writeMergedJSONManifest(geminiSettingsPath, DefaultGeminiSettingsHooksJSON()); gErr != nil {
		return gErr
	}
	cursorHooksPath := filepath.Join(dir, cursorDirToken, hooksFileName)
	if curErr := writeMergedJSONManifest(cursorHooksPath, DefaultCursorHooksJSON()); curErr != nil {
		return curErr
	}
	return selfupdate.ScaffoldOpenCodePlugin(dir)
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
	mergedBytes, mErr := tui.MergeHookManifest(existingBytes, templateBytes)
	if mErr != nil {
		return mErr
	}
	return os.WriteFile(targetPath, mergedBytes, 0644)
}
