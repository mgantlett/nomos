package schemas

import "strings"

// NomosError encapsulates structured agent-focused context for execution boundary constraint failures.
#NomosError: close({
	code:                string & =~"^ERR_[A-Z0-9_]+$"
	description:         string & strings.MinRunes(1)
	remedy?:             string
	explain_command?:    string
	context_files?:      [...string]
	fatal:               bool | *false
	triage_hash?:        string & =~"^ERR-[a-f0-9]+$"
	interactive_prompt?: string
})
