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
type WorktreeHull struct {
	TaskKey           string   `json:"task_key"`
	WorktreePath      string   `json:"worktree_path"`
	Seat              string   `json:"seat,omitempty"`
	Phase             string   `json:"phase"`
	MutatedExports    []string `json:"mutated_exports"`
	ReferencedImports []string `json:"referenced_imports"`
	TouchedFiles      []string `json:"touched_files"`
	TotalMass         float64  `json:"total_mass"`
	MeanEntropy       float64  `json:"mean_entropy"`
}

// DetectCollision evaluates two worktree hulls for mutated file and symbol collisions.
func DetectCollision(a, b *WorktreeHull) *CollisionVector {
	if a == nil || b == nil || a.WorktreePath == b.WorktreePath {
		return nil
	}

	files := intersectStrings(a.TouchedFiles, b.TouchedFiles)
	symbols := intersectStrings(a.MutatedExports, b.MutatedExports)

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
