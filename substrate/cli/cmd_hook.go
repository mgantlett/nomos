// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/spf13/cobra"
)

// HookDecision represents the structured JSON response emitted by PreToolUse hooks.
type HookDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

// NewHookCmd constructs the canonical 'nomos hook' command group.
func NewHookCmd() *cobra.Command {
	var hermeticDir string
	var containerSandbox bool
	var budgetSec int

	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Execute compiled zero-dependency agent lifecycle hooks (session-start, pre-tool, post-tool)",
		Long: `Native compiled Go hook interceptors for Tier 1 coding agents (Claude Code, Cursor, Antigravity).
Reads JSON hook payloads from stdin with zero external jq/bash/python dependencies.`,
		Example: helpHookExample,
	}

	cmd.PersistentFlags().StringVar(&hermeticDir, "hermetic-dir", "", "Confine file mutations and shell paths to a container/sandbox root directory")
	cmd.PersistentFlags().BoolVar(&containerSandbox, "container-sandbox", false, "Enable disposable benchmark container governance (blocks verifier/solution tampering while permitting OS-wide task paths)")
	cmd.PersistentFlags().IntVar(&budgetSec, "budget-sec", 0, "Optional session wall-clock budget in seconds for 75%/90% PostToolUse checkpoint advisories")

	cmd.AddCommand(
		newHookSessionStartCmd(&hermeticDir, &budgetSec),
		newHookPreToolCmd(&hermeticDir, &containerSandbox, &budgetSec),
		newHookPostToolCmd(&hermeticDir, &containerSandbox, &budgetSec),
	)
	return cmd
}

func newHookSessionStartCmd(hermeticDir *string, budgetSec *int) *cobra.Command {
	return &cobra.Command{
		Use:     "session-start",
		Short:   "Execute SessionStart orientation handshake from JSON stdin payload",
		Long:    helpHookSessionStartLong,
		Example: helpHookSessionStartExample,
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

			banner := formatPlainHandshakeBanner(ev)
			return writeSessionStartInjection(cmd.OutOrStdout(), banner)
		},
	}
}

func newHookPreToolCmd(hermeticDir *string, containerSandbox *bool, budgetSec *int) *cobra.Command {
	return &cobra.Command{
		Use:     "pre-tool",
		Short:   "Evaluate Law 3 boundary isolation and Law 7 command safety on PreToolUse payload",
		Long:    helpHookPreToolLong,
		Example: helpHookPreToolExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeHookDecision(cmd.OutOrStdout(), HookDecision{Decision: "allow"})
			}
			payload := parseHookPayload(raw)
			ensureSessionBudgetStamp(payload, *hermeticDir, resolveEffectiveBudgetSeconds(*budgetSec))
			dec := dispatchPreToolHook(payload, *hermeticDir, *containerSandbox)
			return writePreToolResponse(cmd.OutOrStdout(), payload, dec)
		},
	}
}

func newHookPostToolCmd(hermeticDir *string, containerSandbox *bool, budgetSec *int) *cobra.Command {
	return &cobra.Command{
		Use:     "post-tool",
		Short:   "Evaluate Law 4 zero-footprint in-memory AST and syntax radar on PostToolUse payload",
		Long:    helpHookPostToolLong,
		Example: helpHookPostToolExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			payload := parseHookPayload(raw)
			msg := dispatchPostToolHook(payload, *hermeticDir, *containerSandbox, resolveEffectiveBudgetSeconds(*budgetSec))
			if msg == "" {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			return writePostToolAdvisory(cmd.OutOrStdout(), payload, msg)
		},
	}
}

func dispatchPreToolHook(payload *HookPayload, hermeticDir string, containerSandbox bool) HookDecision {
	if containerSandbox && hermeticDir == "" {
		return EvaluateContainerPreToolHook(payload)
	}
	return EvaluatePreToolHook(payload, hermeticDir)
}

func dispatchPostToolHook(payload *HookPayload, hermeticDir string, containerSandbox bool, budgetSec int) string {
	budgetMsg := evaluateSessionBudgetRadar(payload, hermeticDir, budgetSec)
	var syntaxMsg string
	if containerSandbox && hermeticDir == "" {
		syntaxMsg = EvaluateContainerPostToolHook(payload)
	} else {
		syntaxMsg = EvaluatePostToolHook(payload, hermeticDir)
	}
	return joinHookAdvisories(budgetMsg, syntaxMsg)
}

func writeEmptyJSON(w io.Writer) error {
	_, err := fmt.Fprintln(w, "{}")
	return err
}

func writePreToolResponse(w io.Writer, payload *HookPayload, dec HookDecision) error {
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

func writeHookDecision(w io.Writer, dec HookDecision) error {
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

func writePostToolAdvisory(w io.Writer, payload *HookPayload, msg string) error {
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
	return writeJSONMap(w, map[string]any{
		"injectSteps": []map[string]string{
			{"ephemeralMessage": msg},
		},
	})
}

func writeJSONMap(w io.Writer, out map[string]any) error {
	data, err := json.Marshal(out)
	if err != nil {
		return writeEmptyJSON(w)
	}
	_, wErr := fmt.Fprintln(w, string(data))
	return wErr
}

func formatPlainHandshakeBanner(ev *handshake.HandshakeEvent) string {
	var b strings.Builder
	b.WriteString("🤝 Nomos JIT Session Handshake\n")
	b.WriteString(fmt.Sprintf("  Active Repository : %s\n", ev.Repository))
	if ev.ActiveTask != "" {
		b.WriteString(fmt.Sprintf("  Workspace Phase   : %s (Task: %s)\n", ev.Phase, ev.ActiveTask))
	} else {
		b.WriteString(fmt.Sprintf("  Workspace Phase   : %s (Trunk baseline)\n", ev.Phase))
	}
	b.WriteString(fmt.Sprintf("  DoD Quality Gates : %d deterministic gates active\n", ev.GateCount))
	if len(ev.ReadyWavefront) > 0 {
		b.WriteString(fmt.Sprintf("  Ready Wavefront   : [%s]\n", strings.Join(ev.ReadyWavefront, ", ")))
	}
	return b.String()
}
