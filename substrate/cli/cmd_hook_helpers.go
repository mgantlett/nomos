// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// EvaluatePreToolHook enforces Law 3 boundary isolation, Law 7 command safety,
// and Law 1 identical-failure loop circuit-breaking on PreToolUse events.
func EvaluatePreToolHook(p *HookPayload, hermeticDir string) HookDecision {
	return EvaluatePreToolHookWithProfile(p, hermeticDir, resolveHookAdvisoryProfile(0, 0, 0, ""))
}

// EvaluatePreToolHookWithProfile evaluates hard PreToolUse safety invariants alongside
// the bounded loop circuit-breaker threshold from profile.
func EvaluatePreToolHookWithProfile(p *HookPayload, hermeticDir string, profile HookAdvisoryProfile) HookDecision {
	if p == nil {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if p.CommandLine != "" {
		cmdCwd := firstNonEmpty(p.Cwd, resolveHookWorkspaceDir(p, hermeticDir))
		if dec := evaluateCommandGuard(p.CommandLine, cmdCwd, hermeticDir); dec.Decision == hookDecisionDeny {
			return dec
		}
	}
	wsRoot := resolveCanonicalHookWorkspace(p, hermeticDir)
	if dec := evaluateOutboundEgressGuard(p, wsRoot); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateCredentialReadGuard(p, wsRoot); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateMCPPreToolGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateTargetFilePreToolGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateCapabilityFlowGuard(p, hermeticDir); dec.Decision == hookDecisionDeny {
		return dec
	}
	return EvaluateLoopCircuitBreakerWithStrikes(p, hermeticDir, profile.LoopStrikes)
}

func evaluateTargetFilePreToolGuard(p *HookPayload, hermeticDir string) HookDecision {
	targets := p.TargetFiles
	if len(targets) == 0 && p.TargetFile != "" {
		targets = []string{p.TargetFile}
	}
	if len(targets) == 0 {
		return HookDecision{Decision: hookDecisionAllow}
	}
	for _, target := range targets {
		cp := *p
		cp.TargetFile = target
		if cp.CommandLine == "" && hasUnexpandedShellPathToken(target) {
			return HookDecision{
				Decision: hookDecisionDeny,
				Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Native file tools do not expand shell syntax; pass a concrete relative or resolved path.", target),
			}
		}
		var dec HookDecision
		if !isReadOnlyTool(cp.ToolName) {
			dec = evaluateFileMutationGuard(&cp, hermeticDir)
		} else {
			dec = evaluateBinaryReadGuard(&cp, hermeticDir != "")
		}
		if dec.Decision == hookDecisionDeny {
			return dec
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func evaluateMCPPreToolGuard(p *HookPayload, hermeticDir string) HookDecision {
	mcpInv, ok := capbac.ParseMCPInvocation(p.ToolName, p.TargetFile, p.ArgsDigest)
	if !ok {
		return HookDecision{Decision: hookDecisionAllow}
	}
	wsRoot := resolveCanonicalHookWorkspace(p, hermeticDir)
	policy := loadWorkspaceMCPBoundaryPolicy(wsRoot)
	dec := capbac.EvaluateMCPBoundary(wsRoot, mcpInv, policy)
	if !dec.Allowed {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   dec.Reason,
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func loadWorkspaceMCPBoundaryPolicy(wsRoot string) capbac.MCPBoundaryPolicy {
	cfg, err := config.LoadConfig(wsRoot)
	if err != nil || cfg == nil {
		return capbac.MCPBoundaryPolicy{}
	}
	return capbac.MCPBoundaryPolicy{
		AllowedServers: cfg.Governance.Boundary.AllowedMCPServers,
		BlockedServers: cfg.Governance.Boundary.BlockedMCPServers,
	}
}

func evaluateCapabilityFlowGuard(p *HookPayload, hermeticDir string) HookDecision {
	wsRoot := resolveCanonicalHookWorkspace(p, hermeticDir)
	flowDec := capbac.EvaluateWorkspaceToolFlow(wsRoot, capbac.ToolFlowInvocation{
		ToolName:    p.ToolName,
		TargetFile:  p.TargetFile,
		CommandLine: p.CommandLine,
		ArgsDigest:  p.ArgsDigest,
	})
	if !flowDec.Allowed {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   flowDec.Reason,
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func resolveCanonicalHookWorkspace(p *HookPayload, hermeticDir string) string {
	if hermeticDir != "" {
		return filepath.Clean(hermeticDir)
	}
	base := resolveHookWorkspaceDir(p, "")
	if pr, err := workspace.FindRepoRoot(base); err == nil && pr != "" {
		return filepath.Clean(pr)
	}
	return filepath.Clean(base)
}

func isReadOnlyTool(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if mcpInv, ok := capbac.ParseMCPInvocation(lower, "", ""); ok {
		return mcpInv.Mode == capbac.MCPModeRead
	}
	return readOnlyTools[lower]
}

func hasUnexpandedShellPathToken(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "~") {
		return true
	}
	return strings.Contains(trimmed, "$")
}

func evaluateFileMutationGuard(p *HookPayload, hermeticDir string) HookDecision {
	if hasUnexpandedShellPathToken(p.TargetFile) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Pass a canonical resolved path.", p.TargetFile),
		}
	}
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if isSystemRootPath(resolved) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Hermetic Boundary Violation (Law 3): System path mutation '%s' is strictly prohibited.", resolved),
		}
	}
	if hermeticDir != "" {
		return evaluateHermeticDirBoundary(p.TargetFile, resolved, hermeticDir)
	}
	return evaluateWorktreeBoundary(p.TargetFile, resolved, baseCwd, p.ToolName)
}

func evaluateHermeticDirBoundary(rawTarget, resolved, hermeticDir string) HookDecision {
	canonRoot := normalizeCanonicalPath(hermeticDir, "/")
	if resolved == canonRoot || strings.HasPrefix(resolved, canonRoot+"/") {
		return HookDecision{Decision: hookDecisionAllow}
	}
	return HookDecision{
		Decision: hookDecisionDeny,
		Reason:   fmt.Sprintf("Hermetic Container Violation (Law 3): Target '%s' (resolved: '%s') escapes hermetic root '%s'.", rawTarget, resolved, canonRoot),
	}
}

func evaluateWorktreeBoundary(rawTarget, resolved, baseCwd, toolName string) HookDecision {
	if isPermittedWorktreePath(resolved, baseCwd) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if toolName == "" {
		toolName = "file_mutation"
	}
	choreHint := `nomos task chore "<title>"`
	if strings.Contains(resolved, "/intent/") || strings.HasSuffix(resolved, "/nomos.yaml") {
		choreHint = `nomos task chore "<title>" --plane intent`
	}
	return HookDecision{
		Decision: hookDecisionDeny,
		Reason: fmt.Sprintf(
			"Hermetic Worktree Violation (Law 3): Direct trunk mutation of '%s' (resolved: '%s') via '%s' is prohibited. Provision an isolated EDIT worktree in 1 turn via '%s' (or 'nomos task start <KEY> --edit'), then edit strictly inside '/worktrees/'.",
			rawTarget, resolved, toolName, choreHint,
		),
	}
}

func isPermittedWorktreePath(resolved, baseCwd string) bool {
	if strings.Contains(resolved, "/worktrees/") || strings.Contains(resolved, "/.tmp/") {
		return true
	}
	return isExternalAgentBrainPath(resolved, baseCwd)
}

func isExternalAgentBrainPath(resolved, baseCwd string) bool {
	if !strings.Contains(resolved, "/brain/") {
		return false
	}
	if isInsideWorkspaceRoot(resolved, baseCwd) {
		return false
	}
	return hasAgentBrainParentSegment(resolved)
}

func isInsideWorkspaceRoot(resolved, baseCwd string) bool {
	if baseCwd == "" {
		return false
	}
	canonBase := normalizeCanonicalPath(baseCwd, "/")
	if pr, err := workspace.FindRepoRoot(canonBase); err == nil && pr != "" {
		canonBase = filepath.ToSlash(filepath.Clean(pr))
	}
	return resolved == canonBase || strings.HasPrefix(resolved, canonBase+"/")
}

func hasAgentBrainParentSegment(resolved string) bool {
	segs := strings.Split(resolved, "/")
	for i := 1; i < len(segs)-1; i++ {
		if segs[i] != "brain" {
			continue
		}
		parent := segs[i-1]
		if parent == "antigravity" || parent == "antigravity-ide" || parent == "antigravity-cli" {
			return true
		}
		if len(parent) > 1 && strings.HasPrefix(parent, ".") {
			return true
		}
	}
	return false
}

func isSystemRootPath(resolved string) bool {
	for _, sys := range systemRootPrefixes {
		if resolved == sys || strings.HasPrefix(resolved, sys+"/") {
			return true
		}
	}
	return resolved == "/"
}

func normalizeCanonicalPath(raw, base string) string {
	target := raw
	if !filepath.IsAbs(target) {
		if base == "" {
			base = "/"
		}
		target = filepath.Join(base, target)
	}
	cleaned := filepath.Clean(target)
	if eval, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.ToSlash(eval)
	}
	parent := filepath.Dir(cleaned)
	if evalParent, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.ToSlash(filepath.Join(evalParent, filepath.Base(cleaned)))
	}
	return filepath.ToSlash(cleaned)
}
