// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
// @intent task_lifecycle_cli_spec.md
package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/cli/diag"
	"github.com/mgantlett/nomos/substrate/cli/hookcmd"
	"github.com/mgantlett/nomos/substrate/cli/inspectcmd"
	"github.com/mgantlett/nomos/substrate/cli/mcpcmd"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/taskui"
	"github.com/mgantlett/nomos/substrate/cli/tui"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/spf13/cobra"
)

// RootCmdVersion defines the statutory release version of Nomos v4.
var RootCmdVersion = "4.34.1"

func init() {
	tui.ResolveAppVersion = func() string { return RootCmdVersion }
	tui.TemplatesFS = initTemplates
	tui.ScaffoldAmbientHooksFn = scaffoldAmbientHooks
	tui.DetectAdapterDriftFn = detectAdapterAndStealthDrift
	tui.RefreshLocalStealthFn = refreshLocalStealthIfPresent
}

// NewRootCmd constructs the authoritative root Cobra command for Nomos.
func NewRootCmd() *cobra.Command {
	var helpAgent bool
	rootCmd := &cobra.Command{
		Use:     "nomos",
		Version: RootCmdVersion,
		Short:   "Deterministic execution substrate between coding agents and Git",
		Long: `Nomos Open-Core Deterministic Execution Substrate
Progressive guardrails between coding agents and Git: zero-write codebase X-Ray, Branch Guard on normal feat/* branches, 42 polyglot AST quality gates, receipt-gated SQLite memory, and optional isolated git worktrees.`,
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
	rootCmd.Flags().BoolVar(&helpAgent, "help-agent", false, "Show internal agent lifecycle and RPC commands")

	agentGroup, hiddenAgentCmds := registerSubcommands(rootCmd)
	configureRootHelp(rootCmd, &helpAgent, agentGroup, hiddenAgentCmds)
	return rootCmd
}

const (
	defaultAgentGroupTitle  = "Cognitive Memory Commands:"
	expandedAgentGroupTitle = "Agent Boot & Cognitive Memory Commands:"
)

func configureRootHelp(rootCmd *cobra.Command, helpAgent *bool, agentGroup *cobra.Group, hiddenAgentCmds []*cobra.Command) {
	bindHelpSubcommandAgentFlag(rootCmd, helpAgent)
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != rootCmd {
			defaultHelp(cmd, args)
			return
		}
		if *helpAgent {
			agentGroup.Title = expandedAgentGroupTitle
			setHiddenState(hiddenAgentCmds, false)
			defaultHelp(cmd, args)
			setHiddenState(hiddenAgentCmds, true)
			agentGroup.Title = defaultAgentGroupTitle
			return
		}
		defaultHelp(cmd, args)
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "\nAgent Lifecycle & Internal RPCs: run \"nomos --help-agent\" for handshake, hook, directive, capabilities, physics, session, and mcp commands."); err != nil {
			return
		}
	})
}

func bindHelpSubcommandAgentFlag(rootCmd *cobra.Command, helpAgent *bool) {
	rootCmd.InitDefaultHelpCmd()
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "help" && sub.Flags().Lookup("help-agent") == nil {
			sub.Flags().BoolVar(helpAgent, "help-agent", false, "Show internal agent lifecycle and RPC commands")
		}
	}
}

func setHiddenState(cmds []*cobra.Command, hidden bool) {
	for _, sub := range cmds {
		sub.Hidden = hidden
	}
}

func shouldRecordCommandTelemetry(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Name() == "" {
		return false
	}
	if cmd.Name() == "version" || cmd.Name() == "hook" || cmd.Name() == "mcp" {
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
	execErr = EnrichMissingPluginError(rootCmd.Execute())
	recordRootExecutionAnomaly(repoRoot, execErr)
	return execErr
}

// EnrichMissingPluginError attaches an external plugin remedy when an unknown root subcommand is invoked.
func EnrichMissingPluginError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	const prefix = `unknown command "`
	if !strings.Contains(msg, prefix) || !strings.Contains(msg, `for "nomos"`) {
		return err
	}
	start := strings.Index(msg, prefix) + len(prefix)
	rest := msg[start:]
	end := strings.IndexByte(rest, '"')
	if end <= 0 {
		return err
	}
	sub := rest[:end]
	return types.NewNomosError(types.ErrCodeCore, "%s", msg).
		WithRemedy(fmt.Sprintf("install external plugin 'nomos-plugin-%s' on PATH if required (extensions: https://sophialabs.ca/sovereign)", sub))
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

func registerSubcommands(root *cobra.Command) (*cobra.Group, []*cobra.Command) {
	agentGroup := &cobra.Group{ID: "agent", Title: defaultAgentGroupTitle}
	root.AddGroup(
		&cobra.Group{ID: "lifecycle", Title: "Workspace & Lifecycle Commands:"},
		&cobra.Group{ID: "quality", Title: "Quality & Invariant Verification Commands:"},
		agentGroup,
		&cobra.Group{ID: "diagnostics", Title: "Diagnostics & System Commands:"},
	)

	addCommandWithGroup(root, NewInitCmd(), "lifecycle")
	addCommandWithGroup(root, diag.NewStatusCmd(), "lifecycle")
	addCommandWithGroup(root, NewTaskCmd(), "lifecycle")

	addCommandWithGroup(root, NewVerifyCmd(), "quality")
	addCommandWithGroup(root, NewXrayCmd(), "quality")
	physicsCmd := NewPhysicsCmd()
	addHiddenCommandWithGroup(root, physicsCmd, "quality")

	handshakeCmd := NewHandshakeCmd()
	hookCmd := hookcmd.NewHookCmd(RootCmdVersion, newHookInstallCmd())
	directiveCmd := inspectcmd.NewDirectiveCmd()
	capabilitiesCmd := inspectcmd.NewCapabilitiesCmd()
	sessionCmd := inspectcmd.NewSessionCmd()
	mcpCmd := mcpcmd.NewMCPCmd(RootCmdVersion)

	addHiddenCommandWithGroup(root, handshakeCmd, "agent")
	addHiddenCommandWithGroup(root, hookCmd, "agent")
	addHiddenCommandWithGroup(root, directiveCmd, "agent")
	addHiddenCommandWithGroup(root, capabilitiesCmd, "agent")
	addCommandWithGroup(root, inspectcmd.NewMemCmd(), "agent")
	addHiddenCommandWithGroup(root, sessionCmd, "agent")
	addHiddenCommandWithGroup(root, mcpCmd, "agent")

	addCommandWithGroup(root, diag.NewIssueCmd(), "diagnostics")
	addCommandWithGroup(root, selfupdate.NewUpdateCmd(), "diagnostics")
	addCommandWithGroup(root, selfupdate.NewVersionCmd(), "diagnostics")

	tui.DiscoverPlugins(root, resolveActiveProhibitedCommands())
	return agentGroup, []*cobra.Command{physicsCmd, handshakeCmd, hookCmd, directiveCmd, capabilitiesCmd, sessionCmd, mcpCmd}
}

func resolveActiveProhibitedCommands() []string {
	repoRoot, err := workspace.FindRepoRoot("")
	if err != nil || repoRoot == "" {
		return nil
	}
	cfg, err := config.LoadConfig(repoRoot)
	if err != nil || cfg == nil || !cfg.Governance.Boundary.Enforce {
		return nil
	}
	return cfg.Governance.Boundary.ProhibitedCommands
}

func addCommandWithGroup(root *cobra.Command, cmd *cobra.Command, groupID string) {
	cmd.GroupID = groupID
	root.AddCommand(cmd)
}

func addHiddenCommandWithGroup(root *cobra.Command, cmd *cobra.Command, groupID string) {
	cmd.Hidden = true
	cmd.GroupID = groupID
	root.AddCommand(cmd)
}

const staleTrapSweepWindow = 7 * 24 * time.Hour

func runAutomaticSubstrateMaintenance(repoRoot string, force bool) {
	if iErr := tui.EnsureIDEBoundarySettings(repoRoot); iErr != nil {
		// best-effort IDE watcher boundary protection
	}
	if repoRoot != "" {
		if _, pErr := taskui.PruneWorkspaceOrphans(repoRoot, false); pErr != nil {
			// best-effort orphan worktree sweep
		}
	}
	if !storage.ShouldRunPeriodicMaintenance(force) {
		return
	}
	if _, err := storage.MaintainRuntimeStorage(true); err != nil {
		// best-effort background hygiene
	}
	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err == nil {
		if repoRoot != "" {
			cleanRoot := filepath.Clean(repoRoot)
			mem.SweepRepoPitfallHygiene(store, cleanRoot, filepath.Base(cleanRoot), time.Now().UTC())
		}
		if _, mErr := store.Maintain(mem.MaintainOptions{}); mErr != nil {
			// best-effort Mneme compaction
		}
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}
	mem.SweepStaleTrapBuffers(repoRoot, staleTrapSweepWindow)
}

func distillAndMaintainOnSync(repoRoot, wtPath, key, walkthroughPath string, touchedFiles []string) {
	distillAndMaintainOnSyncContent(repoRoot, wtPath, key, readWalkthroughContent(walkthroughPath), touchedFiles)
}

func distillAndMaintainOnSyncContent(repoRoot, wtPath, key, wtContent string, touchedFiles []string) {
	if distilled, dErr := mem.DistillTaskLearnings(repoRoot, key, wtContent, touchedFiles); dErr == nil && distilled > 0 {
		printlnDistilledCount(distilled)
	}
	if tr := hookcmd.LoadHookSessionTrace(wtPath); tr != nil && len(tr.MutatedFiles) > 0 {
		if deadEnds, deErr := mem.RecordRolledBackDeadEnds(repoRoot, key, tr.MutatedFiles, touchedFiles, wtContent); deErr == nil && deadEnds > 0 {
			fmt.Printf("🧠 Nomos Learn: recorded %d rolled-back dead-end(s) into local Mneme\n", deadEnds)
		}
	}
	if patterns, pErr := mem.DistillVerifiedPattern(repoRoot, key, wtContent, touchedFiles); pErr == nil && patterns > 0 {
		fmt.Printf("🧠 Nomos Learn: distilled %d verified structural pattern(s) into local Mneme\n", patterns)
	}
	runAutomaticSubstrateMaintenance(repoRoot, true)
}

func printlnDistilledCount(distilled int) {
	fmt.Printf("🧠 Nomos Learn: distilled %d verified trap->fix pitfall(s) into local Mneme\n", distilled)
}
