package schemas

import "time"

// CapBACToken defines the authoritative schema for authenticated agent capability tokens.
#CapBACToken

#CapBACToken: close({
	id:          string & =~"^(cap_[a-f0-9]{32}|cap_lease_[a-f0-9]+)$"
	phase:       "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"
	task_id:     string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"
	step_budget: int & >= 1 & <= 500
	tokens_used: int & >= 0 & <= step_budget
	created_at?: time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")
})
