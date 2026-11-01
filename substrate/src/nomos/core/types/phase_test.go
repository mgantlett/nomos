package types

import (
	"testing"
	"time"
)

func TestPhaseValidation(t *testing.T) {
	for _, p := range AllPhases {
		if !IsValidPhase(p) {
			t.Errorf("expected %s to be recognized as valid phase", p)
		}
	}
	if IsValidPhase("UNKNOWN") {
		t.Errorf("expected UNKNOWN to be recognized as invalid phase")
	}
}

func TestPhaseTransitions(t *testing.T) {
	tests := []struct {
		from     Phase
		to       Phase
		expected bool
	}{
		{PhaseIdle, PhasePlan, true},
		{PhaseIdle, PhaseEdit, true},
		{PhaseIdle, PhaseReview, false},
		{PhaseIdle, PhaseSync, false},
		{PhasePlan, PhaseEdit, true},
		{PhasePlan, PhaseIdle, true},
		{PhaseEdit, PhaseReview, true},
		{PhaseEdit, PhaseIdle, true},
		{PhaseEdit, PhaseSync, false},
		{PhaseReview, PhaseSync, true},
		{PhaseReview, PhaseEdit, true},
		{PhaseSync, PhaseLearn, true},
		{PhaseSync, PhaseIdle, true},
		{PhaseSync, PhaseEdit, true},
		{PhaseLearn, PhaseIdle, true},
		{PhaseLearn, PhasePlan, false},
	}

	for _, tc := range tests {
		result := IsValidTransition(tc.from, tc.to)
		if result != tc.expected {
			t.Errorf("transition %s -> %s expected %v, got %v", tc.from, tc.to, tc.expected, result)
		}
	}
}

func TestPhaseStateValidation(t *testing.T) {
	validToken := "cap_1234567890abcdef1234567890abcdef"

	tests := []struct {
		name      string
		state     *PhaseState
		expectErr bool
	}{
		{
			name:      "default idle phase state",
			state:     NewDefaultPhaseState(),
			expectErr: false,
		},
		{
			name: "valid active edit phase",
			state: &PhaseState{
				Phase:      PhaseEdit,
				TaskID:     "SUB-1",
				Assignee:   "agent",
				Token:      validToken,
				StepBudget: 50,
				TokensUsed: 1,
				UpdatedAt:  time.Now(),
			},
			expectErr: false,
		},
		{
			name: "active edit phase missing token",
			state: &PhaseState{
				Phase:      PhaseEdit,
				TaskID:     "SUB-1",
				Assignee:   "agent",
				StepBudget: 50,
			},
			expectErr: true,
		},
		{
			name: "active edit phase invalid task key",
			state: &PhaseState{
				Phase:      PhaseEdit,
				TaskID:     "TASK-1",
				Assignee:   "agent",
				Token:      validToken,
				StepBudget: 50,
			},
			expectErr: true,
		},
		{
			name: "invalid step budget over limit",
			state: &PhaseState{
				Phase:      PhaseIdle,
				Assignee:   "agent",
				StepBudget: 501,
			},
			expectErr: true,
		},
		{
			name: "negative tokens used",
			state: &PhaseState{
				Phase:      PhaseIdle,
				Assignee:   "agent",
				StepBudget: 50,
				TokensUsed: -1,
			},
			expectErr: true,
		},
		{
			name:      "nil phase state",
			state:     nil,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.state.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected clean validation, got: %v", err)
			}
		})
	}
}
