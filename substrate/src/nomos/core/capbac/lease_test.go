package capbac

import (
	"context"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func TestCapBACLease_Validate(t *testing.T) {
	tests := []struct {
		name    string
		lease   *CapBACLease
		wantErr bool
	}{
		{
			name: "valid lease",
			lease: &CapBACLease{
				ID:         "cap_lease_0123456789abcdef0123456789abcdef",
				TaskKey:    "INT-725",
				Phase:      types.PhasePlan,
				StepBudget: 50,
				TokensUsed: 5,
				Valid:      true,
			},
			wantErr: false,
		},
		{
			name:    "nil lease",
			lease:   nil,
			wantErr: true,
		},
		{
			name: "invalid lease id prefix",
			lease: &CapBACLease{
				ID:         "cap_tok_0123456789abcdef0123456789abcdef",
				TaskKey:    "INT-725",
				Phase:      types.PhasePlan,
				StepBudget: 50,
			},
			wantErr: true,
		},
		{
			name: "invalid task key",
			lease: &CapBACLease{
				ID:         "cap_lease_0123456789abcdef0123456789abcdef",
				TaskKey:    "TASK-725",
				Phase:      types.PhasePlan,
				StepBudget: 50,
			},
			wantErr: true,
		},
		{
			name: "step budget zero",
			lease: &CapBACLease{
				ID:         "cap_lease_0123456789abcdef0123456789abcdef",
				TaskKey:    "INT-725",
				Phase:      types.PhasePlan,
				StepBudget: 0,
			},
			wantErr: true,
		},
		{
			name: "tokens used exceeds budget",
			lease: &CapBACLease{
				ID:         "cap_lease_0123456789abcdef0123456789abcdef",
				TaskKey:    "INT-725",
				Phase:      types.PhasePlan,
				StepBudget: 50,
				TokensUsed: 55,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.lease.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemoryLeaseManager(t *testing.T) {
	ctx := context.Background()
	mgr := NewLeaseManager()

	// 1. Acquire Lease
	lease, err := mgr.AcquireLease(ctx, "SUB-1194", types.PhaseEdit, 100)
	if err != nil {
		t.Fatalf("AcquireLease failed: %v", err)
	}
	if err := lease.Validate(); err != nil {
		t.Fatalf("lease failed validation: %v", err)
	}

	// 2. Verify Lease
	valid, retrieved, err := mgr.VerifyLease(ctx, lease.ID)
	if err != nil || !valid || retrieved == nil {
		t.Errorf("expected lease %s to be valid: %v", lease.ID, err)
	}

	// 3. Verify Nonexistent Lease
	valid, _, _ = mgr.VerifyLease(ctx, "cap_lease_nonexistent00000000000000")
	if valid {
		t.Errorf("expected nonexistent lease to be invalid")
	}

	// 4. Revoke Lease
	if err := mgr.RevokeLease(ctx, lease.ID); err != nil {
		t.Fatalf("RevokeLease failed: %v", err)
	}

	valid, retrieved, _ = mgr.VerifyLease(ctx, lease.ID)
	if valid || retrieved.Valid {
		t.Errorf("expected revoked lease to be invalid")
	}

	// 5. Invalid Task Key
	_, err = mgr.AcquireLease(ctx, "INVALID-1", types.PhaseEdit, 50)
	if err == nil {
		t.Errorf("expected error acquiring lease for invalid task key")
	}
}
