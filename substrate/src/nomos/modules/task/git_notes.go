// Package task defines domain models, schema constraints, and tracking mechanisms
// enforced by compiled Go SSoT contracts.
package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

var (
	regexSHA1   = regexp.MustCompile(`^[a-f0-9]{40}$`)
	regexSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// ReleaseHookTrace summarizes verified worktree tool and shell activity sealed at 2PC release (#ReleaseHookTrace).
type ReleaseHookTrace struct {
	ToolCallsCount      int      `json:"tool_calls_count"`
	ShellCommandsCount  int      `json:"shell_commands_count"`
	TestCommandsCount   int      `json:"test_commands_count"`
	DeniedInterceptions int      `json:"denied_interceptions"`
	MutatedFiles        []string `json:"mutated_files,omitempty"`
}

// ReleaseNotePayload encapsulates the non-repudiation release receipt in Git Notes (#ReleaseNotePayload).
type ReleaseNotePayload struct {
	TaskKey            string            `json:"task_key"`
	CommitSHA          string            `json:"commit_sha"`
	Timestamp          string            `json:"timestamp"`
	Phase              string            `json:"phase"`
	DoDGatesPassed     int               `json:"dod_gates_passed"`
	CapBACTokenID      string            `json:"capbac_token_id"`
	SessionID          string            `json:"session_id,omitempty"`
	HookTrace          *ReleaseHookTrace `json:"hook_trace,omitempty"`
	WalkthroughContent string            `json:"walkthrough_content"`
	SHA256Receipt      string            `json:"sha256_receipt"`
}

// ValidateReleaseNote asserts that a ReleaseNotePayload satisfies structural invariants.
func ValidateReleaseNote(p ReleaseNotePayload) error {
	if err := ValidateTaskKey(p.TaskKey); err != nil {
		return err
	}
	if !regexSHA1.MatchString(p.CommitSHA) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid commit SHA %q: must be 40-char hex", p.CommitSHA)
	}
	if p.Phase != "SYNC" {
		return types.NewNomosError(types.ErrCodeInvariant, "phase must be 'SYNC', got %q", p.Phase)
	}
	if p.DoDGatesPassed < 42 {
		return types.NewNomosError(types.ErrCodeInvariant, "must pass at least 42 DoD gates, got %d", p.DoDGatesPassed)
	}
	if !regexSHA256.MatchString(p.SHA256Receipt) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid SHA256 receipt: must be 64-char hex")
	}
	return nil
}

// ComputeReleaseReceipt computes the cryptographic SHA256 receipt for a release payload.
func ComputeReleaseReceipt(taskKey, commitSHA, timestamp, tokenID string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s", taskKey, commitSHA, timestamp, tokenID)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// RecordReleaseNote writes the release certificate to Git Notes under refs/notes/nomos/releases.
func RecordReleaseNote(repoRoot, ref string, payload ReleaseNotePayload) error {
	if ref == "" {
		ref = "refs/notes/nomos/releases"
	}
	if err := ValidateReleaseNote(payload); err != nil {
		return err
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return types.NewNomosError(types.ErrCodeCore, "failed marshaling release note: %v", err)
	}

	cmd := workspace.SafeGitCommand(repoRoot, "notes", "--ref", ref, "add", "-f", "-m", string(bytes), payload.CommitSHA)
	if out, err := cmd.CombinedOutput(); err != nil {
		return types.NewNomosError(types.ErrCodeCore, "failed recording git note: %s (%v)", string(out), err)
	}
	return nil
}

