package schemas

// BoundaryConfig declares architectural package and CLI command boundary protections.
#BoundaryConfig: {
	enforce:              bool | *true
	prohibited_packages?: [...string]
	prohibited_commands?: [...string]
	lock_dependencies?:   bool | *false
}

// ThresholdsConfig captures numerical AST and test hygiene quality bounds.
#ThresholdsConfig: {
	max_cognitive_debt_score:      int & >= 0 | *45
	max_cyclomatic_complexity:     int & >= 1 | *15
	max_file_loc:                  int & >= 1 | *400
	max_func_loc:                  int & >= 1 | *60
	min_comment_density_pct:       int & >= 0 & <= 100 | *10
	min_test_coverage_pct:         int & >= 0 & <= 100 | *55
	min_clone_lines?:              int & >= 1
	max_func_params?:              int & >= 1
	max_magic_string_duplication?: int & >= 1
}

// QualityGatesConfig defines verification policies and bilateral plane roots.
#QualityGatesConfig: {
	forbidden_literals?:     [...string]
	forbidden_shims?:        [...string]
	strict_mode:             bool | *true
	polyglot_toolchains?:    bool | *true
	exclude?:                [...string]
	intent_source_roots?:    [...string]
	substrate_source_roots?: [...string]
	contracts?:              #ContractsBoundaryConfig
	thresholds?:             #ThresholdsConfig
}

// GovernanceConfig aggregates boundary and quality gates configurations.
#GovernanceConfig: {
	boundary?:      #BoundaryConfig
	quality_gates?: #QualityGatesConfig
}

// NomosConfig models the root ecosystem configuration for Nomos Gen 4.
#NomosConfig: {
	project:        string | *"nomos"
	default_branch: string | *"main"
	governance?:    #GovernanceConfig
}

// DefaultNomosConfig provides the canonical downstream default configuration instance.
#DefaultNomosConfig: #NomosConfig & {
	project:        "nomos"
	default_branch: "main"
	governance: {
		boundary: {
			enforce:             true
			prohibited_packages: []
			prohibited_commands: []
			lock_dependencies:   false
		}
		quality_gates: {
			forbidden_literals: [
				"\"/home/\"",
				"\"/Users/\"",
				"\"/var/home/\"",
			]
			strict_mode:         true
			polyglot_toolchains: true
			exclude:     []
			intent_source_roots: [
				"intent",
			]
			substrate_source_roots: [
				"substrate",
			]
			thresholds: {
				max_cognitive_debt_score:     45
				max_cyclomatic_complexity:    15
				max_file_loc:                 400
				max_func_loc:                 60
				min_comment_density_pct:      10
				min_test_coverage_pct:        55
				min_clone_lines:              18
				max_func_params:              8
				max_magic_string_duplication: 10
			}
		}
	}
}

#NomosConfig
