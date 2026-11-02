package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZeroFilesystemAliasGate(t *testing.T) {
	tempDir := t.TempDir()
	cleanFile := filepath.Join(tempDir, "real.txt")
	if err := os.WriteFile(cleanFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create real file: %v", err)
	}

	gate := &ZeroFilesystemAliasGate{}
	ctx := &VerifyContext{WorktreePath: tempDir}

	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected clean directory to pass, got: %s", res.Message)
	}

	// Create symlink and assert failure
	symlinkFile := filepath.Join(tempDir, "symlink.txt")
	if err := os.Symlink(cleanFile, symlinkFile); err != nil {
		t.Skip("skipping symlink creation test if OS prevents symlinks")
	}

	resFail, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resFail.Passed {
		t.Fatalf("expected directory with symlink to fail closed")
	}
}

func TestPlaneSeparationGate(t *testing.T) {
	gate := &PlaneSeparationGate{}

	// Baseline or empty task ID passes
	res, err := gate.Evaluate(&VerifyContext{IsBaseline: true})
	if err != nil || !res.Passed {
		t.Fatalf("expected baseline context to pass")
	}

	resEmpty, err := gate.Evaluate(&VerifyContext{TaskID: ""})
	if err != nil || !resEmpty.Passed {
		t.Fatalf("expected empty task ID context to pass")
	}
}

func TestPlaneSeparationGate_CustomRoots(t *testing.T) {
	tempDir := t.TempDir()
	nomosCfg := "project: custom-test\ngovernance:\n    quality_gates:\n        intent_source_roots:\n            - \"docs\"\n        substrate_source_roots:\n            - \"src\"\n"
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(nomosCfg), 0644); err != nil {
		t.Fatalf("failed to write nomos.yaml: %v", err)
	}

	gate := &PlaneSeparationGate{}
	ctxClean := &VerifyContext{
		WorktreePath: tempDir,
		RepoRoot:     tempDir,
		TaskID:       "SUB-50",
	}

	// Clean worktree (zero modified files) passes
	resClean, err := gate.Evaluate(ctxClean)
	if err != nil || !resClean.Passed {
		t.Fatalf("expected clean worktree to pass plane separation: %v", err)
	}
}

func TestUncheckedErrorsGate(t *testing.T) {
	tempDir := t.TempDir()
	cleanGo := filepath.Join(tempDir, "clean.go")
	cleanSrc := "package main\n\nfunc main() {\n\tx := 1\n\tif x == 0 {\n\t\treturn\n\t}\n}\n"
	if err := os.WriteFile(cleanGo, []byte(cleanSrc), 0644); err != nil {
		t.Fatalf("failed to create clean go file: %v", err)
	}

	gate := &UncheckedErrorsGate{}
	ctx := &VerifyContext{WorktreePath: tempDir}

	res, err := gate.Evaluate(ctx)
	if err != nil || !res.Passed {
		t.Fatalf("expected clean go file to pass unchecked error gate, got: %v", err)
	}

	// Add file discarding error via blank identifier
	dirtyGo := filepath.Join(tempDir, "dirty.go")
	dirtySrc := "package main\n\nfunc helper() error { return nil }\nfunc bad() {\n\t_ = helper()\n}\n"
	if err := os.WriteFile(dirtyGo, []byte(dirtySrc), 0644); err != nil {
		t.Fatalf("failed to create dirty go file: %v", err)
	}

	resFail, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}
	if resFail.Passed {
		t.Fatalf("expected file with _ = helper() to fail closed")
	}
}

func TestGoToolchainGate(t *testing.T) {
	gate := &GoToolchainGate{}

	// Bypassed when env var is set
	t.Setenv("NOMOS_IN_TOOLCHAIN_GATE", "1")
	res, err := gate.Evaluate(&VerifyContext{})
	if err != nil || !res.Passed {
		t.Fatalf("expected toolchain gate to bypass re-entrant call")
	}

	// Clean tempdir without go.mod returns passed with no module detected
	t.Setenv("NOMOS_IN_TOOLCHAIN_GATE", "")
	tempDir := t.TempDir()
	resNoMod, err := gate.Evaluate(&VerifyContext{WorktreePath: tempDir})
	if err != nil || !resNoMod.Passed {
		t.Fatalf("expected no go.mod directory to pass safely")
	}
}

func TestWalkTargetFiles_BaselineExcludes(t *testing.T) {
	tempDir := t.TempDir()
	includedDir := filepath.Join(tempDir, "pkg", "core")
	excludedDir := filepath.Join(tempDir, "substrate", "membrane")
	if err := os.MkdirAll(includedDir, 0755); err != nil {
		t.Fatalf("failed to create includedDir: %v", err)
	}
	if err := os.MkdirAll(excludedDir, 0755); err != nil {
		t.Fatalf("failed to create excludedDir: %v", err)
	}

	includedFile := filepath.Join(includedDir, "core.go")
	excludedFile := filepath.Join(excludedDir, "App.svelte")
	if err := os.WriteFile(includedFile, []byte("package core"), 0644); err != nil {
		t.Fatalf("failed to write included file: %v", err)
	}
	if err := os.WriteFile(excludedFile, []byte("<script></script>"), 0644); err != nil {
		t.Fatalf("failed to write excluded file: %v", err)
	}

	nomosConfig := "project: test\ngovernance:\n    quality_gates:\n        exclude:\n            - \"substrate/membrane\"\n"
	if err := os.WriteFile(filepath.Join(tempDir, "nomos.yaml"), []byte(nomosConfig), 0644); err != nil {
		t.Fatalf("failed to write nomos.yaml: %v", err)
	}

	ctx := &VerifyContext{
		RepoRoot:   tempDir,
		IsBaseline: true,
	}

	var visited []string
	err := WalkTargetFiles(ctx, tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			visited = append(visited, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkTargetFiles failed: %v", err)
	}

	for _, v := range visited {
		if filepath.Base(v) == "App.svelte" {
			t.Fatalf("expected excluded file App.svelte to be skipped, but was visited: %s", v)
		}
	}
	hasCore := false
	for _, v := range visited {
		if filepath.Base(v) == "core.go" {
			hasCore = true
		}
	}
	if !hasCore {
		t.Fatalf("expected visited to contain core.go, got: %v", visited)
	}
}

func TestPhaseDisciplineGate_RejectsInvalidPhase(t *testing.T) {
	g := &PhaseDisciplineGate{}
	resValid, err := g.Evaluate(&VerifyContext{Phase: "EDIT"})
	if err != nil || !resValid.Passed {
		t.Fatalf("expected canonical EDIT phase to pass, got %+v (%v)", resValid, err)
	}
	resInvalid, err := g.Evaluate(&VerifyContext{Phase: "MUTATED_BOGUS"})
	if err != nil || resInvalid.Passed {
		t.Fatalf("expected invalid phase MUTATED_BOGUS to fail closed, got %+v (%v)", resInvalid, err)
	}
}

