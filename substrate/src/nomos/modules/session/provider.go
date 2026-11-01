package session

// SessionProvider encapsulates discovery, inspection, and metadata mutations
// for a specific Tier 1 coding agent family (AGY-IDE, AGY-CLI, Claude Code, OpenCode).
type SessionProvider interface {
	// AgentType returns the canonical human-readable agent identity.
	AgentType() string

	// Discover scans the provider's storage root and discovers active agent conversation sessions.
	Discover(repoRoot string, activeWorktrees map[string]string) ([]AgentSession, error)

	// GetDetail inspects an individual session deeply, generating timelines, scorecard, and trajectory.
	GetDetail(sessionID, repoRoot string, activeWorktrees map[string]string) (*SessionDetail, error)

	// SaveName updates or assigns a human-readable title or alias to an agent conversation session.
	SaveName(sessionID, name string) (*SessionMetadata, error)

	// Matches returns true if the session ID or prefix can belong to this provider.
	Matches(sessionID string) bool
}

// DefaultProviders returns the canonical set of Tier 1 agent session providers.
// If rootOverride is specified, providers look within rootOverride or its sub-paths.
func DefaultProviders(rootOverride string) []SessionProvider {
	return []SessionProvider{
		NewProviderAGYIDE(rootOverride),
		NewProviderAGYCLI(rootOverride),
		NewProviderClaudeCode(rootOverride),
		NewProviderOpenCode(rootOverride),
	}
}
