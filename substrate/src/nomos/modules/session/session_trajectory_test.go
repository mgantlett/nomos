package session

import (
	"testing"
)

func TestInferTrajectory(t *testing.T) {
	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:      "test-traj-sess",
			Status:  "ACTIVE",
			TaskKey: "SUB-1088",
		},
	}
	lines := []string{
		`{"content":"/Nomos Handshake"}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"nomos task start SUB-1088"}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"nomos verify -v"}}]}`,
		`{"tool_calls":[{"name":"grep_search","args":{"Query":"danger-strain"}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"./scripts/test_headless.sh"}}]}`,
		`{"tool_calls":[{"name":"replace_file_content","args":{"TargetFile":"client/public/js/render.js"}}]}`,
	}

	traj := InferTrajectory(detail, lines)
	if traj == nil {
		t.Fatalf("expected non-nil trajectory")
	}
	if traj.TotalCount != 7 {
		t.Errorf("expected 7 stages, got %d", traj.TotalCount)
	}
	if traj.CompletedCount < 5 {
		t.Errorf("expected at least 5 completed stages, got %d", traj.CompletedCount)
	}

	foundOracle := false
	foundImpl := false
	for _, s := range traj.Stages {
		if s.ID == "ORACLE" && s.Status == "COMPLETED" {
			foundOracle = true
		}
		if s.ID == "IMPLEMENTATION" && s.Status == "COMPLETED" {
			foundImpl = true
		}
	}
	if !foundOracle {
		t.Errorf("expected ORACLE stage to be completed")
	}
	if !foundImpl {
		t.Errorf("expected IMPLEMENTATION stage to be completed")
	}
}
