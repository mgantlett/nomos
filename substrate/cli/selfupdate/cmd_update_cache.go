// Package selfupdate implements bounded 24-hour release version caching and zero-latency update notices.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

const (
	updateCheckCacheTTL    = 24 * time.Hour
	handshakeUpdateTimeout = 750 * time.Millisecond
)

func resolveUpdateCheckCachePath() string {
	return filepath.Join(config.GetNomosHome(), constants.DirCache, constants.FileUpdateCheckCache)
}

// IsUpdateCheckDisabled reports whether background release checks are disabled by environment policy.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func IsUpdateCheckDisabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(constants.EnvNomosNoUpdateCheck)))
	if val == "1" || val == "true" || val == "yes" {
		return true
	}
	ci := strings.ToLower(strings.TrimSpace(os.Getenv("CI")))
	return ci == "1" || ci == "true"
}

// LoadUpdateCheckCache reads the persisted 24-hour update check state without mutating disk.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func LoadUpdateCheckCache() (*UpdateCheckCache, error) {
	data, err := os.ReadFile(resolveUpdateCheckCachePath())
	if err != nil {
		return nil, err
	}
	var cached UpdateCheckCache
	if uErr := json.Unmarshal(data, &cached); uErr != nil {
		return nil, uErr
	}
	return &cached, nil
}

// SaveUpdateCheckCache atomically writes the 24-hour update check state under NOMOS_HOME/cache.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func SaveUpdateCheckCache(cache UpdateCheckCache) error {
	cachePath := resolveUpdateCheckCachePath()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmpPath := cachePath + ".tmp"
	if wErr := os.WriteFile(tmpPath, data, 0644); wErr != nil {
		return wErr
	}
	return os.Rename(tmpPath, cachePath)
}

// RecordCompletedUpdateCache updates the local 24-hour cache after a verified binary update.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func RecordCompletedUpdateCache(version string, now time.Time) {
	cleanVer := strings.TrimPrefix(strings.TrimSpace(version), "v")
	if sErr := SaveUpdateCheckCache(UpdateCheckCache{
		CheckedAt:     now.UTC().Format(time.RFC3339),
		LatestVersion: cleanVer,
	}); sErr != nil {
		// non-fatal in read-only environments
	}
}

// ReadCachedUpdateNotice evaluates existing cached release metadata with zero filesystem writes or network egress.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func ReadCachedUpdateNotice(currentVersion, binaryPath string) (string, string) {
	if IsUpdateCheckDisabled() {
		return "", ""
	}
	cached, err := LoadUpdateCheckCache()
	if err != nil || cached == nil {
		return "", ""
	}
	latest := strings.TrimPrefix(strings.TrimSpace(cached.LatestVersion), "v")
	return latest, noticeIfNewer(currentVersion, latest, binaryPath)
}

// RefreshAndFormatUpdateNotice refreshes stale (>24h) release metadata once per day and returns a formatted notice when behind.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func RefreshAndFormatUpdateNotice(currentVersion, binaryPath string, now time.Time) string {
	if IsUpdateCheckDisabled() {
		return ""
	}
	cached, err := LoadUpdateCheckCache()
	if err == nil && cached != nil && isCacheFresh(cached.CheckedAt, now) {
		return noticeIfNewer(currentVersion, cached.LatestVersion, binaryPath)
	}
	if shouldSkipNetworkRefresh() {
		return ""
	}
	latest := fetchAndPersistLatestVersion(cached, now)
	return noticeIfNewer(currentVersion, latest, binaryPath)
}

func shouldSkipNetworkRefresh() bool {
	return testing.Testing() && strings.TrimSpace(os.Getenv(constants.EnvNomosUpdateAPIURL)) == ""
}

func fetchAndPersistLatestVersion(cached *UpdateCheckCache, now time.Time) string {
	ctx, cancel := context.WithTimeout(context.Background(), handshakeUpdateTimeout)
	defer cancel()

	latest := ""
	if tag, rErr := ResolveLatestReleaseTag(ctx); rErr == nil {
		latest = strings.TrimPrefix(tag, "v")
	} else if cached != nil {
		latest = strings.TrimPrefix(strings.TrimSpace(cached.LatestVersion), "v")
	}
	if sErr := SaveUpdateCheckCache(UpdateCheckCache{
		CheckedAt:     now.UTC().Format(time.RFC3339),
		LatestVersion: latest,
	}); sErr != nil {
		// non-fatal in read-only containers
	}
	return latest
}

func noticeIfNewer(currentVersion, latestRaw, binaryPath string) string {
	latest := strings.TrimPrefix(strings.TrimSpace(latestRaw), "v")
	if latest == "" || CompareSemver(currentVersion, latest) >= 0 {
		return ""
	}
	return formatUpdateNoticeText(currentVersion, latest, binaryPath)
}

func isCacheFresh(checkedAtRaw string, now time.Time) bool {
	ts, err := time.Parse(time.RFC3339, strings.TrimSpace(checkedAtRaw))
	if err != nil {
		return false
	}
	age := now.Sub(ts)
	return age >= 0 && age < updateCheckCacheTTL
}

func formatUpdateNoticeText(currentVersion, latestVersion, binaryPath string) string {
	_, upgradeCmd := DetectInstallMethod(binaryPath)
	cur := strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
	lat := strings.TrimPrefix(strings.TrimSpace(latestVersion), "v")
	return fmt.Sprintf("%s -> %s (run '%s')", cur, lat, upgradeCmd)
}
