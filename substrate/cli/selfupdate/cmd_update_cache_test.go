package selfupdate_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// @verifies TC-UPD-004
func TestUpdateCheckCache_24hTTLAndOptOut(t *testing.T) {
	nomosHome := t.TempDir()
	t.Setenv(constants.EnvNomosHome, nomosHome)
	t.Setenv(constants.EnvNomosNoUpdateCheck, "")
	t.Setenv("CI", "")

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name":"v99.8.0"}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL)

	baseNow := time.Date(2026, 11, 11, 2, 0, 0, 0, time.UTC)
	binPath := filepath.Join(t.TempDir(), "nomos")

	firstNotice := selfupdate.RefreshAndFormatUpdateNotice("4.14.0", binPath, baseNow)
	if !strings.Contains(firstNotice, "4.14.0 -> 99.8.0") || !strings.Contains(firstNotice, "nomos update") {
		t.Fatalf("expected initial update notice, got %q", firstNotice)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 HTTP refresh on cold cache, got %d", hits.Load())
	}

	secondNotice := selfupdate.RefreshAndFormatUpdateNotice("4.14.0", binPath, baseNow.Add(2*time.Hour))
	if secondNotice != firstNotice || hits.Load() != 1 {
		t.Fatalf("expected 24h cached notice without extra HTTP hit, hits=%d notice=%q", hits.Load(), secondNotice)
	}

	brewPath := filepath.Join("/opt", "homebrew", "bin", "nomos")
	latestVer, brewNotice := selfupdate.ReadCachedUpdateNotice("4.14.0", brewPath)
	if latestVer != "99.8.0" || !strings.Contains(brewNotice, "brew upgrade --cask mgantlett/tap/nomos") {
		t.Fatalf("expected Homebrew upgrade remedy in cached notice, got (%q, %q)", latestVer, brewNotice)
	}

	_ = selfupdate.RefreshAndFormatUpdateNotice("4.14.0", binPath, baseNow.Add(25*time.Hour))
	if hits.Load() != 2 {
		t.Fatalf("expected stale (>24h) cache to refresh once, got %d hits", hits.Load())
	}

	t.Setenv(constants.EnvNomosNoUpdateCheck, "1")
	if !selfupdate.IsUpdateCheckDisabled() {
		t.Fatalf("expected NOMOS_NO_UPDATE_CHECK=1 to disable update checks")
	}
	if optOut := selfupdate.RefreshAndFormatUpdateNotice("4.14.0", binPath, baseNow.Add(72*time.Hour)); optOut != "" {
		t.Fatalf("expected empty notice under NOMOS_NO_UPDATE_CHECK=1, got %q", optOut)
	}
}
