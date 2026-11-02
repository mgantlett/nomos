// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// FormatVersionOutput returns the canonical version and build telemetry string.
func FormatVersionOutput() string {
	return fmt.Sprintf("nomos version %s (%s/%s)\ncompiler: %s · open-core substrate\n",
		RootCmdVersion, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

// NewVersionCmd constructs the canonical 'nomos version' command.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display Nomos substrate version and build telemetry",
		Run: func(cmd *cobra.Command, args []string) {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s %s %s\n%s %s %s\n",
				uiTitleStyle.Render("nomos version"),
				uiBoldValueStyle.Render(RootCmdVersion),
				uiMutedStyle.Render(fmt.Sprintf("(%s/%s)", runtime.GOOS, runtime.GOARCH)),
				uiLabelStyle.Render("compiler:"),
				uiValueStyle.Render(runtime.Version()),
				uiMutedStyle.Render("· open-core substrate"))
		},
	}
}
