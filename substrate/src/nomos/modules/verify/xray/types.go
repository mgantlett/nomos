// Package xray implements read-only zero-init brownfield structural diagnostics
// and Day-0 baseline envelope snapshotting across polyglot repositories.
// @intent intent.md
// @intent tree_sitter_ast_gate_spec.md
// @intent task_lifecycle_cli_spec.md
package xray

// DefaultTopHotspots defines the default number of ranked hotspot files emitted in XrayReport.
const DefaultTopHotspots = 10

// BaselineSchemaVersion identifies the wire contract version for nomos.baseline.json.
const BaselineSchemaVersion = "1.0"

// XrayScanOptions configures read-only brownfield structural scanning (#XrayScanOptions).
type XrayScanOptions struct {
	TopHotspots int `json:"top_hotspots"`
}

// DefaultScanOptions returns canonical XrayScanOptions bounds.
func DefaultScanOptions() XrayScanOptions {
	return XrayScanOptions{TopHotspots: DefaultTopHotspots}
}

// XrayHotspot ranks a single source file by aggregated structural debt (#XrayHotspot).
type XrayHotspot struct {
	File          string   `json:"file"`
	Language      string   `json:"language"`
	LOC           int      `json:"loc"`
	CDS           int      `json:"cds"`
	MaxComplexity int      `json:"max_complexity"`
	MaxFuncLOC    int      `json:"max_func_loc"`
	Clones        int      `json:"clones,omitempty"`
	Stubs         int      `json:"stubs,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
}

// XrayBaselineEnvelope records observed upper-bound structural metrics for baseline locking (#XrayBaselineEnvelope).
type XrayBaselineEnvelope struct {
	ObservedMaxFileLOC      int `json:"observed_max_file_loc"`
	ObservedMaxFuncLOC      int `json:"observed_max_func_loc"`
	ObservedMaxComplexity   int `json:"observed_max_complexity"`
	ObservedMaxCDS          int `json:"observed_max_cds"`
	ObservedMaxPackageFiles int `json:"observed_max_package_files"`
	CloneGroupCount         int `json:"clone_group_count"`
}

// XrayReport summarizes zero-init polyglot structural debt and hangover indicators (#XrayReport).
type XrayReport struct {
	ProjectName        string               `json:"project_name"`
	DurationMillis     int64                `json:"duration_ms"`
	Initialized        bool                 `json:"initialized"`
	FilesScanned       int                  `json:"files_scanned"`
	TotalLOC           int                  `json:"total_loc"`
	Languages          map[string]int       `json:"languages"`
	HangoverScore      int                  `json:"hangover_score"`
	Grade              string               `json:"grade"`
	CloneGroups        int                  `json:"clone_groups"`
	MonolithFiles      int                  `json:"monolith_files"`
	OversizedFuncs     int                  `json:"oversized_funcs"`
	EmptyStubs         int                  `json:"empty_stubs"`
	SkippedTests       int                  `json:"skipped_tests"`
	SecuritySinks      int                  `json:"security_sinks"`
	InferredBoundaries int                  `json:"inferred_boundaries"`
	Hotspots           []XrayHotspot        `json:"hotspots,omitempty"`
	BaselineEnvelope   XrayBaselineEnvelope `json:"baseline_envelope"`
	NextCommand        string               `json:"next_command"`
}

// XrayBaselineManifest persists trunk-pinned Day-0 structural debt bounds in nomos.baseline.json (#XrayBaselineManifest).
type XrayBaselineManifest struct {
	SchemaVersion      string               `json:"schema_version"`
	ProjectName        string               `json:"project_name"`
	RecordedAt         string               `json:"recorded_at"`
	HeadCommit         string               `json:"head_commit,omitempty"`
	HangoverScore      int                  `json:"hangover_score"`
	Grade              string               `json:"grade"`
	FilesScanned       int                  `json:"files_scanned"`
	TotalLOC           int                  `json:"total_loc"`
	InferredBoundaries int                  `json:"inferred_boundaries"`
	BaselineEnvelope   XrayBaselineEnvelope `json:"baseline_envelope"`
}
