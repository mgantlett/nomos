// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	vscodeDirName      = ".vscode"
	vscodeSettingsFile = "settings.json"
	globWorktrees      = "**/worktrees/**"
	globTmp            = "**/.tmp/**"
	globBuild          = "**/build/**"
	globSvelteKit      = "**/.svelte-kit/**"
	globDist           = "**/dist/**"
	globNodeModules    = "**/node_modules/**"
)

var defaultIDEExcludeGlobs = []string{
	globWorktrees,
	globTmp,
	globBuild,
	globSvelteKit,
	globDist,
	globNodeModules,
}

var defaultGoplsFilters = []string{
	"-worktrees",
	"-.tmp",
	"-node_modules",
	"-build",
	"-dist",
}

// EnsureIDEBoundarySettings non-destructively ensures `<repoRoot>/.vscode/settings.json`
// excludes transient Nomos worktrees and build caches from IDE file watchers, search, and gopls.
// This prevents TypeScript, Svelte, and Go language servers from re-indexing cloned worktrees.
func EnsureIDEBoundarySettings(repoRoot string) error {
	if repoRoot == "" {
		return nil
	}
	settingsPath := filepath.Join(repoRoot, vscodeDirName, vscodeSettingsFile)
	rootMap, existingRaw, skip, err := loadExistingSettingsMap(settingsPath)
	if err != nil || skip {
		return err
	}

	changed := mergeBoolExcludeMap(rootMap, "files.exclude", []string{globWorktrees})
	if mergeBoolExcludeMap(rootMap, "files.watcherExclude", defaultIDEExcludeGlobs) {
		changed = true
	}
	if mergeBoolExcludeMap(rootMap, "search.exclude", defaultIDEExcludeGlobs) {
		changed = true
	}
	if mergeGoplsDirectoryFilters(rootMap) {
		changed = true
	}

	if !changed && len(existingRaw) > 0 {
		return nil
	}
	return writeSettingsMapAtomically(settingsPath, rootMap, existingRaw)
}

// loadExistingSettingsMap reads an existing `.vscode/settings.json` object, returning skip=true on JSONC comments.
func loadExistingSettingsMap(settingsPath string) (map[string]any, []byte, bool, error) {
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil, false, nil
		}
		return nil, nil, false, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return make(map[string]any), raw, false, nil
	}
	var parsed map[string]any
	if uErr := json.Unmarshal(raw, &parsed); uErr != nil {
		// Respect hand-edited JSONC files containing comments rather than clobbering them.
		return nil, raw, true, nil
	}
	if parsed == nil {
		parsed = make(map[string]any)
	}
	return parsed, raw, false, nil
}

// mergeBoolExcludeMap ensures every required glob pattern is mapped to `true` in the named exclude map.
func mergeBoolExcludeMap(root map[string]any, sectionKey string, globs []string) bool {
	sub, ok := root[sectionKey].(map[string]any)
	changed := false
	if !ok || sub == nil {
		sub = make(map[string]any)
		root[sectionKey] = sub
		changed = true
	}
	for _, g := range globs {
		if cur, exists := sub[g]; !exists || cur != true {
			sub[g] = true
			changed = true
		}
	}
	return changed
}

// mergeGoplsDirectoryFilters ensures gopls ignores transient worktree and build directories.
func mergeGoplsDirectoryFilters(root map[string]any) bool {
	goplsMap, ok := root["gopls"].(map[string]any)
	changed := false
	if !ok || goplsMap == nil {
		goplsMap = make(map[string]any)
		root["gopls"] = goplsMap
		changed = true
	}

	filters, prunedLegacy := extractStringFilters(goplsMap["build.directoryFilters"])
	if prunedLegacy {
		changed = true
	}
	for _, req := range defaultGoplsFilters {
		if !slices.Contains(filters, req) {
			filters = append(filters, req)
			changed = true
		}
	}
	if changed {
		out := make([]any, len(filters))
		for i, f := range filters {
			out[i] = f
		}
		goplsMap["build.directoryFilters"] = out
	}
	return changed
}

// extractStringFilters normalizes existing gopls directory filters and strips invalid `**` globs.
func extractStringFilters(raw any) ([]string, bool) {
	items, ok := raw.([]any)
	if !ok {
		return nil, raw != nil
	}
	var out []string
	pruned := false
	for _, item := range items {
		s, isStr := item.(string)
		if !isStr {
			continue
		}
		if strings.HasPrefix(s, "-**/") {
			pruned = true
			continue
		}
		out = append(out, s)
	}
	return out, pruned
}

// writeSettingsMapAtomically marshals and writes `.vscode/settings.json` only when bytes differ.
func writeSettingsMapAtomically(settingsPath string, rootMap map[string]any, existingRaw []byte) error {
	encoded, err := json.MarshalIndent(rootMap, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if bytes.Equal(encoded, existingRaw) {
		return nil
	}
	if mErr := os.MkdirAll(filepath.Dir(settingsPath), 0755); mErr != nil {
		return mErr
	}
	return os.WriteFile(settingsPath, encoded, 0644)
}
