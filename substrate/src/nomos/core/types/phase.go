// Package types provides foundational domain types, schema bindings, and
// invariant contracts enforced by compiled Go SSoT types.
package types

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Phase defines the canonical lifecycle phases of a Nomos workspace.
type Phase string

const (
	PhaseIdle   Phase = "IDLE"
	PhasePlan   Phase = "PLAN"
	PhaseEdit   Phase = "EDIT"
	PhaseReview Phase = "REVIEW"
	PhaseSync   Phase = "SYNC"
	PhaseLearn  Phase = "LEARN"
)

var (
	// regexTaskID validates canonical task key format: (INT|SUB)-<digits>[-slug].
	regexTaskID = regexp.MustCompile(`^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$`)
	// regexCapBACToken validates CapBAC capability token format: cap_<32-hex-chars>.
	regexCapBACToken = regexp.MustCompile(`^cap_[a-f0-9]{32}$`)
)

// AllPhases enumerates the 6 canonical lifecycle phases.
var AllPhases = []Phase{
	PhaseIdle,
	PhasePlan,
	PhaseEdit,
	PhaseReview,
	PhaseSync,
	PhaseLearn,
}

// allowedTransitions maps each valid source phase to its permitted target phases.
var allowedTransitions = map[Phase]map[Phase]bool{
	PhaseIdle: {
		PhasePlan: true,
		PhaseEdit: true,
	},
	PhasePlan: {
		PhaseEdit: true,
		PhaseIdle: true,
	},
	PhaseEdit: {
		PhaseReview: true,
		PhaseIdle:   true,
	},
	PhaseReview: {
		PhaseSync: true,
		PhaseEdit: true,
	},
	PhaseSync: {
		PhaseLearn: true,
		PhaseIdle:  true,
		PhaseEdit:  true,
	},
	PhaseLearn: {
		PhaseIdle: true,
	},
}

// Transition defines an edge in the deterministic phase state machine.
type Transition struct {
	From Phase `json:"from"`
	To   Phase `json:"to"`
}

// IsValidPhase checks whether the provided phase string corresponds to a canonical Phase.
func IsValidPhase(p Phase) bool {
	for _, valid := range AllPhases {
		if p == valid {
			return true
		}
	}
	return false
}

// IsValidTransition returns true if moving from state 'from' to 'to' is structurally permitted.
func IsValidTransition(from, to Phase) bool {
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

// PhaseState defines the active workspace phase state, affine CapBAC tokens,
// execution step budgets, and task bindings.
type PhaseState struct {
	Phase       Phase              `json:"phase"`
	TaskID      string             `json:"task_id,omitempty"`
	Assignee    string             `json:"assignee"`
	Token       string             `json:"token,omitempty"`
	StepBudget  int                `json:"step_budget"`
	TokensUsed  int                `json:"tokens_used"`
	NextActions []ActionAffordance `json:"next_actions,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at,omitempty"`
}

// NewDefaultPhaseState returns a quiescent IDLE phase state.
func NewDefaultPhaseState() *PhaseState {
	return &PhaseState{
		Phase:      PhaseIdle,
		Assignee:   "antigravity",
		StepBudget: 50,
		TokensUsed: 0,
		UpdatedAt:  time.Now().UTC(),
	}
}

// Validate enforces the axiomatic constraints of #PhaseState and Law 7 (Fail-Closed Pre-Flight).
func (ps *PhaseState) Validate() error {
	if ps == nil {
		return fmt.Errorf("phase state cannot be nil")
	}
	if !IsValidPhase(ps.Phase) {
		return fmt.Errorf("invalid phase %q: must be one of IDLE, PLAN, EDIT, REVIEW, SYNC, LEARN", ps.Phase)
	}
	if strings.TrimSpace(ps.Assignee) == "" {
		return fmt.Errorf("assignee cannot be empty")
	}
	if ps.StepBudget < 1 || ps.StepBudget > 500 {
		return fmt.Errorf("step budget %d out of range [1, 500]", ps.StepBudget)
	}
	if ps.TokensUsed < 0 {
		return fmt.Errorf("tokens used cannot be negative: %d", ps.TokensUsed)
	}

	// Law 7 (Fail-Closed Pre-Flight): Active non-IDLE phases require valid task_id and CapBAC token
	if ps.Phase != PhaseIdle {
		if !regexTaskID.MatchString(ps.TaskID) {
			return fmt.Errorf("active phase %q requires valid task_id matching ^(INT|SUB)-[0-9]+(-[a-z0-9-]+)?$, got %q",
				ps.Phase, ps.TaskID)
		}
		if !regexCapBACToken.MatchString(ps.Token) {
			return fmt.Errorf("active phase %q requires valid CapBAC token matching ^cap_[a-f0-9]{32}$, got %q",
				ps.Phase, ps.Token)
		}
	}
	return nil
}
