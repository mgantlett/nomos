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

func TestEvaluateSentinel_HermeticWorktreeShellSegmentsAndQuotedGrep(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "nomos")

	cleanCases := []struct {
		name string
		line string
	}{
		{
			name: "chained_cd_into_worktree_before_git_commit",
			line: `{"tool_calls":[{"name":"run_command","args":{"CommandLine":"cd /home/dev/nomos/worktrees/nomos-INT-930 && git add . && git commit -m \"docs: update spec\"","Cwd":""}}]}`,
		},
		{
			name: "git_dash_C_worktree_commit",
			line: `{"tool_calls":[{"name":"run_command","args":{"CommandLine":"git -C worktrees/nomos-SUB-1806 commit -m \"feat: add guard\"","Cwd":"` + repoDir + `"}}]}`,
		},
		{
			name: "quoted_grep_searching_for_git_commit",
			line: `{"tool_calls":[{"name":"run_command","args":{"CommandLine":"grep \"git commit\" transcript_full.jsonl | head -n 5","Cwd":""}}]}`,
		},
		{
			name: "python_inline_mentioning_git_commit",
			line: `{"tool_calls":[{"name":"run_command","args":{"CommandLine":"python3 -c 'print(\"git commit\")'","Cwd":""}}]}`,
		},
	}

	for _, tc := range cleanCases {
		t.Run(tc.name, func(t *testing.T) {
			detail := &SessionDetail{AgentSession: AgentSession{ID: "sess-" + tc.name, Status: "ACTIVE"}}
			sc := EvaluateSentinel(detail, []string{tc.line}, repoDir)
			if sc.OverallScore != 100 || sc.IntegrityStatus != "CLEAN" {
				t.Fatalf("%s: expected CLEAN (100), got %s (%d): %v", tc.name, sc.IntegrityStatus, sc.OverallScore, sc.Violations)
			}
		})
	}

	trunkLine := `{"tool_calls":[{"name":"run_command","args":{"CommandLine":"git add README.md && git commit -m \"direct trunk commit\"","Cwd":""}}]}`
	scTrunk := EvaluateSentinel(&SessionDetail{AgentSession: AgentSession{ID: "sess-trunk", Status: "ACTIVE"}}, []string{trunkLine}, repoDir)
	if scTrunk.OverallScore != 80 || scTrunk.IntegrityStatus != "SUSPICIOUS" {
		t.Fatalf("expected SUSPICIOUS (80) on unquoted trunk git commit, got %s (%d)", scTrunk.IntegrityStatus, scTrunk.OverallScore)
	}
}
