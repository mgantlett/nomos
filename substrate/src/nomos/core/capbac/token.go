// Package capbac implements Capability-Based Access Control (CapBAC) and authorization contracts.
package capbac

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

var (
	// regexTokenID validates token ID format: ^(cap_[a-f0-9]{32}|cap_lease_[a-f0-9]+)$.
	regexTokenID = regexp.MustCompile(`^(cap_[a-f0-9]{32}|cap_lease_[a-f0-9]+)$`)
	// regexTokenTaskID validates task key format: ^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$.
	regexTokenTaskID = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)
)

// CapBACToken models a cryptographic capability token contract.
type CapBACToken struct {
	ID           string      `json:"id"`
	Phase        types.Phase `json:"phase"`
	TaskID       string      `json:"task_id"`
	StepBudget   int         `json:"step_budget"`
	TokensUsed   int         `json:"tokens_used"`
	CreatedAt    string      `json:"created_at,omitempty"`
	WorktreePath string      `json:"worktree_path,omitempty"`
	ExpiresAt    time.Time   `json:"expires_at,omitempty"`
}

// Validate ensures CapBACToken satisfies all structural constraints and Law 7 bounds.
func (t *CapBACToken) Validate() error {
	if t == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "capbac token cannot be nil")
	}
	if !regexTokenID.MatchString(t.ID) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid token id %q: must match ^(cap_[a-f0-9]{32}|cap_lease_[a-f0-9]+)$", t.ID)
	}
	if !types.IsValidPhase(t.Phase) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid phase %q: must be one of IDLE, PLAN, EDIT, REVIEW, SYNC, LEARN", t.Phase)
	}
	if !regexTokenTaskID.MatchString(t.TaskID) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid task_id %q: must match ^(INT|SUB)-[0-9]+", t.TaskID)
	}
	if t.StepBudget < 1 || t.StepBudget > 500 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "step_budget %d out of range [1, 500]", t.StepBudget)
	}
	if t.TokensUsed < 0 || t.TokensUsed > t.StepBudget {
		return types.NewNomosError(types.ErrCodeValidationFailed, "tokens_used %d out of range [0, %d]", t.TokensUsed, t.StepBudget)
	}
	return nil
}

// MintToken creates a cryptographically bounded capability token.
func MintToken(phase types.Phase, taskID, worktree string, stepBudget int, ttl time.Duration) (*CapBACToken, error) {
	if !types.IsValidPhase(phase) {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "cannot mint token with invalid phase %q", phase)
	}

	cleanTask := strings.ToUpper(strings.TrimSpace(taskID))
	if !regexTokenTaskID.MatchString(cleanTask) {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "cannot mint token with invalid taskID %q", taskID)
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "failed generating capability token ID: %v", err)
	}

	if stepBudget < 1 {
		stepBudget = 50
	} else if stepBudget > 500 {
		stepBudget = 500
	}

	if ttl <= 0 {
		ttl = 2 * time.Hour
	}

	now := time.Now().UTC()
	token := &CapBACToken{
		ID:           fmt.Sprintf("cap_%s", hex.EncodeToString(randomBytes)),
		Phase:        phase,
		TaskID:       cleanTask,
		StepBudget:   stepBudget,
		TokensUsed:   0,
		CreatedAt:    now.Format(time.RFC3339),
		WorktreePath: filepath.Clean(worktree),
		ExpiresAt:    now.Add(ttl),
	}

	return token, nil
}

// ValidateWrite checks whether the capability token authorizes a file mutation at targetPath.
func (t *CapBACToken) ValidateWrite(targetPath string) error {
	if t == nil {
		return types.NewNomosError(types.ErrCodeCapBAC, "capbac violation: no capability token presented (workspace immutable)")
	}

	if !t.ExpiresAt.IsZero() && time.Now().UTC().After(t.ExpiresAt) {
		return types.NewNomosError(types.ErrCodeCapBAC, "capbac violation: capability token %s expired at %s", t.ID, t.ExpiresAt.Format(time.RFC3339))
	}

	if t.Phase != types.PhaseEdit {
		return types.NewNomosError(types.ErrCodePhaseViolation, "capbac violation: phase discipline lock active (%s); mutation authorized only in EDIT", t.Phase)
	}

	if t.WorktreePath == "" {
		return types.NewNomosError(types.ErrCodeBoundary, "capbac violation: capability token missing worktree path binding")
	}

	cleanTarget := filepath.Clean(targetPath)
	cleanWorktree := filepath.Clean(t.WorktreePath)

	rel, err := filepath.Rel(cleanWorktree, cleanTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return types.NewNomosError(types.ErrCodeBoundary, "capbac violation: target file %s escapes authorized worktree jail %s", cleanTarget, cleanWorktree)
	}

	return nil
}

// Step increments tokens used, enforcing the step budget invariant.
func (t *CapBACToken) Step() error {
	if t == nil {
		return types.NewNomosError(types.ErrCodeCapBAC, "capbac step violation: nil token")
	}
	if t.TokensUsed >= t.StepBudget {
		return types.NewNomosError(types.ErrCodeCapBAC, "capbac quota violation: step budget exhausted (%d/%d steps)", t.TokensUsed, t.StepBudget)
	}
	t.TokensUsed++
	return nil
}
