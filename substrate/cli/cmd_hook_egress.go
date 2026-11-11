// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

var (
	httpURLTokenRe   = regexp.MustCompile(`https?://[^\s"'` + "`" + `<>;|&()]+`)
	shellNetVerbRe   = regexp.MustCompile(`(?i)(?:^|[;&|(\s])(?:curl|wget|http|xh)\b`)
	egressSecretToks = []*regexp.Regexp{
		regexp.MustCompile(`\bghp_[A-Za-z0-9]{16,}\b`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{16,}\b`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{12,}\b`),
		regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}\b`),
		regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{12,}\b`),
	}
	blockedMetadataHosts = map[string]bool{
		"169.254.169.254":          true,
		"169.254.170.2":            true,
		"100.100.100.200":          true,
		"fd00:ec2::254":            true,
		"metadata.google.internal": true,
		"metadata.goog":            true,
		"metadata.azure.internal":  true,
		"instance-data":            true,
	}
)

// evaluateOutboundEgressGuard enforces cloud metadata SSRF blocking, credential and PII
// exfiltration airlocks, and declarative BoundaryConfig domain policies on PreToolUse.
func evaluateOutboundEgressGuard(p *HookPayload, wsRoot string) HookDecision {
	if p == nil {
		return HookDecision{Decision: hookDecisionAllow}
	}
	urls := extractOutboundHTTPURLs(p)
	if len(urls) == 0 {
		return HookDecision{Decision: hookDecisionAllow}
	}
	rawMaterial := strings.Join([]string{p.TargetURL, p.CommandLine, p.ArgsDigest}, "\n")
	if hasOutboundSecretToken(rawMaterial) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   "❌ [NOMOS INTERCEPT] Outbound Secret Airlock (Law 7): Credential token detected in outbound HTTP request is strictly prohibited.",
		}
	}
	boundary := loadWorkspaceBoundaryConfig(wsRoot)
	var gitEmail string
	var loadedEmail bool
	for _, rawURL := range urls {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			continue
		}
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		if isBlockedMetadataHost(host) {
			return HookDecision{
				Decision: hookDecisionDeny,
				Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Outbound SSRF Violation (Law 7): Cloud metadata or link-local endpoint '%s' is strictly prohibited.", host),
			}
		}
		if dec := evaluateDomainBoundaryPolicy(host, boundary); dec.Decision == hookDecisionDeny {
			return dec
		}
		if isLoopbackHost(host) {
			continue
		}
		if !loadedEmail {
			gitEmail = resolveWorkspaceGitEmail(wsRoot)
			loadedEmail = true
		}
		if containsEmailExfiltration(rawMaterial, gitEmail) {
			return HookDecision{
				Decision: hookDecisionDeny,
				Reason:   "❌ [NOMOS INTERCEPT] Outbound PII Airlock (Law 7): Operator git email detected in outbound HTTP request is strictly prohibited.",
			}
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func extractOutboundHTTPURLs(p *HookPayload) []string {
	var out []string
	if u := strings.TrimSpace(p.TargetURL); strings.HasPrefix(strings.ToLower(u), "http://") || strings.HasPrefix(strings.ToLower(u), "https://") {
		out = append(out, u)
	}
	if cmd := strings.TrimSpace(p.CommandLine); cmd != "" && shellNetVerbRe.MatchString(cmd) {
		matches := httpURLTokenRe.FindAllString(cmd, -1)
		out = append(out, matches...)
	}
	return out
}

func hasOutboundSecretToken(material string) bool {
	for _, pat := range egressSecretToks {
		if pat.MatchString(material) {
			return true
		}
	}
	return false
}

func isBlockedMetadataHost(host string) bool {
	if blockedMetadataHosts[host] {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLinkLocalUnicast()
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func evaluateDomainBoundaryPolicy(host string, boundary config.BoundaryConfig) HookDecision {
	if domainMatchesPolicyList(host, boundary.BlockedEgressDomains) {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Outbound Egress Policy Violation (Law 7): Domain '%s' is blocked by nomos.yaml blocked_egress_domains.", host),
		}
	}
	if isLoopbackHost(host) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	switch boundary.EffectiveEgressMode() {
	case config.EgressModeOffline:
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Outbound Egress Policy Violation (Law 7): External network egress to '%s' is prohibited when egress_mode is 'offline'.", host),
		}
	case config.EgressModeAllowlist:
		if !domainMatchesPolicyList(host, boundary.AllowedEgressDomains) {
			return HookDecision{
				Decision: hookDecisionDeny,
				Reason:   fmt.Sprintf("❌ [NOMOS INTERCEPT] Outbound Egress Policy Violation (Law 7): Domain '%s' is not in nomos.yaml allowed_egress_domains.", host),
			}
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func domainMatchesPolicyList(host string, patterns []string) bool {
	for _, raw := range patterns {
		pat := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "*.")))
		pat = strings.TrimPrefix(pat, ".")
		if pat == "" {
			continue
		}
		if host == pat || strings.HasSuffix(host, "."+pat) {
			return true
		}
	}
	return false
}

func loadWorkspaceBoundaryConfig(wsRoot string) config.BoundaryConfig {
	cfg, err := config.LoadConfig(wsRoot)
	if err != nil || cfg == nil {
		return config.BoundaryConfig{}
	}
	return cfg.Governance.Boundary
}

func resolveWorkspaceGitEmail(wsRoot string) string {
	if strings.TrimSpace(wsRoot) == "" {
		return ""
	}
	out, err := workspace.SafeGitCommand(wsRoot, "config", "--get", "user.email").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func containsEmailExfiltration(material, email string) bool {
	if email == "" || !strings.Contains(email, "@") {
		return false
	}
	lowerMat := strings.ToLower(material)
	lowerEmail := strings.ToLower(email)
	encodedEmail := strings.ReplaceAll(lowerEmail, "@", "%40")
	return strings.Contains(lowerMat, lowerEmail) || strings.Contains(lowerMat, encodedEmail)
}

func evaluateCredentialReadGuard(p *HookPayload, wsRoot string) HookDecision {
	if p == nil || (p.TargetFile == "" && len(p.TargetFiles) == 0 && p.CommandLine == "" && p.ArgsDigest == "") {
		return HookDecision{Decision: hookDecisionAllow}
	}
	boundary := loadWorkspaceBoundaryConfig(wsRoot)
	dec := capbac.EvaluateCredentialReadAirlock(wsRoot, capbac.ToolFlowInvocation{
		ToolName:    p.ToolName,
		TargetFile:  p.TargetFile,
		CommandLine: p.CommandLine,
		ArgsDigest:  p.ArgsDigest,
	}, p.TargetFiles, capbac.CredentialReadPolicy{
		Mode:             boundary.EffectiveCredentialReadMode(),
		BlockedReadPaths: boundary.BlockedReadPaths,
	})
	if !dec.Allowed {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason:   dec.Reason,
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}
