// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	claudeDirToken   = ".claude"
	geminiDirToken   = ".gemini"
	openCodeDirToken = ".opencode"
	settingsFileName = "settings.json"
)

// HookInstallReceipt summarizes additively installed or merged Tier 1 agent hook manifests.
type HookInstallReceipt struct {
	Scope   string   `json:"scope"`
	Targets []string `json:"targets"`
}

// DefaultGeminiSettingsHooksJSON returns the canonical Gemini CLI settings.json hook structure.
func DefaultGeminiSettingsHooksJSON() []byte {
	return buildAgentHookSettingsJSON([]agentHookEventSpec{
		{event: "SessionStart", subcmd: "session-start", timeout: 10000},
		{event: "BeforeTool", matcher: ".*", subcmd: "pre-tool", timeout: 10000},
		{event: "AfterTool", matcher: "write_file|replace|edit.*", subcmd: "post-tool", timeout: 5000},
		{event: "AfterAgent", subcmd: "stop", timeout: 10000},
	})
}

type hookInstallFlags struct {
	claude     bool
	gemini     bool
	opencode   bool
	userScope  bool
	targetDir  string
	jsonOutput bool
}

func newHookInstallCmd() *cobra.Command {
	var f hookInstallFlags
	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install or additively merge Nomos lifecycle hooks for Claude Code, Gemini CLI, or OpenCode",
		Long:    helpHookInstallLong,
		Example: helpHookInstallExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			receipt, err := executeHookInstall(f)
			if err != nil {
				return err
			}
			if f.jsonOutput {
				enc, mErr := json.MarshalIndent(receipt, "", "  ")
				if mErr != nil {
					return mErr
				}
				_, wErr := fmt.Fprintln(cmd.OutOrStdout(), string(enc))
				return wErr
			}
			for _, t := range receipt.Targets {
				if _, wErr := fmt.Fprintf(cmd.OutOrStdout(), "✅ Installed Nomos hooks (%s): %s\n", receipt.Scope, t); wErr != nil {
					return wErr
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&f.claude, "claude", false, "Install or merge Claude Code hooks (SessionStart, PreToolUse, PostToolUse, Stop)")
	cmd.Flags().BoolVar(&f.gemini, "gemini", false, "Install or merge Gemini CLI hooks (SessionStart, BeforeTool, AfterTool, AfterAgent)")
	cmd.Flags().BoolVar(&f.opencode, "opencode", false, "Install or merge OpenCode governance plugin (nomos.ts)")
	cmd.Flags().BoolVar(&f.userScope, "user", false, "Install into user-global agent configuration directories instead of project scope")
	cmd.Flags().StringVarP(&f.targetDir, "dir", "d", "", "Project directory when --user is false (defaults to current directory)")
	cmd.Flags().BoolVar(&f.jsonOutput, flagNameJSON, false, "Emit structured JSON installation receipt")
	return cmd
}

func executeHookInstall(f hookInstallFlags) (*HookInstallReceipt, error) {
	allProviders := !f.claude && !f.gemini && !f.opencode
	wantClaude := f.claude || allProviders
	wantGemini := f.gemini || allProviders
	wantOpenCode := f.opencode || allProviders

	baseDir, scope, err := resolveHookInstallBase(f.userScope, f.targetDir)
	if err != nil {
		return nil, err
	}
	var installed []string
	if wantClaude {
		p := resolveClaudeManifestPath(f.userScope, baseDir)
		if err := writeMergedJSONManifest(p, DefaultClaudeSettingsHooksJSON()); err != nil {
			return nil, err
		}
		installed = append(installed, p)
	}
	if wantGemini {
		p := resolveGeminiManifestPath(f.userScope, baseDir)
		if err := writeMergedJSONManifest(p, DefaultGeminiSettingsHooksJSON()); err != nil {
			return nil, err
		}
		installed = append(installed, p)
	}
	if wantOpenCode {
		p := resolveOpenCodePluginPath(f.userScope, baseDir)
		if err := writeMergedOpenCodePlugin(p); err != nil {
			return nil, err
		}
		installed = append(installed, p)
	}
	return &HookInstallReceipt{Scope: scope, Targets: installed}, nil
}

func resolveHookInstallBase(userScope bool, targetDir string) (string, string, error) {
	if userScope {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("failed resolving user home directory: %w", err)
		}
		return home, "user", nil
	}
	dir := strings.TrimSpace(targetDir)
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("failed resolving working directory: %w", err)
		}
		dir = wd
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	return absDir, "project", nil
}

func resolveClaudeManifestPath(userScope bool, baseDir string) string {
	if userScope {
		if envDir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); envDir != "" {
			return filepath.Join(envDir, settingsFileName)
		}
	}
	return filepath.Join(baseDir, claudeDirToken, settingsFileName)
}

func resolveGeminiManifestPath(userScope bool, baseDir string) string {
	if userScope {
		if envDir := strings.TrimSpace(os.Getenv("GEMINI_CONFIG_DIR")); envDir != "" {
			return filepath.Join(envDir, settingsFileName)
		}
	}
	return filepath.Join(baseDir, geminiDirToken, settingsFileName)
}

func resolveOpenCodePluginPath(userScope bool, baseDir string) string {
	if !userScope {
		return filepath.Join(baseDir, openCodeDirToken, "plugins", "nomos.ts")
	}
	if envDir := strings.TrimSpace(os.Getenv("OPENCODE_CONFIG_DIR")); envDir != "" {
		return filepath.Join(envDir, "plugins", "nomos.ts")
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode", "plugins", "nomos.ts")
	}
	return filepath.Join(baseDir, ".config", "opencode", "plugins", "nomos.ts")
}

func writeMergedJSONManifest(targetPath string, templateBytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	return scaffoldOrMergeHookManifestFile(targetPath, templateBytes)
}

func writeMergedOpenCodePlugin(targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	tmplBytes, err := DefaultOpenCodePluginTS()
	if err != nil {
		return err
	}
	existingBytes, rErr := os.ReadFile(targetPath)
	if rErr != nil {
		if os.IsNotExist(rErr) {
			return os.WriteFile(targetPath, tmplBytes, 0644)
		}
		return rErr
	}
	merged := mergeOpenCodePluginContent(existingBytes, tmplBytes)
	return os.WriteFile(targetPath, merged, 0644)
}
