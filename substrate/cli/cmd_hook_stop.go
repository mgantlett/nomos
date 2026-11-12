// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	hookDecisionBlock    = "block"
	hookDecisionContinue = "continue"
	agyNormalStopReason  = "NO_TOOL_CALL"
	maxStopAuditFiles    = 128
	containerAppWorkdir  = "/app"
)

var skippedStopAuditDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
}

func newHookStopCmd(hermeticDir *string, containerSandbox *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "stop",
		Short:   "Evaluate Law 2 anti-phantom deliverable gate on Stop or AfterAgent turn exit",
		Long:    tui.HelpHookStopLong,
		Example: tui.HelpHookStopExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			payload := parseHookPayload(raw)
			dec := EvaluateStopHook(payload, *hermeticDir, *containerSandbox)
			if dec.Decision != hookDecisionBlock {
				return writeEmptyJSON(cmd.OutOrStdout())
			}
			if payload != nil && payload.IsAntigravityEvent {
				return writeHookDecision(cmd.OutOrStdout(), HookDecision{
					Decision: hookDecisionContinue,
					Reason:   dec.Reason,
				})
			}
			if payload != nil && payload.IsCursorEvent {
				return writeCursorStopResponse(cmd.OutOrStdout(), dec.Reason)
			}
			return writeHookDecision(cmd.OutOrStdout(), dec)
		},
	}
}

// EvaluateStopHook blocks premature agent turn exits when an active worktree holds
// unverified file modifications or when container-sandbox files fail syntax/snooping checks.
func EvaluateStopHook(p *HookPayload, hermeticDir string, containerSandbox bool) HookDecision {
	if p == nil || p.StopHookActive {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if p.TerminationReason != "" && p.TerminationReason != agyNormalStopReason {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if containerSandbox && hermeticDir == "" {
		return evaluateContainerStopHook(p)
	}
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	tr := loadHookSessionTrace(wtRoot)
	if tr == nil || len(tr.MutatedFiles) == 0 {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if tr.DirtyUnverifiedEdits || tr.UnverifiedCalls > 0 {
		return HookDecision{
			Decision: hookDecisionBlock,
			Reason: fmt.Sprintf(
				"❌ [Nomos Anti-Phantom Gate] Active worktree '%s' has %d modified file(s) without a clean post-edit verification pass. Run 'nomos verify' inside the worktree before ending your turn (Law 2).",
				filepath.Base(wtRoot), len(tr.MutatedFiles),
			),
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func evaluateContainerStopHook(p *HookPayload) HookDecision {
	baseCwd := firstNonEmpty(p.Cwd, resolveHookWorkspaceDir(p, ""))
	if baseCwd == "" || baseCwd == "." {
		baseCwd = containerAppWorkdir
	}
	cleanRoot := filepath.Clean(baseCwd)
	if isProtectedContainerVerifierPath(filepath.ToSlash(cleanRoot)) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	scanned := 0
	var fault string
	walkErr := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skippedStopAuditDirs[d.Name()] || isProtectedContainerVerifierPath(filepath.ToSlash(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isStopAuditableFile(path) {
			return nil
		}
		scanned++
		if msg := inspectContainerStopFile(path); msg != "" {
			fault = msg
			return fs.SkipAll
		}
		if scanned >= maxStopAuditFiles {
			return fs.SkipAll
		}
		return nil
	})
	if walkErr != nil && fault == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if fault != "" {
		return HookDecision{
			Decision: hookDecisionBlock,
			Reason:   fmt.Sprintf("❌ [Nomos Anti-Phantom Gate] Cannot complete turn with broken or non-compliant workspace artifact: %s", fault),
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func isStopAuditableFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".rs", ".json", ".jsonl", ".sh":
		return true
	default:
		return false
	}
}

func inspectContainerStopFile(resolved string) string {
	if snoop := validateContainerFileVerifierClean(resolved); snoop != "" {
		return snoop
	}
	if goMsg := validateGoSyntaxInMemory(resolved); goMsg != "" {
		return goMsg
	}
	return validateStructuredFileInMemory(resolved)
}
