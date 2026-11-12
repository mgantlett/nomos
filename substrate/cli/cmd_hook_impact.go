// Package cli implements one-time PostToolUse 1-hop AST impact hints, compact
// co-processor handshake rendering, and PreToolUse symlink/secret-dump guards.
package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

var (
	symlinkCreateCmdRe = regexp.MustCompile(`(?i)(?:^|[;&|]\s*)ln\s+(?:-[a-z]*s[a-z]*|--symbolic)\b`)
	envSecretDumpCmdRe = regexp.MustCompile(`(?i)(?:^|[;&|]\s*)(?:printenv|env|export\s+-p)\b[^\n;|&]*(?:SECRET|TOKEN|PASSWORD|API_KEY|SIGNING_KEY)`)
	siblingEscapeSegRe = regexp.MustCompile(`(?i)(?:^|/)(?:verifier|solution_cache|secrets)(?:/|$)`)
)

func evaluateShellAliasAndRedirectGuard(cmdLine, baseCwd, hermeticDir string) HookDecision {
	if symlinkCreateCmdRe.MatchString(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Filesystem Alias Violation (Law 6 ZeroFilesystemAliasGate): Creating symlinks ('ln -s') is prohibited; materialize explicit canonical files instead.",
		}
	}
	if envSecretDumpCmdRe.MatchString(cmdLine) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Environment Secret Airlock (Law 7): Dumping secret or token environment variables via shell is strictly prohibited.",
		}
	}
	if siblingEscapeSegRe.MatchString(cmdLine) && strings.Contains(cmdLine, "..") {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Hermetic Verifier/Cache Isolation (Law 2 & Law 3): Accessing parent '../verifier', '../solution_cache', or '../secrets' paths is strictly prohibited.",
		}
	}
	return evaluateTrunkShellRedirectGuard(cmdLine, baseCwd, hermeticDir)
}

func evaluateReadToolBoundaryAndBinary(p *HookPayload, hermeticDir string) HookDecision {
	if dec := evaluateBinaryReadGuard(p, hermeticDir != ""); dec.Decision == hookDecisionDeny {
		return dec
	}
	if p == nil || p.TargetFile == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if strings.Contains(p.TargetFile, "..") && siblingEscapeSegRe.MatchString(filepath.ToSlash(p.TargetFile)) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Hermetic Verifier/Cache Isolation (Law 2 & Law 3): Reading '%s' outside the workspace boundary is strictly prohibited.", p.TargetFile),
		}
	}
	if hermeticDir == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	resolved := normalizeCanonicalPath(p.TargetFile, baseCwd)
	return evaluateHermeticDirBoundary(p.TargetFile, resolved, hermeticDir)
}

// evaluateFirstEditImpactAdvisory emits a 1-line 1-hop AST impact and Go interface peer
// hint at most once per mutated source file in a worktree session.
func evaluateFirstEditImpactAdvisory(p *HookPayload, hermeticDir string) string {
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return ""
	}
	tracePath := resolveTraceStateFilePath(wtRoot)
	if tracePath == "" {
		return ""
	}
	canonWt := normalizeCanonicalPath(wtRoot, "/")
	baseCwd := resolveHookWorkspaceDir(p, hermeticDir)
	tr := loadOrInitHookSessionTrace(tracePath, wtRoot)
	relFiles, dirty := collectUnseenImpactedFiles(p, &tr, canonWt, baseCwd)
	if dirty {
		saveHookSessionTrace(tracePath, tr)
	}
	var lines []string
	for _, rel := range relFiles {
		if hint := verify.ComputeFirstEditCoChangeHint(canonWt, rel); hint != "" {
			lines = append(lines, fmt.Sprintf("🧭 [Nomos 1-Hop Impact] %s", hint))
		}
	}
	return strings.Join(lines, "\n")
}

func collectUnseenImpactedFiles(p *HookPayload, tr *hookSessionTrace, canonWt, baseCwd string) ([]string, bool) {
	targets := p.TargetFiles
	if len(targets) == 0 && p.TargetFile != "" {
		targets = []string{p.TargetFile}
	}
	var unseen []string
	dirty := false
	for _, raw := range targets {
		resolved := normalizeCanonicalPath(raw, baseCwd)
		if !strings.HasPrefix(resolved, canonWt+"/") {
			continue
		}
		rel := strings.TrimPrefix(resolved, canonWt+"/")
		if isIgnoredPitfallScopeFile(rel) || containsTraceItem(tr.ImpactedFiles, rel) {
			continue
		}
		tr.ImpactedFiles = appendUniqueBounded(tr.ImpactedFiles, rel, 64)
		dirty = true
		unseen = append(unseen, rel)
	}
	return unseen, dirty
}

const hotspotReadCeilingLOC = 500

// evaluateFirstHotspotReadHint emits a one-time orientation advisory when an agent inside an
// active worktree reads a >=500-LOC brownfield hotspot file, surfacing sibling-file extraction
// guidance and 1-hop AST/interface peers before the first edit is attempted.
func evaluateFirstHotspotReadHint(p *HookPayload, tr *hookSessionTrace, wtRoot string) string {
	resolved, rel, canonWt, ok := resolveUnseenHotspotReadTarget(p, tr, wtRoot)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	lines := bytes.Count(data, []byte("\n")) + 1
	if lines < hotspotReadCeilingLOC {
		return ""
	}
	tr.ImpactedFiles = appendUniqueBounded(tr.ImpactedFiles, rel, 64)
	msg := fmt.Sprintf("🧭 [Nomos Hotspot Read] %s (%d LOC >= %d ceiling): avoid growing in place; keep call-site edits minimal and place net-new helpers in a sibling file", rel, lines, hotspotReadCeilingLOC)
	if peers := verify.ComputeFirstEditCoChangeHint(canonWt, rel); peers != "" {
		msg += " | " + peers
	}
	return msg
}

func resolveUnseenHotspotReadTarget(p *HookPayload, tr *hookSessionTrace, wtRoot string) (string, string, string, bool) {
	if p == nil || tr == nil || wtRoot == "" || p.CommandLine != "" || p.TargetFile == "" {
		return "", "", "", false
	}
	canonWt := normalizeCanonicalPath(wtRoot, "/")
	resolved := normalizeCanonicalPath(p.TargetFile, resolveHookWorkspaceDir(p, ""))
	if !strings.HasPrefix(resolved, canonWt+"/") {
		return "", "", "", false
	}
	rel := strings.TrimPrefix(resolved, canonWt+"/")
	if isIgnoredPitfallScopeFile(rel) || containsTraceItem(tr.ImpactedFiles, rel) {
		return "", "", "", false
	}
	return resolved, rel, canonWt, true
}

func renderCompactHandshakeBanner(event *handshake.HandshakeEvent) {
	if event == nil {
		return
	}
	taskLabel := "trunk"
	if event.ActiveTask != "" {
		taskLabel = event.ActiveTask
	}
	fmt.Printf("[Nomos Compact Handshake] repo=%s phase=%s task=%s gates=%d\n",
		event.Repository, event.Phase, taskLabel, event.GateCount)
	if event.BrownfieldTopology != nil {
		for _, item := range buildBrownfieldTopologyPairs(event.BrownfieldTopology) {
			fmt.Printf("- %s: %s\n", item.label, item.value)
		}
	}
}
