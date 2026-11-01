package session

import (
	"path/filepath"
	"testing"
)

func TestEvaluateSentinelCleanSession(t *testing.T) {
	tempDir := t.TempDir()
	cargoDir := filepath.Join(tempDir, "sophia-cargo")
	wtDir := filepath.Join(cargoDir, "worktrees", "cargo-SUB-1088")

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:           "test-clean-sess",
			WorktreePath: "worktrees/cargo-SUB-1088",
			Status:       "ACTIVE",
		},
	}
	lines := []string{
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"\"nomos verify -v\"","Cwd":"\"` + wtDir + `\""}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"\"wc -l client/public/js/*.js\"","Cwd":"\"` + wtDir + `\""}}]}`,
	}

	sc := EvaluateSentinel(detail, lines, cargoDir)
	if sc.IntegrityStatus != "CLEAN" {
		t.Errorf("expected CLEAN status, got %s", sc.IntegrityStatus)
	}
	if sc.OverallScore != 100 {
		t.Errorf("expected score 100, got %d", sc.OverallScore)
	}
	if len(sc.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(sc.Violations))
	}
	if len(sc.Invariants) != 5 {
		t.Errorf("expected 5 invariants evaluated, got %d", len(sc.Invariants))
	}
}

func TestEvaluateSentinelViolations(t *testing.T) {
	tempDir := t.TempDir()
	cargoDir := filepath.Join(tempDir, "sophia-cargo")

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:           "test-dirty-sess",
			WorktreePath: "",
			Status:       "COMPLETED",
		},
	}
	lines := []string{
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"\"sqlite3 .nomos/nomos_global.db 'SELECT * FROM tasks'\"","Cwd":"\"` + cargoDir + `\""}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"\"git commit -m 'direct mutation on trunk'\"","Cwd":"\"` + cargoDir + `\""}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"\"ls /workspace/nomos/substrate\"","Cwd":"\"` + cargoDir + `\""}}]}`,
	}

	sc := EvaluateSentinel(detail, lines, cargoDir)
	if sc.IntegrityStatus != "VIOLATION" {
		t.Errorf("expected VIOLATION status, got %s", sc.IntegrityStatus)
	}
	if sc.OverallScore >= 100 {
		t.Errorf("expected reduced score, got %d", sc.OverallScore)
	}
	if len(sc.Violations) < 2 {
		t.Errorf("expected multiple violations, got %d", len(sc.Violations))
	}
}
