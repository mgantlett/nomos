// Package types provides foundational domain types, schema bindings, and
// invariant contracts enforced by compiled Go SSoT types.
package types

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// regexNomosErrorCode validates Nomos error code format (e.g., ERR_CORE, ERR_BOUNDARY_VIOLATION).
	regexNomosErrorCode = regexp.MustCompile(`^ERR_[A-Z0-9_]+$`)
	// regexTriageHash validates optional error triage hash format (e.g., ERR-a1b2c3d4).
	regexTriageHash = regexp.MustCompile(`^ERR-[a-f0-9]+$`)
)

// Standard Nomos core error code constants adhering to NomosError invariants.
const (
	ErrCodeCore             = "ERR_CORE"
	ErrCodeIntent           = "ERR_INTENT"
	ErrCodeSubstrate        = "ERR_SUBSTRATE"
	ErrCodeBoundary         = "ERR_BOUNDARY"
	ErrCodeInvariant        = "ERR_INVARIANT"
	ErrCodeCapBAC           = "ERR_CAPBAC"
	ErrCodePhaseViolation     = "ERR_PHASE_VIOLATION"
	ErrCodeValidationFailed   = "ERR_VALIDATION_FAILED"
	ErrCodeCrossPlaneMutation = "ERR_CROSS_PLANE_MUTATION"
)

// NomosError encapsulates structured agent-focused context for execution boundary
// and invariant constraint failures.
type NomosError struct {
	Code              string   `json:"code"`
	Description       string   `json:"description"`
	Remedy            string   `json:"remedy,omitempty"`
	ExplainCommand    string   `json:"explain_command,omitempty"`
	ContextFiles      []string `json:"context_files,omitempty"`
	Fatal             bool     `json:"fatal"`
	TriageHash        string   `json:"triage_hash,omitempty"`
	InteractivePrompt string   `json:"interactive_prompt,omitempty"`
}

// Error implements the standard Go error interface.
func (e *NomosError) Error() string {
	if e == nil {
		return "<nil>"
	}
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Description)
	if e.Remedy != "" {
		msg += fmt.Sprintf(" (Remedy: %s)", e.Remedy)
	}
	if e.ExplainCommand != "" {
		msg += fmt.Sprintf(" (Inspect: %s)", e.ExplainCommand)
	}
	return msg
}

// NewNomosError instantiates a basic NomosError with code and formatted description.
func NewNomosError(code, format string, args ...any) *NomosError {
	desc := fmt.Sprintf(format, args...)
	return &NomosError{
		Code:        code,
		Description: desc,
		Fatal:       false,
	}
}

// WithRemedy attaches an actionable agent remedy to the error.
func (e *NomosError) WithRemedy(remedy string) *NomosError {
	if e != nil {
		e.Remedy = remedy
	}
	return e
}

// WithExplainCommand attaches a canonical inspection CLI command (#NomosError).
// @intent INT-842 #NomosError
func (e *NomosError) WithExplainCommand(cmd string) *NomosError {
	if e != nil {
		e.ExplainCommand = cmd
	}
	return e
}

// WithContextFiles attaches relevant contextual file paths to the error.
func (e *NomosError) WithContextFiles(files ...string) *NomosError {
	if e != nil {
		e.ContextFiles = append(e.ContextFiles, files...)
	}
	return e
}

// WithFatal marks whether the error is unrecoverable.
func (e *NomosError) WithFatal(fatal bool) *NomosError {
	if e != nil {
		e.Fatal = fatal
	}
	return e
}

// WithTriageHash attaches a triage identifier for debuggers.
func (e *NomosError) WithTriageHash(hash string) *NomosError {
	if e != nil {
		e.TriageHash = hash
	}
	return e
}

// WithInteractivePrompt attaches an interactive confirmation prompt.
func (e *NomosError) WithInteractivePrompt(prompt string) *NomosError {
	if e != nil {
		e.InteractivePrompt = prompt
	}
	return e
}

// Validate verifies that the NomosError satisfies all structural constraints.
func (e *NomosError) Validate() error {
	if e == nil {
		return fmt.Errorf("nomos error cannot be nil")
	}
	if !regexNomosErrorCode.MatchString(e.Code) {
		return fmt.Errorf("invalid error code %q: must match regex ^ERR_[A-Z0-9_]+$", e.Code)
	}
	if strings.TrimSpace(e.Description) == "" {
		return fmt.Errorf("error description cannot be empty")
	}
	if e.TriageHash != "" && !regexTriageHash.MatchString(e.TriageHash) {
		return fmt.Errorf("invalid triage hash %q: must match regex ^ERR-[a-f0-9]+$", e.TriageHash)
	}
	return nil
}
