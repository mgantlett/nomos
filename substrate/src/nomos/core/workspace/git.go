// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// SanitizedGitEnv returns the current operating system environment stripped of all Git hook variables.
// This prevents child git processes from accidentally inheriting host repository context.
func SanitizedGitEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") ||
			strings.HasPrefix(e, "GIT_OBJECT_DIRECTORY=") ||
			strings.HasPrefix(e, "GIT_ALTERNATE_OBJECT_DIRECTORIES=") ||
			strings.HasPrefix(e, "GIT_COMMON_DIR=") ||
			strings.HasPrefix(e, "GIT_PREFIX=") ||
			strings.HasPrefix(e, "NOMOS_COMMIT_IN_PROGRESS=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// SafeGitCommand creates an *exec.Cmd targeting the specified directory with sanitized environment variables.
func SafeGitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = SanitizedGitEnv()
	return cmd
}

// RunSafeGit executes a git command in the target directory with a scrubbed environment and returns the output.
func RunSafeGit(dir string, args ...string) ([]byte, error) {
	cmd := SafeGitCommand(dir, args...)
	return cmd.CombinedOutput()
}

// FindRepoRoot discovers the root git repository directory containing the primary .git directory,
// correctly resolving through transient worktrees and subdirectories.
func FindRepoRoot(startDir string) (string, error) {
	if root, err := findGitCommonDir(startDir); err == nil {
		return root, nil
	}
	return walkUpForGitDir(startDir)
}

// FindWorkingTreeRoot discovers the top-level directory of the current working tree or worktree,
// strictly respecting transient worktree boundaries.
func FindWorkingTreeRoot(startDir string) (string, error) {
	cmd := SafeGitCommand(startDir, constants.CmdRevParse, "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		top := strings.TrimSpace(string(out))
		if top != "" {
			return filepath.Clean(top), nil
		}
	}
	return FindRepoRoot(startDir)
}

func findGitCommonDir(startDir string) (string, error) {
	cmd := SafeGitCommand(startDir, constants.CmdRevParse, "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	gitCommon := strings.TrimSpace(string(out))
	if !filepath.IsAbs(gitCommon) {
		base := startDir
		if base == "" {
			var err error
			if base, err = os.Getwd(); err != nil {
				return "", err
			}
		}
		gitCommon = filepath.Join(base, gitCommon)
	}
	repoRoot := filepath.Clean(filepath.Dir(gitCommon))
	if _, statErr := os.Stat(filepath.Join(repoRoot, ".git")); statErr == nil {
		return repoRoot, nil
	}
	return "", types.NewNomosError(types.ErrCodeBoundary, "git common dir invalid")
}

func walkUpForGitDir(startDir string) (string, error) {
	curr := startDir
	if curr == "" {
		var err error
		if curr, err = os.Getwd(); err != nil {
			return "", err
		}
	}
	for i := 0; i < 10; i++ {
		if isGitDir(curr) {
			return filepath.Clean(curr), nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return "", types.NewNomosError(types.ErrCodeBoundary, "unable to determine repository root")
}

func isGitDir(dir string) bool {
	fi, statErr := os.Stat(filepath.Join(dir, ".git"))
	return statErr == nil && fi.IsDir()
}

// TestingHelper abstracts testing.TB without requiring testing package imports in production code.
type TestingHelper interface {
	Helper()
	TempDir() string
	Fatalf(format string, args ...any)
}

// InitHermeticGitRepo provisions an isolated temporary git repository with sanitized environment for tests.
func InitHermeticGitRepo(t TestingHelper) string {
	t.Helper()
	dir := t.TempDir()

	runCmd := func(args ...string) {
		cmd := SafeGitCommand(dir, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, out)
		}
	}

	runCmd("init", "-b", constants.ValMain)
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Hermetic Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed creating test file: %v", err)
	}
	runCmd(constants.DirConfig, "user.name", "Hermetic Nomos")
	runCmd(constants.DirConfig, "user.email", "hermetic@nomos.local")
	runCmd("add", ".")
	runCmd("commit", "-m", "initial hermetic commit")

	return dir
}
