// Package mcpcmd defines complete, disambiguated MCP tool descriptors and JSON schemas.
// @intent task_lifecycle_cli_spec.md
package mcpcmd

func readOnlyToolAnnotations() map[string]any {
	return map[string]any{
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
}

func directoryPropertySchema() map[string]any {
	return map[string]any{
		propTypeKey: schemaTypeString,
		propDescKey: "Target repository root or subdirectory path (defaults to current working directory).",
		"default":   ".",
		"minLength": 1,
		"examples":  []string{".", "./substrate"},
	}
}

func boundedLimitSchema(desc string, def, max int) map[string]any {
	return map[string]any{
		propTypeKey: schemaTypeInteger,
		propDescKey: desc,
		"default":   def,
		"minimum":   1,
		"maximum":   max,
		"examples":  []int{def},
	}
}

func arrayWrapperOutputSchema(itemDesc string) map[string]any {
	return map[string]any{
		propTypeKey: schemaTypeObject,
		keyProperties: map[string]any{
			"count": map[string]any{propTypeKey: schemaTypeInteger, propDescKey: "Number of returned records."},
			"items": map[string]any{
				propTypeKey: "array",
				propDescKey: itemDesc,
				"items":     map[string]any{propTypeKey: schemaTypeObject},
			},
		},
		"required": []string{"count", "items"},
	}
}

func buildSessionToolDescriptors(ro, dirProp map[string]any) []MCPToolDescriptor {
	return []MCPToolDescriptor{
		{
			Name:  "nomos_handshake",
			Title: "Synthesize JIT Session Orientation",
			Description: "Synthesize read-only Just-In-Time (JIT) session orientation, active task phase, and Next Actions for a repository without modifying files or Git hooks. " +
				"Use when starting an agent turn to discover the active worktree and allowed phase actions. Do NOT use when checking global daemon/DB health without JIT directives (use nomos_status instead) or listing backlog tasks (use nomos_task_dag instead).",
			InputSchema: objectSchema(map[string]any{"dir": dirProp}, nil),
			OutputSchema: objectSchema(map[string]any{
				"phase":        map[string]any{propTypeKey: schemaTypeString, propDescKey: "Active Nomos lifecycle phase."},
				"next_actions": map[string]any{propTypeKey: "array", propDescKey: "Deterministic phase affordances."},
			}, nil),
			Annotations: ro,
		},
		{
			Name:  "nomos_status",
			Title: "Inspect Workspace & Database Health Status",
			Description: "Return a concise snapshot of workspace lifecycle phase, active Git branch, SQLite ledger health, and aggregate task counts. " +
				"Use when checking overall host/database readiness or current branch state. Do NOT use when requesting full JIT phase playbooks (use nomos_handshake instead) or scanning code debt (use nomos_xray instead).",
			InputSchema: objectSchema(map[string]any{}, nil),
			OutputSchema: objectSchema(map[string]any{
				"phase":  map[string]any{propTypeKey: schemaTypeString, propDescKey: "Current workspace lifecycle phase."},
				"branch": map[string]any{propTypeKey: schemaTypeString, propDescKey: "Current Git branch name."},
			}, nil),
			Annotations: ro,
		},
	}
}

func buildLedgerToolDescriptors(ro, dirProp map[string]any) []MCPToolDescriptor {
	return []MCPToolDescriptor{
		{
			Name:  "nomos_task_dag",
			Title: "List Unblocked Task DAG Wavefront",
			Description: "Query unblocked causal DAG wavefront tasks (including Agent-to-Agent scope_paths, TC-* obligations, and active worktree MTPO overlap flags) ready for execution without mutating task ledger state. " +
				"Use when selecting the next unblocked INT-* or SUB-* task to start (to submit a new multi-node DAG subgraph, invoke CLI 'nomos task batch -F - --json'). Do NOT use when recalling past architectural decisions or lessons (use nomos_mem_recall instead).",
			InputSchema: objectSchema(map[string]any{
				"dir":   dirProp,
				"limit": boundedLimitSchema("Maximum number of unblocked wavefront tasks to return.", 10, 100),
			}, nil),
			OutputSchema: arrayWrapperOutputSchema("Unblocked wavefront task candidates with scope_paths, obligations, and overlaps_active."),
			Annotations:  ro,
		},
		{
			Name:  "nomos_mem_recall",
			Title: "Search Nomos Mneme Episodic Memory",
			Description: "Search Nomos Mneme institutional memory records (decisions, patterns, pitfalls, axioms) matching a keyword or semantic query. " +
				"Use when retrieving prior architectural rationale, learned pitfalls, or workspace conventions before editing code. Do NOT use when inspecting compiled quality gate rules (use nomos_capabilities instead) or listing open backlog tasks (use nomos_task_dag instead).",
			InputSchema: objectSchema(map[string]any{
				"query": map[string]any{
					propTypeKey: schemaTypeString,
					propDescKey: "Keyword or technical phrase to search in Nomos Mneme.",
					"minLength": 1,
					"examples":  []string{"worktree boundary", "plane separation"},
				},
				"limit": boundedLimitSchema("Maximum number of matching memory records to return.", 5, 50),
			}, []string{"query"}),
			OutputSchema: arrayWrapperOutputSchema("Matching Nomos Mneme memory records."),
			Annotations:  ro,
		},
	}
}

func buildAnalysisToolDescriptors(ro, dirProp map[string]any) []MCPToolDescriptor {
	return []MCPToolDescriptor{
		{
			Name:  "nomos_verify_impact",
			Title: "Compute 1-Hop AST Reverse-Dependency Blast Radius",
			Description: "Compute the deterministic 1-hop AST reverse-dependency blast radius of packages impacted by uncommitted or branch-modified files. " +
				"Use when assessing which downstream packages and test suites are affected by current edits. Do NOT use when auditing full-repository structural complexity or AI Hangover Score (use nomos_xray instead).",
			InputSchema: objectSchema(map[string]any{"dir": dirProp}, nil),
			OutputSchema: map[string]any{
				propTypeKey: schemaTypeObject,
				keyProperties: map[string]any{
					"dir":                 map[string]any{propTypeKey: schemaTypeString, propDescKey: "Resolved repository root path."},
					"modified_files":      map[string]any{propTypeKey: "array", propDescKey: "Modified workspace-relative files.", "items": map[string]any{propTypeKey: schemaTypeString}},
					"impacted_packages":   map[string]any{propTypeKey: "array", propDescKey: "Impacted package paths.", "items": map[string]any{propTypeKey: schemaTypeString}},
					"indirect_boundaries": map[string]any{propTypeKey: "array", propDescKey: "Advisory runtime, SQL, wire-tag, CLI flag, and non-AST boundaries.", "items": map[string]any{propTypeKey: schemaTypeObject}},
				},
				"required": []string{"dir", "impacted_packages"},
			},
			Annotations: ro,
		},
		{
			Name:  "nomos_xray",
			Title: "Run Zero-Write Polyglot AST Repository X-Ray",
			Description: "Run a zero-write polyglot static AST structural X-Ray across a local repository directory to measure AI Hangover Score, cyclomatic/cognitive debt, and top hotspot files. " +
				"Use when auditing overall codebase health, complexity hotspots, or brownfield baseline posture. Do NOT use when computing only the diff-scoped blast radius of modified files (use nomos_verify_impact instead) or looking up a single gate definition (use nomos_capabilities instead).",
			InputSchema: objectSchema(map[string]any{
				"dir": dirProp,
				"top": boundedLimitSchema("Number of highest-debt hotspot files to return.", 10, 50),
			}, nil),
			OutputSchema: objectSchema(map[string]any{
				"hangover_score": map[string]any{propTypeKey: schemaTypeInteger, propDescKey: "Composite 0-100 structural debt score."},
				"grade":          map[string]any{propTypeKey: schemaTypeString, propDescKey: "Structural health letter grade."},
			}, nil),
			Annotations: ro,
			Meta: map[string]any{
				"ui": map[string]any{
					"resourceUri": mcpAppXrayURI,
				},
			},
		},
	}
}

func capabilitiesToolDescriptor(ro, dirProp map[string]any) MCPToolDescriptor {
	return MCPToolDescriptor{
		Name:  "nomos_capabilities",
		Title: "Inspect Compiled DoD Gates, Hooks, Directives & Intent Contracts",
		Description: "Inspect compiled Nomos Definition of Done quality gates, ambient Git/session hooks, embedded phase directives, or resolve a specific gate ID, #StructName contract, TC-* obligation, or ERR_* error code. " +
			"Use when looking up gate invariants, hook behavior, or slicing a bijective specification contract and its verifying tests. Do NOT use when searching project-specific historical decisions in Mneme (use nomos_mem_recall instead) or scanning live source files (use nomos_xray instead).",
		InputSchema: objectSchema(map[string]any{
			"dir": dirProp,
			"id": map[string]any{
				propTypeKey: schemaTypeString,
				propDescKey: "Optional capability, gate, #StructName contract, TC-* obligation, ERR_* code, or spec filename to inspect.",
				"examples":  []string{"plane_separation", "#GateResult", "TC-DOD-07"},
			},
			"type": map[string]any{
				propTypeKey: schemaTypeString,
				propDescKey: "Optional capability category filter.",
				"enum":      []string{"gate", "hook", "directive", "engine", "cli"},
				"examples":  []string{"gate"},
			},
		}, nil),
		OutputSchema: objectSchema(map[string]any{
			"gates_count": map[string]any{propTypeKey: schemaTypeInteger, propDescKey: "Total active quality gates."},
		}, nil),
		Annotations: ro,
	}
}
