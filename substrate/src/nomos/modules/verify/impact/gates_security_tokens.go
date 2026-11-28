// Package impact implements modern credential token scanning and dependency manifest locks.
package impact

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
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

var packageJSONLockedGraphKeys = []string{
	"bin",
	"bundleDependencies",
	"bundledDependencies",
	"dependencies",
	"devDependencies",
	"optionalDependencies",
	"overrides",
	"peerDependencies",
	"peerDependenciesMeta",
	"pnpm",
	"resolutions",
	"scripts",
	"workspaces",
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
		if !lockedDependencyManifests[base] {
			continue
		}
		if base == "package.json" && (isPackageJSONMetadataOnlyChange(root, rel) || isZeroDepPackagingPackageJSON(root, rel)) {
			continue
		}
		if base == "pyproject.toml" && IsZeroDepPackagingPyproject(root, rel) {
			continue
		}
		violations = append(violations, rel+" (dependency manifest locked via boundary.lock_dependencies)")
	}
	return violations
}

// IsZeroDepPackagingPyproject validates zero-dependency packaging manifests (@intent task_lifecycle_cli_spec.md).
func IsZeroDepPackagingPyproject(root, rel string) bool {
	slashRel := filepath.ToSlash(rel)
	if !strings.HasPrefix(slashRel, "packaging/") {
		return false
	}
	wtBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slashRel)))
	if err != nil {
		return false
	}
	raw := string(wtBytes)
	if strings.Contains(raw, "optional-dependencies") || strings.Contains(raw, "dependency-groups") {
		return false
	}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if strings.HasPrefix(trimmed, "dependencies =") {
			return trimmed == "dependencies = []"
		}
	}
	return false
}

func isZeroDepPackagingPackageJSON(root, rel string) bool {
	slashRel := filepath.ToSlash(rel)
	if !strings.HasPrefix(slashRel, "packaging/") {
		return false
	}
	wtBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slashRel)))
	if err != nil {
		return false
	}
	wtGraph, ok := extractPackageJSONLockedGraph(wtBytes)
	return ok && string(wtGraph) == "{}"
}

func isPackageJSONMetadataOnlyChange(root, rel string) bool {
	slashRel := filepath.ToSlash(rel)
	headBytes, err := workspace.SafeGitCommand(root, "show", "HEAD:"+slashRel).Output()
	if err != nil {
		return false
	}
	wtBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(slashRel)))
	if err != nil {
		return false
	}
	headGraph, okHead := extractPackageJSONLockedGraph(headBytes)
	wtGraph, okWT := extractPackageJSONLockedGraph(wtBytes)
	return okHead && okWT && bytes.Equal(headGraph, wtGraph)
}

func extractPackageJSONLockedGraph(raw []byte) ([]byte, bool) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed == nil {
		return nil, false
	}
	subset := make(map[string]interface{}, len(packageJSONLockedGraphKeys))
	for _, k := range packageJSONLockedGraphKeys {
		if val, exists := parsed[k]; exists {
			subset[k] = val
		}
	}
	encoded, err := json.Marshal(subset)
	if err != nil {
		return nil, false
	}
	return encoded, true
}
