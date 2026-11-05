// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const containerDefaultWorkdir = "/app"

var containerVerifierRoots = []string{
	"/logs/verifier",
	"/solution",
	"/tests",
}

// EvaluateContainerPreToolHook enforces disposable benchmark container guardrails:
// blocks reward-hacking access to verifier/solution roots, destructive wildcard
// deletions, credential exfiltration, and 3-strike identical-failure tool loops
// while permitting legitimate OS-wide task paths (/etc, /tmp, /var, sqlite3).
func EvaluateContainerPreToolHook(p *HookPayload) HookDecision {
	if p == nil {
		return HookDecision{Decision: hookDecisionAllow}
	}
	baseCwd := firstNonEmpty(p.Cwd, resolveHookWorkspaceDir(p, ""))
	if baseCwd == "" || baseCwd == "." {
		baseCwd = containerDefaultWorkdir
	}
	if p.CommandLine != "" {
		if dec := evaluateContainerCommandGuard(p.CommandLine, baseCwd); dec.Decision == hookDecisionDeny {
			return dec
		}
	}
	if dec := evaluateMCPPreToolGuard(p, baseCwd); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateContainerTargetGuard(p, baseCwd); dec.Decision == hookDecisionDeny {
		return dec
	}
	if dec := evaluateCapabilityFlowGuard(p, baseCwd); dec.Decision == hookDecisionDeny {
		return dec
	}
	return EvaluateLoopCircuitBreaker(p, "")
}

func evaluateContainerCommandGuard(cmdLine, baseCwd string) HookDecision {
	if wildRmPattern.MatchString(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Wildcard or root/parent deletion ('rm ... *', 'rm -rf /', 'rm -rf ~', 'rm -rf ..') is strictly prohibited (Law 7). Verify item-level eligibility and remove explicit file paths only.",
		}
	}
	if isForbiddenNomosGoInstall(cmdLine, baseCwd, baseCwd) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Manual binary installation ('go install' of nomos) is prohibited in container sandbox mode (Law 4).",
		}
	}
	if isForbiddenGitHooksCmd(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Tampering with CapBAC Git defense hooks (.git/hooks) is strictly prohibited (Law 3 & Law 7).",
		}
	}
	if hit := findProtectedVerifierToken(cmdLine, baseCwd); hit != "" {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Anti-Reward-Hacking Violation (Law 2 & Law 3): Shell access targeting protected verifier/oracle path '%s' is strictly prohibited.", hit),
		}
	}
	return evaluateInteractiveCommandGuard(cmdLine)
}

func evaluateContainerTargetGuard(p *HookPayload, baseCwd string) HookDecision {
	if p.TargetFile == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if p.CommandLine == "" && hasUnexpandedShellPathToken(p.TargetFile) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Hermetic Path Violation (Law 3): Unexpanded shell variable or tilde in target path '%s'. Pass a concrete resolved path.", p.TargetFile),
		}
	}
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if isProtectedContainerVerifierPath(resolved) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("Anti-Reward-Hacking Violation (Law 2 & Law 3): Access to protected verifier/oracle path '%s' is strictly prohibited.", resolved),
		}
	}
	return evaluateBinaryReadGuard(p, true)
}

func isProtectedContainerVerifierPath(resolved string) bool {
	for _, root := range containerVerifierRoots {
		if resolved == root || strings.HasPrefix(resolved, root+"/") {
			return true
		}
	}
	return false
}

func findProtectedVerifierToken(cmdLine, baseCwd string) string {
	for _, rawTok := range tokenizeShellWords(cmdLine) {
		cleaned := strings.Trim(rawTok, "\"'`();|&<>")
		if idx := strings.Index(cleaned, "="); idx >= 0 && !strings.HasPrefix(cleaned, "/") {
			cleaned = cleaned[idx+1:]
		}
		if cleaned == "" || strings.HasPrefix(cleaned, "-") {
			continue
		}
		if strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, ".") {
			resolved := normalizeCanonicalPath(cleaned, baseCwd)
			if isProtectedContainerVerifierPath(resolved) {
				return resolved
			}
		}
	}
	return ""
}

func tokenizeShellWords(cmdLine string) []string {
	replacer := strings.NewReplacer(
		";", " ", "|", " ", "&", " ",
		">", " ", "<", " ", "(", " ", ")", " ",
	)
	return strings.Fields(replacer.Replace(cmdLine))
}

// EvaluateContainerPostToolHook runs sub-15ms in-memory syntax and AST verification
// across edited container files while avoiding host-only LOC noise on non-Nomos repos.
func EvaluateContainerPostToolHook(p *HookPayload) string {
	if p == nil || p.IsPostInvoke || p.TargetFile == "" {
		return ""
	}
	if p.CommandLine != "" || capbac.IsMCPToolName(p.ToolName) || isReadOnlyTool(p.ToolName) {
		return ""
	}
	baseCwd := firstNonEmpty(p.Cwd, resolveHookWorkspaceDir(p, ""))
	if baseCwd == "" || baseCwd == "." {
		baseCwd = containerDefaultWorkdir
	}
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if syntaxMsg := validateStructuredFileInMemory(resolved); syntaxMsg != "" {
		return syntaxMsg
	}
	if goMsg := validateGoSyntaxInMemory(resolved); goMsg != "" {
		return goMsg
	}
	if hasExplicitNomosConfig(resolved, baseCwd) {
		return evaluateFastRadarInMemory(resolved, baseCwd)
	}
	return ""
}

func validateGoSyntaxInMemory(resolved string) string {
	if strings.ToLower(filepath.Ext(resolved)) != ".go" {
		return ""
	}
	src, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	fset := token.NewFileSet()
	if _, pErr := parser.ParseFile(fset, resolved, src, parser.AllErrors); pErr != nil {
		first := strings.Split(strings.TrimSpace(pErr.Error()), "\n")[0]
		return fmt.Sprintf("🚨 [Nomos AST Radar] %s: %s", filepath.Base(resolved), first)
	}
	return ""
}

func hasExplicitNomosConfig(resolved, baseCwd string) bool {
	repoRoot := baseCwd
	if pr, err := workspace.FindRepoRoot(resolved); err == nil && pr != "" {
		repoRoot = pr
	}
	candidates := []string{
		filepath.Join(repoRoot, "nomos.yaml"),
		filepath.Join(repoRoot, "intent", "config", "nomos.yaml"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}
