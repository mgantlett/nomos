package types

import (
	"strings"
	"testing"
)

func TestNomosErrorFormatting(t *testing.T) {
	err := NewNomosError(ErrCodeCore, "database connection failed").
		WithRemedy("check connection string").
		WithContextFiles("config/nomos.yaml").
		WithFatal(true).
		WithTriageHash("ERR-a1b2c3d4")

	if err.Error() != "[ERR_CORE] database connection failed (Remedy: check connection string)" {
		t.Errorf("unexpected error format: %s", err.Error())
	}

	if !err.Fatal {
		t.Errorf("expected fatal error")
	}

	if len(err.ContextFiles) != 1 || err.ContextFiles[0] != "config/nomos.yaml" {
		t.Errorf("unexpected context files: %v", err.ContextFiles)
	}
}

func TestNomosErrorValidation(t *testing.T) {
	tests := []struct {
		name      string
		err       *NomosError
		expectErr bool
	}{
		{
			name: "valid minimal error",
			err: &NomosError{
				Code:        "ERR_CORE",
				Description: "something failed",
			},
			expectErr: false,
		},
		{
			name: "valid error with valid triage hash",
			err: &NomosError{
				Code:        "ERR_INVARIANT_FAILED",
				Description: "invariant 4 failed",
				TriageHash:  "ERR-1234abcd",
			},
			expectErr: false,
		},
		{
			name: "invalid error code lowercase",
			err: &NomosError{
				Code:        "err_core",
				Description: "failed",
			},
			expectErr: true,
		},
		{
			name: "empty description",
			err: &NomosError{
				Code:        "ERR_EMPTY_DESC",
				Description: "   ",
			},
			expectErr: true,
		},
		{
			name: "invalid triage hash prefix",
			err: &NomosError{
				Code:        "ERR_TEST",
				Description: "test error",
				TriageHash:  "HASH-1234",
			},
			expectErr: true,
		},
		{
			name:      "nil error validation",
			err:       nil,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("expected validation error but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("expected clean validation but got: %v", err)
			}
		})
	}
}

func TestNilNomosErrorFormatting(t *testing.T) {
	var err *NomosError
	if err.Error() != "<nil>" {
		t.Errorf("expected <nil> for nil error representation")
	}
	var nilErr *NomosError
	nilErr = nilErr.WithRemedy("remedy")
	if nilErr != nil {
		t.Errorf("expected nil result from nil receiver")
	}
}

func TestNomosErrorInteractivePrompt(t *testing.T) {
	err := NewNomosError(ErrCodeBoundary, "boundary breached").
		WithRemedy("revert trunk edit").
		WithExplainCommand("nomos capabilities plane_separation").
		WithInteractivePrompt("Proceed anyway? (y/n)")

	if !strings.Contains(err.InteractivePrompt, "Proceed") {
		t.Errorf("expected interactive prompt to be set")
	}
	if !strings.Contains(err.Error(), "Inspect: nomos capabilities plane_separation") {
		t.Errorf("expected explain command in formatted error, got: %s", err.Error())
	}
}
