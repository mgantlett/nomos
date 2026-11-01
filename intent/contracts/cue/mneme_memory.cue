package schemas

import "strings"

// MnemeMemory defines the formal contract for Nomos Mneme episodic and semantic memory records.
#MnemeMemory: close({
	id?:            int & >= 1
	repo_id?:       string
	kind:           "reflection" | "axiom" | "decision" | "release" | "legacy_spec" | "fact" | "preference" | "architecture" | "task" | "commit" | "review" | "pitfall" | "pattern"
	topic:          string & strings.MinRunes(3)
	content:        string & strings.MinRunes(5)
	task_id?:       "-" | (string & =~"^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$") | (string & =~"^[A-Z0-9]+(-[A-Z0-9]+)*$")
	embedding?:     [float, ...float]
	status?:        "active" | "superseded" | "tombstoned" | "consolidated" | "evolved"
	superseded_by?: int & >= 0
	access_count?:  int & >= 0
	created_at:     string & =~"^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}.*"
})
