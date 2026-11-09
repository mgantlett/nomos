// Package mem implements automated pitfall scope hygiene, ephemeral test-trace expiry,
// and cross-directory habit pattern synthesis for Nomos Learn.
package mem

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ephemeralTestAssertionTTL     = 72 * time.Hour
	crossScopeConsolidationMinDir = 3
)

// PitfallHygieneReport summarizes automated pitfall cleanup and cross-folder synthesis actions.
type PitfallHygieneReport struct {
	DeadScopesTombstoned    int `json:"dead_scopes_tombstoned"`
	ExpiredTestsTombstoned  int `json:"expired_tests_tombstoned"`
	CrossScopeHabitsUpdated int `json:"cross_scope_habits_updated"`
}

// SweepRepoPitfallHygiene tombstones pitfalls pointing at deleted repo directories,
// expires stale one-off unit test failure traces, and synthesizes repo-wide habit
// patterns when a single gate trips across >= 3 directories (@intent: #Memory).
func SweepRepoPitfallHygiene(store *Store, repoRoot, repoID string, now time.Time) PitfallHygieneReport {
	var rep PitfallHygieneReport
	if store == nil || strings.TrimSpace(repoID) == "" {
		return rep
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	all, err := store.ListAll()
	if err != nil {
		return rep
	}
	checkDiskPaths := isAuthoritativeRepoRoot(repoRoot)
	normRepo := strings.ToLower(strings.TrimSpace(repoID))
	surviving := sweepStalePitfalls(store, all, repoRoot, normRepo, checkDiskPaths, now, &rep)
	rep.CrossScopeHabitsUpdated = consolidateCrossScopeGateHabits(store, surviving, repoID, normRepo)
	return rep
}

// isAuthoritativeRepoRoot returns true when repoRoot is a real repository checkout on disk.
func isAuthoritativeRepoRoot(repoRoot string) bool {
	clean := strings.TrimSpace(repoRoot)
	if clean == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(clean, ".git")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(clean, "nomos.yaml")); err == nil {
		return true
	}
	return false
}

// isEphemeralTestAssertionContent detects unit-test runner assertion failures (--- FAIL / TAP not ok)
// that represent fixed single-test bugs rather than generalizable architectural rules.
func isEphemeralTestAssertionContent(content string) bool {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "trap: --- fail:") || strings.Contains(lower, "trap: go compilation/test failed: --- fail:") {
		return true
	}
	return strings.Contains(lower, "trap:") && strings.Contains(lower, "not ok ")
}

// sweepStalePitfalls tombstones dead-directory and expired unit-test pitfalls, returning active survivors.
func sweepStalePitfalls(
	store *Store,
	all []*Memory,
	repoRoot, normRepo string,
	checkDiskPaths bool,
	now time.Time,
	rep *PitfallHygieneReport,
) []*Memory {
	var surviving []*Memory
	for _, m := range all {
		if !isActiveRepoPitfall(m, normRepo) {
			continue
		}
		if checkDiskPaths && isMissingRepoScopeDir(repoRoot, extractScopeFromPitfallTopic(m.Topic)) {
			if _, err := store.Dismiss(m.ID); err == nil {
				rep.DeadScopesTombstoned++
				continue
			}
		}
		if isExpiredEphemeralTestPitfall(m, now) {
			if _, err := store.Dismiss(m.ID); err == nil {
				rep.ExpiredTestsTombstoned++
				continue
			}
		}
		surviving = append(surviving, m)
	}
	return surviving
}

// isMissingRepoScopeDir checks whether a relative directory scope has been removed from repoRoot.
func isMissingRepoScopeDir(repoRoot, scope string) bool {
	clean := strings.TrimSpace(scope)
	if clean == "" || clean == workspaceScopeLabel || clean == "root" || clean == "." {
		return false
	}
	info, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(clean)))
	if err != nil {
		return os.IsNotExist(err)
	}
	return !info.IsDir()
}

// isExpiredEphemeralTestPitfall returns true when a unit-test assertion failure has aged past its 72h retry window.
func isExpiredEphemeralTestPitfall(m *Memory, now time.Time) bool {
	if !isEphemeralTestAssertionContent(m.Content) || m.CreatedAt.IsZero() {
		return false
	}
	return now.Sub(m.CreatedAt) > ephemeralTestAssertionTTL
}

// extractScopeFromPitfallTopic parses the trailing <scope> segment from pitfall:<gate>:<scope>.
func extractScopeFromPitfallTopic(topic string) string {
	trimmed := strings.TrimSpace(topic)
	if !strings.HasPrefix(strings.ToLower(trimmed), "pitfall:") {
		return ""
	}
	rest := trimmed[len("pitfall:"):]
	idx := strings.IndexByte(rest, ':')
	if idx < 0 || idx+1 >= len(rest) {
		return ""
	}
	return strings.TrimSpace(rest[idx+1:])
}

// extractGateSlugFromPitfallTopic parses the <gate> slug from pitfall:<gate>:<scope>.
func extractGateSlugFromPitfallTopic(topic string) string {
	trimmed := strings.ToLower(strings.TrimSpace(topic))
	if !strings.HasPrefix(trimmed, "pitfall:") {
		return ""
	}
	rest := trimmed[len("pitfall:"):]
	idx := strings.IndexByte(rest, ':')
	if idx <= 0 {
		return rest
	}
	return rest[:idx]
}

// consolidateCrossScopeGateHabits synthesizes one repo-wide KindPattern when the same gate trips across >= 3 directories.
func consolidateCrossScopeGateHabits(store *Store, activePitfalls []*Memory, repoID, normRepo string) int {
	byGate := groupPitfallsByGateSlug(activePitfalls)
	updated := 0
	for gateSlug, scopesMap := range byGate {
		if len(scopesMap) < crossScopeConsolidationMinDir {
			continue
		}
		if upsertHabitPattern(store, repoID, normRepo, gateSlug, scopesMap) {
			updated++
		}
	}
	return updated
}

// groupPitfallsByGateSlug buckets non-ephemeral active pitfalls by gate slug and distinct directory scope.
func groupPitfallsByGateSlug(activePitfalls []*Memory) map[string]map[string]*Memory {
	byGate := make(map[string]map[string]*Memory)
	for _, m := range activePitfalls {
		if isEphemeralTestAssertionContent(m.Content) {
			continue
		}
		gateSlug := extractGateSlugFromPitfallTopic(m.Topic)
		scope := extractScopeFromPitfallTopic(m.Topic)
		if gateSlug == "" || scope == "" || scope == workspaceScopeLabel {
			continue
		}
		if byGate[gateSlug] == nil {
			byGate[gateSlug] = make(map[string]*Memory)
		}
		byGate[gateSlug][scope] = m
	}
	return byGate
}

// upsertHabitPattern saves and deduplicates a consolidated repo-wide habit blueprint under pattern:habit:<gate>.
func upsertHabitPattern(store *Store, repoID, normRepo, gateSlug string, scopesMap map[string]*Memory) bool {
	scopes := make([]string, 0, len(scopesMap))
	var newest *Memory
	for sc, rec := range scopesMap {
		scopes = append(scopes, sc)
		if newest == nil || rec.ID > newest.ID {
			newest = rec
		}
	}
	sort.Strings(scopes)
	topic := "pattern:habit:" + gateSlug
	exemplar := extractVerifiedFixTail(newest.Content)
	content := fmt.Sprintf("[Repo-Wide Habit (%s across %d dirs: %s)] %s",
		strings.ReplaceAll(gateSlug, "_", " "),
		len(scopes),
		strings.Join(scopes, ", "),
		exemplar,
	)
	rec := &Memory{
		RepoID:  repoID,
		Kind:    KindPattern,
		Topic:   topic,
		Content: content,
		TaskID:  newest.TaskID,
		Status:  StatusActive,
	}
	if err := store.Save(rec); err != nil {
		return false
	}
	store.supersedeOlderTopicPatterns(normRepo, topic, rec.ID)
	return true
}

// extractVerifiedFixTail extracts the concise Trap -> Verified Fix clause from a distilled pitfall body.
func extractVerifiedFixTail(content string) string {
	idx := strings.Index(content, "] ")
	if idx >= 0 && idx+2 < len(content) {
		return strings.TrimSpace(content[idx+2:])
	}
	return strings.TrimSpace(content)
}
