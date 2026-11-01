package schemas

import "strings"

// WorktreeBoundaryConfig defines the contract for hermetic worktree boundaries and protected branches.
#WorktreeBoundaryConfig: close({
	protected_branches: [...string]
	worktree_base_dir:  string & strings.MinRunes(1)
	exempt_patterns?:   [...string]
})

// WorktreeSession captures the runtime state of an isolated task worktree.
#WorktreeSession: close({
	task_key:      =~"^(INT|SUB)-[0-9]+$"
	branch_name:   string & strings.MinRunes(1)
	worktree_path: string & strings.MinRunes(1)
	created_at:    string
})
