// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

var wildRmPattern = regexp.MustCompile(`(?i)(?:^|[;&|]\s*|\bsudo\s+)rm\s+(?:(?:-[a-z]*r[a-z]*|--recursive)\s+(?:/|\*+|~|\.\.?)(?:\s|$|/)|(?:-[a-z]+\s+)*[^\n;|&]*\*)`)

const (
	hookDecisionDeny  = "deny"
	hookDecisionAllow = "allow"
)

func evaluateCommandGuard(cmdLine, baseCwd, hermeticDir string) HookDecision {
	if wildRmPattern.MatchString(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Wildcard or root/parent deletion ('rm ... *', 'rm -rf /', 'rm -rf ~', 'rm -rf ..') is strictly prohibited (Law 7). Verify item-level eligibility and remove explicit file paths only.",
		}
	}
	if isForbiddenNomosGoInstall(cmdLine, baseCwd, hermeticDir) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Manual binary installation ('go install' of nomos) outside a clean IDLE 'main' trunk is prohibited by Law 4 (Single Source of Truth). Sync active tasks to 'main' first or invoke the installed 'nomos' CLI binary directly.",
		}
	}
	if isForbiddenSQLiteCmd(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Direct SQLite access prohibited (Law 1 & ADR-0062). Use canonical CLI commands: 'nomos mem recall', 'nomos mem list', or 'nomos task show'.",
		}
	}
	if isForbiddenGitStashCmd(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] 'git stash' is prohibited (Law 2 & Law 3). Use 'nomos task chore \"<title>\"' or 'git restore <file>'.",
		}
	}
	if isForbiddenGitHooksCmd(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Tampering with CapBAC Git defense hooks (.git/hooks) is strictly prohibited (Law 3 & Law 7).",
		}
	}
	if hermeticDir != "" && strings.Contains(cmdLine, "/etc/") {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Hermetic Container Violation (Law 3): System directory access outside hermetic root is prohibited.",
		}
	}
	if hermeticDir != "" {
		if dec := evaluateInteractiveCommandGuard(cmdLine); dec.Decision == hookDecisionDeny {
			return dec
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func isForbiddenSQLiteCmd(cmdLine string) bool {
	if !strings.Contains(cmdLine, "sqlite3") {
		return false
	}
	return strings.Contains(cmdLine, ".nomos") || strings.Contains(cmdLine, ".db")
}

func isForbiddenGitStashCmd(cmdLine string) bool {
	if !strings.Contains(cmdLine, "git stash") {
		return false
	}
	if strings.Contains(cmdLine, "git stash clear") || strings.Contains(cmdLine, "git stash list") {
		return false
	}
	return true
}

func isForbiddenGitHooksCmd(cmdLine string) bool {
	if !strings.Contains(cmdLine, ".git/hooks") {
		return false
	}
	for _, verb := range []string{"rm ", "unlink ", "chmod ", "mv "} {
		if strings.Contains(cmdLine, verb) {
			return true
		}
	}
	return false
}

// EvaluatePostToolHook runs Law 4 zero-footprint in-memory syntax and AST radar verification on mutated files.
// It returns an empty string (100% silent) on Bash/Read/MCP channels and valid file mutations.
func EvaluatePostToolHook(p *HookPayload, hermeticDir string) string {
	if p == nil || p.IsPostInvoke || p.TargetFile == "" {
		return ""
	}
	if p.CommandLine != "" || capbac.IsMCPToolName(p.ToolName) || isReadOnlyTool(p.ToolName) {
		return ""
	}
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	if !isPostToolTargetInScope(resolved, hermeticDir) {
		return ""
	}
	if syntaxMsg := validateStructuredFileInMemory(resolved); syntaxMsg != "" {
		return syntaxMsg
	}
	return evaluateFastRadarInMemory(resolved, baseCwd)
}

func isPostToolTargetInScope(resolved, hermeticDir string) bool {
	if hermeticDir != "" {
		canonRoot := normalizeCanonicalPath(hermeticDir, "/")
		return resolved == canonRoot || strings.HasPrefix(resolved, canonRoot+"/")
	}
	return strings.Contains(resolved, "/worktrees/")
}

func validateStructuredFileInMemory(resolved string) string {
	ext := strings.ToLower(filepath.Ext(resolved))
	if ext == ".py" {
		return validatePythonASTInMemory(resolved)
	}
	if ext != ".json" && ext != ".jsonl" {
		return ""
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	if ext == ".json" {
		if !json.Valid(data) {
			return fmt.Sprintf("🚨 [Nomos Syntax Guard] %s: invalid JSON syntax detected", filepath.Base(resolved))
		}
		return ""
	}
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if !json.Valid(trimmed) {
			return fmt.Sprintf("🚨 [Nomos Syntax Guard] %s:%d: invalid JSONL record syntax detected", filepath.Base(resolved), i+1)
		}
	}
	return ""
}

func validatePythonASTInMemory(resolved string) string {
	pyBin, err := exec.LookPath("python3")
	if err != nil {
		return ""
	}
	cmd := exec.Command(pyBin, "-B", "-c", "import ast,sys; ast.parse(open(sys.argv[1],encoding='utf-8').read())", resolved)
	out, runErr := cmd.CombinedOutput()
	if runErr == nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := lines[len(lines)-1]
	return fmt.Sprintf("🚨 [Nomos AST Radar] %s: %s", filepath.Base(resolved), last)
}

func evaluateFastRadarInMemory(resolved, baseCwd string) string {
	repoRoot := baseCwd
	if pr, err := workspace.FindRepoRoot(resolved); err == nil {
		repoRoot = pr
	}
	res, err := verify.EvaluateFastFileRadar(repoRoot, resolved)
	if err != nil || res == nil {
		return ""
	}
	var lines []string
	for _, b := range res.Breaches {
		lines = append(lines, fmt.Sprintf("🚨 [Nomos Radar Breach] %s: %s", res.File, b))
	}
	for _, w := range res.Warnings {
		lines = append(lines, fmt.Sprintf("⚠️  [Nomos Radar >=80%% Budget] %s: %s", res.File, w))
	}
	return strings.Join(lines, "\n")
}
