package session

import (
	"os"
	"path/filepath"
	"strings"
)

// ProviderAGYCLI implements SessionProvider for Antigravity CLI (agy).
type ProviderAGYCLI struct {
	rootOverride string
}

// NewProviderAGYCLI instantiates a provider for AGY-CLI.
func NewProviderAGYCLI(rootOverride string) *ProviderAGYCLI {
	return &ProviderAGYCLI{rootOverride: rootOverride}
}

// AgentType returns the canonical identity for AGY-CLI.
func (p *ProviderAGYCLI) AgentType() string {
	return "AGY-CLI (Tier 1)"
}

// ResolveRoot returns the directory where AGY-CLI conversation sessions reside.
func (p *ProviderAGYCLI) ResolveRoot() string {
	if p.rootOverride != "" {
		cliCandidate := filepath.Join(p.rootOverride, "antigravity-cli", "brain")
		if info, err := os.Stat(cliCandidate); err == nil && info.IsDir() {
			return cliCandidate
		}
		lower := strings.ToLower(p.rootOverride)
		if strings.Contains(lower, "cli") {
			if info, err := os.Stat(p.rootOverride); err == nil && info.IsDir() {
				return p.rootOverride
			}
		}
		return ""
	}
	if env := os.Getenv("ANTIGRAVITY_CLI_APP_DATA_DIR"); env != "" {
		candidate := filepath.Join(env, "brain")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".gemini", "antigravity-cli", "brain")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}

// Discover scans the AGY-CLI brain directory for conversation sessions.
func (p *ProviderAGYCLI) Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error) {
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
		sess, err := parseBrainSessionDirWithRepo(sessDir, entry.Name(), p.AgentType(), repoRoot, activeWorktrees)
		if err == nil && sess != nil {
			sessions = append(sessions, *sess)
		}
	}
	return sessions, nil
}

// GetDetail inspects an AGY-CLI session directory deeply.
func (p *ProviderAGYCLI) GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}
	targetDir, fullID, err := resolveBrainSessionDir(root, sessionID)
	if err != nil {
		return nil, err
	}

	baseSess, err := parseBrainSessionDirWithRepo(targetDir, fullID, p.AgentType(), repoRoot, activeWorktrees)
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

// SaveName assigns a title to an AGY-CLI session.
func (p *ProviderAGYCLI) SaveName(sessionID, name string) (*SessionMetadata, error) {
	root := p.ResolveRoot()
	if root == "" {
		return nil, os.ErrNotExist
	}
	return saveBrainSessionName(root, sessionID, name)
}

// Matches returns true if the session ID could match an AGY-CLI session.
func (p *ProviderAGYCLI) Matches(sessionID string) bool {
	clean := strings.TrimSpace(sessionID)
	return uuidRegex.MatchString(clean) || len(clean) >= 6
}
