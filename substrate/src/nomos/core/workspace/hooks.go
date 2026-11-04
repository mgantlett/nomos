// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

const PreCommitHookScript = `#!/usr/bin/env bash
export NOMOS_IPC_IN_GIT_HOOK=1
export NOMOS_IN_GIT_HOOK=1
{
    echo "🛡️ Nomos Verification Check (pre-commit)"

    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null)
    CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null)

    # 0. Allow initial repository setup if repository has zero commits (unborn HEAD) or during nomos init
    if [[ "${NOMOS_INIT_IN_PROGRESS:-0}" == "1" ]] || ! git rev-parse --verify HEAD >/dev/null 2>&1; then
        exit 0
    fi

    # 1. Direct commits to protected trunk branches are forbidden by Law 3
    if [[ "$CURRENT_BRANCH" == "intent" || "$CURRENT_BRANCH" == "substrate" || "$CURRENT_BRANCH" == "main" || "$CURRENT_BRANCH" == "master" || "$CURRENT_BRANCH" == "develop" ]]; then
        echo -e "\n\e[31m🚨 Commit Blocked: Direct commits to $CURRENT_BRANCH are forbidden by Nomos Root Mutability Hook (Law 3).\e[0m"
        echo -e "Please create a task branch via \e[33mnomos task start <KEY>\e[0m or \e[33mnomos task chore\e[0m.\n"
        exit 1
    fi

    # 2. Direct commits in root repository workspace are forbidden; all mutations occur in worktrees
    if [[ ! "$REPO_ROOT" =~ /worktrees/ ]]; then
        echo -e "\n\e[31m🚨 Commit Blocked: Commits directly in root repository workspace ($REPO_ROOT) are forbidden by Law 3.\e[0m"
        echo -e "All code mutations and commits must execute inside transient task worktrees."
        echo -e "Please use \e[33mnomos task chore\e[0m to establish a task ID and worktree before mutating code.\n"
        exit 1
    fi

    # 3. Enforce canonical CLI commit entry point (Law 1 & Law 6)
    if [[ "${NOMOS_COMMIT_IN_PROGRESS:-0}" != "1" ]]; then
        echo -e "\n\e[31m🚨 Commit Blocked: Direct 'git commit' is forbidden by Law 1 & Law 6.\e[0m"
        echo -e "Please commit via \e[33mnomos task commit <KEY> -m \"<summary>\"\e[0m or \e[33mnomos task sync <KEY> -F <walkthrough>\e[0m.\n"
        exit 1
    fi

    # 4. Check staged files to ensure no staged paths target root workspace files outside worktree
    STAGED_FILES=$(git diff --cached --name-only 2>/dev/null)
    for file in $STAGED_FILES; do
        if [[ "$file" =~ ^\.\./ || "$file" == /* ]]; then
            echo -e "\n\e[31m🚨 Commit Blocked: Staged file escapes worktree boundary ($file) (Law 3).\e[0m"
            exit 1
        fi
    done

    NOMOS_BIN=""
    if command -v nomos >/dev/null 2>&1; then
        NOMOS_BIN="nomos"
    elif [ -n "${GOBIN:-}" ] && [ -x "${GOBIN}/nomos" ]; then
        NOMOS_BIN="${GOBIN}/nomos"
    elif [ -x "$HOME/go/bin/nomos" ]; then
        NOMOS_BIN="$HOME/go/bin/nomos"
    elif [ -x "$HOME/.local/bin/nomos" ]; then
        NOMOS_BIN="$HOME/.local/bin/nomos"
    fi

    if [ -n "$NOMOS_BIN" ]; then
        "$NOMOS_BIN" verify || {
            echo -e "\n\e[31m❌ Commit Blocked by Nomos DoD constraints\e[0m"
            exit 1
        }
    else
        echo "⚠️ nomos binary not found in PATH or GOBIN. Skipping verification."
    fi
}
`

const CommitMsgHookScript = `#!/usr/bin/env bash
{
    echo "🛡️ Nomos Golden Commit Verification Check"

    # Allow initial repository setup if repository has zero commits (unborn HEAD) or during nomos init
    if [[ "${NOMOS_INIT_IN_PROGRESS:-0}" == "1" ]] || ! git rev-parse --verify HEAD >/dev/null 2>&1; then
        exit 0
    fi

    FIRST_LINE=$(head -n 1 "$1" | tr -d '\r')

    # Allow standard Git merge commits
    if printf '%s\n' "$FIRST_LINE" | grep -qE '^Merge (branch|pull request|remote-tracking branch) '; then
        exit 0
    fi

    # 1. Enforce canonical [<INT|SUB-N>] + Conventional Commit subject (<= 110 chars ceiling, <= 80 target)
    if ! printf '%s\n' "$FIRST_LINE" | grep -qE '^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?\][[:space:]]+(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:[[:space:]]+[^[:space:]]'; then
        echo -e "\n\e[31m❌ Commit Blocked: Subject must match '[<INT|SUB-N>] <feat|fix|docs|chore|refactor|test>(<scope>): <summary>'.\e[0m"
        echo "Please use 'nomos task commit' or 'nomos task sync -F <walkthrough>' to generate compliant Golden Commits."
        exit 1
    fi
    if printf '%s\n' "$FIRST_LINE" | grep -qiE '(\[(task[[:space:]]+)?(int|sub|arc)-[0-9]+[a-z]?(-[a-z0-9-]+)?\]|\((int|sub|arc)-[0-9]+[a-z]?(-[a-z0-9-]+)?\))[[:space:]]*$'; then
        echo -e "\n\e[31m❌ Commit Blocked: Subject contains a duplicate trailing task key token.\e[0m"
        exit 1
    fi
    if [[ "${#FIRST_LINE}" -gt 110 ]]; then
        echo -e "\n\e[31m❌ Commit Blocked: Subject exceeds 110 characters (${#FIRST_LINE}); target <= 80 characters.\e[0m"
        exit 1
    fi

    # 2. Reject '---' patch-scissors lines, legacy Dual-Layer headers, raw diff dumps, and shims
    if grep -qE '^---[[:space:]]*$' "$1"; then
        echo -e "\n\e[31m❌ Commit Blocked: Standalone '---' truncates git interpret-trailers; use '___' as the Markdown divider.\e[0m"
        exit 1
    fi
    if grep -qiE '^\*\*(architectural rationale|blast radius & contracts|resolution mechanics):\*\*' "$1"; then
        echo -e "\n\e[31m❌ Commit Blocked: Legacy Dual-Layer bold headers ('**Blast Radius & Contracts:**', etc.) are prohibited.\e[0m"
        exit 1
    fi
    if grep -qE '^[-*][[:space:]]+[MADRCU][0-9]*[[:space:]]+[^[:space:]]|^[-*][[:space:]]+\.\.\.[[:space:]]+\(\+[0-9]+[[:space:]]+additional[[:space:]]+' "$1"; then
        echo -e "\n\e[31m❌ Commit Blocked: Raw git diff file-list dumps and truncation filler are prohibited (Law 4 SSoT).\e[0m"
        exit 1
    fi
    if grep -qiE 'satisfy (int|sub)-[0-9]+ deterministic execution requirements|scoped strictly to (int|sub)-[0-9]+ implementation files|implemented and verified via deterministic quality gates for (int|sub)-[0-9]+' "$1"; then
        echo -e "\n\e[31m❌ Commit Blocked: Canned boilerplate commit body shims are prohibited (Law 2).\e[0m"
        exit 1
    fi

    # 3. Require substantive rationale prose and a canonical 'Task: <KEY>' trailer matching the subject key
    SUBJECT_KEY=$(printf '%s\n' "$FIRST_LINE" | sed -E 's/^\[((INT|SUB)-[0-9]+(-[a-z0-9-]+)?)\].*$/\1/')
    if ! grep -qE "^Task:[[:space:]]+${SUBJECT_KEY}[[:space:]]*$" "$1"; then
        echo -e "\n\e[31m❌ Commit Blocked: Missing 'Task: ${SUBJECT_KEY}' Git trailer matching subject prefix.\e[0m"
        exit 1
    fi
    BODY_PROSE=$(tail -n +2 "$1" | grep -vE '^[[:space:]]*#' | grep -vE '^___+[[:space:]]*$' | grep -vE '^[A-Za-z][A-Za-z0-9-]*:[[:space:]]+' | tr -d '[:space:]')
    if [[ "${#BODY_PROSE}" -lt 10 ]]; then
        echo -e "\n\e[31m❌ Commit Blocked: Commit body requires substantive rationale prose above the 'Task:' trailer.\e[0m"
        exit 1
    fi
}
`

const PrePushHookScript = `#!/usr/bin/env bash
{
    echo "🛡️ Nomos Pre-Push Verification Check"

    NOMOS_BIN=""
    if command -v nomos >/dev/null 2>&1; then
        NOMOS_BIN="nomos"
    elif [ -n "${GOBIN:-}" ] && [ -x "${GOBIN}/nomos" ]; then
        NOMOS_BIN="${GOBIN}/nomos"
    elif [ -x "$HOME/go/bin/nomos" ]; then
        NOMOS_BIN="$HOME/go/bin/nomos"
    elif [ -x "$HOME/.local/bin/nomos" ]; then
        NOMOS_BIN="$HOME/.local/bin/nomos"
    fi

    while read -r local_ref local_oid remote_ref remote_oid; do
        if [[ "$remote_ref" == "refs/heads/main" || "$remote_ref" == "refs/heads/master" ]]; then
            if [[ "$local_ref" != "refs/heads/main" && "$local_ref" != "refs/heads/master" && -n "$local_ref" && "$local_ref" != "(delete)" ]]; then
                echo -e "\n\e[31m🚨 Push Blocked: Cannot push non-trunk branch ($local_ref) directly to remote trunk ($remote_ref) (Law 3).\e[0m"
                echo -e "You must merge via 'nomos task sync' 2PC protocol first.\n"
                exit 1
            fi

            if [[ "$local_ref" != "(delete)" && -n "$NOMOS_BIN" ]]; then
                "$NOMOS_BIN" verify --range "${remote_oid}..${local_oid}" || {
                    echo -e "\n\e[31m❌ Push Blocked by Nomos DoD constraints on trunk\e[0m"
                    exit 1
                }
            fi
        fi
    done

    exit 0
}
`

func resolveGitCommonDir(repoRoot string) (string, error) {
	cmd := SafeGitCommand(repoRoot, "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if !filepath.IsAbs(p) {
			p = filepath.Join(repoRoot, p)
		}
		return filepath.Clean(p), nil
	}
	return filepath.Join(repoRoot, ".git"), nil
}

// InstallHooks installs the pre-commit, commit-msg, and pre-push defense hooks into the target repository's .git/hooks directory.
func InstallHooks(repoRoot string) error {
	cleanRoot := repoRoot
	if cleanRoot == "" {
		discovered, err := FindRepoRoot("")
		if err != nil {
			return err
		}
		cleanRoot = discovered
	}

	gitDir, gErr := resolveGitCommonDir(cleanRoot)
	if gErr != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed resolving git dir: %v", gErr)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed creating hooks directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit"), []byte(PreCommitHookScript), 0755); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed writing pre-commit hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "commit-msg"), []byte(CommitMsgHookScript), 0755); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed writing commit-msg hook: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-push"), []byte(PrePushHookScript), 0755); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed writing pre-push hook: %v", err)
	}
	return nil
}

// EnsureHooksInstalled audits .git/hooks and installs canonical hooks if missing or drifting.
func EnsureHooksInstalled(repoRoot string) (bool, error) {
	cleanRoot := repoRoot
	if cleanRoot == "" {
		discovered, err := FindRepoRoot("")
		if err != nil {
			return false, err
		}
		cleanRoot = discovered
	}

	gitDir, gErr := resolveGitCommonDir(cleanRoot)
	if gErr != nil {
		return false, types.NewNomosError(types.ErrCodeBoundary, "failed resolving git dir: %v", gErr)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	preCommit := filepath.Join(hooksDir, "pre-commit")
	commitMsg := filepath.Join(hooksDir, "commit-msg")
	prePush := filepath.Join(hooksDir, "pre-push")

	if hookMatches(preCommit, PreCommitHookScript) &&
		hookMatches(commitMsg, CommitMsgHookScript) &&
		hookMatches(prePush, PrePushHookScript) {
		return false, nil // all hooks present, executable, and canonical
	}

	return true, InstallHooks(cleanRoot)
}

func hookMatches(path, expected string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.Mode()&0111 == 0 {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(data) == expected
}
