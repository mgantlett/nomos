package schemas

import "strings"

// ThreatLevel categorizes continuous collision proximity between concurrent worktrees.
#ThreatLevel: "GREEN" | "YELLOW" | "RED"

// FileMass asserts the structural mass and inertia of a source file.
#FileMass: close({
	path:                  string & strings.MinRunes(1)
	loc:                   int & >= 0
	cyclomatic_complexity: int & >= 0
	ast_nodes:             int & >= 0
	fan_in:                int & >= 0
	fan_out:               int & >= 0
	total_mass:            float & >= 0.0
})

// CollisionVector defines real-time AST symbol or file intersection between two worktrees.
#CollisionVector: close({
	worktree_a:          string & strings.MinRunes(1)
	worktree_b:          string & strings.MinRunes(1)
	intersected_symbols: [...string & strings.MinRunes(1)]
	intersected_files:   [...string & strings.MinRunes(1)]
	threat_level:        #ThreatLevel
	timestamp_epoch_ms:  int & >= 0
})

// WorktreeHull defines the bounding envelope and mutation surface of a transient task worktree.
#WorktreeHull: close({
	task_key:           =~"^(INT|SUB)-[0-9]+$"
	worktree_path:      string & strings.MinRunes(1)
	seat?:              string & strings.MinRunes(1)
	phase:              string & strings.MinRunes(1)
	mutated_exports:    [...string & strings.MinRunes(1)]
	referenced_imports: [...string & strings.MinRunes(1)]
	touched_files:      [...string & strings.MinRunes(1)]
	total_mass:         float & >= 0.0
	mean_entropy:       float & >= 0.0 & <= 8.0
})

