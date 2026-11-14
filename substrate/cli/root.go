// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"errors"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/diag"
	"github.com/mgantlett/nomos/substrate/cli/inspectcmd"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/tui"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/spf13/cobra"
)

// RootCmdVersion defines the statutory release version of Nomos v4.
var RootCmdVersion = "4.18.0"

func init() {
	tui.ResolveAppVersion = func() string { return RootCmdVersion }
	tui.TemplatesFS = initTemplates
	tui.ScaffoldAmbientHooksFn = scaffoldAmbientHooks
}

// NewRootCmd constructs the authoritative root Cobra command for Nomos.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "nomos",
		Version: RootCmdVersion,
		Short:   "Deterministic execution substrate between coding agents and Git",
		Long: `Nomos Open-Core Deterministic Execution Substrate
The deterministic runtime between any coding agent and Git: causal INT/SUB spec-vs-code DAGs, isolated git worktrees, 42 polyglot anti-reward-hacking AST gates, receipt-gated SQLite memory, and atomic 2PC releases.`,
		Example:       tui.HelpRootExample,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if shouldRecordCommandTelemetry(cmd) {
				telemetry.RecordInvocationSafe("", "cli-"+cmd.Name())
			}
		},
	}
	rootCmd.SetVersionTemplate(selfupdate.FormatVersionOutput())

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
			diag.RecordSubstrateAnomaly(repoRoot, "", "nomos", diag.AnomalyKindPanic, "cli.Execute", fmt.Sprint(rec), 0)
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
		diag.RecordSubstrateAnomaly(repoRoot, "", "nomos", diag.AnomalyKindSubstrateError, nErr.Code, nErr.Description, 0)
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
	addCommandWithGroup(root, diag.NewStatusCmd(), "lifecycle")
	addCommandWithGroup(root, NewTaskCmd(), "lifecycle")

	addCommandWithGroup(root, NewVerifyCmd(), "quality")
	addCommandWithGroup(root, NewXrayCmd(), "quality")
	addCommandWithGroup(root, NewPhysicsCmd(), "quality")

	addCommandWithGroup(root, NewHandshakeCmd(), "agent")
	addCommandWithGroup(root, NewHookCmd(), "agent")
	addCommandWithGroup(root, inspectcmd.NewDirectiveCmd(), "agent")
	addCommandWithGroup(root, inspectcmd.NewCapabilitiesCmd(), "agent")
	addCommandWithGroup(root, inspectcmd.NewMemCmd(), "agent")
	addCommandWithGroup(root, inspectcmd.NewSessionCmd(), "agent")

	addCommandWithGroup(root, diag.NewIssueCmd(), "diagnostics")
	addCommandWithGroup(root, selfupdate.NewUpdateCmd(), "diagnostics")
	addCommandWithGroup(root, selfupdate.NewVersionCmd(), "diagnostics")

	tui.DiscoverPlugins(root)
}

func addCommandWithGroup(root *cobra.Command, cmd *cobra.Command, groupID string) {
	cmd.GroupID = groupID
	root.AddCommand(cmd)
}
