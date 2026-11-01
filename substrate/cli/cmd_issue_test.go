package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "redacts github token",
			input:    "failed with token ghp_123456789012345678901234567890 in log",
			expected: "failed with token [REDACTED_GH_TOKEN] in log",
		},
		{
			name:     "redacts api key",
			input:    "key sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456789",
			expected: "key [REDACTED_API_KEY]",
		},
		{
			name:     "redacts bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "redacts url credentials",
			input:    "clone https://user:secretpass123@github.com/repo.git",
			expected: "clone https://user:[REDACTED]@github.com/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeText(tt.input)
			if !strings.Contains(got, tt.expected) {
				t.Errorf("sanitizeText(%q) = %q, expected containing %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatIssueMarkdown(t *testing.T) {
	report := &IssueReport{
		Title:          "Crash in AST Gate",
		Description:    "Unexpected panic during AST parse",
		NomosVersion:   "4.0.0",
		GoVersion:      "go1.24.0",
		Platform:       "linux/amd64",
		GitVersion:     "git version 2.43.0",
		ActivePhase:    "EDIT",
		ActiveBranch:   "feature/SUB-1283",
		IsWorktree:     true,
		DatabaseStatus: "CONNECTED",
	}

	md := formatIssueMarkdown(report)
	if !strings.Contains(md, "Crash in AST Gate") && !strings.Contains(md, "Unexpected panic") {
		t.Errorf("markdown missing description")
	}
	if !strings.Contains(md, "Nomos Version:** `4.0.0`") {
		t.Errorf("markdown missing nomos version")
	}
	if !strings.Contains(md, "linux/amd64") {
		t.Errorf("markdown missing platform")
	}
}

func TestIssueCmdDryRun(t *testing.T) {
	cmd := NewIssueCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Test CLI crash", "--dry-run"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error running nomos issue dry-run: %v", err)
	}
}
