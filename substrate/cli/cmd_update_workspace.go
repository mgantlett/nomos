// Package cli implements workspace protocol and ambient hook drift detection and idempotent refresh.
package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const schemaRawPrefix = "https://raw.githubusercontent.com/mgantlett/nomos/v"

var managedHookScriptNames = []string{
	"session-start.sh",
	"worktree-guard.sh",
	"nomos-build-guard.sh",
	"post-edit-radar.sh",
	"stop-guard.sh",
}

// DetectWorkspaceContractDrift inspects a Nomos-governed repository for managed template or schema tag drift.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func DetectWorkspaceContractDrift(repoRoot string) []string {
	cleanRoot := strings.TrimSpace(repoRoot)
	if cleanRoot == "" {
		return nil
	}
	configPath := filepath.Join(cleanRoot, constants.FileNomosYAML)
	if _, err := os.Stat(configPath); err != nil {
		return nil
	}

	var drifted []string
	if hasSchemaPinDrift(configPath) {
		drifted = append(drifted, constants.FileNomosYAML)
	}
	if hasProtocolFileDrift(filepath.Join(cleanRoot, "AGENTS.md")) {
		drifted = append(drifted, "AGENTS.md")
	}
	if hasClaudeFileDrift(filepath.Join(cleanRoot, "CLAUDE.md")) {
		drifted = append(drifted, "CLAUDE.md")
	}
	if hasAmbientHookScriptsDrift(cleanRoot) {
		drifted = append(drifted, filepath.ToSlash(filepath.Join(agentsDirToken, "scripts")))
	}
	return drifted
}

func hasSchemaPinDrift(configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	pinVer := extractNomosSchemaPinVersion(string(data))
	if pinVer == "" {
		return false
	}
	return CompareSemver(pinVer, RootCmdVersion) < 0
}

func extractNomosSchemaPinVersion(content string) string {
	idx := strings.Index(content, schemaRawPrefix)
	if idx == -1 {
		return ""
	}
	rest := content[idx+len(schemaRawPrefix):]
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return ""
	}
	return strings.TrimSpace(rest[:slash])
}

func hasProtocolFileDrift(targetPath string) bool {
	existing, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	tmpl, tErr := initTemplates.ReadFile("templates/AGENTS.md")
	if tErr != nil {
		return false
	}
	merged := mergeProtocolContent(existing, tmpl)
	return !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(merged))
}

func hasClaudeFileDrift(targetPath string) bool {
	existing, err := os.ReadFile(targetPath)
	if err != nil {
		return false
	}
	merged := mergeClaudeImportContent(existing)
	return !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(merged))
}

func hasAmbientHookScriptsDrift(repoRoot string) bool {
	manifestPath := filepath.Join(repoRoot, agentsDirToken, hooksFileName)
	if _, err := os.Stat(manifestPath); err != nil {
		return false
	}
	scriptsDir := filepath.Join(repoRoot, agentsDirToken, "scripts")
	for _, name := range managedHookScriptNames {
		expected, tErr := initTemplates.ReadFile("templates/hooks/scripts/" + name)
		if tErr != nil {
			continue
		}
		actual, rErr := os.ReadFile(filepath.Join(scriptsDir, name))
		if rErr != nil || !bytes.Equal(actual, expected) {
			return true
		}
	}
	return false
}

// SyncWorkspaceContracts refreshes managed AGENTS.md, CLAUDE.md, ambient hooks, schema pin, and git hooks in repoRoot.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func SyncWorkspaceContracts(repoRoot string) ([]string, error) {
	absDir, err := validateSyncWorkspaceDir(repoRoot)
	if err != nil {
		return nil, err
	}
	refreshed, rErr := refreshManagedWorkspaceArtifacts(absDir)
	if rErr != nil {
		return nil, rErr
	}
	if gErr := refreshWorkspaceGitHooks(absDir); gErr != nil {
		return nil, gErr
	}
	return refreshed, nil
}

func validateSyncWorkspaceDir(repoRoot string) (string, error) {
	if strings.TrimSpace(repoRoot) == "" {
		return "", types.NewNomosError(types.ErrCodeBoundary, "workspace directory is required for --sync-workspace")
	}
	absDir, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed resolving workspace path %s: %v", repoRoot, err)
	}
	configPath := filepath.Join(absDir, constants.FileNomosYAML)
	if _, sErr := os.Stat(configPath); sErr != nil {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "no %s found in %s", constants.FileNomosYAML, absDir).
			WithRemedy("run 'nomos init' first to initialize Nomos governance in this repository")
	}
	return absDir, nil
}

func refreshManagedWorkspaceArtifacts(absDir string) ([]string, error) {
	var refreshed []string
	configPath := filepath.Join(absDir, constants.FileNomosYAML)
	updated, uErr := refreshNomosSchemaPin(configPath)
	if uErr != nil {
		return nil, uErr
	}
	if updated {
		refreshed = append(refreshed, constants.FileNomosYAML)
	}
	if err := scaffoldAgentsConfig(absDir); err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed refreshing AGENTS.md: %v", err)
	}
	refreshed = append(refreshed, "AGENTS.md")

	if _, cStatErr := os.Stat(filepath.Join(absDir, "CLAUDE.md")); cStatErr == nil {
		if err := scaffoldClaudeConfig(absDir); err != nil {
			return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed refreshing CLAUDE.md: %v", err)
		}
		refreshed = append(refreshed, "CLAUDE.md")
	}
	if err := scaffoldAmbientHooks(absDir); err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed refreshing ambient lifecycle hooks: %v", err)
	}
	refreshed = append(refreshed, filepath.ToSlash(filepath.Join(agentsDirToken, hooksFileName)))
	return refreshed, nil
}

func refreshWorkspaceGitHooks(absDir string) error {
	gitDir := filepath.Join(absDir, constants.DirGit)
	fi, err := os.Stat(gitDir)
	if err != nil || !fi.IsDir() {
		return nil
	}
	if hErr := workspace.InstallHooks(absDir); hErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed refreshing git defense hooks: %v", hErr)
	}
	return nil
}

func refreshNomosSchemaPin(configPath string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	content := string(data)
	pinVer := extractNomosSchemaPinVersion(content)
	if pinVer == "" || CompareSemver(pinVer, RootCmdVersion) >= 0 {
		return false, nil
	}
	oldToken := schemaRawPrefix + pinVer + "/nomos.schema.json"
	newToken := schemaRawPrefix + strings.TrimPrefix(RootCmdVersion, "v") + "/nomos.schema.json"
	updated := strings.Replace(content, oldToken, newToken, 1)
	if updated == content {
		return false, nil
	}
	if wErr := os.WriteFile(configPath, []byte(updated), 0644); wErr != nil {
		return false, types.NewNomosError(types.ErrCodeSubstrate, "failed updating schema pin in %s: %v", configPath, wErr)
	}
	return true, nil
}

// FormatWorkspaceDriftNotice returns a 1-line notice when managed workspace contracts lag behind the installed binary.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func FormatWorkspaceDriftNotice(repoRoot string) string {
	drifted := DetectWorkspaceContractDrift(repoRoot)
	if len(drifted) == 0 {
		return ""
	}
	return fmt.Sprintf("managed contracts behind v%s (%s) · run 'nomos update --sync-workspace'",
		strings.TrimPrefix(RootCmdVersion, "v"), strings.Join(drifted, ", "))
}
