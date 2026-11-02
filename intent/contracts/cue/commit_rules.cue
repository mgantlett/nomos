package schemas

import "strings"

#CommitHydrationSource: "explicit_flag" | "walkthrough" | "task_card" | "git_diff"

#CommitMessage: close({
	raw:                  string & strings.MinRunes(15) & =~"^\\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?\\]\\s+(feat|fix|docs|chore|refactor|test)(\\([a-z0-9_,/-]+\\))?:\\s+.*$"
	task_key:             =~"^\\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?\\]$"
	type:                 "feat" | "fix" | "docs" | "chore" | "refactor" | "test"
	scope?:               string & =~"^[a-z0-9_,/-]+$"
	summary:              string & strings.MinRunes(5) & strings.MaxRunes(120) & =~"^[^\\r\\n]+$"
	rationale:            string & strings.MinRunes(10)
	blast_radius:         string & strings.MinRunes(5)
	resolution_mechanics: string & strings.MinRunes(10)
})

// CommitValidationReport captures gate evaluation across branch commits.
#CommitValidationReport: close({
	gate_name:         string
	passed:            bool
	evaluated_commits: int & >= 0
	violations:        [...string]
	remedy?:           string
})
