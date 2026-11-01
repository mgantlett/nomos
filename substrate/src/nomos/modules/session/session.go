package session

import (
	"bufio"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// AgentSession captures the live state, telemetry, and coordinates of an agent conversation arc.
type AgentSession struct {
	ID            string    `json:"id"`
	Name          string    `json:"name,omitempty"`
	AgentType     string    `json:"agent_type"`
	TaskKey       string    `json:"task_key,omitempty"`
	TaskTitle     string    `json:"task_title,omitempty"`
	WorktreePath  string    `json:"worktree_path,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	Phase         string    `json:"phase"`
	TurnCount     int       `json:"turn_count"`
	ToolCallCount int       `json:"tool_call_count"`
	LastActiveAt  time.Time `json:"last_active_at"`
	Status        string    `json:"status"` // ACTIVE, IDLE, COMPLETED
	RecentSummary string    `json:"recent_summary,omitempty"`
}

// SessionVisualGraph represents the topological view of active agent sessions.
type SessionVisualGraph struct {
	Mermaid  string         `json:"mermaid"`
	Sessions []AgentSession `json:"sessions"`
}

type transcriptMetrics struct {
	turns     int
	toolCalls int
	taskKey   string
	worktree  string
	summary   string
}

var (
	taskKeyRegex  = regexp.MustCompile(`\b(INT|SUB)-\d+\b`)
	worktreeRegex = regexp.MustCompile(`worktrees/([a-zA-Z0-9_\-\.]+)`)
	uuidRegex     = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// DiscoverSessions inspects all registered Tier 1 agent providers and repo worktrees to discover active sessions.
func DiscoverSessions(rootOverride, repoRoot string) ([]AgentSession, error) {
	activeWorktrees := discoverActiveWorktrees(repoRoot)
	providers := DefaultProviders(rootOverride)

	var allSessions []AgentSession
	for _, p := range providers {
		sessions, err := p.Discover(repoRoot, activeWorktrees)
		if err != nil {
			continue
		}
		allSessions = append(allSessions, sessions...)
	}
	sortSessionsByActive(allSessions)
	return allSessions, nil
}

func sortSessionsByActive(sessions []AgentSession) {
	sort.SliceStable(sessions, func(i, j int) bool {
		if !sessions[i].LastActiveAt.Equal(sessions[j].LastActiveAt) {
			return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
		}
		return sessions[i].ID < sessions[j].ID
	})
}

var repoRegex = regexp.MustCompile(`(?:workstations|gsi)/([a-zA-Z0-9_\-]+)`)

func detectSessionRepo(worktreePath, transcriptHead, repoRoot string) string {
	if worktreePath != "" {
		if m := repoRegex.FindStringSubmatch(worktreePath); len(m) > 1 {
			return normalizeRepoName(m[1])
		}
		parts := strings.Split(filepath.Clean(worktreePath), string(filepath.Separator))
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] == "worktrees" && i > 0 {
				return normalizeRepoName(parts[i-1])
			}
		}
	}
	if transcriptHead != "" {
		if m := repoRegex.FindStringSubmatch(transcriptHead); len(m) > 1 {
			return normalizeRepoName(m[1])
		}
	}
	if repoRoot != "" {
		return normalizeRepoName(filepath.Base(repoRoot))
	}
	return "nomos"
}

func normalizeRepoName(r string) string {
	return strings.TrimSpace(r)
}

func resolveSessionWorktree(taskKey, worktree string, activeWorktrees map[string]string) (string, string, string) {
	if taskKey != "" && activeWorktrees[taskKey] != "" {
		return activeWorktrees[taskKey], taskKey, "EDIT"
	}
	if worktree != "" {
		for k, wt := range activeWorktrees {
			if strings.Contains(wt, worktree) {
				return wt, k, "EDIT"
			}
		}
	}
	return "", taskKey, "IDLE"
}

func calculateSessionStatus(modTime time.Time, phase string) (string, string) {
	age := time.Since(modTime)
	if age < 15*time.Minute {
		if phase == "IDLE" {
			return "ACTIVE", "REVIEW"
		}
		return "ACTIVE", phase
	}
	if age < 2*time.Hour {
		return "IDLE", phase
	}
	return "COMPLETED", phase
}

// discoverActiveWorktrees parses worktrees in the target repository.
func discoverActiveWorktrees(repoRoot string) map[string]string {
	result := make(map[string]string)
	if repoRoot == "" {
		repoRoot = "."
	}
	cmd := workspace.SafeGitCommand(repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return result
	}
	parseWorktreePorcelain(string(out), result)
	return result
}

func parseWorktreePorcelain(output string, result map[string]string) {
	var currentPath string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if matches := taskKeyRegex.FindString(branch); matches != "" {
				result[matches] = currentPath
			}
		}
	}
}

