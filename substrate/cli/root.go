// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"errors"
	"fmt"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/spf13/cobra"
)

// RootCmdVersion defines the statutory release version of Nomos v4.
var RootCmdVersion = "4.9.0"

// NewRootCmd constructs the authoritative root Cobra command for Nomos.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "nomos",
		Version: RootCmdVersion,
		Short:   "Transactional governance harness between coding agents and Git",
		Long: `Nomos Open-Core Transactional Governance Harness
The deterministic runtime between any coding agent and Git: causal INT/SUB spec-vs-code DAGs, isolated git worktrees, 42 polyglot anti-reward-hacking AST gates, receipt-gated SQLite memory, and atomic 2PC releases.`,
		Example:       helpRootExample,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if shouldRecordCommandTelemetry(cmd) {
				telemetry.RecordInvocationSafe("", "cli-"+cmd.Name())
			}
		},
	}
	rootCmd.SetVersionTemplate(FormatVersionOutput())

	registerSubcommands(rootCmd)
	return rootCmd
}

func shouldRecordCommandTelemetry(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Name() == "" {
		return false
	}
	if cmd.Name() == "version" || cmd.Name() == "hook" {
		return false
	}
	for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Name() == "hook" {
			return false
		}
	}
	return true
}

// Execute parses command-line arguments, recovers internal runtime panics into the local
// diagnostic ring buffer, and executes the authoritative root command.
func Execute() (execErr error) {
	repoRoot, rErr := workspace.FindRepoRoot("")
	if rErr != nil {
		repoRoot = "."
	}
	defer func() {
		if rec := recover(); rec != nil {
			RecordSubstrateAnomaly(repoRoot, "", "nomos", anomalyKindPanic, "cli.Execute", fmt.Sprint(rec), 0)
			execErr = types.NewNomosError(types.ErrCodeCore, "recovered internal substrate panic: %v", rec).
				WithRemedy("run 'nomos issue' to inspect and export a scrubbed local diagnostic report")
		}
	}()

	rootCmd := NewRootCmd()
	execErr = rootCmd.Execute()
	recordRootExecutionAnomaly(repoRoot, execErr)
	return execErr
}

func recordRootExecutionAnomaly(repoRoot string, err error) {
	if err == nil {
		return
	}
	var nErr *types.NomosError
	if errors.As(err, &nErr) && (nErr.Code == types.ErrCodeCore || nErr.Code == types.ErrCodeSubstrate) {
		RecordSubstrateAnomaly(repoRoot, "", "nomos", anomalyKindSubstrateError, nErr.Code, nErr.Description, 0)
	}
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
	addCommandWithGroup(root, NewPhysicsCmd(), "quality")

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
