package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestResolveCanonicalGoBinDir_GOBINOverride(t *testing.T) {
	customGobin := filepath.Join(t.TempDir(), "custom-gobin")
	t.Setenv("GOBIN", customGobin)

	got := resolveCanonicalGoBinDir("/fake/home")
	if got != customGobin {
		t.Fatalf("expected GOBIN %q, got %q", customGobin, got)
	}
}

func TestPurgeLegacyDuplicateBinary(t *testing.T) {
	fakeHome := t.TempDir()
	legacyDir := filepath.Join(fakeHome, constants.DirNomosHome, "bin")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacyDir: %v", err)
	}
	for _, name := range []string{"nomos", "nomos-plugin-alpha"} {
		p := filepath.Join(legacyDir, name)
		if err := os.WriteFile(p, []byte("stale"), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	canonical := filepath.Join(fakeHome, "go", "bin", "nomos")
	purgeLegacyDuplicateBinary(fakeHome, canonical)

	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Fatalf("expected empty legacy directory %s to be purged, err=%v", legacyDir, err)
	}
}

func TestDiscoverRecompileTargets(t *testing.T) {
	subDir := t.TempDir()
	binDir := t.TempDir()
	for _, cmdName := range []string{"nomos", "nomos-plugin-demo", "daemon-svc", "dev-helper"} {
		dir := filepath.Join(subDir, "cmd", cmdName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(binDir, "daemon-svc"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := discoverRecompileTargets(subDir, binDir)
	want := []string{"daemon-svc", "nomos", "nomos-plugin-demo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected targets %v, got %v", want, got)
	}
}

func TestRecompileSubstrateBinary_NotNomosRepo(t *testing.T) {
	tempDir := t.TempDir()
	recompileSubstrateBinary(tempDir)
}
