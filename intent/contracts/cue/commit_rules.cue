package schemas

import "strings"

#CommitMessage: close({
	raw:      string & strings.MinRunes(15) & =~"^\\[(INT|SUB)-[0-9]+(-[a-z0-9-]+)?\\]\\s+(feat|fix|docs|chore|refactor|test)(\\([a-z0-9_-]+\\))?:\\s+.*$"
	task_key: =~"^\\[(INT|SUB)-[0-9]+(-[a-z0-9-]+)?\\]$"
	type:     "feat" | "fix" | "docs" | "chore" | "refactor" | "test"
	summary:  string & strings.MinRunes(10) & strings.MaxRunes(100) & =~"^[^\\r\\n]+$"
})

// CommitValidationReport captures gate evaluation across branch commits.
#CommitValidationReport: close({
	gate_name:         string
	passed:            bool
	evaluated_commits: int & >= 0
	violations:        [...string]
	remedy?:           string
})
