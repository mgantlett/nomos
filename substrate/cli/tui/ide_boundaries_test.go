package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureIDEBoundarySettings_FreshAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureIDEBoundarySettings(dir); err != nil {
		t.Fatalf("EnsureIDEBoundarySettings failed: %v", err)
	}

	settingsPath := filepath.Join(dir, ".vscode", "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed reading settings.json: %v", err)
	}

	var parsed map[string]any
	if uErr := json.Unmarshal(raw, &parsed); uErr != nil {
		t.Fatalf("invalid json written: %v", uErr)
	}
	watchers, ok := parsed["files.watcherExclude"].(map[string]any)
	if !ok || watchers["**/worktrees/**"] != true || watchers["**/.svelte-kit/**"] != true {
		t.Fatalf("expected worktrees and svelte-kit watcher exclusions, got: %#v", parsed["files.watcherExclude"])
	}

	past := time.Now().Add(-2 * time.Minute)
	if cErr := os.Chtimes(settingsPath, past, past); cErr != nil {
		t.Fatalf("chtimes failed: %v", cErr)
	}
	if err := EnsureIDEBoundarySettings(dir); err != nil {
		t.Fatalf("second EnsureIDEBoundarySettings failed: %v", err)
	}
	info, sErr := os.Stat(settingsPath)
	if sErr != nil {
		t.Fatalf("stat failed: %v", sErr)
	}
	if !info.ModTime().Equal(past) {
		t.Errorf("expected idempotent no-write on second run, mtime changed from %v to %v", past, info.ModTime())
	}
}

func TestEnsureIDEBoundarySettings_PreservesCustomAndUpgradesLegacyGopls(t *testing.T) {
	dir := t.TempDir()
	vsDir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	seed := `{
  "window.title": "Custom Title",
  "gopls": {
    "build.directoryFilters": ["-**/worktrees", "-vendor"]
  }
}`
	settingsPath := filepath.Join(vsDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(seed), 0644); err != nil {
		t.Fatalf("seed write failed: %v", err)
	}

	if err := EnsureIDEBoundarySettings(dir); err != nil {
		t.Fatalf("EnsureIDEBoundarySettings failed: %v", err)
	}

	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var parsed map[string]any
	if uErr := json.Unmarshal(raw, &parsed); uErr != nil {
		t.Fatalf("unmarshal failed: %v", uErr)
	}
	if parsed["window.title"] != "Custom Title" {
		t.Errorf("expected custom window.title preserved, got %v", parsed["window.title"])
	}
	goplsMap := parsed["gopls"].(map[string]any)
	filters := goplsMap["build.directoryFilters"].([]any)
	foundWorktrees := false
	foundVendor := false
	for _, f := range filters {
		if f == "-**/worktrees" {
			t.Errorf("expected legacy glob -**/worktrees removed, got %v", filters)
		}
		if f == "-worktrees" {
			foundWorktrees = true
		}
		if f == "-vendor" {
			foundVendor = true
		}
	}
	if !foundWorktrees || !foundVendor {
		t.Errorf("expected -worktrees and -vendor in filters, got %v", filters)
	}
}

func TestEnsureIDEBoundarySettings_SkipsJSONCCommentsSafely(t *testing.T) {
	dir := t.TempDir()
	vsDir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	jsonc := "// user comment\n{\n  \"editor.tabSize\": 2\n}\n"
	settingsPath := filepath.Join(vsDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(jsonc), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := EnsureIDEBoundarySettings(dir); err != nil {
		t.Fatalf("expected nil error on JSONC file, got %v", err)
	}
	after, rErr := os.ReadFile(settingsPath)
	if rErr != nil {
		t.Fatalf("read failed: %v", rErr)
	}
	if string(after) != jsonc {
		t.Errorf("expected JSONC file untouched, got:\n%s", string(after))
	}
}
