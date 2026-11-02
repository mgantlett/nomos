// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
	hookMsg := "VERIFIED"
	if installed {
		hookMsg = "AUTO-INSTALLED (pre-commit, commit-msg, pre-push)"
	}

	fmt.Println(renderBrandTitle("🤝 Nomos JIT Session Handshake"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Active Repository", 17, uiBoldValueStyle.Render(event.Repository)))
	fmt.Println(kvLine("Workstation Seat", 17, uiValueStyle.Render(event.Seat)))

	if event.ActiveTask != "" {
		phaseVal := fmt.Sprintf("%s %s",
			styleSemanticBadge(string(event.Phase)),
			uiMutedStyle.Render("(Task: "+styleTaskKey(event.ActiveTask)+")"))
		fmt.Println(kvLine("Workspace Phase", 17, phaseVal))
		if event.TokenID != "" {
			leaseVal := fmt.Sprintf("%s %s",
				uiCommandStyle.Render(event.TokenID),
				uiMutedStyle.Render(fmt.Sprintf("[Budget: %d, Used: %d]", event.StepBudget, event.TokensUsed)))
			fmt.Println(kvLine("CapBAC Lease", 17, leaseVal))
		}
	} else {
		phaseVal := fmt.Sprintf("%s %s",
			styleSemanticBadge(string(event.Phase)),
			uiMutedStyle.Render("(Trunk baseline)"))
		fmt.Println(kvLine("Workspace Phase", 17, phaseVal))
	}

	if installed {
		fmt.Println(kvLine("Defense Hooks", 17, uiSuccessStyle.Render(hookMsg)))
	} else {
		fmt.Println(kvLine("Defense Hooks", 17, styleSemanticBadge(hookMsg)))
	}
	if event.DatabasesHealthy {
		fmt.Println(kvLine("Database Health", 17, styleSemanticBadge("HEALTHY")))
	} else {
		fmt.Println(kvLine("Database Health", 17, styleSemanticBadge("UNHEALTHY")))
	}
	fmt.Println(kvLine("DoD Quality Gates", 17, fmt.Sprintf("%s deterministic gates active",
		uiBoldValueStyle.Render(fmt.Sprintf("%d", event.GateCount)))))
	fmt.Println(kvLine("Ready Wavefront", 17, formatHandshakeWavefront(event.ReadyWavefront)))
	if len(event.Directives) > 0 {
		dirVal := fmt.Sprintf("%s active %s",
			uiBoldValueStyle.Render(fmt.Sprintf("%d", len(event.Directives))),
			uiMutedStyle.Render("("+strings.Join(event.Directives, ", ")+")"))
		fmt.Println(kvLine("JIT Directives", 17, dirVal))
	}
	renderHandshakeMnemeTelemetry(event)
	renderNextActions(os.Stdout, event.NextActions)
	fmt.Println(uiRule(ruleNarrow))
}

func formatHandshakeWavefront(keys []string) string {
	if len(keys) == 0 {
		return uiMutedStyle.Render("[None]")
	}
	styled := make([]string, 0, len(keys))
	for _, k := range keys {
		styled = append(styled, styleTaskKey(k))
	}
	return fmt.Sprintf("[%s] %s",
		strings.Join(styled, ", "),
		uiMutedStyle.Render(fmt.Sprintf("(%d task(s))", len(keys))))
}

func renderHandshakeMnemeTelemetry(event *handshake.HandshakeEvent) {
	if len(event.ImpactSummary) > 0 {
		fmt.Println(kvLine("1-Hop Impact Map", 17, fmt.Sprintf("%d modified source target(s)", len(event.ImpactSummary))))
		for _, item := range event.ImpactSummary {
			fmt.Printf("    %s %s\n", uiSubstrateStyle.Render("•"), uiValueStyle.Render(item))
		}
	}
	if len(event.LearnedPitfalls) > 0 {
		fmt.Println(kvLine("Learned Pitfalls", 17, uiWarnStyle.Render(fmt.Sprintf("%d distilled repo trap(s) active", len(event.LearnedPitfalls)))))
		for _, p := range event.LearnedPitfalls {
			fmt.Printf("    ⚡ %s\n", uiValueStyle.Render(p))
		}
	}
	if len(event.VerifiedPatterns) > 0 {
		fmt.Println(kvLine("Verified Patterns", 17, fmt.Sprintf("%d structural blueprint(s) active", len(event.VerifiedPatterns))))
		for _, pat := range event.VerifiedPatterns {
			fmt.Printf("    🧩 %s\n", uiMutedStyle.Render(pat))
		}
	}
	if len(event.GraduationCandidates) > 0 {
		fmt.Println(kvLine("Tier B Candidates", 17, uiIntentStyle.Render(fmt.Sprintf("%d recurring invariant(s) ready for nomos.yaml", len(event.GraduationCandidates)))))
		for _, cand := range event.GraduationCandidates {
			fmt.Printf("    🎓 %s\n", uiMutedStyle.Render(cand))
		}
	}
}
