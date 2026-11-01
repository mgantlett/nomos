package schemas

import "strings"

// CapabilityMetrics represents the persisted invocation metrics for a capability.
#CapabilityMetrics: close({
	id:               string & strings.MinRunes(1)
	name:             string & strings.MinRunes(1)
	plane:            "intent" | "substrate"
	type:             string & strings.MinRunes(1)
	enabled:          bool
	invocation_count: int & >= 0
	last_accessed_at: string
})

