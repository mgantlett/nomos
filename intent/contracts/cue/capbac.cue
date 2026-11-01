package schemas

import "time"

// CapBACLease defines the formal CapBAC object capability lease issued to a task worktree.
#CapBACLease: close({
	id:          string & =~"^cap_lease_[a-f0-9]{32}$"
	task_key:    string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"
	phase:       #Phase
	step_budget: int & >= 1 & <= 500
	tokens_used: int & >= 0 & <= step_budget
	issued_at:   time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")
	valid:       bool | *true
})

tokens_used: int & >= 0 | *0
step_budget: int & >= 1 & <= 500 | *50
phase:       #Phase | *"IDLE"

// Relational Pre-Flight Invariant (Law 7): tokens_used must not exceed step_budget
valid: *false | bool
if (tokens_used <= step_budget) && (phase == "IDLE" || phase == "PLAN" || phase == "EDIT" || phase == "REVIEW" || phase == "SYNC" || phase == "LEARN") {
	valid: true
}
