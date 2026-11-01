package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionMetadata represents persistent, operator-assigned metadata for an agent session.
type SessionMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TaskKey   string    `json:"task_key,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionMetaFileName defines the canonical filename for local session metadata.
const SessionMetaFileName = "session_meta.json"

// readSessionCustomName reads session_meta.json from the session directory if present.
func readSessionCustomName(sessDir string) string {
	metaPath := filepath.Join(sessDir, SessionMetaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}
	var meta SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return ""
	}
	return strings.TrimSpace(meta.Name)
}

// SaveSessionName persists a custom name to metadata inside the session store of the matching provider.
func SaveSessionName(rootOverride, sessionID, name string) (*SessionMetadata, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("session title/name is required")
	}
	providers := DefaultProviders(rootOverride)
	for _, p := range providers {
		meta, err := p.SaveName(sessionID, trimmedName)
		if err == nil && meta != nil {
			return meta, nil
		}
	}
	return nil, fmt.Errorf("session %q not found across supported Tier 1 agent providers", sessionID)
}

