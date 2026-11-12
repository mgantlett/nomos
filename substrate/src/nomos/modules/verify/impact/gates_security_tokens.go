// Package impact implements modern credential token scanning and dependency manifest locks.
package impact

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

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

// MatchSecretTokenLabel executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func MatchSecretTokenLabel(content string) string {
	return capbac.MatchSecretTokenLabel(content)
}

// IsProhibitedEnvFileName executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsProhibitedEnvFileName(base string) bool {
	lower := strings.ToLower(strings.TrimSpace(base))
	if allowedEnvTemplates[lower] {
		return false
	}
	return lower == ".env" || strings.HasPrefix(lower, ".env.")
}

// IsGitIgnoredPath executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func IsGitIgnoredPath(root, targetPath string) bool {
	cmd := workspace.SafeGitCommand(root, "check-ignore", "-q", targetPath)
	return cmd.Run() == nil
}

// ScanUnignoredEnvFiles executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func ScanUnignoredEnvFiles(root string) []string {
	var leaks []string
	seen := make(map[string]bool)

	checkRel := func(rel string) {
		clean := strings.TrimSpace(filepath.ToSlash(rel))
		if clean == "" || seen[clean] {
			return
		}
		if IsProhibitedEnvFileName(filepath.Base(clean)) {
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
				if idx := strings.LastIndex(rel, vscope.SepArrow); idx >= 0 {
					rel = rel[idx+len(vscope.SepArrow):]
				}
				checkRel(rel)
			}
		}
	}
	return leaks
}

// ScanLockedDependencyMutations executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func ScanLockedDependencyMutations(ctx *vscope.VerifyContext, root string, lockDeps bool) []string {
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
