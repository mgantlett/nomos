// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// ProtectedTrunkBranches defines protected branches that cannot be modified directly in the root repository.
var ProtectedTrunkBranches = []string{constants.ValMain, constants.ValMaster, "intent", "substrate"}

// regexWorktreeTaskKey enforces the CUE #WorktreeSession task_key constraint: ^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$.
var regexWorktreeTaskKey = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)

// WorktreeBoundaryConfig binds the CUE #WorktreeBoundaryConfig schema contract.
type WorktreeBoundaryConfig struct {
	ProtectedBranches []string `json:"protected_branches"`
	WorktreeBaseDir   string   `json:"worktree_base_dir"`
	ExemptPatterns    []string `json:"exempt_patterns,omitempty"`
}

// Validate ensures WorktreeBoundaryConfig satisfies all CUE schema constraints.
func (c *WorktreeBoundaryConfig) Validate() error {
	if c == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "worktree boundary config cannot be nil")
	}
	if strings.TrimSpace(c.WorktreeBaseDir) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "worktree_base_dir cannot be empty")
	}
	return nil
}

// WorktreeSession binds the CUE #WorktreeSession schema contract.
type WorktreeSession struct {
	TaskKey      string `json:"task_key"`
	BranchName   string `json:"branch_name"`
	WorktreePath string `json:"worktree_path"`
	CreatedAt    string `json:"created_at"`
}

// Validate ensures WorktreeSession satisfies all CUE schema constraints.
func (s *WorktreeSession) Validate() error {
	if s == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "worktree session cannot be nil")
	}
	if !regexWorktreeTaskKey.MatchString(s.TaskKey) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task_key %q must match ^(INT|SUB)-[0-9]+", s.TaskKey)
	}
	if strings.TrimSpace(s.BranchName) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "branch_name cannot be empty")
	}
	if strings.TrimSpace(s.WorktreePath) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "worktree_path cannot be empty")
	}
	return nil
}

// Manager orchestrates git worktrees for tasks adhering to Law 3.
type Manager struct {
	RepoRoot string `json:"repo_root"`
}

// NewManager creates a worktree manager for the target repository root.
// If repoRoot is empty, it auto-discovers the root repository.
func NewManager(repoRoot string) (*Manager, error) {
	root, err := resolveRootPath(repoRoot)
	if err != nil {
		return nil, err
	}
	clean := filepath.Clean(root)
	if !isGitDir(clean) {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "invalid repository root %s: missing .git", clean)
	}
	return &Manager{RepoRoot: clean}, nil
}

func resolveRootPath(repoRoot string) (string, error) {
	if repoRoot == "" {
		return FindRepoRoot("")
	}
	clean := filepath.Clean(repoRoot)
	if isGitDir(clean) {
		return clean, nil
	}
	discovered, err := FindRepoRoot(repoRoot)
	if err == nil {
		return discovered, nil
	}
	return repoRoot, nil
}

// WorktreesDir returns the canonical worktrees directory path (<repoRoot>/worktrees).
func (m *Manager) WorktreesDir() string {
	return filepath.Join(m.RepoRoot, constants.DirWorktrees)
}

// extractBaseKey splits key at hyphen after prefix and number (e.g. INT-219-SLUG -> INT-219).
func extractBaseKey(key string) string {
	parts := strings.Split(key, "-")
	if len(parts) >= 2 {
		return fmt.Sprintf("%s-%s", parts[0], parts[1])
	}
	return key
}

// resolveWorktreePrefix resolves the prefix for worktree folders (defaults to repo basename or "nomos").
func resolveWorktreePrefix(repoRoot string) string {
	if seat := os.Getenv("NOMOS_SEAT"); seat != "" {
		return strings.ToLower(strings.TrimSpace(seat))
	}
	if repoRoot != "" {
		return filepath.Base(repoRoot)
	}
	return constants.AppName
}

// matchWorktreeDir evaluates whether an existing directory matches a task key or base key.
func matchWorktreeDir(name, prefix, repoBase, cleanID, baseKey string) bool {
	if strings.HasSuffix(name, "-"+baseKey) || strings.Contains(name, "-"+baseKey+"-") {
		return true
	}
	prefixes := []string{prefix, repoBase, constants.AppName}
	for _, p := range prefixes {
		if name == p+"-"+cleanID || name == p+"-"+baseKey || strings.HasPrefix(name, p+"-"+baseKey+"-") {
			return true
		}
	}
	return false
}

// findExistingWorktree searches the worktrees directory for an existing directory matching taskID.
func (m *Manager) findExistingWorktree(cleanID, prefix, repoBase string) string {
	baseKey := extractBaseKey(cleanID)
	entries, err := os.ReadDir(m.WorktreesDir())
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() && matchWorktreeDir(entry.Name(), prefix, repoBase, cleanID, baseKey) {
			return filepath.Join(m.WorktreesDir(), entry.Name())
		}
	}
	return ""
}

// WorktreePath returns the designated worktree path for a given task ID.
func (m *Manager) WorktreePath(taskID string) string {
	cleanID := strings.ToUpper(strings.TrimSpace(taskID))
	prefix := resolveWorktreePrefix(m.RepoRoot)
	exact := filepath.Join(m.WorktreesDir(), fmt.Sprintf("%s-%s", prefix, cleanID))
	if _, err := os.Stat(exact); err == nil {
		return exact
	}

	repoBase := filepath.Base(m.RepoRoot)
	legacyExact := filepath.Join(m.WorktreesDir(), fmt.Sprintf("%s-%s", repoBase, cleanID))
	if _, err := os.Stat(legacyExact); err == nil {
		return legacyExact
	}

	if found := m.findExistingWorktree(cleanID, prefix, repoBase); found != "" {
		return found
	}
	return exact
}

// BranchName returns the feature branch name for a task.
// If an isolated worktree already exists, it reads the active branch from git.
func (m *Manager) BranchName(taskID string) string {
	cleanID := strings.ToUpper(strings.TrimSpace(taskID))
	wtPath := m.WorktreePath(taskID)
	if _, err := os.Stat(wtPath); err == nil {
		cmd := SafeGitCommand(wtPath, constants.CmdRevParse, "--abbrev-ref", constants.ValHead)
		if out, err := cmd.Output(); err == nil {
			branch := strings.TrimSpace(string(out))
			if branch != "" && branch != constants.ValHead {
				return branch
			}
		}
	}
	return fmt.Sprintf("feature/%s", cleanID)
}

// CreateWorktree provisions an isolated git worktree for a task.
func (m *Manager) CreateWorktree(taskID string) (string, error) {
	wtPath := m.WorktreePath(taskID)
	branch := m.BranchName(taskID)

	if _, err := os.Stat(filepath.Join(wtPath, constants.DirGit)); err == nil {
		return wtPath, nil
	}
	if _, err := os.Stat(wtPath); err == nil {
		if remErr := os.RemoveAll(wtPath); remErr != nil {
			// best-effort orphan directory cleanup before git worktree add
		}
	}

	if err := os.MkdirAll(m.WorktreesDir(), 0755); err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed creating worktrees directory: %v", err)
	}

	// Try git worktree add -b <branch> <path> HEAD
	cmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "add", "-b", branch, wtPath, constants.ValHead)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Self-heal stale administrative worktree locks (e.g. after manual 'rm -rf worktrees/...') before re-attaching
		if pErr := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "prune").Run(); pErr != nil {
			// non-fatal prune
		}
		// If branch exists, attach without -b
		fallbackCmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "add", wtPath, branch)
		if fbOut, fbErr := fallbackCmd.CombinedOutput(); fbErr != nil {
			return "", types.NewNomosError(types.ErrCodeBoundary, "failed adding git worktree: %s / %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(fbOut)))
		}
	}

	if err := m.scaffoldGoWork(wtPath); err != nil {
		return wtPath, err
	}
	return wtPath, nil
}

// discoverWorkspaceModules identifies all Go modules located at the root or immediate child directories.
func discoverWorkspaceModules(wtPath string) []string {
	var uses []string
	if fi, err := os.Stat(filepath.Join(wtPath, "go.mod")); err == nil && !fi.IsDir() {
		uses = append(uses, "\t.")
	}
	entries, err := os.ReadDir(wtPath)
	if err != nil {
		return uses
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == constants.DirWorktrees || name == "vendor" {
			continue
		}
		subMod := filepath.Join(wtPath, name, "go.mod")
		if fi, err := os.Stat(subMod); err == nil && !fi.IsDir() {
			uses = append(uses, "\t./"+name)
		}
	}
	return uses
}

func siblingNomosReplace(repoRoot, wtPath string) string {
	if filepath.Base(repoRoot) == constants.AppName {
		return ""
	}
	siblingNomos := filepath.Join(repoRoot, "..", constants.AppName, constants.DirSubstrate)
	fi, err := os.Stat(siblingNomos)
	if err != nil || !fi.IsDir() {
		return ""
	}
	rel, errRel := filepath.Rel(wtPath, siblingNomos)
	if errRel != nil {
		return ""
	}
	return fmt.Sprintf("\nreplace github.com/mgantlett/nomos/substrate => %s\n", rel)
}

// scaffoldGoWork synthesizes a transient go.work file inside the hermetic worktree if Go modules exist.
func (m *Manager) scaffoldGoWork(wtPath string) error {
	uses := discoverWorkspaceModules(wtPath)
	if len(uses) == 0 {
		return nil
	}

	payload := fmt.Sprintf("go 1.26.5\n\nuse (\n%s\n)\n", strings.Join(uses, "\n"))
	payload += siblingNomosReplace(m.RepoRoot, wtPath)
	return os.WriteFile(filepath.Join(wtPath, "go.work"), []byte(payload), 0644)
}

// RemoveWorktree purges a completed task worktree from disk and prunes git metadata.
func (m *Manager) RemoveWorktree(taskID string) error {
	wtPath := m.WorktreePath(taskID)
	branch := m.BranchName(taskID)
	exists := true
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		exists = false
	}

	if exists {
		cmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "remove", "--force", wtPath)
		out, err := cmd.CombinedOutput()
		// Unconditionally remove wtPath so gitignored directories (.svelte-kit, .tmp, go.work) never remain.
		removeErr := os.RemoveAll(wtPath)
		pruneCmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "prune", "--expire", "now")
		if errPrune := pruneCmd.Run(); errPrune != nil {
			return types.NewNomosError(types.ErrCodeBoundary, "failed pruning worktree: %v", errPrune)
		}
		if _, statErr := os.Stat(wtPath); statErr == nil {
			return types.NewNomosError(types.ErrCodeBoundary, "failed removing worktree: %s (%v), manual remove err: %v", strings.TrimSpace(string(out)), err, removeErr)
		}
	} else {
		pruneCmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "prune", "--expire", "now")
		if errPrune := pruneCmd.Run(); errPrune != nil {
			return types.NewNomosError(types.ErrCodeBoundary, "failed pruning dangling worktrees: %v", errPrune)
		}
	}
	if !IsProtectedBranch(branch) {
		if bErr := SafeGitCommand(m.RepoRoot, "branch", "-D", branch).Run(); bErr != nil {
			// branch may already be deleted
		}
	}

	// Inoculate against Git's worktree removal bare-flip bug
	if errBare := SafeGitCommand(m.RepoRoot, constants.DirConfig, "core.bare", "false").Run(); errBare != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed restoring core.bare: %v", errBare)
	}
	return nil
}


// ListWorktrees inspects the repository and returns active worktrees as WorktreeSessions.
func (m *Manager) ListWorktrees() ([]WorktreeSession, error) {
	cmd := SafeGitCommand(m.RepoRoot, constants.ValWorktree, "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "failed listing worktrees: %v", err)
	}

	var sessions []WorktreeSession
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	var currentPath, currentBranch string

	flush := func() {
		if currentPath != "" && currentPath != m.RepoRoot {
			base := filepath.Base(currentPath)
			taskKey := extractTaskKeyFromPath(base)
			if taskKey != "" {
				sessions = append(sessions, WorktreeSession{
					TaskKey:      taskKey,
					BranchName:   currentBranch,
					WorktreePath: currentPath,
					CreatedAt:    time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
		currentPath = ""
		currentBranch = ""
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			currentBranch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	flush()

	return sessions, nil
}

func extractTaskKeyFromPath(pathBase string) string {
	parts := strings.Split(pathBase, "-")
	for i := 0; i < len(parts); i++ {
		if parts[i] == "INT" || parts[i] == "SUB" {
			if i+1 < len(parts) {
				key := fmt.Sprintf("%s-%s", parts[i], parts[i+1])
				if regexWorktreeTaskKey.MatchString(key) {
					return key
				}
			}
		}
	}
	return ""
}

// IsProtectedBranch evaluates whether a branch name represents a protected trunk.
func IsProtectedBranch(branch string) bool {
	norm := strings.ToLower(strings.TrimSpace(branch))
	for _, p := range ProtectedTrunkBranches {
		if norm == p {
			return true
		}
	}
	return false
}
