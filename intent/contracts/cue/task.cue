package schemas

import (
	"strings"
	"time"
)

#TaskKey: string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$"

// TaskRecord defines the formal specification for Living Ledger task records and causal DAG state.
#TaskRecord: close({
	id?:           int & >= 1
	key:           #TaskKey
	plane:         "intent" | "substrate"
	schema_type:   "intent" | "substrate" | "membrane" | "infra" | "cycle" | "debt" | "spike"
	title:         string & strings.MinRunes(5)
	description?:  string & strings.MinRunes(1)
	status:        "BACKLOG" | "TODO" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "DONE" | "BLOCKED" | "ABORTED"
	tier:          "T1" | "T2"
	priority:      int & >= 0 & <= 100
	created_at:    time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")
	updated_at:    time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")
	completed_at?: null | time.Format(time.RFC3339) | (string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}")
	dependencies?: [...#TaskKey]
	twin_key?:     #TaskKey
	arc_id?:       string
	intent_delta?:   "none" | "contract" | "spec"
	burden?:         int & >= 0
	context_burden?: int & >= 0
	arch_layer?:     int & >= 0

	// Relational Invariant (Law 7): INT-* keys map to intent, SUB-* keys map to substrate
	if strings.HasPrefix(key, "INT-") {
		plane: "intent"
	}
	if strings.HasPrefix(key, "SUB-") {
		plane: "substrate"
	}

	// Relational Invariant (Law 7): When completed, completed_at must not be null
	if status == "DONE" {
		completed_at: !=null
	}
})

// WavefrontCandidate defines the structured candidate task emitted by nomos task dag --json.
#WavefrontCandidate: close({
	key:          #TaskKey
	title:        string & strings.MinRunes(5)
	plane:        "intent" | "substrate"
	schema_type:  "intent" | "substrate" | "membrane" | "infra" | "cycle" | "debt" | "spike"
	status:       "BACKLOG" | "TODO" | "PLAN" | "EDIT" | "REVIEW" | "SYNC" | "DONE" | "BLOCKED" | "ABORTED"
	tier:         "T1" | "T2"
	twin_key?:    #TaskKey
	is_unblocked: bool | *true
})

