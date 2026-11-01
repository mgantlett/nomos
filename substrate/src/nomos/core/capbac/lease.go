// Package capbac implements Capability-Based Access Control (CapBAC) and authorization contracts.
package capbac

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

var (
	// regexLeaseID validates lease ID format: ^cap_lease_[a-f0-9]{32}$.
	regexLeaseID = regexp.MustCompile(`^cap_lease_[a-f0-9]{32}$`)
	// regexLeaseTaskKey validates task key: ^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$.
	regexLeaseTaskKey = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)
)

// CapBACLease binds the CUE #CapBACLease schema contract.
type CapBACLease struct {
	ID         string      `json:"id"`
	TaskKey    string      `json:"task_key"`
	Phase      types.Phase `json:"phase"`
	StepBudget int         `json:"step_budget"`
	TokensUsed int         `json:"tokens_used"`
	IssuedAt   string      `json:"issued_at"`
	Valid      bool        `json:"valid"`
}

// Validate ensures CapBACLease satisfies all CUE schema constraints.
func (l *CapBACLease) Validate() error {
	if l == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "capbac lease cannot be nil")
	}
	if !regexLeaseID.MatchString(l.ID) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid lease id %q: must match ^cap_lease_[a-f0-9]{32}$", l.ID)
	}
	if !regexLeaseTaskKey.MatchString(l.TaskKey) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid task_key %q: must match ^(INT|SUB)-[0-9]+", l.TaskKey)
	}
	if !types.IsValidPhase(l.Phase) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid phase %q", l.Phase)
	}
	if l.StepBudget < 1 || l.StepBudget > 500 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "step_budget %d out of range [1, 500]", l.StepBudget)
	}
	if l.TokensUsed < 0 || l.TokensUsed > l.StepBudget {
		return types.NewNomosError(types.ErrCodeValidationFailed, "tokens_used %d out of range [0, %d]", l.TokensUsed, l.StepBudget)
	}
	return nil
}

// LeaseManager defines the interface for leasing workspace capabilities.
type LeaseManager interface {
	AcquireLease(ctx context.Context, taskKey string, phase types.Phase, stepBudget int) (*CapBACLease, error)
	VerifyLease(ctx context.Context, leaseID string) (bool, *CapBACLease, error)
	RevokeLease(ctx context.Context, leaseID string) error
}

// MemoryLeaseManager provides an in-memory, thread-safe implementation of LeaseManager.
type MemoryLeaseManager struct {
	mu     sync.RWMutex
	leases map[string]*CapBACLease
}

// NewLeaseManager provisions a thread-safe in-memory LeaseManager.
func NewLeaseManager() LeaseManager {
	return &MemoryLeaseManager{
		leases: make(map[string]*CapBACLease),
	}
}

// AcquireLease provisions a new CapBACLease adhering to CUE constraints.
func (m *MemoryLeaseManager) AcquireLease(ctx context.Context, taskKey string, phase types.Phase, stepBudget int) (*CapBACLease, error) {
	cleanKey := strings.ToUpper(strings.TrimSpace(taskKey))
	if !regexLeaseTaskKey.MatchString(cleanKey) {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "invalid task key %q for lease acquisition", taskKey)
	}
	if !types.IsValidPhase(phase) {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "invalid phase %q for lease acquisition", phase)
	}

	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, types.NewNomosError(types.ErrCodeCapBAC, "failed generating lease ID: %v", err)
	}

	if stepBudget < 1 {
		stepBudget = 50
	} else if stepBudget > 500 {
		stepBudget = 500
	}

	lease := &CapBACLease{
		ID:         fmt.Sprintf("cap_lease_%s", hex.EncodeToString(randomBytes)),
		TaskKey:    cleanKey,
		Phase:      phase,
		StepBudget: stepBudget,
		TokensUsed: 0,
		IssuedAt:   time.Now().UTC().Format(time.RFC3339),
		Valid:      true,
	}

	m.mu.Lock()
	m.leases[lease.ID] = lease
	m.mu.Unlock()

	return lease, nil
}

// VerifyLease checks whether the provided leaseID is active, valid, and unexhausted.
func (m *MemoryLeaseManager) VerifyLease(ctx context.Context, leaseID string) (bool, *CapBACLease, error) {
	m.mu.RLock()
	lease, exists := m.leases[leaseID]
	m.mu.RUnlock()

	if !exists || lease == nil {
		return false, nil, nil
	}
	if !lease.Valid || lease.TokensUsed > lease.StepBudget {
		return false, lease, nil
	}
	return true, lease, nil
}

// RevokeLease invalidates an active lease.
func (m *MemoryLeaseManager) RevokeLease(ctx context.Context, leaseID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lease, exists := m.leases[leaseID]
	if !exists || lease == nil {
		return types.NewNomosError(types.ErrCodeCapBAC, "lease %q not found for revocation", leaseID)
	}
	lease.Valid = false
	return nil
}
