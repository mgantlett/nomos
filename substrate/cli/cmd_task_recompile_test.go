package cli

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestRunPostSyncScript_ExecutesWhenPresent(t *testing.T) {
	repoDir := t.TempDir()
	scriptsDir := filepath.Join(repoDir, constants.DirAgents, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scriptsDir: %v", err)
	}
	markerPath := filepath.Join(repoDir, "synced.txt")
	scriptBody := "#!/usr/bin/env bash\necho 'post-sync-ok' > \"" + markerPath + "\"\n"
	scriptPath := filepath.Join(scriptsDir, "post-sync.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("write post-sync.sh: %v", err)
	}

	recompileSubstrateBinary(repoDir)

	got, err := os.ReadFile(markerPath)
	if err != nil || !strings.Contains(string(got), "post-sync-ok") {
		t.Fatalf("expected post-sync.sh to execute and write marker, err=%v got=%q", err, string(got))
	}
}

func TestBuildStaticRecompileEnv_EnforcesZeroCGO(t *testing.T) {
	input := []string{"PATH=/usr/bin", "CGO_ENABLED=1", "HOME=/tmp/test"}
	got := buildStaticRecompileEnv(input)

	cgoCount := 0
	for _, entry := range got {
		if entry == "CGO_ENABLED=1" {
			t.Fatalf("expected CGO_ENABLED=1 to be stripped, got %v", got)
		}
		if entry == "CGO_ENABLED=0" {
			cgoCount++
		}
	}
	if cgoCount != 1 {
		t.Fatalf("expected exactly one CGO_ENABLED=0 entry, got %d in %v", cgoCount, got)
	}
}

func TestCompileSingleTarget_ProducesStaticZeroInterpBinary(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOWORK", "off")

	subDir := t.TempDir()
	binDir := t.TempDir()
	cmdDir := filepath.Join(subDir, "cmd", "demo-static")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir cmdDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "go.mod"), []byte("module example.com/demostatic\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	mainSrc := "package main\n\nimport (\n\t\"fmt\"\n\t\"net\"\n)\n\nfunc main() {\n\t_, _ = net.Interfaces()\n\tfmt.Println(\"static-ok\")\n}\n"
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	compileSingleTarget(subDir, binDir, "demo-static")

	binPath := filepath.Join(binDir, "demo-static")
	out, err := exec.Command(binPath).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "static-ok") {
		t.Fatalf("expected compiled binary to run cleanly, err=%v out=%q", err, string(out))
	}

	if runtime.GOOS == "linux" {
		ef, eErr := elf.Open(binPath)
		if eErr != nil {
			t.Fatalf("open ELF binary: %v", eErr)
		}
		defer ef.Close()
		for _, prog := range ef.Progs {
			if prog.Type == elf.PT_INTERP {
				t.Fatalf("expected static zero-CGO binary with zero PT_INTERP dynamic linker segment, found PT_INTERP")
			}
		}
	}
}
