package schemas

// Polyglot AST Validation and Source Root Configuration Schema (Law 2, Law 4, Law 5)
#SupportedLanguage: "go" | "typescript" | "tsx" | "javascript" | "svelte" | "markdown" | "yaml" | "python" | "rust" | "c" | "cpp" | "java" | "kotlin" | "csharp" | "html" | "css"

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

#FindingRule: "magic_port" | "localhost_url" | "ecosystem_map" | "ip_leak" | "cognitive_complexity" | "docstring_density" | "anti_shim_stub" | "test_skip_reward_hack" | "coverage_ratchet"

#PolyglotASTFinding: close({
	file:       string
	line:       int & >= 0
	func_name?: string
	rule:       #FindingRule
	message:    string
})

#PolyglotCoverageReport: close({
	format:       "lcov" | "cobertura"
	source_root:  string
	line_rate:    number & >= 0 & <= 100
	min_coverage: number & >= 0 & <= 100
	passed:       bool
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
	compilation_cache:   bool | *true
	pure_go:             bool | *true
	supported_languages: [...#SupportedLanguage]
	query_roots:         [...string]
})
