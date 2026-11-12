// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"strings"
	"time"

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
		Example: tui.HelpHandshakeExample,
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

			renderHandshakeBanner(event, installed, hookRoot)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output handshake event in machine-readable JSON format")
	return cmd
}

func renderHandshakeBanner(event *handshake.HandshakeEvent, installed bool, hookRoot string) {
	if resolveAdvisoryStyle("", "NOMOS_ADVISORY_STYLE") == advisoryStyleTerse {
		renderCompactHandshakeBanner(event)
		return
	}
	if event != nil && !event.StrictMode && event.BrownfieldTopology != nil {
		renderBrownfieldHandshakeBanner(event, event.BrownfieldTopology, hookRoot)
		return
	}
	hookMsg := "VERIFIED"
	if installed {
		hookMsg = "AUTO-INSTALLED (pre-commit, commit-msg, pre-push)"
	}

	fmt.Println(tui.RenderBrandTitle("🤝 Nomos JIT Session Handshake"))
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	fmt.Println(tui.KvLine("Active Repository", 17, tui.UiBoldValueStyle.Render(event.Repository)))
	fmt.Println(tui.KvLine("Workstation Seat", 17, tui.UiValueStyle.Render(event.Seat)))
	renderHandshakePhaseAndLease(event)

	if installed {
		fmt.Println(tui.KvLine("Defense Hooks", 17, tui.UiSuccessStyle.Render(hookMsg)))
	} else {
		fmt.Println(tui.KvLine("Defense Hooks", 17, tui.StyleSemanticBadge(hookMsg)))
	}
	if event.DatabasesHealthy {
		fmt.Println(tui.KvLine("Database Health", 17, tui.StyleSemanticBadge("HEALTHY")))
	} else {
		fmt.Println(tui.KvLine("Database Health", 17, tui.StyleSemanticBadge("UNHEALTHY")))
	}
	fmt.Println(tui.KvLine("DoD Quality Gates", 17, fmt.Sprintf("%s deterministic gates active",
		tui.UiBoldValueStyle.Render(fmt.Sprintf("%d", event.GateCount)))))
	fmt.Println(tui.KvLine("Ready Wavefront", 17, formatHandshakeWavefront(event.ReadyWavefront)))
	if len(event.Directives) > 0 {
		dirVal := fmt.Sprintf("%s active %s",
			tui.UiBoldValueStyle.Render(fmt.Sprintf("%d", len(event.Directives))),
			tui.UiMutedStyle.Render("("+strings.Join(event.Directives, ", ")+")"))
		fmt.Println(tui.KvLine("JIT Directives", 17, dirVal))
	}
	renderHandshakeUpdateAndDriftAdvisories(hookRoot)
	renderHandshakeMnemeTelemetry(event)
	tui.RenderNextActions(os.Stdout, event.NextActions)
	fmt.Println(tui.UiRule(tui.RuleNarrow))
}

func renderHandshakePhaseAndLease(event *handshake.HandshakeEvent) {
	if event.ActiveTask == "" {
		phaseVal := fmt.Sprintf("%s %s",
			tui.StyleSemanticBadge(string(event.Phase)),
			tui.UiMutedStyle.Render("(Trunk baseline)"))
		fmt.Println(tui.KvLine("Workspace Phase", 17, phaseVal))
		return
	}
	phaseVal := fmt.Sprintf("%s %s",
		tui.StyleSemanticBadge(string(event.Phase)),
		tui.UiMutedStyle.Render("(Task: "+tui.StyleTaskKey(event.ActiveTask)+")"))
	fmt.Println(tui.KvLine("Workspace Phase", 17, phaseVal))
	if event.TokenID != "" {
		leaseVal := fmt.Sprintf("%s %s",
			tui.UiCommandStyle.Render(event.TokenID),
			tui.UiMutedStyle.Render(fmt.Sprintf("[Budget: %d, Used: %d]", event.StepBudget, event.TokensUsed)))
		fmt.Println(tui.KvLine("CapBAC Lease", 17, leaseVal))
	}
}

func renderHandshakeUpdateAndDriftAdvisories(hookRoot string) {
	binPath, err := selfupdate.ResolveCurrentBinaryPath()
	if err != nil {
		binPath = ""
	}
	if updNotice := selfupdate.RefreshAndFormatUpdateNotice(RootCmdVersion, binPath, time.Now()); updNotice != "" {
		fmt.Println(tui.KvLine("Update Available", 17, tui.UiWarnStyle.Render(updNotice)))
	}
	if driftNotice := selfupdate.FormatWorkspaceDriftNotice(hookRoot); driftNotice != "" {
		fmt.Println(tui.KvLine("Contract Sync", 17, tui.UiWarnStyle.Render(driftNotice)))
	}
}

func formatHandshakeWavefront(keys []string) string {
	if len(keys) == 0 {
		return tui.UiMutedStyle.Render("[None]")
	}
	styled := make([]string, 0, len(keys))
	for _, k := range keys {
		styled = append(styled, tui.StyleTaskKey(k))
	}
	return fmt.Sprintf("[%s] %s",
		strings.Join(styled, ", "),
		tui.UiMutedStyle.Render(fmt.Sprintf("(%d task(s))", len(keys))))
}

func renderHandshakeMnemeTelemetry(event *handshake.HandshakeEvent) {
	if len(event.StaleObligations) > 0 {
		fmt.Println(tui.KvLine("Stale Obligations", 17, tui.UiWarnStyle.Render(fmt.Sprintf("⚠️  %d spec(s) modified since last SUB-* verification receipt", len(event.StaleObligations)))))
		for _, s := range event.StaleObligations {
			fmt.Printf("    ⚠️  %s %s\n", tui.UiValueStyle.Render(s.SpecPath), tui.UiMutedStyle.Render("(last verified by "+s.LastVerifiedBy+")"))
		}
	}
	if len(event.ImpactSummary) > 0 {
		fmt.Println(tui.KvLine("1-Hop Impact Map", 17, fmt.Sprintf("%d modified source target(s)", len(event.ImpactSummary))))
		for _, item := range event.ImpactSummary {
			fmt.Printf("    %s %s\n", tui.UiSubstrateStyle.Render("•"), tui.UiValueStyle.Render(item))
		}
	}
	if len(event.LearnedPitfalls) > 0 {
		fmt.Println(tui.KvLine("Learned Pitfalls", 17, tui.UiWarnStyle.Render(fmt.Sprintf("%d distilled repo trap(s) active", len(event.LearnedPitfalls)))))
		for _, p := range event.LearnedPitfalls {
			fmt.Printf("    ⚡ %s\n", tui.UiValueStyle.Render(p))
		}
	}
	if len(event.VerifiedPatterns) > 0 {
		fmt.Println(tui.KvLine("Verified Patterns", 17, fmt.Sprintf("%d structural blueprint(s) active", len(event.VerifiedPatterns))))
		for _, pat := range event.VerifiedPatterns {
			fmt.Printf("    🧩 %s\n", tui.UiMutedStyle.Render(pat))
		}
	}
	if len(event.GraduationCandidates) > 0 {
		fmt.Println(tui.KvLine("Tier B Candidates", 17, tui.UiIntentStyle.Render(fmt.Sprintf("%d recurring invariant(s) ready for nomos.yaml", len(event.GraduationCandidates)))))
		for _, cand := range event.GraduationCandidates {
			fmt.Printf("    🎓 %s\n", tui.UiMutedStyle.Render(cand))
		}
	}
}
