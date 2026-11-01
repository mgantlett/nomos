package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestNewManager(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr.RepoRoot != repoDir {
		t.Errorf("mgr.RepoRoot = %s; want %s", mgr.RepoRoot, repoDir)
	}

	nonGitDir, err := os.MkdirTemp("/tmp", "nomos-non-git-*")
	if err != nil {
		t.Fatalf("failed creating temp dir in /tmp: %v", err)
	}
	defer os.RemoveAll(nonGitDir)

	_, err = NewManager(nonGitDir)
	if err == nil {
		t.Errorf("expected error initializing Manager on non-git directory")
	}
}


func TestWorktreesDir(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	mgr, _ := NewManager(repoDir)

	expected := filepath.Join(repoDir, constants.DirWorktrees)
	if mgr.WorktreesDir() != expected {
		t.Errorf("mgr.WorktreesDir() = %s; want %s", mgr.WorktreesDir(), expected)
	}
}

func TestWorktreePathAndBranch(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	mgr, _ := NewManager(repoDir)

	taskKey := "SUB-1234"
	wtPath := mgr.WorktreePath(taskKey)
	if wtPath == "" {
		t.Fatalf("mgr.WorktreePath returned empty string")
	}

	branch := mgr.BranchName(taskKey)
	expectedBranch := "feature/SUB-1234"
	if branch != expectedBranch {
		t.Errorf("mgr.BranchName() = %s; want %s", branch, expectedBranch)
	}
}

func TestCreateAndRemoveWorktree(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	mgr, _ := NewManager(repoDir)

	taskKey := "SUB-999"
	wtPath, err := mgr.CreateWorktree(taskKey)
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if fi, err := os.Stat(wtPath); err != nil || !fi.IsDir() {
		t.Fatalf("worktree directory was not created: %v", err)
	}

	// In non-Go repository, go.work must NOT be synthesized
	goWorkPath := filepath.Join(wtPath, "go.work")
	if _, err := os.Stat(goWorkPath); !os.IsNotExist(err) {
		t.Errorf("expected no transient go.work in non-Go worktree: %v", err)
	}

	// Verify ListWorktrees
	sessions, err := mgr.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	found := false
	for _, s := range sessions {
		if s.TaskKey == taskKey {
			found = true
			if err := s.Validate(); err != nil {
				t.Errorf("session validation failed: %v", err)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected task %s in ListWorktrees output", taskKey)
	}

	// Remove worktree
	if err := mgr.RemoveWorktree(taskKey); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree directory to be removed from disk")
	}
}

func TestCreateWorktreeGoWorkScaffolding(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	goModPath := filepath.Join(repoDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/test\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatalf("failed creating go.mod: %v", err)
	}
	cmd := SafeGitCommand(repoDir, "add", "go.mod")
	_ = cmd.Run()
	commitCmd := SafeGitCommand(repoDir, "commit", "-m", "add go.mod")
	commitCmd.Env = append(commitCmd.Env, "NOMOS_COMMIT_IN_PROGRESS=1")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("failed committing go.mod: %v", err)
	}

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	taskKey := "SUB-1000"
	wtPath, err := mgr.CreateWorktree(taskKey)
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer func() {
		_ = mgr.RemoveWorktree(taskKey)
	}()

	goWorkPath := filepath.Join(wtPath, "go.work")
	data, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("expected transient go.work in Go worktree: %v", err)
	}
	if !strings.Contains(string(data), "\t.") {
		t.Errorf("expected go.work to use '.', got:\n%s", string(data))
	}
}

func TestIsProtectedBranch(t *testing.T) {
	tests := []struct {
		branch    string
		protected bool
	}{
		{"main", true},
		{"MAIN", true},
		{"master", true},
		{"intent", true},
		{"substrate", true},
		{"feature/SUB-1234", false},
		{"dev", false},
		{"chore/fix", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			if got := IsProtectedBranch(tt.branch); got != tt.protected {
				t.Errorf("IsProtectedBranch(%s) = %v; want %v", tt.branch, got, tt.protected)
			}
		})
	}
}

func TestWorktreeBoundaryConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *WorktreeBoundaryConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &WorktreeBoundaryConfig{
				ProtectedBranches: []string{"main", "master"},
				WorktreeBaseDir:   "worktrees",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "empty base dir",
			config: &WorktreeBoundaryConfig{
				ProtectedBranches: []string{"main"},
				WorktreeBaseDir:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorktreeSession_Validate(t *testing.T) {
	tests := []struct {
		name    string
		session *WorktreeSession
		wantErr bool
	}{
		{
			name: "valid intent session",
			session: &WorktreeSession{
				TaskKey:      "INT-726",
				BranchName:   "feature/INT-726",
				WorktreePath: "/repo/worktrees/nomos-INT-726",
				CreatedAt:    "2026-10-23T12:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "valid substrate session with slug",
			session: &WorktreeSession{
				TaskKey:      "SUB-1193-workspace",
				BranchName:   "feature/SUB-1193",
				WorktreePath: "/repo/worktrees/nomos-SUB-1193",
				CreatedAt:    "2026-10-23T12:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "invalid task key prefix",
			session: &WorktreeSession{
				TaskKey:      "FEAT-100",
				BranchName:   "feature/FEAT-100",
				WorktreePath: "/repo/worktrees/nomos-FEAT-100",
			},
			wantErr: true,
		},
		{
			name: "missing branch name",
			session: &WorktreeSession{
				TaskKey:      "SUB-100",
				BranchName:   "",
				WorktreePath: "/repo/worktrees/nomos-SUB-100",
			},
			wantErr: true,
		},
		{
			name:    "nil session",
			session: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.session.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveWorktree_SweepsGitignoredDirsAndBranch(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	gitignore := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(".svelte-kit/\n.tmp/\n"), 0644); err != nil {
		t.Fatalf("failed writing .gitignore: %v", err)
	}
	_ = SafeGitCommand(repoDir, "add", ".gitignore").Run()
	commitCmd := SafeGitCommand(repoDir, "commit", "-m", "add gitignore")
	commitCmd.Env = append(commitCmd.Env, "NOMOS_COMMIT_IN_PROGRESS=1")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("failed committing .gitignore: %v", err)
	}

	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	wtPath, err := mgr.CreateWorktree("SUB-1100")
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	ignoredDir := filepath.Join(wtPath, ".svelte-kit", "output")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "chunk.js"), []byte("export default 1;\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := mgr.RemoveWorktree("SUB-1100"); err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected worktree %s (including gitignored subdirectories) to be completely removed", wtPath)
	}
}

func TestPruneOrphanWorktrees_PreservesActiveAndSweepsOrphans(t *testing.T) {
	repoDir := InitHermeticGitRepo(t)
	mgr, err := NewManager(repoDir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	activeWT, err := mgr.CreateWorktree("SUB-1201")
	if err != nil {
		t.Fatalf("CreateWorktree SUB-1201 failed: %v", err)
	}
	orphanDir := filepath.Join(mgr.WorktreesDir(), "ghost-SUB-1202", ".svelte-kit")
	if err := os.MkdirAll(orphanDir, 0755); err != nil {
		t.Fatalf("MkdirAll orphanDir failed: %v", err)
	}
	tmpDir := filepath.Join(repoDir, constants.DirTmp)
	_ = os.MkdirAll(tmpDir, 0755)
	staleWTFile := filepath.Join(tmpDir, "walkthrough-SUB-1202.md")
	_ = os.WriteFile(staleWTFile, []byte("# Stale\n"), 0644)

	activeKeys := map[string]bool{"SUB-1201": true}
	dryRep, err := mgr.PruneOrphanWorktrees(true, activeKeys)
	if err != nil {
		t.Fatalf("PruneOrphanWorktrees dry-run failed: %v", err)
	}
	if len(dryRep.RemovedDirs) != 1 || dryRep.RemovedDirs[0] != "ghost-SUB-1202" {
		t.Fatalf("expected ghost-SUB-1202 in dry-run RemovedDirs, got %+v", dryRep.RemovedDirs)
	}
	if _, err := os.Stat(orphanDir); err != nil {
		t.Fatalf("expected orphanDir to remain after dry-run: %v", err)
	}

	execRep, err := mgr.PruneOrphanWorktrees(false, activeKeys)
	if err != nil {
		t.Fatalf("PruneOrphanWorktrees failed: %v", err)
	}
	if len(execRep.RemovedDirs) != 1 || len(execRep.RemovedTempFiles) != 1 {
		t.Fatalf("unexpected prune report: %+v", execRep)
	}
	if _, err := os.Stat(filepath.Join(mgr.WorktreesDir(), "ghost-SUB-1202")); !os.IsNotExist(err) {
		t.Fatalf("expected ghost-SUB-1202 to be deleted")
	}
	if _, err := os.Stat(activeWT); err != nil {
		t.Fatalf("expected active worktree SUB-1201 to be preserved: %v", err)
	}
}
