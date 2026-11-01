package schemas

// Polyglot AST Validation and Source Root Configuration Schema (Law 2, Law 4, Law 5)
#SupportedLanguage: "go" | "typescript" | "javascript" | "svelte" | "markdown" | "yaml" | "python" | "rust" | "c" | "cpp" | "html" | "css"

#SourceRootConfig: close({
	path:             string & !~"^/" & !~"\\.\\."
	language:         #SupportedLanguage
	exclude_patterns: [...string]
})

#PolyglotWorkspaceManifest: close({
	source_roots:     [...#SourceRootConfig]
	strict_polyglot:  bool | *false
})

#TreeSitterQueryConfig: close({
	language:         #SupportedLanguage
	query_pattern:    string
	target_rule:      #FindingRule
	capture_symbol?:  string
})

#FindingRule: "magic_port" | "localhost_url" | "ecosystem_map" | "ip_leak" | "cognitive_complexity" | "docstring_density"

#PolyglotASTFinding: close({
	file:       string
	line:       int & >= 0
	func_name?: string
	rule:       #FindingRule
	message:    string
})

#PolyglotScanReport: close({
	worktree_path: string
	scanned_files: int & >= 0
	findings:      [...#PolyglotASTFinding]
	passed:        bool
})

#TreeSitterWasmConfig: close({
	engine_name:         string & !~"^$"
	runtime:             "wazero"
	pure_go:             bool | *true
	supported_languages: [...#SupportedLanguage]
	query_roots:         [...string]
})

