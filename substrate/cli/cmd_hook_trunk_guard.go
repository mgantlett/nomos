// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// evaluateTrunkShellRedirectGuard blocks unquoted shell output redirects ('>' / '>>')
// targeting files on a Nomos-governed protected trunk outside '/worktrees/' and '/.tmp/'.
func evaluateTrunkShellRedirectGuard(cmdLine, baseCwd, hermeticDir string) HookDecision {
	if hermeticDir != "" || !strings.Contains(cmdLine, ">") {
		return HookDecision{Decision: hookDecisionAllow}
	}
	cwd := resolveInitialEvalDir(baseCwd)
	for _, rawSeg := range splitUnquotedCommandSegments(cmdLine) {
		segs, subs := parseShellCommandSegments(rawSeg)
		for _, sub := range subs {
			if dec := evaluateTrunkShellRedirectGuard(sub, cwd, hermeticDir); dec.Decision == hookDecisionDeny {
				return dec
			}
		}
		if len(segs) > 0 {
			if nextCwd, ok := matchCdCommand(segs[0], cwd); ok {
				cwd = nextCwd
				continue
			}
		}
		for _, target := range extractUnquotedRedirectTargets(rawSeg) {
			if dec := checkRedirectTargetAgainstTrunk(target, cwd); dec.Decision == hookDecisionDeny {
				return dec
			}
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func checkRedirectTargetAgainstTrunk(rawTarget, cwd string) HookDecision {
	if isBenignRedirectSink(rawTarget) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	resolved := normalizeCanonicalPath(rawTarget, cwd)
	if isPermittedWorktreePath(resolved) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	parentDir := filepath.Dir(resolved)
	repoRoot, err := workspace.FindRepoRoot(parentDir)
	if err != nil || repoRoot == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if !hasExplicitNomosConfig(resolved, repoRoot) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	return HookDecision{
		Decision: hookDecisionDeny,
		Reason: fmt.Sprintf(
			"❌ [NOMOS INTERCEPT] Hermetic Worktree Violation (Law 3): Shell redirect targeting protected trunk path '%s' (resolved: '%s') is prohibited. Write temporary output to '.tmp/' or execute inside an isolated '/worktrees/' task directory.",
			rawTarget, resolved,
		),
	}
}

func isBenignRedirectSink(raw string) bool {
	if raw == "" || raw == "-" || strings.HasPrefix(raw, "&") || strings.Contains(raw, "$") {
		return true
	}
	return strings.HasPrefix(raw, "/dev/")
}

type rawCommandSplitter struct {
	out      []string
	cur      strings.Builder
	inSingle bool
	inDouble bool
}

func splitUnquotedCommandSegments(cmdLine string) []string {
	var sp rawCommandSplitter
	for i := 0; i < len(cmdLine); i++ {
		i = sp.consumeByte(cmdLine, i)
	}
	sp.flush()
	return sp.out
}

func (sp *rawCommandSplitter) consumeByte(cmdLine string, i int) int {
	ch := cmdLine[i]
	if ch == '\\' && !sp.inSingle && i+1 < len(cmdLine) {
		sp.cur.WriteByte(ch)
		sp.cur.WriteByte(cmdLine[i+1])
		return i + 1
	}
	sp.toggleQuotes(ch)
	if !sp.inSingle && !sp.inDouble && isCommandSeparatorByte(ch) {
		sp.flush()
		return i
	}
	sp.cur.WriteByte(ch)
	return i
}

func (sp *rawCommandSplitter) toggleQuotes(ch byte) {
	if ch == '\'' && !sp.inDouble {
		sp.inSingle = !sp.inSingle
	} else if ch == '"' && !sp.inSingle {
		sp.inDouble = !sp.inDouble
	}
}

func (sp *rawCommandSplitter) flush() {
	if s := strings.TrimSpace(sp.cur.String()); s != "" {
		sp.out = append(sp.out, s)
	}
	sp.cur.Reset()
}

func isCommandSeparatorByte(ch byte) bool {
	return ch == ';' || ch == '|' || ch == '&' || ch == '\n'
}

func extractUnquotedRedirectTargets(seg string) []string {
	var targets []string
	inSingle, inDouble := false, false
	for i := 0; i < len(seg); i++ {
		ch := seg[i]
		if ch == '\\' && !inSingle && i+1 < len(seg) {
			i++
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble || ch != '>' {
			continue
		}
		target, nextIdx := consumeRedirectTargetAt(seg, i)
		if target != "" {
			targets = append(targets, target)
		}
		i = nextIdx
	}
	return targets
}

func consumeRedirectTargetAt(seg string, idx int) (string, int) {
	if idx > 0 && (seg[idx-1] == '=' || seg[idx-1] == '-') {
		return "", idx
	}
	j := idx + 1
	if j < len(seg) && seg[j] == '>' {
		j++
	}
	if j < len(seg) && (seg[j] == '&' || seg[j] == '=' || seg[j] == '(') {
		return "", j
	}
	for j < len(seg) && (seg[j] == ' ' || seg[j] == '\t') {
		j++
	}
	start := j
	for j < len(seg) && !strings.ContainsRune(" \t;|<>()", rune(seg[j])) {
		j++
	}
	tok := strings.Trim(seg[start:j], "\"'`")
	return tok, j - 1
}
