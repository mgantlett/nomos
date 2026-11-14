// Package mtpo implements Monotonic Trajectory Pre-Order (MTPO) sequencing,
// sub-25ms AST read/write set ledgering, and cross-worktree serializability mediation.
// @intent software_physics_engine_spec.md
// @intent nomos_hermetic_worktree_spec.md
package mtpo

// Trajectory lifecycle status constants in worktree_mtpo.
const (
	StatusActive    = "ACTIVE"
	StatusCommitted = "COMMITTED"
	StatusAborted   = "ABORTED"

	HazardWriteWrite = "WRITE_WRITE"
	HazardReadWrite  = "READ_WRITE"
)

// MTPOLedgerEntry records the Monotonic Trajectory Pre-Order sequence and AST read/write sets.
// @intent software_physics_engine_spec.md #MTPOLedgerEntry
type MTPOLedgerEntry struct {
	Seq               int64    `json:"seq"`
	RepoRoot          string   `json:"repo_root"`
	TaskKey           string   `json:"task_key"`
	WorktreePath      string   `json:"worktree_path"`
	BaseCommit        string   `json:"base_commit"`
	Status            string   `json:"status"`
	WriteFiles        []string `json:"write_files"`
	WriteSymbols      []string `json:"write_symbols"`
	ReadSymbols       []string `json:"read_symbols"`
	ReferencedImports []string `json:"referenced_imports"`
	UpdatedAt         string   `json:"updated_at"`
}

// MTPOConflict describes an AST write-write or read-write hazard between ordered trajectories.
// @intent software_physics_engine_spec.md #MTPOConflict
type MTPOConflict struct {
	EarlierSeq         int64    `json:"earlier_seq"`
	EarlierTaskKey     string   `json:"earlier_task_key"`
	LaterSeq           int64    `json:"later_seq"`
	LaterTaskKey       string   `json:"later_task_key"`
	HazardKind         string   `json:"hazard_kind"`
	OverlappingFiles   []string `json:"overlapping_files,omitempty"`
	OverlappingSymbols []string `json:"overlapping_symbols,omitempty"`
	Advisory           string   `json:"advisory"`
}

// SerializabilityVerdict reports cross-worktree AST serializability before trunk fast-forward.
// @intent software_physics_engine_spec.md #SerializabilityVerdict
type SerializabilityVerdict struct {
	TaskKey        string         `json:"task_key"`
	Seq            int64          `json:"seq"`
	Passed         bool           `json:"passed"`
	MergedConflict []MTPOConflict `json:"merged_conflict,omitempty"`
	ActiveHazards  []MTPOConflict `json:"active_hazards,omitempty"`
	Remedy         string         `json:"remedy,omitempty"`
}
