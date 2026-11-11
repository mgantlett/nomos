// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
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
		RootCmdVersion, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// NewVersionCmd constructs the canonical 'nomos version' command.
func NewVersionCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Display Nomos substrate version and build telemetry",
		Long:    helpVersionLong,
		Example: helpVersionExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeVersion(cmd.OutOrStdout(), jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit version metadata as structured JSON")
	return cmd
}

func executeVersion(out io.Writer, jsonOutput bool) error {
	binPath, bErr := resolveCurrentBinaryPath()
	if bErr != nil {
		binPath = ""
	}
	latestVer, notice := ReadCachedUpdateNotice(RootCmdVersion, binPath)
	if jsonOutput {
		info := VersionInfo{
			Version:         RootCmdVersion,
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
		renderBrandTitle("nomos version"),
		uiChipStyle.Render(" "+RootCmdVersion+" "),
		uiMutedStyle.Render(fmt.Sprintf("(%s/%s)", runtime.GOOS, runtime.GOARCH))))
	writeOutLine(out, fmt.Sprintf("%s %s %s %s",
		uiLabelStyle.Render("compiler:"),
		uiValueStyle.Render(runtime.Version()),
		uiIntentStyle.Render("◈"),
		uiMutedStyle.Render("open-core substrate · sophialabs.ca")))
	if notice != "" {
		writeOutLine(out, fmt.Sprintf("%s %s",
			uiWarnStyle.Render("update:"),
			uiValueStyle.Render(notice)))
	}
	return nil
}
