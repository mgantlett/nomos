// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// VersionInfo models structured build provenance for 'nomos version --json'.
type VersionInfo struct {
	Version   string `json:"version"`
	GOOS      string `json:"goos"`
	GOARCH    string `json:"goarch"`
	GoVersion string `json:"go_version"`
	Edition   string `json:"edition"`
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
			out := cmd.OutOrStdout()
			if jsonOutput {
				info := VersionInfo{
					Version:   RootCmdVersion,
					GOOS:      runtime.GOOS,
					GOARCH:    runtime.GOARCH,
					GoVersion: runtime.Version(),
					Edition:   "open-core",
				}
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			fmt.Fprintf(out, "%s %s %s\n%s %s %s %s\n",
				renderBrandTitle("nomos version"),
				uiChipStyle.Render(" "+RootCmdVersion+" "),
				uiMutedStyle.Render(fmt.Sprintf("(%s/%s)", runtime.GOOS, runtime.GOARCH)),
				uiLabelStyle.Render("compiler:"),
				uiValueStyle.Render(runtime.Version()),
				uiIntentStyle.Render("◈"),
				uiMutedStyle.Render("open-core substrate · sophialabs.ca"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit version metadata as structured JSON")
	return cmd
}
