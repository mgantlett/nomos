// Package task defines domain models, schema constraints, and tracking mechanisms
// synthesized directly from authoritative Intent CUE schemas (task_inspection.cue).
package task

import (
	"fmt"
	"strings"
)

// TaskInspectionRecord defines the DTO contract emitted by nomos task show (#TaskInspectionRecord).
type TaskInspectionRecord struct {
	Key          string   `json:"key"`
	Title        string   `json:"title"`
	Status       Status   `json:"status"`
	Plane        string   `json:"plane"`
	Project      string   `json:"project,omitempty"`
	Tier         Tier     `json:"tier"`
	Priority     int      `json:"priority"`
	TwinKey      string   `json:"twin_key,omitempty"`
	BlockedBy    []string `json:"blocked_by,omitempty"`
	Blocks       []string `json:"blocks,omitempty"`
	WorktreePath string   `json:"worktree_path,omitempty"`
	BranchName   string   `json:"branch_name,omitempty"`
	Description  string   `json:"description"`
}

// BuildInspectionRecord constructs a complete TaskInspectionRecord.
func BuildInspectionRecord(task TaskRecord, blockedBy, blocks []string, worktreePath, branchName string) TaskInspectionRecord {
	return TaskInspectionRecord{
		Key:          task.Key,
		Title:        task.Title,
		Status:       task.Status,
		Plane:        task.Plane,
		Project:      task.Project,
		Tier:         task.Tier,
		Priority:     task.Priority,
		TwinKey:      task.TwinKey,
		BlockedBy:    blockedBy,
		Blocks:       blocks,
		WorktreePath: worktreePath,
		BranchName:   branchName,
		Description:  task.Description,
	}
}

// FormatInspectionCard generates a clean Markdown summary card for terminal display.
func FormatInspectionCard(rec TaskInspectionRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Task: %s\n\n", rec.Key))
	sb.WriteString(fmt.Sprintf("- **Title**: %s\n", rec.Title))
	if rec.Project != "" {
		sb.WriteString(fmt.Sprintf("- **Project**: %s\n", rec.Project))
	}
	sb.WriteString(fmt.Sprintf("- **Plane**: %s | **Tier**: %s | **Status**: %s\n", rec.Plane, rec.Tier, rec.Status))
	if rec.TwinKey != "" {
		sb.WriteString(fmt.Sprintf("- **Twin Key**: %s\n", rec.TwinKey))
	}
	if len(rec.BlockedBy) > 0 {
		sb.WriteString(fmt.Sprintf("- **Blocked By**: [%s]\n", strings.Join(rec.BlockedBy, ", ")))
	}
	if len(rec.Blocks) > 0 {
		sb.WriteString(fmt.Sprintf("- **Blocks**: [%s]\n", strings.Join(rec.Blocks, ", ")))
	}
	if rec.WorktreePath != "" {
		sb.WriteString(fmt.Sprintf("- **Worktree**: %s (branch: %s)\n", rec.WorktreePath, rec.BranchName))
	}
	sb.WriteString("\n## Description\n")
	sb.WriteString(rec.Description)
	sb.WriteString("\n")
	return sb.String()
}
