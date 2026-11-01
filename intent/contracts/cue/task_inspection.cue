package schemas

import "strings"

// TaskInspectionRecord defines the DTO contract emitted by nomos task show.
#TaskInspectionRecord: close({
	key:          string & =~"^(INT|SUB)-[0-9]+$"
	title:        string & strings.MinRunes(5)
	status:       "BACKLOG" | "TODO" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "DONE" | "BLOCKED" | "ABORTED"
	plane:        "intent" | "substrate"
	tier:         "T1" | "T2"
	priority:     int & >= 0 & <= 100
	twin_key?:    string & =~"^(INT|SUB)-[0-9]+$"
	blocked_by?:  [...string]
	blocks?:      [...string]
	worktree_path?: string
	branch_name?: string
	description:  string
})

