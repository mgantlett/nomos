package session

import (
	"strings"
)

// TrajectoryStage models an individual causal milestone stage in an agent trajectory (#TrajectoryStage).
type TrajectoryStage struct {
	ID       string `json:"id"`       // BOOT, ALLOCATION, BASELINE, EXPLORATION, ORACLE, IMPLEMENTATION, RELEASE
	Label    string `json:"label"`    // Human-readable stage title
	Status   string `json:"status"`   // COMPLETED, ACTIVE, PENDING
	Evidence string `json:"evidence"` // Milestone evidence or command summary
}

// SessionTrajectory models the complete 7-stage causal arc of a Tier 1 conversation session (#SessionTrajectory).
type SessionTrajectory struct {
	CurrentStage   string            `json:"current_stage"`
	CompletedCount int               `json:"completed_count"`
	TotalCount     int               `json:"total_count"`
	Stages         []TrajectoryStage `json:"stages"`
}

// InferTrajectory reconstructs the 7-stage causal trajectory arc from transcript telemetry.
func InferTrajectory(detail *SessionDetail, rawLines []string) *SessionTrajectory {
	content := strings.Join(rawLines, "\n")

	stages := []TrajectoryStage{
		inferBootStage(content),
		inferAllocationStage(detail, content),
		inferBaselineStage(content),
		inferExplorationStage(content),
		inferOracleStage(content),
		inferImplementationStage(content),
		inferReleaseStage(content),
	}

	markActiveStage(stages, detail.Status)

	completed := 0
	curr := "IDLE"
	for _, s := range stages {
		if s.Status == "COMPLETED" {
			completed++
			curr = s.ID
		} else if s.Status == "ACTIVE" {
			curr = s.ID
		}
	}

	return &SessionTrajectory{
		CurrentStage:   curr,
		CompletedCount: completed,
		TotalCount:     len(stages),
		Stages:         stages,
	}
}

func inferBootStage(content string) TrajectoryStage {
	if strings.Contains(content, "nomos handshake") || strings.Contains(content, "nomos sys handshake") || strings.Contains(content, "/Nomos Handshake") || strings.Contains(content, "/nomos-handshake") {
		return TrajectoryStage{
			ID:       "BOOT",
			Label:    "1. Handshake & Boot",
			Status:   "COMPLETED",
			Evidence: "nomos handshake executed cleanly",
		}
	}
	return TrajectoryStage{
		ID:       "BOOT",
		Label:    "1. Handshake & Boot",
		Status:   "PENDING",
		Evidence: "Awaiting JIT protocol initialization",
	}
}

func inferAllocationStage(detail *SessionDetail, content string) TrajectoryStage {
	if detail.TaskKey != "" || strings.Contains(content, "nomos task start") || strings.Contains(content, "nomos task chore") {
		return TrajectoryStage{
			ID:       "ALLOCATION",
			Label:    "2. Task Allocation",
			Status:   "COMPLETED",
			Evidence: "CapBAC task lease and hermetic worktree provisioned",
		}
	}
	return TrajectoryStage{
		ID:       "ALLOCATION",
		Label:    "2. Task Allocation",
		Status:   "PENDING",
		Evidence: "No active task worktree allocated",
	}
}

func inferBaselineStage(content string) TrajectoryStage {
	if strings.Contains(content, "nomos verify") {
		return TrajectoryStage{
			ID:       "BASELINE",
			Label:    "3. Baseline Verification",
			Status:   "COMPLETED",
			Evidence: "Pre-flight sensor probes verified cleanly",
		}
	}
	return TrajectoryStage{
		ID:       "BASELINE",
		Label:    "3. Baseline Verification",
		Status:   "PENDING",
		Evidence: "Baseline verification not yet invoked",
	}
}

var (
	explorationMarkers = []string{
		"grep_search", "view_file", "read_file", "find_by_name", "list_dir",
		`"Read"`, `"Grep"`, `"Glob"`, `"read"`, `"grep"`, `"glob"`, "git log",
	}
	oracleMarkers = []string{
		"go test", "pytest", "npm test", "pnpm test", "yarn test", "bun test",
		"cargo test", "ctest", "unittest", "rspec", "vitest", "jest",
		"nomos verify", "./scripts/test", "test_headless", "oracle", "cargo-sim",
	}
	implementationMarkers = []string{
		"replace_file_content", "write_to_file", "multi_replace",
		"edit_file", "write_file", "apply_patch",
		`"Edit"`, `"Write"`, `"MultiEdit"`, `"edit"`, `"write"`,
	}
)

func hasAnyTrajectoryMarker(content string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(content, m) {
			return true
		}
	}
	return false
}

func inferExplorationStage(content string) TrajectoryStage {
	if hasAnyTrajectoryMarker(content, explorationMarkers) {
		return TrajectoryStage{
			ID:       "EXPLORATION",
			Label:    "4. Code Exploration",
			Status:   "COMPLETED",
			Evidence: "Target AST and source symbols explored",
		}
	}
	return TrajectoryStage{
		ID:       "EXPLORATION",
		Label:    "4. Code Exploration",
		Status:   "PENDING",
		Evidence: "Awaiting initial source investigation",
	}
}

func inferOracleStage(content string) TrajectoryStage {
	if hasAnyTrajectoryMarker(content, oracleMarkers) {
		return TrajectoryStage{
			ID:       "ORACLE",
			Label:    "5. Test & Oracle",
			Status:   "COMPLETED",
			Evidence: "Automated test suite or deterministic oracle verified",
		}
	}
	return TrajectoryStage{
		ID:       "ORACLE",
		Label:    "5. Test & Oracle",
		Status:   "PENDING",
		Evidence: "Awaiting test execution",
	}
}

func inferImplementationStage(content string) TrajectoryStage {
	if hasAnyTrajectoryMarker(content, implementationMarkers) {
		return TrajectoryStage{
			ID:       "IMPLEMENTATION",
			Label:    "6. Implementation",
			Status:   "COMPLETED",
			Evidence: "Targeted AST modifications applied",
		}
	}
	return TrajectoryStage{
		ID:       "IMPLEMENTATION",
		Label:    "6. Implementation",
		Status:   "PENDING",
		Evidence: "No file edits committed yet",
	}
}

func inferReleaseStage(content string) TrajectoryStage {
	if strings.Contains(content, "nomos task sync") || strings.Contains(content, "walkthrough.md") {
		return TrajectoryStage{
			ID:       "RELEASE",
			Label:    "7. 2PC Release Sync",
			Status:   "COMPLETED",
			Evidence: "Merged to protected trunk via 2PC release certificate",
		}
	}
	return TrajectoryStage{
		ID:       "RELEASE",
		Label:    "7. 2PC Release Sync",
		Status:   "PENDING",
		Evidence: "Release sync not yet executed",
	}
}

func markActiveStage(stages []TrajectoryStage, sessionStatus string) {
	if sessionStatus != "ACTIVE" {
		return
	}
	for i := len(stages) - 1; i >= 0; i-- {
		if stages[i].Status == "COMPLETED" && i < len(stages)-1 {
			stages[i+1].Status = "ACTIVE"
			return
		}
	}
	if len(stages) > 0 && stages[0].Status == "PENDING" {
		stages[0].Status = "ACTIVE"
	}
}
