// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/spf13/cobra"
)

// RootCmdVersion defines the statutory release version of Nomos v4.
var RootCmdVersion = "4.1.0"

// NewRootCmd constructs the authoritative root Cobra command for Nomos.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "nomos",
		Version: RootCmdVersion,
		Short:   "Nomos Deterministic Engineering Substrate",
		Long: `Nomos Open-Core Deterministic Engineering Substrate
Governs state, quality gates, and hermetic worktrees under indirect control.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cmd != nil && cmd.Name() != "" {
				telemetry.RecordInvocationSafe("", "cli-"+cmd.Name())
			}
		},
	}
	rootCmd.SetVersionTemplate(FormatVersionOutput())

	registerSubcommands(rootCmd)
	return rootCmd
}

// Execute parses command-line arguments and runs the root command.
func Execute() error {
	rootCmd := NewRootCmd()
	return rootCmd.Execute()
}

func registerSubcommands(root *cobra.Command) {
	root.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Workspace & Lifecycle Commands:"},
		&cobra.Group{ID: "quality", Title: "Quality & Invariant Verification Commands:"},
		&cobra.Group{ID: "agent", Title: "Agent Boot & Cognitive Memory Commands:"},
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics & System Commands:"},
	)

	addCommandWithGroup(root, NewInitCmd(), "lifecycle")
	addCommandWithGroup(root, NewStatusCmd(), "lifecycle")
	addCommandWithGroup(root, NewTaskCmd(), "lifecycle")

	addCommandWithGroup(root, NewVerifyCmd(), "quality")

	addCommandWithGroup(root, NewHandshakeCmd(), "agent")
	addCommandWithGroup(root, NewHookCmd(), "agent")
	addCommandWithGroup(root, NewDirectiveCmd(), "agent")
	addCommandWithGroup(root, NewCapabilitiesCmd(), "agent")
	addCommandWithGroup(root, NewMemCmd(), "agent")
	addCommandWithGroup(root, NewSessionCmd(), "agent")

	addCommandWithGroup(root, NewIssueCmd(), "diagnostics")
	addCommandWithGroup(root, NewVersionCmd(), "diagnostics")

	discoverPlugins(root)
}

func addCommandWithGroup(root *cobra.Command, cmd *cobra.Command, groupID string) {
	cmd.GroupID = groupID
	root.AddCommand(cmd)
}

