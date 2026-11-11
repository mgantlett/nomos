// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
// @intent intent.md
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/spf13/cobra"
)

const (
	installMethodStandalone = "standalone"
	installMethodHomebrew   = "homebrew"
	installMethodAUR        = "aur"
	installMethodNix        = "nix"

	upgradeCmdStandalone = "nomos update"
	upgradeCmdHomebrew   = "brew upgrade --cask mgantlett/tap/nomos"
	upgradeCmdAUR        = "yay -Syu nomos-bin"
	upgradeCmdNix        = "nix profile upgrade nomos"

	updateHTTPTimeout = 30 * time.Second
)

// UpdateReport summarizes binary update evaluation, provenance guard, and workspace contract sync (#UpdateReport).
// @intent task_lifecycle_cli_spec.md #UpdateReport
type UpdateReport struct {
	CurrentVersion        string   `json:"current_version"`
	TargetVersion         string   `json:"target_version"`
	UpdateAvailable       bool     `json:"update_available"`
	Updated               bool     `json:"updated"`
	CheckOnly             bool     `json:"check_only,omitempty"`
	InstallMethod         string   `json:"install_method"`
	BinaryPath            string   `json:"binary_path"`
	UpgradeCommand        string   `json:"upgrade_command,omitempty"`
	ChecksumSHA256        string   `json:"checksum_sha256,omitempty"`
	WorkspaceDrift        bool     `json:"workspace_drift"`
	WorkspaceDriftFiles   []string `json:"workspace_drift_files,omitempty"`
	WorkspaceSynchronized bool     `json:"workspace_synchronized,omitempty"`
}

// UpdateCheckCache persists bounded 24-hour release version discovery state (#UpdateCheckCache).
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
type UpdateCheckCache struct {
	CheckedAt     string `json:"checked_at"`
	LatestVersion string `json:"latest_version,omitempty"`
}

type updateOptions struct {
	checkOnly     bool
	targetVersion string
	syncWorkspace bool
	targetDir     string
	jsonOutput    bool
}

// NewUpdateCmd constructs the canonical 'nomos update' command (Law 6 single verb, zero aliases).
// @intent task_lifecycle_cli_spec.md #UpdateReport
func NewUpdateCmd() *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Verify and install SHA-256-attested Nomos binary updates and refresh workspace contracts",
		Long:    helpUpdateLong,
		Example: helpUpdateExample,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeUpdate(cmd, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.checkOnly, "check", false, "Check release availability and workspace contract drift without mutating files")
	cmd.Flags().StringVar(&opts.targetVersion, "version", "", "Explicit release tag to install or inspect (e.g. v4.14.0)")
	cmd.Flags().BoolVar(&opts.syncWorkspace, "sync-workspace", false, "Refresh managed AGENTS.md, CLAUDE.md, ambient hooks, and nomos.yaml schema pin")
	cmd.Flags().StringVarP(&opts.targetDir, "dir", "d", "", "Target repository directory for --sync-workspace (defaults to active repository)")
	cmd.Flags().BoolVar(&opts.jsonOutput, flagNameJSON, false, "Emit structured UpdateReport JSON")
	return cmd
}

// DetectInstallMethod classifies binary ownership to protect package-manager installations.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func DetectInstallMethod(binaryPath string) (string, string) {
	lower := strings.ToLower(filepath.ToSlash(strings.TrimSpace(binaryPath)))
	if strings.Contains(lower, "/opt/homebrew/") ||
		strings.Contains(lower, "/cellar/") ||
		strings.Contains(lower, "/caskroom/") ||
		strings.Contains(lower, "linuxbrew") {
		return installMethodHomebrew, upgradeCmdHomebrew
	}
	if strings.Contains(lower, "/nix/store/") {
		return installMethodNix, upgradeCmdNix
	}
	if strings.HasPrefix(lower, "/usr/bin/") || strings.HasPrefix(lower, "/bin/") {
		return installMethodAUR, upgradeCmdAUR
	}
	return installMethodStandalone, upgradeCmdStandalone
}

func resolveCurrentBinaryPath() (string, error) {
	if custom := strings.TrimSpace(os.Getenv(constants.EnvNomosBinaryPath)); custom != "" {
		return filepath.Abs(custom)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed resolving executable path: %v", err)
	}
	if resolved, rErr := filepath.EvalSymlinks(exe); rErr == nil && resolved != "" {
		return resolved, nil
	}
	return exe, nil
}

func resolveUpdateWorkspaceRoot(explicitDir string) string {
	if strings.TrimSpace(explicitDir) != "" {
		if abs, err := filepath.Abs(explicitDir); err == nil {
			return abs
		}
		return explicitDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if root, rErr := workspace.FindRepoRoot(cwd); rErr == nil && root != "" {
		return root
	}
	return cwd
}

func executeUpdate(cmd *cobra.Command, opts updateOptions) error {
	binPath, err := resolveCurrentBinaryPath()
	if err != nil {
		return err
	}
	method, upgradeCmd := DetectInstallMethod(binPath)
	wsRoot := resolveUpdateWorkspaceRoot(opts.targetDir)

	ctx, cancel := context.WithTimeout(cmd.Context(), updateHTTPTimeout)
	defer cancel()

	targetTag, updateAvail, rErr := resolveDesiredUpdateTarget(ctx, opts)
	if rErr != nil {
		return rErr
	}

	driftFiles := DetectWorkspaceContractDrift(wsRoot)
	rep := UpdateReport{
		CurrentVersion:      RootCmdVersion,
		TargetVersion:       strings.TrimPrefix(targetTag, "v"),
		UpdateAvailable:     updateAvail,
		CheckOnly:           opts.checkOnly,
		InstallMethod:       method,
		BinaryPath:          binPath,
		UpgradeCommand:      upgradeCmd,
		WorkspaceDrift:      len(driftFiles) > 0,
		WorkspaceDriftFiles: driftFiles,
	}

	if err := performRequestedMutations(&ctx, opts, &rep); err != nil {
		return err
	}
	return emitUpdateOutput(cmd.OutOrStdout(), rep, opts.jsonOutput)
}

func resolveDesiredUpdateTarget(ctx context.Context, opts updateOptions) (string, bool, error) {
	if strings.TrimSpace(opts.targetVersion) != "" {
		tag, err := NormalizeReleaseTag(opts.targetVersion)
		if err != nil {
			return "", false, err
		}
		return tag, CompareSemver(RootCmdVersion, tag) != 0, nil
	}
	tag, err := ResolveLatestReleaseTag(ctx)
	if err != nil {
		if opts.syncWorkspace && !opts.checkOnly {
			fallback := "v" + strings.TrimPrefix(RootCmdVersion, "v")
			return fallback, false, nil
		}
		return "", false, err
	}
	if cErr := SaveUpdateCheckCache(UpdateCheckCache{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		LatestVersion: strings.TrimPrefix(tag, "v"),
	}); cErr != nil {
		// non-fatal cache write in read-only environments
	}
	return tag, CompareSemver(RootCmdVersion, tag) < 0, nil
}

func performRequestedMutations(ctx *context.Context, opts updateOptions, rep *UpdateReport) error {
	if opts.checkOnly {
		return nil
	}
	shouldReplaceBinary := rep.UpdateAvailable || strings.TrimSpace(opts.targetVersion) != ""
	if shouldReplaceBinary {
		if rep.InstallMethod != installMethodStandalone {
			return types.NewNomosError(
				types.ErrCodeBoundary,
				"refusing to overwrite %s-managed nomos binary at %s",
				rep.InstallMethod, rep.BinaryPath,
			).WithRemedy(fmt.Sprintf("upgrade via package manager: %s", rep.UpgradeCommand))
		}
		sum, err := downloadVerifyAndReplaceBinary(*ctx, "v"+strings.TrimPrefix(rep.TargetVersion, "v"), rep.BinaryPath)
		if err != nil {
			return err
		}
		rep.Updated = true
		rep.ChecksumSHA256 = sum
		RecordCompletedUpdateCache(rep.TargetVersion, time.Now())
	}
	if opts.syncWorkspace {
		wsRoot := resolveUpdateWorkspaceRoot(opts.targetDir)
		if _, sErr := SyncWorkspaceContracts(wsRoot); sErr != nil {
			return sErr
		}
		rep.WorkspaceSynchronized = true
		rep.WorkspaceDriftFiles = DetectWorkspaceContractDrift(wsRoot)
		rep.WorkspaceDrift = len(rep.WorkspaceDriftFiles) > 0
	}
	return nil
}

func clearDarwinQuarantineBestEffort(binaryPath string) {
	if runtime.GOOS != "darwin" {
		return
	}
	if _, err := exec.LookPath("xattr"); err != nil {
		return
	}
	qCmd := exec.Command("xattr", "-dr", "com.apple.quarantine", binaryPath)
	if qErr := qCmd.Run(); qErr != nil {
		// best-effort quarantine attribute removal
	}
}

func emitUpdateOutput(out io.Writer, rep UpdateReport, jsonOutput bool) error {
	if jsonOutput {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling UpdateReport JSON: %v", err)
		}
		_, wErr := fmt.Fprintln(out, string(data))
		return wErr
	}
	renderHumanUpdateReport(out, rep)
	return nil
}

func renderHumanUpdateReport(out io.Writer, rep UpdateReport) {
	writeOutLine(out, renderBrandTitle("⬆️  Nomos Substrate Update"))
	writeOutLine(out, uiRule(ruleNarrow))
	writeOutLine(out, kvLine("Current Version", 18, uiValueStyle.Render(rep.CurrentVersion)))
	writeOutLine(out, kvLine("Target Version", 18, uiBoldValueStyle.Render(rep.TargetVersion)))
	writeOutLine(out, kvLine("Install Method", 18, uiValueStyle.Render(rep.InstallMethod+" ("+rep.BinaryPath+")")))
	if rep.Updated {
		writeOutLine(out, kvLine("Binary Status", 18, uiSuccessStyle.Render("UPDATED (SHA-256 verified: "+shortDigest(rep.ChecksumSHA256)+")")))
	} else if rep.UpdateAvailable {
		writeOutLine(out, kvLine("Binary Status", 18, uiWarnStyle.Render("UPDATE AVAILABLE ("+rep.UpgradeCommand+")")))
	} else {
		writeOutLine(out, kvLine("Binary Status", 18, uiSuccessStyle.Render("UP TO DATE")))
	}
	if rep.WorkspaceSynchronized {
		writeOutLine(out, kvLine("Workspace Sync", 18, uiSuccessStyle.Render("SYNCHRONIZED")))
	} else if rep.WorkspaceDrift {
		writeOutLine(out, kvLine("Workspace Sync", 18, uiWarnStyle.Render("DRIFT DETECTED ("+strings.Join(rep.WorkspaceDriftFiles, ", ")+") · run 'nomos update --sync-workspace'")))
	}
	writeOutLine(out, uiRule(ruleNarrow))
}

func shortDigest(sum string) string {
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}

func writeOutLine(out io.Writer, line string) {
	if _, err := fmt.Fprintln(out, line); err != nil {
		// stdout closed
	}
}
