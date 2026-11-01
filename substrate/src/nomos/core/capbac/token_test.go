package capbac

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func TestCapBACToken_Validate(t *testing.T) {
	tests := []struct {
		name    string
		token   *CapBACToken
		wantErr bool
	}{
		{
			name: "valid token",
			token: &CapBACToken{
				ID:         "cap_0123456789abcdef0123456789abcdef",
				Phase:      types.PhaseEdit,
				TaskID:     "SUB-1194",
				StepBudget: 50,
				TokensUsed: 10,
			},
			wantErr: false,
		},
		{
			name:    "nil token",
			token:   nil,
			wantErr: true,
		},
		{
			name: "invalid token id prefix",
			token: &CapBACToken{
				ID:         "tok_0123456789abcdef0123456789abcdef",
				Phase:      types.PhaseEdit,
				TaskID:     "SUB-1194",
				StepBudget: 50,
			},
			wantErr: true,
		},
		{
			name: "invalid phase",
			token: &CapBACToken{
				ID:         "cap_0123456789abcdef0123456789abcdef",
				Phase:      types.Phase("UNKNOWN"),
				TaskID:     "SUB-1194",
				StepBudget: 50,
			},
			wantErr: true,
		},
		{
			name: "invalid task id",
			token: &CapBACToken{
				ID:         "cap_0123456789abcdef0123456789abcdef",
				Phase:      types.PhaseEdit,
				TaskID:     "TASK-1",
				StepBudget: 50,
			},
			wantErr: true,
		},
		{
			name: "zero step budget",
			token: &CapBACToken{
				ID:         "cap_0123456789abcdef0123456789abcdef",
				Phase:      types.PhaseEdit,
				TaskID:     "SUB-1194",
				StepBudget: 0,
			},
			wantErr: true,
		},
		{
			name: "tokens used exceeds step budget",
			token: &CapBACToken{
				ID:         "cap_0123456789abcdef0123456789abcdef",
				Phase:      types.PhaseEdit,
				TaskID:     "SUB-1194",
				StepBudget: 50,
				TokensUsed: 51,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.token.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMintToken(t *testing.T) {
	tok, err := MintToken(types.PhaseEdit, "SUB-1194", "/tmp/wt", 100, time.Hour)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}
	if err := tok.Validate(); err != nil {
		t.Fatalf("minted token failed validation: %v", err)
	}

	// Invalid phase
	_, err = MintToken(types.Phase("INVALID"), "SUB-1194", "/tmp/wt", 100, time.Hour)
	if err == nil {
		t.Errorf("expected error minting token with invalid phase")
	}

	// Invalid task ID
	_, err = MintToken(types.PhaseEdit, "FEAT-100", "/tmp/wt", 100, time.Hour)
	if err == nil {
		t.Errorf("expected error minting token with invalid taskID")
	}
}

func TestCapBACToken_ValidateWrite(t *testing.T) {
	wtDir := "/workspace/worktrees/nomos-SUB-1"

	tok, err := MintToken(types.PhaseEdit, "SUB-1", wtDir, 50, time.Hour)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}

	// 1. Target inside worktree succeeds
	validTarget := filepath.Join(wtDir, "pkg/main.go")
	if err := tok.ValidateWrite(validTarget); err != nil {
		t.Errorf("expected write to succeed inside worktree: %v", err)
	}

	// 2. Jail escape fails
	escapedTarget := "/workspace/substrate/main.go"
	if err := tok.ValidateWrite(escapedTarget); err == nil {
		t.Errorf("expected jail escape error, got nil")
	}

	// 3. Phase discipline lock: not in EDIT phase
	planTok, _ := MintToken(types.PhasePlan, "SUB-1", wtDir, 50, time.Hour)
	if err := planTok.ValidateWrite(validTarget); err == nil {
		t.Errorf("expected phase discipline lock error during PLAN phase, got nil")
	}

	// 4. Expired token fails
	expiredTok, _ := MintToken(types.PhaseEdit, "SUB-1", wtDir, 50, time.Hour)
	expiredTok.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	if err := expiredTok.ValidateWrite(validTarget); err == nil {
		t.Errorf("expected expired token error, got nil")
	}


	// 5. Nil token fails
	var nilTok *CapBACToken
	if err := nilTok.ValidateWrite(validTarget); err == nil {
		t.Errorf("expected nil token error, got nil")
	}
}

func TestCapBACToken_Step(t *testing.T) {
	tok, err := MintToken(types.PhaseEdit, "SUB-1", "/tmp/wt", 2, time.Hour)
	if err != nil {
		t.Fatalf("MintToken failed: %v", err)
	}

	if err := tok.Step(); err != nil {
		t.Errorf("step 1 failed: %v", err)
	}
	if err := tok.Step(); err != nil {
		t.Errorf("step 2 failed: %v", err)
	}
	// Step 3 exceeds budget of 2
	if err := tok.Step(); err == nil {
		t.Errorf("expected budget exhaustion error on step 3, got nil")
	}
}
