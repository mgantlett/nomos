package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProviderClaudeCode implements SessionProvider for Anthropic's Claude Code CLI.
type ProviderClaudeCode struct {
	rootOverride string
}

// NewProviderClaudeCode instantiates a provider for Claude Code.
func NewProviderClaudeCode(rootOverride string) *ProviderClaudeCode {
	return &ProviderClaudeCode{rootOverride: rootOverride}
}

// AgentType returns the canonical identity for Claude Code.
func (p *ProviderClaudeCode) AgentType() string {
	return "Claude Code (Tier 1)"
}

// ResolveRoot returns the directory where Claude Code configurations and projects reside.
func (p *ProviderClaudeCode) ResolveRoot() string {
	if p.rootOverride != "" {
		candidates := []string{
			filepath.Join(p.rootOverride, ".claude"),
			filepath.Join(p.rootOverride, "claude"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				return c
			}
		}
		if pInfo, pErr := os.Stat(filepath.Join(p.rootOverride, "projects")); pErr == nil && pInfo.IsDir() {
			return p.rootOverride
		}
		return ""
	}
	if env := os.Getenv("CLAUDE_CONFIG_DIR"); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".claude")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

// Discover scans Claude Code projects for active session JSONL files.
func (p *ProviderClaudeCode) Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, nil
	}

	files := p.findSessionFiles(root)
	var sessions []AgentSession
	for _, f := range files {
		sess, err := p.parseClaudeSessionFile(f, activeWorktrees)
		if err == nil && sess != nil {
			sessions = append(sessions, *sess)
		}
	}
	return sessions, nil
}

func (p *ProviderClaudeCode) findSessionFiles(root string) []string {
	var files []string
	seen := make(map[string]bool)

	searchDirs := []string{
		filepath.Join(root, "projects"),
	}
	if pInfo, pErr := os.Stat(filepath.Join(root, "projects")); pErr != nil || !pInfo.IsDir() {
		searchDirs = []string{root}
	}

	for _, sDir := range searchDirs {
		if _, err := os.Stat(sDir); err != nil {
			continue
		}
		walkErr := filepath.Walk(sDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(info.Name(), ".jsonl") && info.Name() != "history.jsonl" {
				if !seen[path] {
					seen[path] = true
					files = append(files, path)
				}
			}
			return nil
		})
		if walkErr != nil {
			continue
		}
	}
	return files
}

func (p *ProviderClaudeCode) parseClaudeSessionFile(path string, activeWorktrees map[string]string) (*AgentSession, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	records, errRead := readClaudeLines(path)
	if errRead != nil {
		return nil, errRead
	}

	turns, toolCalls, cwd, taskKey, summary := scanClaudeSessionMetrics(records)
	worktreePath, resolvedKey, phase := resolveSessionWorktree(taskKey, cwd, activeWorktrees)
	status, finalPhase := calculateSessionStatus(info.ModTime(), phase)
	repo := detectSessionRepo(worktreePath, cwd, "")

	return &AgentSession{
		ID:            id,
		Name:          p.readClaudeCustomName(path, id),
		AgentType:     p.AgentType(),
		TaskKey:       resolvedKey,
		WorktreePath:  worktreePath,
		Repo:          repo,
		Phase:         finalPhase,
		TurnCount:     turns,
		ToolCallCount: toolCalls,
		LastActiveAt:  info.ModTime(),
		Status:        status,
		RecentSummary: summary,
	}, nil
}

func scanClaudeSessionMetrics(records []claudeRecord) (int, int, string, string, string) {
	turns := 0
	toolCalls := 0
	cwd := ""
	taskKey := ""
	summary := ""

	for _, rec := range records {
		if rec.Cwd != "" {
			cwd = rec.Cwd
		}
		if rec.GitBranch != "" {
			if m := taskKeyRegex.FindString(rec.GitBranch); m != "" {
				taskKey = m
			}
		}
		if rec.Type == "user" {
			tInc, sText := inspectClaudeUserTurn(rec)
			turns += tInc
			if summary == "" && sText != "" {
				summary = cleanSummary(sText)
			}
		} else if rec.Type == "assistant" {
			toolCalls += countClaudeAssistantTools(rec)
		}
	}

	if taskKey == "" && cwd != "" {
		if m := taskKeyRegex.FindString(cwd); m != "" {
			taskKey = m
		}
	}
	return turns, toolCalls, cwd, taskKey, summary
}

func inspectClaudeUserTurn(rec claudeRecord) (int, string) {
	items, text := parseClaudeContent(rec.Message.Content)
	for _, it := range items {
		if it.Type == "tool_result" {
			return 0, ""
		}
	}
	return 1, text
}

func countClaudeAssistantTools(rec claudeRecord) int {
	items, text := parseClaudeContent(rec.Message.Content)
	if text == "" {
		// no text content
	}
	count := 0
	for _, it := range items {
		if it.Type == "tool_use" {
			count++
		}
	}
	return count
}

// GetDetail deeply inspects a Claude Code session JSONL file.
func (p *ProviderClaudeCode) GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}

	path, fullID, err := p.resolveSessionFile(root, sessionID)
	if err != nil {
		return nil, err
	}

	baseSess, err := p.parseClaudeSessionFile(path, activeWorktrees)
	if err != nil {
		return nil, err
	}
	baseSess.ID = fullID

	detail := &SessionDetail{
		AgentSession: *baseSess,
		ToolsSummary: make(map[string]int),
		Events:       make([]SessionEvent, 0),
		Persona:      detectPersona(baseSess, repoRoot),
	}

	records, errRead := readClaudeLines(path)
	if errRead != nil {
		return nil, errRead
	}

	p.buildDetailEvents(records, detail, repoRoot)
	return detail, nil
}

// SaveName assigns a title to a Claude Code session.
func (p *ProviderClaudeCode) SaveName(sessionID, name string) (*SessionMetadata, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}

	path, fullID, err := p.resolveSessionFile(root, sessionID)
	if err != nil {
		return nil, err
	}

	metaPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
	meta := SessionMetadata{
		ID:        fullID,
		Name:      name,
		UpdatedAt: time.Now().UTC(),
	}

	data, errMarshal := json.MarshalIndent(meta, "", "  ")
	if errMarshal != nil {
		return nil, errMarshal
	}

	if errWrite := os.WriteFile(metaPath, data, 0644); errWrite != nil {
		return nil, errWrite
	}

	return &meta, nil
}

// Matches returns true if sessionID could match a Claude Code session.
func (p *ProviderClaudeCode) Matches(sessionID string) bool {
	clean := strings.TrimSpace(sessionID)
	return uuidRegex.MatchString(clean) || len(clean) >= 6
}

func (p *ProviderClaudeCode) resolveSessionFile(root, sessionID string) (string, string, error) {
	target := strings.ToLower(strings.TrimSpace(sessionID))
	files := p.findSessionFiles(root)

	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if strings.EqualFold(base, target) {
			return f, base, nil
		}
	}
	for _, f := range files {
		base := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if strings.HasPrefix(strings.ToLower(base), target) {
			return f, base, nil
		}
	}
	return "", "", fmt.Errorf("claude session %q not found in %s", sessionID, root)
}

func (p *ProviderClaudeCode) readClaudeCustomName(path, id string) string {
	metaPath := strings.TrimSuffix(path, ".jsonl") + ".meta.json"
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta SessionMetadata
		if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
			return meta.Name
		}
	}
	return ""
}
