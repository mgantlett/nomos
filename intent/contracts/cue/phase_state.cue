package schemas

import (
	"strings"
	"time"
)

// Phase defines the canonical lifecycle phases of a Nomos workspace.
#Phase: "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"

// Transition defines the deterministic phase state machine allowed transitions.
#Transition:
	{from: "IDLE", to: "PLAN"} |
	{from: "IDLE", to: "EDIT"} |
	{from: "PLAN", to: "EDIT"} |
	{from: "EDIT", to: "REVIEW"} |
	{from: "REVIEW", to: "SYNC"} |
	{from: "SYNC", to: "LEARN"} |
	{from: "SYNC", to: "IDLE"} |
	{from: "LEARN", to: "IDLE"} |
	{from: "EDIT", to: "IDLE"} |
	{from: "PLAN", to: "IDLE"} |
	{from: "REVIEW", to: "EDIT"} |
	{from: "SYNC", to: "EDIT"}

// PhaseState defines the active workspace phase state, affine CapBAC tokens,
// execution step budgets, and task bindings.
#PhaseState: close({
	phase:        #Phase
	task_id?:     string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"
	assignee:     string & strings.MinRunes(1) | *"antigravity"
	token?:       string & =~"^cap_[a-f0-9]{32}$"
	step_budget:  int & >= 1 & <= 500 | *50
	tokens_used:  int & >= 0 | *0
	next_actions?: [...#ActionAffordance]
	updated_at?:  time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")

	// Fail-Closed Pre-Flight (Law 7): Active non-IDLE phases require task_id and CapBAC token
	if phase != "IDLE" {
		task_id: string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"
		token:   string & =~"^cap_[a-f0-9]{32}$"
	}
})
