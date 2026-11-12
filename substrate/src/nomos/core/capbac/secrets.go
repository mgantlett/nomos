// Package capbac implements Capability-Based Access Control (CapBAC), compositional
// capability-flow taint tracking, and zero-trust boundary contracts.
package capbac

import "regexp"

type secretTokenRule struct {
	label string
	re    *regexp.Regexp
}

var canonicalSecretRules = []secretTokenRule{
	{label: "PEM private key", re: regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	{label: "AWS access key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{label: "OpenAI API key", re: regexp.MustCompile(`sk-(?:proj|svcacct)-[A-Za-z0-9_-]{20,}`)},
	{label: "Anthropic API key", re: regexp.MustCompile(`sk-ant-api03-[A-Za-z0-9_-]{20,}`)},
	{label: "Google API key", re: regexp.MustCompile(`\bAIzaSy[A-Za-z0-9_-]{33}\b`)},
	{label: "GitHub token", re: regexp.MustCompile(`(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}`)},
	{label: "GitLab token", re: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`)},
	{label: "Stripe live key", re: regexp.MustCompile(`(?:sk|rk)_live_[0-9a-zA-Z]{20,}`)},
	{label: "Slack token", re: regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{20,}`)},
	{label: "HuggingFace token", re: regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`)},
	{label: "xAI API key", re: regexp.MustCompile(`xai-[A-Za-z0-9]{30,}`)},
	{label: "npm registry token", re: regexp.MustCompile(`\bnpm_[A-Za-z0-9_]{12,}\b`)},
	{label: "Webhook signing secret", re: regexp.MustCompile(`\bwhsec_[A-Za-z0-9_]{12,}\b`)},
}

// MatchSecretTokenLabel scans content against the canonical credential signature registry
// and returns the matched secret label or an empty string when clean.
// @intent nomos_hermetic_worktree_spec.md #MCPInvocation
func MatchSecretTokenLabel(content string) string {
	if content == "" {
		return ""
	}
	for _, pat := range canonicalSecretRules {
		if pat.re.MatchString(content) {
			return pat.label
		}
	}
	return ""
}

