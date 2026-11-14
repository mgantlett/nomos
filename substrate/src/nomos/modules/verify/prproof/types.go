// Package prproof implements zero-token diff AST micro-mutation probing and
// tamper-evident Pull Request proof receipts (Law 2, Law 4, Law 7).
// @intent nomos_intent_traceability_spec.md
// @intent tree_sitter_ast_gate_spec.md
package prproof

// PRProofSchemaVersion identifies the wire schema version for PRProofReceipt payloads.
const PRProofSchemaVersion = "1.0"

// DiffMutant records one single-point AST operator or literal mutation probed against package tests.
// @intent nomos_intent_traceability_spec.md #DiffMutant
type DiffMutant struct {
	ID         string `json:"id"`
	File       string `json:"file"`
	Function   string `json:"function"`
	Line       int    `json:"line"`
	OriginalOp string `json:"original_op"`
	MutatedOp  string `json:"mutated_op"`
	Killed     bool   `json:"killed"`
}

// DiffMutationReport summarizes zero-token AST micro-mutation probing across modified functions.
// @intent nomos_intent_traceability_spec.md #DiffMutationReport
type DiffMutationReport struct {
	BaseRef         string       `json:"base_ref"`
	MutantsTotal    int          `json:"mutants_total"`
	MutantsKilled   int          `json:"mutants_killed"`
	MutantsSurvived int          `json:"mutants_survived"`
	KillScorePct    float64      `json:"kill_score_pct"`
	Passed          bool         `json:"passed"`
	DurationMillis  int64        `json:"duration_ms"`
	Survivors       []DiffMutant `json:"survivors,omitempty"`
}

// GatePostureSummary captures 42-gate verification posture for PR proof sealing.
// @intent nomos_intent_traceability_spec.md #GatePostureSummary
type GatePostureSummary struct {
	GatesPassed  int  `json:"gates_passed"`
	GatesTotal   int  `json:"gates_total"`
	PipelinePass bool `json:"pipeline_pass"`
}

// PRProofOptions configures base-ref selection, diff mutation probing, and receipt file output.
// @intent nomos_intent_traceability_spec.md #PRProofOptions
type PRProofOptions struct {
	BaseRef    string `json:"base_ref,omitempty"`
	MutateDiff bool   `json:"mutate_diff,omitempty"`
	OutputPath string `json:"output_path,omitempty"`
}

// PRDebtDelta compares current structural hangover metrics against the base ref or baseline manifest.
// @intent nomos_intent_traceability_spec.md #PRDebtDelta
type PRDebtDelta struct {
	BaseRef          string `json:"base_ref"`
	BaseScore        int    `json:"base_score"`
	HeadScore        int    `json:"head_score"`
	DeltaScore       int    `json:"delta_score"`
	HeadGrade        string `json:"head_grade"`
	BaselineManifest bool   `json:"baseline_manifest"`
}

// PRProofReceipt seals 42-gate posture, plane separation, debt delta, and mutation score.
// @intent nomos_intent_traceability_spec.md #PRProofReceipt
type PRProofReceipt struct {
	SchemaVersion  string              `json:"schema_version"`
	ProjectName    string              `json:"project_name"`
	BaseRef        string              `json:"base_ref"`
	HeadCommit     string              `json:"head_commit"`
	GatesPassed    int                 `json:"gates_passed"`
	GatesTotal     int                 `json:"gates_total"`
	PlaneSummary   string              `json:"plane_summary"`
	DebtDelta      PRDebtDelta         `json:"debt_delta"`
	MutationReport *DiffMutationReport `json:"mutation_report,omitempty"`
	Passed         bool                `json:"passed"`
	ReceiptSHA256  string              `json:"receipt_sha256"`
}
