package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func TestForbiddenLiteralsGate(t *testing.T) {
	tempDir := t.TempDir()
	gate := &ForbiddenLiteralsGate{}
	if gate.Name() != "Forbidden Literals Invariant" {
		t.Errorf("unexpected name: %s", gate.Name())
	}

	ctx := &VerifyContext{WorktreePath: tempDir}
	res, err := gate.Evaluate(ctx)
	if err != nil || !res.Passed {
		t.Fatalf("expected clean pass on empty dir, got %v, err: %v", res, err)
	}

	// Add forbidden literal in Go and TypeScript files
	badFile := filepath.Join(tempDir, "bad.go")
	_ = os.WriteFile(badFile, []byte("package main\nvar x = \"/home/user/test\"\n"), 0644)
	resBad, errBad := gate.Evaluate(ctx)
	if errBad != nil {
		t.Fatalf("unexpected error: %v", errBad)
	}
	if resBad.Passed {
		t.Errorf("expected failure when forbidden literal present in .go")
	}
	_ = os.Remove(badFile)

	badTS := filepath.Join(tempDir, "bad.ts")
	_ = os.WriteFile(badTS, []byte("export const p = \"/home/user/test\";\n"), 0644)
	resBadTS, errBadTS := gate.Evaluate(ctx)
	if errBadTS != nil || resBadTS.Passed {
		t.Errorf("expected failure when forbidden literal present in .ts")
	}
}

func TestReviewReceiptGate(t *testing.T) {
	gate := &ReviewReceiptGate{}
	ctx := &VerifyContext{Phase: string(types.PhaseEdit)}
	res, err := gate.Evaluate(ctx)
	if err != nil || !res.Passed {
		t.Errorf("expected pass for edit phase: %v", res)
	}

	tempDir := t.TempDir()
	syncCtx := &VerifyContext{
		WorktreePath: tempDir,
		RepoRoot:     tempDir,
		TaskID:       "SUB-1234",
		Phase:        string(types.PhaseSync),
	}
	syncRes, _ := gate.Evaluate(syncCtx)
	if syncRes.Passed {
		t.Errorf("expected failure in sync phase when walkthrough is missing")
	}

	tmpDir := filepath.Join(tempDir, ".tmp")
	_ = os.MkdirAll(tmpDir, 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "walkthrough.md"), []byte("# Walkthrough"), 0644)
	syncResGood, _ := gate.Evaluate(syncCtx)
	if !syncResGood.Passed {
		t.Errorf("expected pass in sync phase when walkthrough exists")
	}
}

func TestPolyglotAntiShimAndDocstrings(t *testing.T) {
	tempDir := t.TempDir()
	ctx := &VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir, AllFiles: true}

	// Polyglot empty block shim in TypeScript
	tsFile := filepath.Join(tempDir, "empty.ts")
	_ = os.WriteFile(tsFile, []byte("export function emptyShim() {}\n"), 0644)
	antiShim := &AntiShimGate{}
	resShim, err := antiShim.Evaluate(ctx)
	if err != nil || resShim.Passed {
		t.Errorf("expected AntiShimGate failure on empty TypeScript function, got: %v, err: %v", resShim, err)
	}
	_ = os.Remove(tsFile)

	// Polyglot undocumented functions in Python
	pyFile := filepath.Join(tempDir, "undoc.py")
	_ = os.WriteFile(pyFile, []byte("def foo():\n    return 1\ndef bar():\n    return 2\n"), 0644)
	docGate := &DocstringDensityGate{}
	resDoc, errDoc := docGate.Evaluate(ctx)
	if errDoc != nil || resDoc.Passed {
		t.Errorf("expected DocstringDensityGate failure on undocumented Python functions, got: %v, err: %v", resDoc, errDoc)
	}
}

func TestZeroAliasTypeAliasAndOrphanChecks(t *testing.T) {
	tempDir := t.TempDir()
	subSrc := filepath.Join(tempDir, "substrate", "src", "nomos", "orphan")
	cliDir := filepath.Join(tempDir, "substrate", "cli")
	_ = os.MkdirAll(subSrc, 0755)
	_ = os.MkdirAll(cliDir, 0755)
	_ = os.WriteFile(filepath.Join(cliDir, "root.go"), []byte("package cli\n"), 0644)
	_ = os.WriteFile(filepath.Join(subSrc, "alias.go"), []byte("package orphan\ntype MyAlias = string\n"), 0644)

	ctx := &VerifyContext{WorktreePath: tempDir, RepoRoot: tempDir, AllFiles: true}
	aliasGate := &ZeroAliasGate{}
	resAlias, err := aliasGate.Evaluate(ctx)
	if err != nil || resAlias.Passed {
		t.Errorf("expected ZeroAliasGate failure on Go type alias, got: %v, err: %v", resAlias, err)
	}

	deadGate := &DeadCodeGate{}
	resDead, errDead := deadGate.Evaluate(ctx)
	if errDead != nil || resDead.Passed {
		t.Errorf("expected DeadCodeGate failure on unimported internal package, got: %v, err: %v", resDead, errDead)
	}
}
