// Package task defines domain models, schema constraints, and tracking mechanisms
// enforced by compiled Go SSoT contracts.
package task

import (
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// StatusCancelled represents legacy cancelled task records treated as terminal.
const StatusCancelled Status = "CANCELLED"

// ResolveRepoProject determines the canonical repository project identifier from nomos.yaml or directory basename.
func ResolveRepoProject(repoRoot string) string {
	cleanRoot := filepath.Clean(strings.TrimSpace(repoRoot))
	if cleanRoot == "" || cleanRoot == "." || cleanRoot == string(filepath.Separator) {
		return constants.AppName
	}
	repoBase := filepath.Base(cleanRoot)
	if cfg, err := config.LoadConfig(cleanRoot); err == nil && cfg != nil {
		cfgProj := strings.TrimSpace(cfg.Project)
		if cfgProj != "" && (cfgProj != constants.AppName || repoBase == constants.AppName) {
			return cfgProj
		}
	}
	if repoBase != "" && repoBase != "." && repoBase != string(filepath.Separator) {
		return repoBase
	}
	return constants.AppName
}

// MatchesProject evaluates whether a task record's project belongs to targetProject.
// When allowUnassigned is true, legacy or synthetic records with an empty project also match.
func MatchesProject(recordProject, targetProject string, allowUnassigned bool) bool {
	target := strings.TrimSpace(targetProject)
	if target == "" {
		return true
	}
	rec := strings.TrimSpace(recordProject)
	if rec == "" {
		return allowUnassigned
	}
	return strings.EqualFold(rec, target)
}

// FilterTasksByProject filters task records by targetProject.
func FilterTasksByProject(tasks []TaskRecord, targetProject string, allowUnassigned bool) []TaskRecord {
	if strings.TrimSpace(targetProject) == "" {
		return tasks
	}
	out := make([]TaskRecord, 0, len(tasks))
	for _, t := range tasks {
		if MatchesProject(t.Project, targetProject, allowUnassigned) {
			out = append(out, t)
		}
	}
	return out
}

// IsTerminalStatus reports whether a task status is finished or closed.
func IsTerminalStatus(status Status) bool {
	switch Status(strings.ToUpper(strings.TrimSpace(string(status)))) {
	case StatusDone, StatusAborted, StatusCancelled:
		return true
	default:
		return false
	}
}

// IsInProgressStatus reports whether a task is actively checked out in a lifecycle execution phase.
func IsInProgressStatus(status Status) bool {
	switch Status(strings.ToUpper(strings.TrimSpace(string(status)))) {
	case StatusPlan, StatusEdit, StatusInProgress, StatusReview, StatusSync:
		return true
	default:
		return false
	}
}

