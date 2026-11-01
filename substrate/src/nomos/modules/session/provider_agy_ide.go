package session

import (
	"os"
	"path/filepath"
	"strings"
)

// ProviderAGYIDE implements SessionProvider for Google Antigravity IDE.
type ProviderAGYIDE struct {
	rootOverride string
}

// NewProviderAGYIDE instantiates a provider for AGY-IDE.
func NewProviderAGYIDE(rootOverride string) *ProviderAGYIDE {
	return &ProviderAGYIDE{rootOverride: rootOverride}
}

// AgentType returns the canonical identity for AGY-IDE.
func (p *ProviderAGYIDE) AgentType() string {
	return "AGY-IDE (Tier 1)"
}

// ResolveRoot returns the directory where AGY-IDE conversation sessions reside.
func (p *ProviderAGYIDE) ResolveRoot() string {
	if p.rootOverride != "" {
		ideCandidate := filepath.Join(p.rootOverride, "antigravity-ide", "brain")
		if info, err := os.Stat(ideCandidate); err == nil && info.IsDir() {
			return ideCandidate
		}
		lower := strings.ToLower(p.rootOverride)
		if strings.Contains(lower, "cli") || strings.Contains(lower, "claude") || strings.Contains(lower, "opencode") {
			return ""
		}
		if info, err := os.Stat(p.rootOverride); err == nil && info.IsDir() {
			return p.rootOverride
		}
		return ""
	}
	if env := os.Getenv("ANTIGRAVITY_APP_DATA_DIR"); env != "" {
		candidate := filepath.Join(env, "brain")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".gemini", "antigravity-ide", "brain")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

// Discover scans the AGY-IDE brain directory for conversation sessions.
func (p *ProviderAGYIDE) Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var sessions []AgentSession
	for _, entry := range entries {
		if !entry.IsDir() || !uuidRegex.MatchString(entry.Name()) {
			continue
		}
		sessDir := filepath.Join(root, entry.Name())
		sess, err := parseBrainSessionDir(sessDir, entry.Name(), p.AgentType(), activeWorktrees)
		if err == nil && sess != nil {
			sessions = append(sessions, *sess)
		}
	}
	return sessions, nil
}

// GetDetail inspects an AGY-IDE session directory deeply.
func (p *ProviderAGYIDE) GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}
	targetDir, fullID, err := resolveBrainSessionDir(root, sessionID)
	if err != nil {
		return nil, err
	}

	baseSess, err := parseBrainSessionDir(targetDir, fullID, p.AgentType(), activeWorktrees)
	if err != nil {
		return nil, err
	}

	detail := &SessionDetail{
		AgentSession: *baseSess,
		ToolsSummary: make(map[string]int),
		Events:       make([]SessionEvent, 0),
		Persona:      detectPersona(baseSess, repoRoot),
	}

	transcriptPath := filepath.Join(targetDir, ".system_generated", "logs", "transcript.jsonl")
	if errPop := populateBrainDetail(detail, transcriptPath, repoRoot); errPop != nil {
		return nil, errPop
	}

	return detail, nil
}

// SaveName assigns a title to an AGY-IDE session.
func (p *ProviderAGYIDE) SaveName(sessionID, name string) (*SessionMetadata, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}
	return saveBrainSessionName(root, sessionID, name)
}

// Matches returns true if the session ID could match an AGY-IDE session.
func (p *ProviderAGYIDE) Matches(sessionID string) bool {
	clean := strings.TrimSpace(sessionID)
	return uuidRegex.MatchString(clean) || len(clean) >= 6
}
