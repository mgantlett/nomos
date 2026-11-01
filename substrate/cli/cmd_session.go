// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/session"
	"github.com/spf13/cobra"
)

// NewSessionCmd constructs the canonical 'nomos session' command group.
func NewSessionCmd() *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:   "session",
		Short: "Manage Tier 1 agent conversation sessions and operator identity labels",
		Long: `Manage Tier 1 agent conversation sessions, name tags, and metadata contracts.
Enables operators to tag and rename sessions so fleet and dashboard views
can distinguish active mission contexts across AGY-IDE, AGY-CLI, Claude Code, and OpenCode.`,
	}

	sessionCmd.AddCommand(newSessionNameCmd())
	sessionCmd.AddCommand(newSessionListCmd())
	sessionCmd.AddCommand(newSessionVisualCmd())
	sessionCmd.AddCommand(newSessionShowCmd())
	return sessionCmd
}

func newSessionNameCmd() *cobra.Command {
	var sessionID string
	var brainDir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "name [NAME]",
		Short: "Assign a human-readable title or alias to an agent conversation session",
		Long: `Assign a human-readable title or alias to an agent conversation session.
If no session ID is specified via --id, the currently active session is auto-discovered.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := extractNameFromArgs(args)
			if name == "" {
				return types.NewNomosError(types.ErrCodeValidationFailed, "session title/name is required (e.g. nomos session name \"Mission Refactoring\")")
			}

			targetID := sessionID
			if targetID == "" {
				discovered, err := session.DiscoverSessions(brainDir, "")
				if err != nil || len(discovered) == 0 {
					return types.NewNomosError(types.ErrCodeCore, "no active agent conversation sessions found; specify target via --id")
				}
				targetID = discovered[0].ID
			}

			meta, err := session.SaveSessionName(brainDir, targetID, name)
			if err != nil {
				return types.NewNomosError(types.ErrCodeCore, "failed saving session metadata: %v", err)
			}

			if jsonOutput {
				data, mErr := json.MarshalIndent(meta, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling JSON: %v", mErr)
				}
				fmt.Println(string(data))
				return nil
			}

			renderSessionNamedCard(meta)
			return nil
		},
	}

	cmd.Flags().StringVarP(&sessionID, "id", "i", "", "Target conversation session ID (defaults to active session)")
	cmd.Flags().StringVarP(&brainDir, "brain", "b", "", "Session storage or brain directory path override")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in machine-readable JSON format")
	return cmd
}

func extractNameFromArgs(args []string) string {
	name := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "name=") {
			trimmed := strings.TrimPrefix(arg, "name=")
			trimmed = strings.Trim(trimmed, "\"")
			trimmed = strings.Trim(trimmed, "'")
			name = trimmed
		} else if name == "" {
			name = strings.Trim(arg, "\"'")
		} else {
			name = name + " " + strings.Trim(arg, "\"'")
		}
	}
	return strings.TrimSpace(name)
}

func newSessionListCmd() *cobra.Command {
	var brainDir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered agent conversation sessions and their metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := session.DiscoverSessions(brainDir, "")
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed discovering sessions: %v", err)
			}

			if jsonOutput {
				data, mErr := json.MarshalIndent(sessions, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling JSON: %v", mErr)
				}
				fmt.Println(string(data))
				return nil
			}

			renderSessionListTable(sessions)
			return nil
		},
	}

	cmd.Flags().StringVarP(&brainDir, "brain", "b", "", "Session storage or brain directory path override")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in machine-readable JSON format")
	return cmd
}

func newSessionVisualCmd() *cobra.Command {
	var brainDir string
	var repoRoot string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "visual",
		Short: "Generate Mermaid visual topology of active agent sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := session.DiscoverSessions(brainDir, repoRoot)
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed discovering sessions: %v", err)
			}

			mermaid := session.GenerateMermaidVisual(sessions)
			if jsonOutput {
				payload := session.SessionVisualGraph{
					Mermaid:  mermaid,
					Sessions: sessions,
				}
				data, mErr := json.MarshalIndent(payload, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling JSON: %v", mErr)
				}
				fmt.Println(string(data))
				return nil
			}

			fmt.Println(mermaid)
			return nil
		},
	}

	cmd.Flags().StringVarP(&brainDir, "brain", "b", "", "Session storage or brain directory path override")
	cmd.Flags().StringVarP(&repoRoot, "root", "r", "", "Repository root directory path override")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in machine-readable JSON format for external dashboards and integrations")
	return cmd
}

func newSessionShowCmd() *cobra.Command {
	var brainDir string
	var repoRoot string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "show [SESSION_ID]",
		Short: "Deeply inspect an agent conversation session, events timeline, and Sentinel scorecard",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := ""
			if len(args) > 0 {
				sessionID = args[0]
			}
			if sessionID == "" {
				discovered, err := session.DiscoverSessions(brainDir, repoRoot)
				if err != nil || len(discovered) == 0 {
					return types.NewNomosError(types.ErrCodeCore, "no active agent conversation sessions found")
				}
				sessionID = discovered[0].ID
			}

			detail, err := session.GetSessionDetail(brainDir, repoRoot, sessionID)
			if err != nil {
				return types.NewNomosError(types.ErrCodeCore, "failed inspecting session %q: %v", sessionID, err)
			}

			if jsonOutput {
				data, mErr := json.MarshalIndent(detail, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling JSON: %v", mErr)
				}
				fmt.Println(string(data))
				return nil
			}

			renderSessionDetailCard(detail)
			return nil
		},
	}

	cmd.Flags().StringVarP(&brainDir, "brain", "b", "", "Session storage or brain directory path override")
	cmd.Flags().StringVarP(&repoRoot, "root", "r", "", "Repository root directory path override")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in machine-readable JSON format for external dashboards and integrations")
	return cmd
}

func renderSessionNamedCard(meta *session.SessionMetadata) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))

	fmt.Println(titleStyle.Render("🏷️  Nomos Session Renamed"))
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Printf("  Session ID : %s\n", idStyle.Render(meta.ID))
	fmt.Printf("  Title      : %s\n", valStyle.Render(meta.Name))
	fmt.Printf("  Updated At : %s\n", meta.UpdatedAt.Format(time.RFC3339))
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────"))
}

func renderSessionListTable(sessions []session.AgentSession) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	idleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	agentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	fmt.Println(titleStyle.Render("📋 Nomos Agent Conversation Sessions"))
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────────────────────"))
	fmt.Printf("  %-38s %-20s %-10s %-22s %s\n", "SESSION ID", "AGENT TYPE", "STATUS", "LAST ACTIVE", "SESSION NAME / TITLE")
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────────────────────"))

	for _, s := range sessions {
		statusStr := idleStyle.Render(s.Status)
		if s.Status == "ACTIVE" {
			statusStr = activeStyle.Render(s.Status)
		}
		nameStr := s.Name
		if nameStr == "" {
			nameStr = dimStyle.Render("[unnamed]")
		}
		agStr := agentStyle.Render(s.AgentType)
		fmt.Printf("  %-38s %-20s %-10s %-22s %s\n", s.ID, agStr, statusStr, s.LastActiveAt.Format("2006-01-02 15:04:05"), nameStr)
	}
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────────────────────────────────────────────────────"))
}

func renderSessionDetailCard(detail *session.SessionDetail) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	valStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	gatePass := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))

	fmt.Println(titleStyle.Render("🔍 Nomos Agent Session Inspection"))
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Printf("  Session ID   : %s\n", idStyle.Render(detail.ID))
	fmt.Printf("  Agent Type   : %s\n", valStyle.Render(detail.AgentType))
	if detail.Name != "" {
		fmt.Printf("  Session Name : %s\n", valStyle.Render(detail.Name))
	}
	if detail.TaskKey != "" {
		fmt.Printf("  Task Key     : %s (Phase: %s)\n", valStyle.Render(detail.TaskKey), detail.Phase)
	}
	if detail.WorktreePath != "" {
		fmt.Printf("  Worktree     : %s\n", detail.WorktreePath)
	}
	fmt.Printf("  Turn Count   : %d | Tool Calls: %d\n", detail.TurnCount, detail.ToolCallCount)
	fmt.Printf("  Last Active  : %s (Status: %s)\n", detail.LastActiveAt.Format(time.RFC3339), detail.Status)

	if detail.Sentinel != nil {
		fmt.Println(dimStyle.Render("  ────────────────── Sentinel Evaluation ───────────────────"))
		fmt.Printf("  Integrity    : %s (Score: %d/100)\n", gatePass.Render(detail.Sentinel.IntegrityStatus), detail.Sentinel.OverallScore)
	}
	if detail.Trajectory != nil {
		fmt.Println(dimStyle.Render("  ────────────────── Causal Trajectory ─────────────────────"))
		fmt.Printf("  Stage        : %s (%d/%d completed)\n", valStyle.Render(detail.Trajectory.CurrentStage), detail.Trajectory.CompletedCount, detail.Trajectory.TotalCount)
	}
	fmt.Println(dimStyle.Render("────────────────────────────────────────────────────────────"))
}
