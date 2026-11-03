// Package task defines domain models, schema constraints, and tracking mechanisms
// synthesized directly from authoritative Intent CUE schemas (task.cue).
package task

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

var regexTaskKey = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)

// Status represents the formal lifecycle state of a task.
type Status string

const (
	StatusBacklog Status = "BACKLOG"
	StatusTodo    Status = "TODO"
	StatusReady   Status = "READY"
	StatusPlan    Status = "PLAN"
	StatusEdit    Status = "EDIT"
	StatusInProgress Status = "IN_PROGRESS"
	StatusReview  Status = "REVIEW"
	StatusSync    Status = "SYNC"
	StatusDone    Status = "DONE"
	StatusBlocked Status = "BLOCKED"
	StatusAborted Status = "ABORTED"
)

// SchemaType defines the plane-tailored task classification.
type SchemaType string

const (
	SchemaIntent    SchemaType = "intent"
	SchemaSubstrate SchemaType = "substrate"
	SchemaMembrane  SchemaType = "membrane"
	SchemaInfra     SchemaType = "infra"
	SchemaCycle     SchemaType = "cycle"
	SchemaDebt      SchemaType = "debt"
	SchemaSpike     SchemaType = "spike"
)

// Tier represents the cognitive complexity tier of a task.
type Tier string

const (
	Tier1 Tier = "T1"
	Tier2 Tier = "T2"
)

// ValidateTaskKey verifies that a key strictly conforms to pure 2-plane singularity (Law 6).
func ValidateTaskKey(key string) error {
	if !regexTaskKey.MatchString(key) {
		return types.NewNomosError(types.ErrCodeInvariant, "invalid task key %q: must match regex ^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$", key)
	}
	return nil
}

// TaskRecord defines the formal specification for Living Ledger task records (#TaskRecord).
type TaskRecord struct {
	ID            int        `json:"id,omitempty"`
	Key           string     `json:"key"`
	Plane         string     `json:"plane"`
	Project       string     `json:"project,omitempty"`
	SchemaType    SchemaType `json:"schema_type"`
	Title         string     `json:"title"`
	Description   string     `json:"description,omitempty"`
	Status        Status     `json:"status"`
	Tier          Tier       `json:"tier"`
	Priority      int        `json:"priority"`
	CreatedAt     string     `json:"created_at"`
	UpdatedAt     string     `json:"updated_at"`
	CompletedAt   *string    `json:"completed_at,omitempty"`
	Dependencies  []string   `json:"dependencies,omitempty"`
	TwinKey       string     `json:"twin_key,omitempty"`
	Assignee      string     `json:"assignee,omitempty"`
	ArcID         string     `json:"arc_id,omitempty"`
	IntentDelta   string     `json:"intent_delta,omitempty"`
	Burden        int        `json:"burden,omitempty"`
	ContextBurden int        `json:"context_burden,omitempty"`
	ArchLayer     int        `json:"arch_layer,omitempty"`
}

// ValidateTaskRecord validates relational invariants defined in task.cue (Law 7).
func ValidateTaskRecord(r *TaskRecord) error {
	if r == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task record cannot be nil")
	}
	if err := ValidateTaskKey(r.Key); err != nil {
		return err
	}
	if len(r.Title) < 5 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "title must be at least 5 characters")
	}

	// Relational plane invariants
	if strings.HasPrefix(r.Key, "INT-") && r.Plane != "intent" {
		return types.NewNomosError(types.ErrCodeInvariant, "INT-* task key must have plane 'intent', got %q", r.Plane)
	}
	if strings.HasPrefix(r.Key, "SUB-") && r.Plane != "substrate" {
		return types.NewNomosError(types.ErrCodeInvariant, "SUB-* task key must have plane 'substrate', got %q", r.Plane)
	}

	// Completion invariant
	if r.Status == StatusDone && (r.CompletedAt == nil || *r.CompletedAt == "") {
		return types.NewNomosError(types.ErrCodeInvariant, "completed task (status DONE) must have completed_at timestamp")
	}

	return nil
}

// WavefrontCandidate defines the structured candidate task emitted by nomos task dag --json (#WavefrontCandidate).
type WavefrontCandidate struct {
	Key         string     `json:"key"`
	Title       string     `json:"title"`
	Plane       string     `json:"plane"`
	Project     string     `json:"project,omitempty"`
	SchemaType  SchemaType `json:"schema_type"`
	Status      Status     `json:"status"`
	Tier        Tier       `json:"tier"`
	TwinKey     string     `json:"twin_key,omitempty"`
	IsUnblocked bool       `json:"is_unblocked"`
}

// FormatTaskKey constructs a canonical task key from prefix and integer.
func FormatTaskKey(prefix string, num int) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(prefix))
	if upper != "INT" && upper != "SUB" {
		return "", types.NewNomosError(types.ErrCodeInvariant, "invalid prefix %q: must be INT or SUB", prefix)
	}
	if num <= 0 {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "task number must be positive, got %d", num)
	}
	return fmt.Sprintf("%s-%d", upper, num), nil
}

