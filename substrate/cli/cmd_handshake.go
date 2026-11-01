// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/spf13/cobra"
)

// NewHandshakeCmd constructs the canonical 'nomos handshake' command.
func NewHandshakeCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "handshake",
		Short: "Initialize JIT session orientation telemetry and verify CapBAC tokens",
		Long: `Run on session boot to initialize state, sync protocols, and receive dynamic
Just-In-Time (JIT) Phase Invariant Envelopes under indirect control.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := os.Getwd()
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed resolving working directory: %v", err)
			}

			hookRoot := repoRoot
			if pr, pErr := workspace.FindRepoRoot(repoRoot); pErr == nil {
				hookRoot = pr
			}

			installed, hErr := workspace.EnsureHooksInstalled(hookRoot)
			if hErr != nil {
				return hErr
			}

			runAutomaticSubstrateMaintenance(hookRoot, false)
			telemetry.RecordWorkflowInvocationSafe("", "wf-handshake", "Nomos Handshake", "human_workflow")

			event, sErr := handshake.SynthesizeDynamicHandshake(repoRoot)
			if sErr != nil {
				return sErr
			}

			if jsonOutput {
				data, mErr := json.MarshalIndent(event, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling handshake JSON: %v", mErr)
				}
				fmt.Println(string(data))
				return nil
			}

			renderHandshakeBanner(event, installed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output handshake event in machine-readable JSON format")
	return cmd
}

func renderHandshakeBanner(event *handshake.HandshakeEvent, installed bool) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#38bdf8"))
	badgeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10b981"))
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ef4444"))
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))

	hookMsg := "VERIFIED"
	if installed {
		hookMsg = "AUTO-INSTALLED (pre-commit, commit-msg, pre-push)"
	}

	fmt.Println(titleStyle.Render("🤝 Nomos JIT Session Handshake"))
	fmt.Println(lineStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Printf("  Active Repository : %s\n", event.Repository)
	fmt.Printf("  Workstation Seat  : %s\n", event.Seat)

	if event.ActiveTask != "" {
		fmt.Printf("  Workspace Phase   : %s (Task: %s)\n", event.Phase, event.ActiveTask)
		if event.TokenID != "" {
			fmt.Printf("  CapBAC Lease      : %s [Budget: %d, Used: %d]\n", event.TokenID, event.StepBudget, event.TokensUsed)
		}
	} else {
		fmt.Printf("  Workspace Phase   : %s (Trunk baseline)\n", event.Phase)
	}

	fmt.Printf("  Defense Hooks     : %s\n", badgeStyle.Render(hookMsg))

	if event.DatabasesHealthy {
		fmt.Printf("  Database Health   : %s\n", badgeStyle.Render("HEALTHY"))
	} else {
		fmt.Printf("  Database Health   : %s\n", errStyle.Render("UNHEALTHY"))
	}

	fmt.Printf("  DoD Quality Gates : %d deterministic gates active\n", event.GateCount)

	if len(event.ReadyWavefront) > 0 {
		fmt.Printf("  Ready Wavefront   : [%s] (%d task(s))\n", strings.Join(event.ReadyWavefront, ", "), len(event.ReadyWavefront))
	} else {
		fmt.Printf("  Ready Wavefront   : [None]\n")
	}

	if len(event.Directives) > 0 {
		fmt.Printf("  JIT Directives    : %d active (%s)\n", len(event.Directives), strings.Join(event.Directives, ", "))
	}
	renderHandshakeMnemeTelemetry(event)
	renderNextActions(os.Stdout, event.NextActions)
	fmt.Println(lineStyle.Render("────────────────────────────────────────────────────────────"))
}

func renderHandshakeMnemeTelemetry(event *handshake.HandshakeEvent) {
	if len(event.ImpactSummary) > 0 {
		fmt.Printf("  1-Hop Impact Map  : %d modified source target(s)\n", len(event.ImpactSummary))
		for _, item := range event.ImpactSummary {
			fmt.Printf("    • %s\n", item)
		}
	}
	if len(event.LearnedPitfalls) > 0 {
		fmt.Printf("  Learned Pitfalls  : %d distilled repo trap(s) active\n", len(event.LearnedPitfalls))
		for _, p := range event.LearnedPitfalls {
			fmt.Printf("    ⚡ %s\n", p)
		}
	}
	if len(event.VerifiedPatterns) > 0 {
		fmt.Printf("  Verified Patterns : %d structural blueprint(s) active\n", len(event.VerifiedPatterns))
		for _, pat := range event.VerifiedPatterns {
			fmt.Printf("    🧩 %s\n", pat)
		}
	}
	if len(event.GraduationCandidates) > 0 {
		fmt.Printf("  Tier B Candidates : %d recurring invariant(s) ready for nomos.yaml\n", len(event.GraduationCandidates))
		for _, cand := range event.GraduationCandidates {
			fmt.Printf("    🎓 %s\n", cand)
		}
	}
}
