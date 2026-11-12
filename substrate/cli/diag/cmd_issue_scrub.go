// Package diag implements deterministic Cobra command dispatch, software physics telemetry,
// session management, and terminal UI contracts for Nomos.
package diag

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	BranchTopologyTrunk    = "trunk"
	BranchTopologyWorktree = "task-worktree"
	BranchTopologyDetached = "detached"
	BranchTopologyCustom   = "custom-branch"
	BranchTopologyUnknown  = "unknown"
)

var (
	ghTokenRe    = regexp.MustCompile(`(gh[pousr]_[a-zA-Z0-9]{20,}|github_pat_[a-zA-Z0-9_]{20,})`)
	cloudKeyRe   = regexp.MustCompile(`(sk-[a-zA-Z0-9_\-]{20,}|AIza[0-9A-Za-z_\-]{25,}|A(?:KIA|SIA)[0-9A-Z]{16}|xox[baprs]-[0-9A-Za-z\-]{16,})`)
	bearerAuthRe = regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_\-\.=]+`)
	uriCredsRe   = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)([^/\s:@]+):([^/\s@]+)@`)
	pemBlockRe   = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	pemHeaderRe  = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	posixHomeRe  = regexp.MustCompile(`/(?:Users|home|var/home)/[a-zA-Z0-9._\-]+/`)
)

// classifyBranchTopology maps a git branch string onto a non-identifying topological class
// so private corporate, customer, or feature branch names never leak into diagnostic reports.
func ClassifyBranchTopology(branch string, isWorktree bool) string {
	trimmed := strings.TrimSpace(branch)
	if trimmed == "" || strings.EqualFold(trimmed, BranchTopologyUnknown) {
		if isWorktree {
			return BranchTopologyWorktree
		}
		return BranchTopologyUnknown
	}
	if trimmed == "HEAD" || strings.Contains(strings.ToLower(trimmed), "detached") {
		return BranchTopologyDetached
	}
	lower := strings.ToLower(trimmed)
	if lower == "main" || lower == "master" || lower == "trunk" {
		return BranchTopologyTrunk
	}
	if isWorktree || strings.HasPrefix(trimmed, "feature/INT-") || strings.HasPrefix(trimmed, "feature/SUB-") {
		return BranchTopologyWorktree
	}
	return BranchTopologyCustom
}

// sanitizeText scrubs personal home paths and credentials when no repository root is supplied.
func SanitizeText(text string) string {
	return SanitizeDiagnosticText(text, "")
}

// sanitizeDiagnosticText scrubs repository root paths, user home directories, multi-scheme URI
// credentials, GitHub PATs, cloud API keys, bearer tokens, and PEM private keys.
func SanitizeDiagnosticText(text, repoRoot string) string {
	if text == "" {
		return ""
	}
	out := redactRepoAndHomePaths(text, repoRoot)
	out = pemBlockRe.ReplaceAllString(out, "[REDACTED_PRIVATE_KEY]")
	out = pemHeaderRe.ReplaceAllString(out, "[REDACTED_PRIVATE_KEY]")
	out = ghTokenRe.ReplaceAllString(out, "[REDACTED_GH_TOKEN]")
	out = cloudKeyRe.ReplaceAllString(out, "[REDACTED_API_KEY]")
	out = bearerAuthRe.ReplaceAllString(out, "Bearer [REDACTED]")
	out = uriCredsRe.ReplaceAllString(out, "${1}user:[REDACTED]@")
	return out
}

func redactRepoAndHomePaths(text, repoRoot string) string {
	out := text
	cleanRoot := strings.TrimSpace(repoRoot)
	if cleanRoot != "" && cleanRoot != "." && cleanRoot != "/" {
		absRoot := cleanRoot
		if resolved, err := filepath.Abs(cleanRoot); err == nil {
			absRoot = resolved
		}
		slashRoot := strings.TrimRight(filepath.ToSlash(absRoot), "/")
		if slashRoot != "" {
			out = strings.ReplaceAll(out, slashRoot, "<repo-root>")
			if absRoot != slashRoot {
				out = strings.ReplaceAll(out, strings.TrimRight(absRoot, `\`), "<repo-root>")
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		out = strings.ReplaceAll(out, home, "~")
		slashHome := filepath.ToSlash(home)
		if slashHome != home {
			out = strings.ReplaceAll(out, slashHome, "~")
		}
	}
	return posixHomeRe.ReplaceAllString(out, "~/")
}
