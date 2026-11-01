package schemas

import "strings"

#TaskID: string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"

// HandshakeEvent represents the formal single source of truth (SSoT) specification
// for Just-In-Time (JIT) session orientation telemetry, CapBAC tokens, and DoD quality gate status.
#HandshakeEvent: close({
	seat:              string & strings.MinRunes(1)
	repository:        string & strings.MinRunes(1)
	plane?:            "intent" | "substrate"
	phase:             "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"
	active_task?:      #TaskID
	assignee?:         string & strings.MinRunes(1)
	token_id?:         string & strings.MinRunes(1)
	step_budget?:      int & >= 0
	tokens_used?:      int & >= 0
	databases_healthy: bool
	gate_count:        int & >= 0
	ready_wavefront:   [...#TaskID]
	directives?:            [...string & strings.MinRunes(1)]
	learned_pitfalls?:      [...string & strings.MinRunes(1)]
	verified_patterns?:     [...string & strings.MinRunes(1)]
	graduation_candidates?: [...string & strings.MinRunes(1)]
	impact_summary?:        [...string & strings.MinRunes(1)]
	next_actions?:     [...#ActionAffordance]
	deprecated_terms?: [string]: string
	timestamp?:        string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}.*"
})

// ActionAffordance defines a deterministic HATEOAS next-step hypermedia command for Tier 1 coding agents.
#ActionAffordance: close({
	command: string & strings.MinRunes(1)
	summary: string & strings.MinRunes(1)
	phase:   #DirectivePhase
})

#DirectivePhase: "IDLE" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "LEARN"


#DirectiveFrontmatter: close({
	title:          string & strings.MinRunes(1)
	description?:   string
	authors:        [...string]
	classification: "public" | "internal"
	date:           string
	last_updated:   string
	plane:          "intent" | "substrate"
	status:         "active" | "archived" | "superseded"
	type:           string & strings.MinRunes(1)
	phases:         [...#DirectivePhase]
	category?:      string
	priority?:      "low" | "medium" | "high" | "critical"
	items?:         [...string]
	summary?:       string
})


