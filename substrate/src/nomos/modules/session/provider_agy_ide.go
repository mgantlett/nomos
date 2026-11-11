package session

import (
	"os"
	"path/filepath"
	"strings"
)

// ProviderAGYIDE implements SessionProvider for Google Antigravity 2.0 and Antigravity IDE.
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

// ResolveRoot returns the primary directory where AGY-IDE conversation sessions reside.
func (p *ProviderAGYIDE) ResolveRoot() string {
	roots := p.ResolveRoots()
	if len(roots) == 0 {
		return ""
	}
	return roots[0]
}

// ResolveRoots returns all existing brain directories across Antigravity 2.0 and legacy AGY-IDE layouts.
func (p *ProviderAGYIDE) ResolveRoots() []string {
	var out []string
	addIfDir := func(candidate string) {
		if candidate == "" {
			return
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			for _, existing := range out {
				if existing == candidate {
					return
				}
			}
			out = append(out, candidate)
		}
	}
	if p.rootOverride != "" {
		addIfDir(filepath.Join(p.rootOverride, "antigravity", "brain"))
		addIfDir(filepath.Join(p.rootOverride, "antigravity-ide", "brain"))
		if len(out) > 0 {
			return out
		}
		lower := strings.ToLower(p.rootOverride)
		if strings.Contains(lower, "cli") || strings.Contains(lower, "claude") || strings.Contains(lower, "opencode") {
			return nil
		}
		addIfDir(p.rootOverride)
		return out
	}
	if env := os.Getenv("ANTIGRAVITY_APP_DATA_DIR"); env != "" {
		addIfDir(filepath.Join(env, "brain"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		addIfDir(filepath.Join(home, ".gemini", "antigravity", "brain"))
		addIfDir(filepath.Join(home, ".gemini", "antigravity-ide", "brain"))
	}
	return out
}

// Discover scans all AGY-IDE brain directories and deduplicates sessions by UUID preferring newest activity.
func (p *ProviderAGYIDE) Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error) {
	roots := p.ResolveRoots()
	if len(roots) == 0 {
		return nil, nil
	}
	byID := make(map[string]AgentSession)
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !uuidRegex.MatchString(entry.Name()) {
				continue
			}
			sessDir := filepath.Join(root, entry.Name())
			sess, pErr := parseBrainSessionDirWithRepo(sessDir, entry.Name(), p.AgentType(), repoRoot, activeWorktrees)
			if pErr != nil || sess == nil {
				continue
			}
			if prev, exists := byID[sess.ID]; !exists || sess.LastActiveAt.After(prev.LastActiveAt) {
				byID[sess.ID] = *sess
			}
		}
	}
	sessions := make([]AgentSession, 0, len(byID))
	for _, s := range byID {
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// GetDetail inspects an AGY-IDE session directory deeply across configured brain roots.
func (p *ProviderAGYIDE) GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error) {
	targetDir, fullID, err := resolveBrainSessionAcrossRoots(p.ResolveRoots(), sessionID)
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

// SaveName assigns a title to an AGY-IDE session across configured brain roots.
func (p *ProviderAGYIDE) SaveName(sessionID, name string) (*SessionMetadata, error) {
	for _, root := range p.ResolveRoots() {
		if meta, err := saveBrainSessionName(root, sessionID, name); err == nil {
			return meta, nil
		}
	}
	return nil, os.ErrNotExist
}

// Matches returns true if the session ID could match an AGY-IDE session.
func (p *ProviderAGYIDE) Matches(sessionID string) bool {
	clean := strings.TrimSpace(sessionID)
	return uuidRegex.MatchString(clean) || len(clean) >= 6
}
