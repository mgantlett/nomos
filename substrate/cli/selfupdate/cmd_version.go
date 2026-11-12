// Package selfupdate implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package selfupdate

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"runtime"

	"github.com/spf13/cobra"
)

// VersionInfo models structured build provenance for 'nomos version --json'.
type VersionInfo struct {
	Version         string `json:"version"`
	GOOS            string `json:"goos"`
	GOARCH          string `json:"goarch"`
	GoVersion       string `json:"go_version"`
	Edition         string `json:"edition"`
	LatestVersion   string `json:"latest_version,omitempty"`
	UpdateAvailable bool   `json:"update_available,omitempty"`
}

// FormatVersionOutput returns the canonical version and build telemetry string.
func FormatVersionOutput() string {
	return fmt.Sprintf("nomos version %s (%s/%s)\ncompiler: %s · open-core substrate\n",
		tui.ResolveAppVersion(), runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// NewVersionCmd constructs the canonical 'nomos version' command.
func NewVersionCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Display Nomos substrate version and build telemetry",
		Long:    tui.HelpVersionLong,
		Example: tui.HelpVersionExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeVersion(cmd.OutOrStdout(), jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit version metadata as structured JSON")
	return cmd
}

func executeVersion(out io.Writer, jsonOutput bool) error {
	binPath, bErr := ResolveCurrentBinaryPath()
	if bErr != nil {
		binPath = ""
	}
	latestVer, notice := ReadCachedUpdateNotice(tui.ResolveAppVersion(), binPath)
	if jsonOutput {
		info := VersionInfo{
			Version:         tui.ResolveAppVersion(),
			GOOS:            runtime.GOOS,
			GOARCH:          runtime.GOARCH,
			GoVersion:       runtime.Version(),
			Edition:         "open-core",
			LatestVersion:   latestVer,
			UpdateAvailable: notice != "",
		}
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		_, wErr := fmt.Fprintln(out, string(data))
		return wErr
	}
	writeOutLine(out, fmt.Sprintf("%s %s %s",
		tui.RenderBrandTitle("nomos version"),
		tui.UiChipStyle.Render(" "+tui.ResolveAppVersion()+" "),
		tui.UiMutedStyle.Render(fmt.Sprintf("(%s/%s)", runtime.GOOS, runtime.GOARCH))))
	writeOutLine(out, fmt.Sprintf("%s %s %s %s",
		tui.UiLabelStyle.Render("compiler:"),
		tui.UiValueStyle.Render(runtime.Version()),
		tui.UiIntentStyle.Render("◈"),
		tui.UiMutedStyle.Render("open-core substrate · sophialabs.ca")))
	if notice != "" {
		writeOutLine(out, fmt.Sprintf("%s %s",
			tui.UiWarnStyle.Render("update:"),
			tui.UiValueStyle.Render(notice)))
	}
	return nil
}
