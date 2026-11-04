// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"path/filepath"
	"regexp"
	"strings"
)

var scopeSanitizeRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func derivePrimaryScope(taskKey string, stagedFiles []string) string {
	counts := make(map[string]int)
	var order []string
	for _, f := range stagedFiles {
		sc := scopeFromRelPath(f)
		if sc == "" {
			continue
		}
		if counts[sc] == 0 {
			order = append(order, sc)
		}
		counts[sc]++
	}
	if best := selectMostFrequentScope(order, counts); best != "" {
		return best
	}
	if strings.HasPrefix(strings.ToUpper(taskKey), "INT-") {
		return "intent"
	}
	return ""
}

func selectMostFrequentScope(order []string, counts map[string]int) string {
	best := ""
	bestCount := 0
	for _, sc := range order {
		if counts[sc] > bestCount {
			best = sc
			bestCount = counts[sc]
		}
	}
	return best
}

func scopeFromRelPath(rel string) string {
	slash := strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "./")
	parts := strings.Split(slash, "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	raw := selectScopeSegment(parts)
	return strings.Trim(scopeSanitizeRe.ReplaceAllString(strings.ToLower(raw), "-"), "-_")
}

func selectScopeSegment(parts []string) string {
	switch parts[0] {
	case "README.md":
		return "readme"
	case "intent":
		return selectIntentScope(parts)
	case "substrate":
		return selectSubstrateScope(parts)
	default:
		return selectGenericScope(parts)
	}
}

func selectIntentScope(parts []string) string {
	if len(parts) >= 3 && (parts[1] == "contracts" || parts[1] == "architecture") {
		return parts[2]
	}
	return "intent"
}

func selectSubstrateScope(parts []string) string {
	if len(parts) < 2 {
		return "substrate"
	}
	if parts[1] == "pkg" && len(parts) >= 4 {
		return parts[3]
	}
	if parts[1] == "src" {
		return selectNomosSrcScope(parts)
	}
	return parts[1]
}

func selectNomosSrcScope(parts []string) string {
	if len(parts) >= 5 && parts[2] == "nomos" {
		return parts[4]
	}
	return "substrate"
}

func selectGenericScope(parts []string) string {
	if len(parts) > 1 {
		return parts[0]
	}
	return strings.TrimSuffix(parts[0], filepath.Ext(parts[0]))
}

func allMarkdownFiles(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(f)), ".md") {
			return false
		}
	}
	return true
}
