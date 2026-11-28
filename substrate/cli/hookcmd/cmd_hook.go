// Package hookcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package hookcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/hookcmd/shellguard"
	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/spf13/cobra"
)

// NewHookCmd constructs the canonical 'nomos hook' command group.
// @intent task_lifecycle_cli_spec.md
func NewHookCmd(rootVersion string, installSubCmd *cobra.Command) *cobra.Command {
	var hermeticDir string
	var containerSandbox bool
	var budgetSec int
	var advFlags hookAdvisoryFlags

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Execute compiled zero-dependency agent lifecycle hooks (session-start, pre-tool, post-tool, stop, install)",
		Long: `Native compiled Go hook interceptors for Tier 1 coding agents (Claude Code, Gemini CLI, OpenCode, Cursor, Antigravity).
Reads JSON hook payloads from stdin with zero external jq/bash/python dependencies.`,
		Example: tui.HelpHookExample,
	}

	cmd.PersistentFlags().StringVar(&hermeticDir, "hermetic-dir", "", "Confine file mutations and shell paths to a container/sandbox root directory")
	cmd.PersistentFlags().BoolVar(&containerSandbox, "container-sandbox", false, "Enable disposable benchmark container governance (blocks verifier/solution tampering while permitting OS-wide task paths)")
	cmd.PersistentFlags().IntVar(&budgetSec, "budget-sec", 0, "Optional session wall-clock budget in seconds for PostToolUse checkpoint advisories")
	cmd.PersistentFlags().IntVar(&advFlags.warnPct, "budget-warn-pct", 0, "Bounded budget warning checkpoint percentage [25..90] (default 75, env NOMOS_BUDGET_WARN_PCT)")
	cmd.PersistentFlags().IntVar(&advFlags.critPct, "budget-crit-pct", 0, "Bounded budget critical checkpoint percentage [50..99] (default 90, env NOMOS_BUDGET_CRIT_PCT)")
	cmd.PersistentFlags().IntVar(&advFlags.loopStrikes, "loop-strikes", 0, "Bounded identical-failure loop circuit-breaker threshold [2..10] (default 3, env NOMOS_LOOP_STRIKES)")
	cmd.PersistentFlags().StringVar(&advFlags.advisoryStyle, "advisory-style", "", "PostToolUse advisory style: standard or terse (default standard, env NOMOS_ADVISORY_STYLE)")
	cmd.PersistentFlags().StringVar(&advFlags.advisoryProfile, "advisory-profile", "", "Model pacing preset: standard or carbon (default standard, env NOMOS_ADVISORY_PROFILE)")

	cmd.AddCommand(
		newHookSessionStartCmd(&hermeticDir, &containerSandbox, &budgetSec, rootVersion),
		newHookPreToolCmd(&hermeticDir, &containerSandbox, &budgetSec, &advFlags),
		newHookPostToolCmd(&hermeticDir, &containerSandbox, &budgetSec, &advFlags),
		newHookStopCmd(&hermeticDir, &containerSandbox),
		newHookPostCommitCmd(),
		newHookPrePushCmd(),
	)
	if installSubCmd != nil {
		cmd.AddCommand(installSubCmd)
	}
	return cmd
}

func newHookSessionStartCmd(hermeticDir *string, containerSandbox *bool, budgetSec *int, rootVersion string) *cobra.Command {
	return &cobra.Command{
		Use:     "session-start",
		Short:   "Execute SessionStart orientation handshake from JSON stdin payload",
		Long:    tui.HelpHookSessionStartLong,
		Example: tui.HelpHookSessionStartExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			payload := parseHookPayload(raw)
			ensureSessionBudgetStamp(payload, *hermeticDir, resolveEffectiveBudgetSeconds(*budgetSec))
			targetDir := resolveHookWorkspaceDir(payload, *hermeticDir)

			hookRoot := targetDir
			if pr, pErr := workspace.FindRepoRoot(targetDir); pErr == nil {
				hookRoot = pr
			}
			if _, hErr := workspace.EnsureHooksInstalled(hookRoot); hErr != nil && cmd.ErrOrStderr() != nil {
				// Non-fatal in read-only or non-git containers
			}
			if rErr := capbac.ResetWorkspaceFlowState(hookRoot); rErr != nil && cmd.ErrOrStderr() != nil {
				// Non-fatal in read-only containers
			}

			ev, sErr := handshake.SynthesizeDynamicHandshake(targetDir)
			if sErr != nil || ev == nil {
				return writeEmptyJSON(cmd.OutOrStdout())
			}

			banner := formatHandshakeBanner(ev, hookRoot, *hermeticDir == "" && !*containerSandbox, *containerSandbox, rootVersion)
			if payload.IsCursorEvent {
				return writeCursorSessionStartResponse(cmd.OutOrStdout(), banner)
			}
			return writeSessionStartInjection(cmd.OutOrStdout(), banner)
		},
	}
}

func newHookPreToolCmd(hermeticDir *string, containerSandbox *bool, budgetSec *int, advFlags *hookAdvisoryFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "pre-tool",
		Short:   "Evaluate Law 3 boundary isolation and Law 7 command safety on PreToolUse payload",
		Long:    tui.HelpHookPreToolLong,
		Example: tui.HelpHookPreToolExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeHookDecision(cmd.OutOrStdout(), shellguard.HookDecision{Decision: "allow"})
			}
			payload := parseHookPayload(raw)
			ensureSessionBudgetStamp(payload, *hermeticDir, resolveEffectiveBudgetSeconds(*budgetSec))
			dec := dispatchPreToolHook(payload, *hermeticDir, *containerSandbox, advFlags.resolve())
			recordPreToolSessionTrace(payload, *hermeticDir, dec.Decision == hookDecisionDeny)
			if dec.Decision == hookDecisionDeny {
				recordPreToolShieldDeny(payload, *hermeticDir, dec.Reason)
			}
			return writePreToolResponse(cmd.OutOrStdout(), payload, dec)
		},
	}
}

func newHookPostToolCmd(hermeticDir *string, containerSandbox *bool, budgetSec *int, advFlags *hookAdvisoryFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "post-tool",
		Short:   "Evaluate Law 4 zero-footprint in-memory AST and syntax radar on PostToolUse payload",
		Long:    tui.HelpHookPostToolLong,
		Example: tui.HelpHookPostToolExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			payload := parseHookPayload(raw)
			if payload.IsPostInvoke {
				pending := popAntigravityAdvisory(payload, *hermeticDir)
				if pending == "" {
					return writeEmptyJSON(cmd.OutOrStdout())
				}
				return writeSessionStartInjection(cmd.OutOrStdout(), pending)
			}
			msg := dispatchPostToolHook(payload, *hermeticDir, *containerSandbox, resolveEffectiveBudgetSeconds(*budgetSec), advFlags.resolve())
			if payload.IsAntigravityEvent && payload.SessionID != "" {
				queueAntigravityAdvisory(payload, *hermeticDir, msg)
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			if msg == "" {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			return writePostToolAdvisory(cmd.OutOrStdout(), payload, msg)
		},
	}
}

func dispatchPreToolHook(payload *shellguard.HookPayload, hermeticDir string, containerSandbox bool, profile HookAdvisoryProfile) shellguard.HookDecision {
	var dec shellguard.HookDecision
	if containerSandbox && hermeticDir == "" {
		dec = EvaluateContainerPreToolHookWithProfile(payload, profile)
	} else {
		dec = EvaluatePreToolHookWithProfile(payload, hermeticDir, profile)
	}
	if dec.Decision == hookDecisionDeny {
		return dec
	}
	return evaluateReadSpiralSearchCircuitBreaker(payload, hermeticDir)
}

func dispatchPostToolHook(payload *shellguard.HookPayload, hermeticDir string, containerSandbox bool, budgetSec int, profile HookAdvisoryProfile) string {
	recordPostToolSessionTrace(payload, hermeticDir)
	budgetMsg := evaluateSessionBudgetRadarWithProfile(payload, hermeticDir, budgetSec, profile)
	churnMsg := evaluateWorktreeChurnAdvisory(payload, hermeticDir, profile.AdvisoryStyle)
	pacingMsg := evaluateReadSpiralAdvisoryWithProfile(payload, hermeticDir, profile)
	healthMsg := evaluatePostToolHostHealth(payload, hermeticDir, profile.AdvisoryStyle)
	var syntaxMsg string
	if containerSandbox && hermeticDir == "" {
		syntaxMsg = EvaluateContainerPostToolHook(payload)
	} else {
		syntaxMsg = EvaluatePostToolHook(payload, hermeticDir)
	}
	combined := joinHookAdvisories(joinHookAdvisories(budgetMsg, churnMsg), pacingMsg)
	return joinHookAdvisories(joinHookAdvisories(combined, healthMsg), syntaxMsg)
}

func writeEmptyJSON(w io.Writer) error {
	_, err := fmt.Fprintln(w, "{}")
	return err
}

func writePreToolResponse(w io.Writer, payload *shellguard.HookPayload, dec shellguard.HookDecision) error {
	if payload != nil && payload.IsCursorEvent {
		return writeCursorPreToolResponse(w, dec)
	}
	if payload != nil && payload.IsClaudeEvent {
		if dec.Decision != "deny" {
			return writeEmptyJSON(w)
		}
		out := map[string]any{
			"hookSpecificOutput": map[string]string{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": dec.Reason,
			},
		}
		data, err := json.Marshal(out)
		if err != nil {
			return writeEmptyJSON(w)
		}
		_, wErr := fmt.Fprintln(w, string(data))
		return wErr
	}
	return writeHookDecision(w, dec)
}

func writeHookDecision(w io.Writer, dec shellguard.HookDecision) error {
	data, err := json.Marshal(dec)
	if err != nil {
		_, wErr := fmt.Fprintln(w, `{"decision":"allow"}`)
		return wErr
	}
	_, wErr := fmt.Fprintln(w, string(data))
	return wErr
}

func writeSessionStartInjection(w io.Writer, msg string) error {
	out := map[string]any{
		"injectSteps": []map[string]any{
			{
				"systemMessage": map[string]string{
					"systemMessage": msg,
				},
			},
		},
	}
	data, err := json.Marshal(out)
	if err != nil {
		return writeEmptyJSON(w)
	}
	_, wErr := fmt.Fprintln(w, string(data))
	return wErr
}

func writePostToolAdvisory(w io.Writer, payload *shellguard.HookPayload, msg string) error {
	if payload != nil && payload.IsCursorEvent {
		return writeCursorPostToolResponse(w, msg)
	}
	if payload != nil && payload.IsGeminiEvent {
		return writeJSONMap(w, map[string]any{
			"hookSpecificOutput": map[string]string{
				"additionalContext": msg,
			},
		})
	}
	if payload != nil && payload.HasToolInput {
		return writeJSONMap(w, map[string]any{
			"hookSpecificOutput": map[string]string{
				"hookEventName":     "PostToolUse",
				"additionalContext": msg,
			},
		})
	}
	return writeSessionStartInjection(w, msg)
}

func writeJSONMap(w io.Writer, out map[string]any) error {
	data, err := json.Marshal(out)
	if err != nil {
		return writeEmptyJSON(w)
	}
	_, wErr := fmt.Fprintln(w, string(data))
	return wErr
}

// FormatPlainHandshakeBanner renders a plain-text SessionStart handshake banner without update checks.
// @intent task_lifecycle_cli_spec.md
func FormatPlainHandshakeBanner(ev *handshake.HandshakeEvent) string {
	return FormatPlainHandshakeBannerWithRoot(ev, "", false, "")
}

// FormatPlainHandshakeBannerWithRoot renders a plain-text SessionStart handshake banner with optional drift/update checks.
// @intent task_lifecycle_cli_spec.md
func FormatPlainHandshakeBannerWithRoot(ev *handshake.HandshakeEvent, hookRoot string, allowUpdateCheck bool, rootVersion string) string {
	return formatHandshakeBanner(ev, hookRoot, allowUpdateCheck, false, rootVersion)
}

func formatHandshakeBanner(ev *handshake.HandshakeEvent, hookRoot string, allowUpdateCheck, containerSandbox bool, rootVersion string) string {
	var b strings.Builder
	b.WriteString("🤝 Nomos JIT Session Handshake\n")
	b.WriteString(fmt.Sprintf("  Active Repository : %s\n", ev.Repository))
	if ev.ActiveTask != "" {
		b.WriteString(fmt.Sprintf("  Workspace Phase   : %s (Task: %s)\n", ev.Phase, ev.ActiveTask))
	} else {
		b.WriteString(fmt.Sprintf("  Workspace Phase   : %s (Trunk baseline)\n", ev.Phase))
	}
	if len(ev.ActiveWorktrees) > 0 {
		b.WriteString(fmt.Sprintf("  Active Worktrees  : [%s] (isolated - safe to run parallel tasks)\n", strings.Join(ev.ActiveWorktrees, ", ")))
	}
	b.WriteString(fmt.Sprintf("  DoD Quality Gates : %d deterministic gates active\n", ev.GateCount))
	appendPlainBrownfieldTopology(&b, ev)
	if len(ev.ReadyWavefront) > 0 {
		b.WriteString(fmt.Sprintf("  Ready Wavefront   : [%s]\n", strings.Join(ev.ReadyWavefront, ", ")))
	}
	appendPlainUpdateAndDriftAdvisories(&b, hookRoot, allowUpdateCheck, rootVersion)
	appendPlainWorkspaceHullWithMode(&b, hookRoot, containerSandbox)
	appendPlainTurnZeroTelemetryWithMode(&b, ev, containerSandbox)
	return b.String()
}

func appendPlainUpdateAndDriftAdvisories(b *strings.Builder, hookRoot string, allowUpdateCheck bool, rootVersion string) {
	if allowUpdateCheck && rootVersion != "" {
		binPath, err := selfupdate.ResolveCurrentBinaryPath()
		if err != nil {
			binPath = ""
		}
		if updNotice := selfupdate.RefreshAndFormatUpdateNotice(rootVersion, binPath, time.Now()); updNotice != "" {
			b.WriteString(fmt.Sprintf("  Update Available  : %s\n", updNotice))
		}
	}
	if hookRoot != "" {
		if driftNotice := selfupdate.FormatWorkspaceDriftNotice(hookRoot); driftNotice != "" {
			b.WriteString(fmt.Sprintf("  Contract Sync     : %s\n", driftNotice))
		}
	}
}

func newHookPrePushCmd() *cobra.Command {
	var targetDir string
	cmd := &cobra.Command{
		Use:     "pre-push",
		Short:   "Enforce CapBAC IFC release-sink taint check during Git pre-push hooks",
		Long:    tui.HelpHookPrePushLong,
		Example: tui.HelpHookPrePushExample,
		RunE: func(_ *cobra.Command, _ []string) error {
			dir := strings.TrimSpace(targetDir)
			if dir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return nil
				}
				dir = cwd
			}
			return EvaluatePrePushIFCGate(dir)
		},
	}
	cmd.Flags().StringVarP(&targetDir, "dir", "d", "", "Target repository directory (defaults to current working directory)")
	return cmd
}

// EvaluatePrePushIFCGate enforces active CapBAC untrusted external and MCP taints for Git pre-push hooks.
// @intent nomos_hermetic_worktree_spec.md #CapabilityFlowGraph
func EvaluatePrePushIFCGate(targetDir string) error {
	cleanDir := filepath.Clean(strings.TrimSpace(targetDir))
	repoRoot := cleanDir
	if discovered, err := workspace.FindRepoRoot(cleanDir); err == nil && discovered != "" {
		repoRoot = discovered
	}
	for _, candidate := range []string{repoRoot, cleanDir} {
		if candidate == "" {
			continue
		}
		if dec := capbac.CheckWorkspaceReleaseTaint(candidate); !dec.Allowed {
			return types.NewNomosError(types.ErrCodeBoundary, "%s", dec.Reason)
		}
	}
	return nil
}
