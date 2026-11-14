// Package cli implements software physics telemetry and collision detection for Nomos worktrees.
package cli

import (
	"time"
)

// ThreatLevel categorizes continuous collision proximity between concurrent worktrees (#ThreatLevel).
type ThreatLevel string

const (
	ThreatGreen  ThreatLevel = "GREEN"
	ThreatYellow ThreatLevel = "YELLOW"
	ThreatRed    ThreatLevel = "RED"
)

// FileMass asserts the structural mass and inertia of a source file (#FileMass).
type FileMass struct {
	Path                 string  `json:"path"`
	LOC                  int     `json:"loc"`
	CyclomaticComplexity int     `json:"cyclomatic_complexity"`
	ASTNodes             int     `json:"ast_nodes"`
	FanIn                int     `json:"fan_in"`
	FanOut               int     `json:"fan_out"`
	TotalMass            float64 `json:"total_mass"`
}

// CollisionVector defines real-time AST symbol or file intersection between two worktrees (#CollisionVector).
type CollisionVector struct {
	WorktreeA          string      `json:"worktree_a"`
	WorktreeB          string      `json:"worktree_b"`
	IntersectedSymbols []string    `json:"intersected_symbols"`
	IntersectedFiles   []string    `json:"intersected_files"`
	ThreatLevel        ThreatLevel `json:"threat_level"`
	TimestampEpochMs   int64       `json:"timestamp_epoch_ms"`
}

// WorktreeHull defines the bounding envelope and mutation surface of a worktree (#WorktreeHull).
// @intent software_physics_engine_spec.md #WorktreeHull
type WorktreeHull struct {
	MTPOSeq           int64    `json:"mtpo_seq,omitempty"`
	TaskKey           string   `json:"task_key"`
	WorktreePath      string   `json:"worktree_path"`
	Seat              string   `json:"seat,omitempty"`
	Phase             string   `json:"phase"`
	MutatedExports    []string `json:"mutated_exports"`
	ReadSymbols       []string `json:"read_symbols,omitempty"`
	ReferencedImports []string `json:"referenced_imports"`
	TouchedFiles      []string `json:"touched_files"`
	TotalMass         float64  `json:"total_mass"`
	MeanEntropy       float64  `json:"mean_entropy"`
}

// DetectCollision evaluates two worktree hulls for mutated file and symbol collisions.
// @intent software_physics_engine_spec.md #CollisionVector
func DetectCollision(a, b *WorktreeHull) *CollisionVector {
	if a == nil || b == nil || a.WorktreePath == b.WorktreePath {
		return nil
	}

	files := intersectStrings(a.TouchedFiles, b.TouchedFiles)
	symbols := unionUniqueStrings(
		intersectStrings(a.MutatedExports, b.MutatedExports),
		intersectStrings(a.MutatedExports, b.ReadSymbols),
		intersectStrings(b.MutatedExports, a.ReadSymbols),
	)

	threat := ThreatGreen
	if len(files) > 0 || len(symbols) > 0 {
		threat = ThreatYellow
	}
	if len(symbols) > 0 {
		threat = ThreatRed
	}

	return &CollisionVector{
		WorktreeA:          a.WorktreePath,
		WorktreeB:          b.WorktreePath,
		IntersectedSymbols: symbols,
		IntersectedFiles:   files,
		ThreatLevel:        threat,
		TimestampEpochMs:   time.Now().UTC().UnixMilli(),
	}
}

func unionUniqueStrings(groups ...[]string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, grp := range groups {
		for _, item := range grp {
			if item != "" && !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	return out
}

func intersectStrings(s1, s2 []string) []string {
	set := make(map[string]bool, len(s1))
	for _, item := range s1 {
		set[item] = true
	}
	var res []string
	for _, item := range s2 {
		if set[item] {
			res = append(res, item)
		}
	}
	return res
}
