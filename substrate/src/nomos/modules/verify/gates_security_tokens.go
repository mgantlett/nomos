// Package verify implements modern credential token scanning and dependency manifest locks.
package verify

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

type secretPattern struct {
	label string
	re    *regexp.Regexp
}

var canonicalSecretPatterns = []secretPattern{
	{label: "PEM private key", re: regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	{label: "AWS access key", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{label: "OpenAI API key", re: regexp.MustCompile(`sk-(?:proj|svcacct)-[A-Za-z0-9_-]{20,}`)},
	{label: "Anthropic API key", re: regexp.MustCompile(`sk-ant-api03-[A-Za-z0-9_-]{20,}`)},
	{label: "Google API key", re: regexp.MustCompile(`AIzaSy[A-Za-z0-9_-]{33}`)},
	{label: "GitHub token", re: regexp.MustCompile(`(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}`)},
	{label: "GitLab token", re: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`)},
	{label: "Stripe live key", re: regexp.MustCompile(`(?:sk|rk)_live_[0-9a-zA-Z]{20,}`)},
	{label: "Slack token", re: regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{20,}`)},
	{label: "HuggingFace token", re: regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`)},
	{label: "xAI API key", re: regexp.MustCompile(`xai-[A-Za-z0-9]{30,}`)},
}

var allowedEnvTemplates = map[string]bool{
	".env.example":  true,
	".env.sample":   true,
	".env.template": true,
	".env.test":     true,
	".env.defaults": true,
	".env.schema":   true,
}

var lockedDependencyManifests = map[string]bool{
	"go.mod":            true,
	"go.sum":            true,
	"package.json":      true,
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"bun.lock":          true,
	"bun.lockb":         true,
	"cargo.toml":        true,
	"cargo.lock":        true,
	"pyproject.toml":    true,
	"poetry.lock":       true,
	"uv.lock":           true,
	"requirements.txt":  true,
	"gemfile":           true,
	"gemfile.lock":      true,
}

func matchSecretTokenLabel(content string) string {
	for _, pat := range canonicalSecretPatterns {
		if pat.re.MatchString(content) {
			return pat.label
		}
	}
	return ""
}

func isProhibitedEnvFileName(base string) bool {
	lower := strings.ToLower(strings.TrimSpace(base))
	if allowedEnvTemplates[lower] {
		return false
	}
	return lower == ".env" || strings.HasPrefix(lower, ".env.")
}

func isGitIgnoredPath(root, targetPath string) bool {
	cmd := workspace.SafeGitCommand(root, "check-ignore", "-q", targetPath)
	return cmd.Run() == nil
}

func scanUnignoredEnvFiles(root string) []string {
	var leaks []string
	seen := make(map[string]bool)

	checkRel := func(rel string) {
		clean := strings.TrimSpace(filepath.ToSlash(rel))
		if clean == "" || seen[clean] {
			return
		}
		if isProhibitedEnvFileName(filepath.Base(clean)) {
			seen[clean] = true
			leaks = append(leaks, clean+" (unignored .env file)")
		}
	}

	if out, err := workspace.SafeGitCommand(root, "ls-files").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			checkRel(line)
		}
	}
	if out, err := workspace.SafeGitCommand(root, "status", "--porcelain").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if len(line) > 3 && !strings.HasPrefix(line, "D ") && !strings.HasPrefix(line, " D") {
				rel := strings.TrimSpace(line[3:])
				if idx := strings.LastIndex(rel, sepArrow); idx >= 0 {
					rel = rel[idx+len(sepArrow):]
				}
				checkRel(rel)
			}
		}
	}
	return leaks
}

func scanLockedDependencyMutations(ctx *VerifyContext, root string, lockDeps bool) []string {
	if !lockDeps {
		return nil
	}
	var taskID string
	if ctx != nil {
		taskID = ctx.TaskID
	}
	modified := ListModifiedWorktreeFiles(root, taskID)
	var violations []string
	for _, rel := range modified {
		base := strings.ToLower(filepath.Base(rel))
		if lockedDependencyManifests[base] {
			violations = append(violations, rel+" (dependency manifest locked via boundary.lock_dependencies)")
		}
	}
	return violations
}

