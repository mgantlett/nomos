package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProviderOpenCode implements SessionProvider for OpenCode CLI.
type ProviderOpenCode struct {
	rootOverride string
}

// NewProviderOpenCode instantiates a provider for OpenCode.
func NewProviderOpenCode(rootOverride string) *ProviderOpenCode {
	return &ProviderOpenCode{rootOverride: rootOverride}
}

// AgentType returns the canonical identity for OpenCode.
func (p *ProviderOpenCode) AgentType() string {
	return "OpenCode (Tier 1)"
}

// ResolveRoot returns the directory where OpenCode storage resides.
func (p *ProviderOpenCode) ResolveRoot() string {
	if p.rootOverride != "" {
		return p.resolveCustomOpenCodeRoot()
	}
	return p.resolveDefaultOpenCodeRoot()
}

func (p *ProviderOpenCode) resolveCustomOpenCodeRoot() string {
	candidates := []string{
		filepath.Join(p.rootOverride, "storage"),
		filepath.Join(p.rootOverride, "opencode", "storage"),
		filepath.Join(p.rootOverride, ".local", "share", "opencode", "storage"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			if sInfo, sErr := os.Stat(filepath.Join(c, "session")); sErr == nil && sInfo.IsDir() {
				return c
			}
		}
	}
	if sInfo, sErr := os.Stat(filepath.Join(p.rootOverride, "session")); sErr == nil && sInfo.IsDir() {
		return p.rootOverride
	}
	return ""
}

func (p *ProviderOpenCode) resolveDefaultOpenCodeRoot() string {
	if env := os.Getenv("OPENCODE_STORAGE_DIR"); env != "" {
		if info, err := os.Stat(env); err == nil && info.IsDir() {
			return env
		}
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		candidate := filepath.Join(xdg, "opencode", "storage")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, ".local", "share", "opencode", "storage"),
		filepath.Join(home, ".opencode", "storage"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

// Discover scans OpenCode storage for conversation sessions.
func (p *ProviderOpenCode) Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, nil
	}

	sessionDir := filepath.Join(root, "session")
	if _, err := os.Stat(sessionDir); err != nil {
		return nil, nil
	}

	var sessionFiles []string
	walkErr := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".json") && !strings.HasSuffix(info.Name(), ".meta.json") {
			sessionFiles = append(sessionFiles, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	var sessions []AgentSession
	for _, f := range sessionFiles {
		sess, err := p.parseOpenCodeSessionFile(root, f, activeWorktrees)
		if err == nil && sess != nil {
			sessions = append(sessions, *sess)
		}
	}
	return sessions, nil
}

func (p *ProviderOpenCode) parseOpenCodeSessionFile(root, filePath string, activeWorktrees map[string]string) (*AgentSession, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var doc openCodeSessionDoc
	if errUn := json.Unmarshal(data, &doc); errUn != nil {
		return nil, errUn
	}

	lastActive := time.UnixMilli(doc.Time.Updated)
	if doc.Time.Updated == 0 {
		if info, sErr := os.Stat(filePath); sErr == nil {
			lastActive = info.ModTime()
		}
	}

	turns, toolCalls, firstPrompt := p.scanSessionMessages(root, doc.ID)
	taskKey := extractTaskKeyFromOpenCode(doc.Directory, doc.Title)
	worktreePath, resolvedKey, phase := resolveSessionWorktree(taskKey, doc.Directory, activeWorktrees)
	status, finalPhase := calculateSessionStatus(lastActive, phase)
	repo := detectSessionRepo(worktreePath, doc.Directory, "")

	summary := cleanSummary(firstPrompt)
	if summary == "" {
		summary = doc.Title
	}

	return &AgentSession{
		ID:            doc.ID,
		Name:          p.readOpenCodeCustomName(filePath, doc.Title),
		AgentType:     p.AgentType(),
		TaskKey:       resolvedKey,
		WorktreePath:  worktreePath,
		Repo:          repo,
		Phase:         finalPhase,
		TurnCount:     turns,
		ToolCallCount: toolCalls,
		LastActiveAt:  lastActive,
		Status:        status,
		RecentSummary: summary,
	}, nil
}

func extractTaskKeyFromOpenCode(directory, title string) string {
	if m := taskKeyRegex.FindString(directory); m != "" {
		return m
	}
	if m := taskKeyRegex.FindString(title); m != "" {
		return m
	}
	return ""
}

// GetDetail inspects an OpenCode session deeply.
func (p *ProviderOpenCode) GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}

	filePath, doc, err := p.resolveSessionDoc(root, sessionID)
	if err != nil {
		return nil, err
	}

	baseSess, err := p.parseOpenCodeSessionFile(root, filePath, activeWorktrees)
	if err != nil {
		return nil, err
	}

	detail := &SessionDetail{
		AgentSession: *baseSess,
		ToolsSummary: make(map[string]int),
		Events:       make([]SessionEvent, 0),
		Persona:      detectPersona(baseSess, repoRoot),
	}

	p.populateOpenCodeDetailEvents(root, doc.ID, detail, repoRoot)
	return detail, nil
}

// SaveName assigns a title to an OpenCode session.
func (p *ProviderOpenCode) SaveName(sessionID, name string) (*SessionMetadata, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}

	filePath, doc, err := p.resolveSessionDoc(root, sessionID)
	if err != nil {
		return nil, err
	}

	doc.Title = name
	doc.Time.Updated = time.Now().UnixMilli()
	if sData, errMar := json.MarshalIndent(doc, "", "  "); errMar == nil {
		if writeErr := os.WriteFile(filePath, sData, 0644); writeErr != nil {
			return nil, writeErr
		}
	}

	metaPath := strings.TrimSuffix(filePath, ".json") + ".meta.json"
	meta := SessionMetadata{
		ID:        doc.ID,
		Name:      name,
		UpdatedAt: time.Now().UTC(),
	}
	if mData, mErr := json.MarshalIndent(meta, "", "  "); mErr == nil {
		if writeErr := os.WriteFile(metaPath, mData, 0644); writeErr != nil {
			return nil, writeErr
		}
	}

	return &meta, nil
}

// Matches returns true if sessionID could match an OpenCode session.
func (p *ProviderOpenCode) Matches(sessionID string) bool {
	clean := strings.TrimSpace(sessionID)
	return strings.HasPrefix(clean, "ses_") || len(clean) >= 6
}

func (p *ProviderOpenCode) resolveSessionDoc(root, sessionID string) (string, *openCodeSessionDoc, error) {
	target := strings.ToLower(strings.TrimSpace(sessionID))
	sessionDir := filepath.Join(root, "session")

	var matchedPath string
	var matchedDoc openCodeSessionDoc

	walkErr := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".json") && !strings.HasSuffix(info.Name(), ".meta.json") {
			base := strings.TrimSuffix(info.Name(), ".json")
			if strings.EqualFold(base, target) || strings.HasPrefix(strings.ToLower(base), target) {
				data, rErr := os.ReadFile(path)
				if rErr == nil && json.Unmarshal(data, &matchedDoc) == nil {
					matchedPath = path
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != filepath.SkipAll {
		return "", nil, walkErr
	}

	if matchedPath == "" {
		return "", nil, fmt.Errorf("opencode session %q not found in %s", sessionID, root)
	}
	return matchedPath, &matchedDoc, nil
}

func (p *ProviderOpenCode) readOpenCodeCustomName(filePath, defaultTitle string) string {
	metaPath := strings.TrimSuffix(filePath, ".json") + ".meta.json"
	if data, err := os.ReadFile(metaPath); err == nil {
		var meta SessionMetadata
		if json.Unmarshal(data, &meta) == nil && meta.Name != "" {
			return meta.Name
		}
	}
	if defaultTitle != "" && !strings.HasPrefix(defaultTitle, "New session -") {
		return defaultTitle
	}
	return ""
}
