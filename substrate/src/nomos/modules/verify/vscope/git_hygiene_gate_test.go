package vscope

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitSubprocessHygieneGate_Name(t *testing.T) {
	gate := &GitSubprocessHygieneGate{}
	if gate.Name() != "Git Subprocess Hygiene Invariant (Law 1/3)" {
		t.Errorf("unexpected gate name: %s", gate.Name())
	}
}

func TestGitSubprocessHygieneGate_EvaluateClean(t *testing.T) {
	gate := &GitSubprocessHygieneGate{}
	tempDir := t.TempDir()

	cleanCode := `package main
import "fmt"
func main() { fmt.Println("ok") }
`
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(cleanCode), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	ctx := &VerifyContext{RepoRoot: tempDir, WorktreePath: tempDir}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.Passed {
		t.Errorf("expected clean code to pass, got: %+v", res)
	}
}

func TestGitSubprocessHygieneGate_EvaluateViolation(t *testing.T) {
	gate := &GitSubprocessHygieneGate{}
	tempDir := t.TempDir()

	violatingCode := `package main
import "os/exec"
func run() {
	_ = exec.Command("git", "status")
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "bad.go"), []byte(violatingCode), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	ctx := &VerifyContext{RepoRoot: tempDir, WorktreePath: tempDir}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.Passed {
		t.Errorf("expected violation to fail, got: %+v", res)
	}
}
