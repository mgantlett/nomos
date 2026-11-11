// Package capbac implements declarative PreToolUse credential read airlocks
// for host credential stores, workspace .env files, and blocked_read_paths globs.
package capbac

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	credentialModeHostDeny  = "host_deny"
	credentialModeStrict    = "strict"
	credentialModeTaintOnly = "taint_only"
)

var (
	metadataOnlyCmdRe = regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?(?:\S+/)?(?:nomos|git\s+(?:status|diff|log|commit|branch|rev-parse|notes))\b`)
	shellArgSplitRe   = regexp.MustCompile(`[\s"'` + "`" + `=;|&()<>]+`)
)

// CredentialReadPolicy defines declarative PreToolUse host and workspace credential read restrictions.
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
type CredentialReadPolicy struct {
	Mode             string
	BlockedReadPaths []string
}

// EffectiveMode returns the normalized credential read mode, defaulting to "host_deny".
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
func (p CredentialReadPolicy) EffectiveMode() string {
	norm := strings.ToLower(strings.TrimSpace(p.Mode))
	switch norm {
	case credentialModeStrict, credentialModeTaintOnly:
		return norm
	default:
		return credentialModeHostDeny
	}
}

// EvaluateCredentialReadAirlock evaluates whether a PreToolUse invocation attempts
// to read restricted host credential stores, strict .env files, or blocked_read_paths globs.
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
func EvaluateCredentialReadAirlock(wsRoot string, inv ToolFlowInvocation, extraTargets []string, policy CredentialReadPolicy) FlowDecision {
	mode := policy.EffectiveMode()
	candidates := collectCredentialReadCandidates(inv, extraTargets)
	for _, cand := range candidates {
		if matchedGlob, blocked := MatchesBlockedReadPath(wsRoot, cand, policy.BlockedReadPaths); blocked {
			return FlowDecision{
				Allowed: false,
				Reason: fmt.Sprintf(
					"❌ [NOMOS INTERCEPT] Credential Read Airlock (Law 7): Access to '%s' via '%s' is blocked by boundary.blocked_read_paths rule '%s' in nomos.yaml.",
					cand, nonEmptyTool(inv.ToolName), matchedGlob,
				),
			}
		}
		if reason := evaluateModeCredentialDeny(cand, inv.ToolName, mode); reason != "" {
			return FlowDecision{Allowed: false, Reason: reason}
		}
	}
	return FlowDecision{Allowed: true, Role: FlowRoleNeutral}
}

func evaluateModeCredentialDeny(cand, toolName, mode string) string {
	if mode == credentialModeTaintOnly {
		return ""
	}
	if IsHostCredentialStorePath(cand) {
		return fmt.Sprintf(
			"❌ [NOMOS INTERCEPT] Credential Read Airlock (Law 7): Reading host credential store '%s' via '%s' is prohibited under boundary.credential_read_mode=%s.",
			cand, nonEmptyTool(toolName), mode,
		)
	}
	if mode == credentialModeStrict && IsRestrictedWorkspaceEnvPath(cand) {
		return fmt.Sprintf(
			"❌ [NOMOS INTERCEPT] Credential Read Airlock (Law 7): Reading credential env file '%s' via '%s' is prohibited under boundary.credential_read_mode=strict.",
			cand, nonEmptyTool(toolName),
		)
	}
	return ""
}

// IsHostCredentialStorePath reports whether rawPath targets a host credential directory or key file.
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
func IsHostCredentialStorePath(rawPath string) bool {
	norm := normalizeCredentialCandidate(rawPath)
	if norm == "" {
		return false
	}
	lower := strings.ToLower(norm)
	base := path.Base(lower)
	if sensitiveBasenames[base] {
		return true
	}
	if matchesHostCredentialDir(lower) {
		return true
	}
	return matchesSSHPrivateMaterial(lower, base)
}

func matchesHostCredentialDir(lower string) bool {
	for _, seg := range []string{"/.aws", "/.kube", "/.gnupg", "/.config/gcloud"} {
		if strings.HasSuffix(lower, seg) || strings.Contains(lower, seg+"/") {
			return true
		}
	}
	return false
}

func matchesSSHPrivateMaterial(lower, base string) bool {
	if strings.HasSuffix(lower, "/.ssh") {
		return true
	}
	if !strings.Contains(lower, "/.ssh/") {
		return false
	}
	if strings.HasSuffix(base, ".pub") || base == "known_hosts" {
		return false
	}
	return true
}

// IsRestrictedWorkspaceEnvPath reports whether rawPath is a non-template .env file.
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
func IsRestrictedWorkspaceEnvPath(rawPath string) bool {
	norm := normalizeCredentialCandidate(rawPath)
	if norm == "" {
		return false
	}
	base := strings.ToLower(path.Base(norm))
	if allowedTemplateEnvFiles[base] {
		return false
	}
	return base == ".env" || strings.HasPrefix(base, ".env.")
}

// MatchesBlockedReadPath checks whether rawPath matches any configured blocked_read_paths glob.
// @intent nomos_hermetic_worktree_spec.md #CredentialReadPolicy
func MatchesBlockedReadPath(wsRoot, rawPath string, patterns []string) (string, bool) {
	if len(patterns) == 0 {
		return "", false
	}
	norm := normalizeCredentialCandidate(rawPath)
	if norm == "" {
		return "", false
	}
	rel := toWorkspaceRelativeSlash(wsRoot, norm)
	for _, rawPat := range patterns {
		pat := strings.TrimSpace(filepath.ToSlash(rawPat))
		if pat == "" {
			continue
		}
		if matchSlashGlob(pat, rel) || matchSlashGlob(pat, norm) || matchSlashGlob(pat, path.Base(norm)) {
			return rawPat, true
		}
	}
	return "", false
}

func toWorkspaceRelativeSlash(wsRoot, normPath string) string {
	trimmed := strings.TrimPrefix(normPath, "./")
	if wsRoot == "" || !strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	cleanRoot := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(wsRoot)), "/")
	if cleanRoot != "" && strings.HasPrefix(trimmed, cleanRoot+"/") {
		return strings.TrimPrefix(trimmed, cleanRoot+"/")
	}
	return trimmed
}

func matchSlashGlob(pattern, candidate string) bool {
	cleanPat := strings.TrimPrefix(pattern, "./")
	cleanCand := strings.TrimPrefix(candidate, "./")
	if strings.HasSuffix(cleanPat, "/**") {
		prefix := strings.TrimSuffix(cleanPat, "/**")
		return cleanCand == prefix || strings.HasPrefix(cleanCand, prefix+"/")
	}
	if strings.Contains(cleanPat, "**") {
		reStr := "^" + strings.ReplaceAll(regexp.QuoteMeta(cleanPat), `\*\*`, `.*`) + "$"
		reStr = strings.ReplaceAll(reStr, `\*`, `[^/]*`)
		if re, err := regexp.Compile(reStr); err == nil {
			return re.MatchString(cleanCand)
		}
	}
	ok, err := path.Match(cleanPat, cleanCand)
	return err == nil && ok
}

func normalizeCredentialCandidate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.Trim(trimmed, `"'`+"`"+`;|&(),[]{}`)
	if trimmed == "" || strings.Contains(trimmed, "://") {
		return ""
	}
	slashed := strings.ReplaceAll(trimmed, `\`, `/`)
	for _, homePrefix := range []string{"~/", "$HOME/", "${HOME}/"} {
		if strings.HasPrefix(slashed, homePrefix) {
			return "/~/" + strings.TrimPrefix(slashed, homePrefix)
		}
	}
	if slashed == "~" {
		return "/~"
	}
	return path.Clean(slashed)
}

func collectCredentialReadCandidates(inv ToolFlowInvocation, extraTargets []string) []string {
	out := collectExplicitTargetCandidates(inv.TargetFile, extraTargets)
	out = appendCommandReadCandidates(out, inv.CommandLine)
	if _, isMCP := ParseMCPInvocation(inv.ToolName, "", ""); isMCP {
		out = appendDigestReadCandidates(out, inv.ArgsDigest)
	}
	return out
}

func collectExplicitTargetCandidates(primary string, extras []string) []string {
	var out []string
	if primary != "" {
		out = append(out, primary)
	}
	for _, t := range extras {
		if t != "" && t != primary {
			out = append(out, t)
		}
	}
	return out
}

func appendCommandReadCandidates(out []string, rawCmd string) []string {
	cmd := strings.TrimSpace(rawCmd)
	if cmd == "" || metadataOnlyCmdRe.MatchString(cmd) {
		return out
	}
	for _, tok := range shellArgSplitRe.Split(cmd, -1) {
		if tok != "" && !strings.HasPrefix(tok, "-") {
			out = append(out, tok)
		}
	}
	return out
}

func appendDigestReadCandidates(out []string, digest string) []string {
	for _, m := range jsonPathTokenRe.FindAllStringSubmatch(digest, -1) {
		if len(m) > 1 && m[1] != "" && !strings.ContainsAny(m[1], " \t\r\n") {
			out = append(out, m[1])
		}
	}
	return out
}

func nonEmptyTool(toolName string) string {
	if trimmed := strings.TrimSpace(toolName); trimmed != "" {
		return trimmed
	}
	return "tool"
}
