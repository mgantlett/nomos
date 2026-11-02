// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	fmt.Println(renderBrandTitle("🏷️  Nomos Session Renamed"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Session ID", 10, uiCommandStyle.Render(meta.ID)))
	fmt.Println(kvLine("Title", 10, uiBoldValueStyle.Render(meta.Name)))
	fmt.Println(kvLine("Updated At", 10, uiMutedStyle.Render(meta.UpdatedAt.Format(time.RFC3339))))
	fmt.Println(uiRule(ruleNarrow))
}

func renderSessionListTable(sessions []session.AgentSession) {
	fmt.Println(renderBrandTitle("📋 Nomos Agent Conversation Sessions"))
	fmt.Println(uiRule(ruleSession))
	fmt.Printf("  %s %s %s %s %s\n",
		padStyled("SESSION ID", 38, uiHeaderStyle),
		padStyled("AGENT TYPE", 20, uiHeaderStyle),
		padStyled("STATUS", 10, uiHeaderStyle),
		padStyled("LAST ACTIVE", 22, uiHeaderStyle),
		uiHeaderStyle.Render("SESSION NAME / TITLE"))
	fmt.Println(uiRule(ruleSession))

	for _, s := range sessions {
		nameStr := uiValueStyle.Render(s.Name)
		if s.Name == "" {
			nameStr = uiMutedStyle.Render("[unnamed]")
		}
		fmt.Printf("  %s %s %s %s %s\n",
			padStyled(s.ID, 38, uiCommandStyle),
			padStyled(s.AgentType, 20, uiWarnStyle),
			styleSemanticBadgeWidth(s.Status, 10),
			padStyled(s.LastActiveAt.Format("2006-01-02 15:04:05"), 22, uiMutedStyle),
			nameStr)
	}
	fmt.Println(uiRule(ruleSession))
}

func renderSessionDetailCard(detail *session.SessionDetail) {
	fmt.Println(renderBrandTitle("🔍 Nomos Agent Session Inspection"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Session ID", 12, uiCommandStyle.Render(detail.ID)))
	fmt.Println(kvLine("Agent Type", 12, uiBoldValueStyle.Render(detail.AgentType)))
	if detail.Name != "" {
		fmt.Println(kvLine("Session Name", 12, uiBoldValueStyle.Render(detail.Name)))
	}
	if detail.TaskKey != "" {
		fmt.Println(kvLine("Task Key", 12, fmt.Sprintf("%s (Phase: %s)",
			styleTaskKey(detail.TaskKey), styleSemanticBadge(detail.Phase))))
	}
	if detail.WorktreePath != "" {
		fmt.Println(kvLine("Worktree", 12, uiMutedStyle.Render(detail.WorktreePath)))
	}
	fmt.Println(kvLine("Turn Count", 12, fmt.Sprintf("%s %s Tool Calls: %s",
		uiBoldValueStyle.Render(fmt.Sprintf("%d", detail.TurnCount)),
		uiMutedStyle.Render("|"),
		uiBoldValueStyle.Render(fmt.Sprintf("%d", detail.ToolCallCount)))))
	fmt.Println(kvLine("Last Active", 12, fmt.Sprintf("%s (Status: %s)",
		uiMutedStyle.Render(detail.LastActiveAt.Format(time.RFC3339)),
		styleSemanticBadge(detail.Status))))

	if detail.Sentinel != nil {
		fmt.Println(uiRuleStyle.Render("  ────────────────── Sentinel Evaluation ───────────────────"))
		fmt.Println(kvLine("Integrity", 12, fmt.Sprintf("%s (Score: %d/100)",
			styleSemanticBadge(detail.Sentinel.IntegrityStatus), detail.Sentinel.OverallScore)))
	}
	if detail.Trajectory != nil {
		fmt.Println(uiRuleStyle.Render("  ────────────────── Causal Trajectory ─────────────────────"))
		fmt.Println(kvLine("Stage", 12, fmt.Sprintf("%s (%d/%d completed)",
			uiBoldValueStyle.Render(detail.Trajectory.CurrentStage),
			detail.Trajectory.CompletedCount, detail.Trajectory.TotalCount)))
	}
	fmt.Println(uiRule(ruleNarrow))
}
