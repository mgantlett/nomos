package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestValidateWorktreeBoundary(t *testing.T) {
	root := "/test/nomos"

	tests := []struct {
		name          string
		currentBranch string
		targetFile    string
		wantErr       bool
	}{
		{
			name:          "protected branch direct root mutation forbidden",
			currentBranch: constants.ValMain,
			targetFile:    filepath.Join(root, "substrate/main.go"),
			wantErr:       true,
		},
		{
			name:          "protected branch mutation inside worktree permitted",
			currentBranch: constants.ValMain,
			targetFile:    filepath.Join(root, "worktrees/nomos-SUB-100/main.go"),
			wantErr:       false,
		},
		{
			name:          "feature branch direct root mutation permitted during merge/checkout",
			currentBranch: "feature/SUB-100",
			targetFile:    filepath.Join(root, "substrate/main.go"),
			wantErr:       false,
		},
		{
			name:          "file outside repo root permitted",
			currentBranch: constants.ValMain,
			targetFile:    "/tmp/some/other/file.go",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreeBoundary(root, tt.currentBranch, tt.targetFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorktreeBoundary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// Task Boundary (Bug 0.5) tests
	taskTests := []struct {
		name       string
		targetFile string
		activeTask string
		wantErr    bool
	}{
		{
			name:       "worktree matching active task succeeds",
			targetFile: filepath.Join(root, "worktrees/nomos-SUB-1230/main.go"),
			activeTask: "SUB-1230",
			wantErr:    false,
		},
		{
			name:       "worktree mismatching active task fails closed (Bug 0.5)",
			targetFile: filepath.Join(root, "worktrees/nomos-SUB-1174/main.go"),
			activeTask: "SUB-1222",
			wantErr:    true,
		},
	}
	for _, tt := range taskTests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreeTaskBoundary(root, constants.ValMain, tt.targetFile, tt.activeTask)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorktreeTaskBoundary() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUncommittedBoundaries(t *testing.T) {
	tmpDir := t.TempDir()
	realFile := filepath.Join(tmpDir, "walkthrough.md")
	_ = os.WriteFile(realFile, []byte("data"), 0644)
	_ = os.MkdirAll(filepath.Join(tmpDir, "substrate"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, "substrate", "main.go"), []byte("package main"), 0644)

	tests := []struct {
		name          string
		repoRoot      string
		currentBranch string
		files         []string
		wantErr       bool
	}{
		{
			name:          "protected branch with root files fails closed",
			repoRoot:      tmpDir,
			currentBranch: constants.ValMain,
			files:         []string{"substrate/main.go"},
			wantErr:       true,
		},
		{
			name:          "feature branch with worktree files succeeds",
			repoRoot:      tmpDir,
			currentBranch: "feature/SUB-1",
			files:         []string{"worktrees/nomos-SUB-1/pkg.go"},
			wantErr:       false,
		},
		{
			name:          "walkthrough directly in root fails closed",
			repoRoot:      tmpDir,
			currentBranch: "feature/SUB-1",
			files:         []string{"walkthrough.md"},
			wantErr:       true,
		},
		{
			name:          "prefixed walkthrough directly in root fails closed",
			repoRoot:      tmpDir,
			currentBranch: "feature/SUB-1",
			files:         []string{"walkthrough-SUB-999.md"},
			wantErr:       true,
		},
		{
			name:          "walkthrough inside .tmp succeeds",
			repoRoot:      tmpDir,
			currentBranch: "feature/SUB-1",
			files:         []string{".tmp/walkthrough.md"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUncommittedBoundaries(tt.repoRoot, tt.currentBranch, tt.files)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUncommittedBoundaries() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
