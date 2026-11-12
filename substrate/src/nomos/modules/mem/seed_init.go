// Package mem seeds idempotent Day-0 repository architecture baselines into Nomos Mneme
// during 'nomos init' so cold-start downstream sessions orient immediately (Law 1 & Law 4).
package mem

import (
	"os"
	"path/filepath"
	"strings"
)

const initTopologyTopic = "init-topology"

// SeedRepositoryArchitectureBaseline idempotently persists a Day-0 KindArchitecture memory
// for repoRoot and supersedes stale init-topology entries when topology evolves (@intent task_lifecycle_cli_spec.md #Memory).
func SeedRepositoryArchitectureBaseline(repoRoot, summary string) error {
	cleanRoot := filepath.Clean(strings.TrimSpace(repoRoot))
	cleanSummary := strings.TrimSpace(summary)
	if cleanRoot == "" || cleanSummary == "" {
		return nil
	}
	if shouldSkipEphemeralTempSeed(cleanRoot) {
		return nil
	}
	repoID := strings.ToLower(filepath.Base(cleanRoot))
	if repoID == "" || repoID == "." {
		return nil
	}

	store, err := NewStore("")
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	rec := &Memory{
		RepoID:  repoID,
		Kind:    KindArchitecture,
		Topic:   initTopologyTopic,
		Content: cleanSummary,
		TaskID:  "INIT",
		Status:  StatusActive,
	}
	if err := store.Save(rec); err != nil {
		return err
	}
	if rec.ID > 0 {
		store.supersedeOlderTopicArchitecture(repoID, initTopologyTopic, rec.ID)
	}
	return nil
}

func shouldSkipEphemeralTempSeed(cleanRoot string) bool {
	if strings.TrimSpace(os.Getenv("NOMOS_HOME")) != "" {
		return false
	}
	tmpDir := filepath.Clean(os.TempDir())
	return tmpDir != "" && strings.HasPrefix(cleanRoot, tmpDir+string(filepath.Separator))
}

func (s *Store) supersedeOlderTopicArchitecture(repoID, topic string, newestID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `UPDATE memories SET status = 'superseded', superseded_by = ?
	          WHERE kind = 'architecture' AND status = 'active' AND lower(repo_id) = lower(?) AND lower(topic) = lower(?) AND id != ?`
	if _, err := s.db.Exec(query, newestID, strings.TrimSpace(repoID), strings.TrimSpace(topic), newestID); err != nil {
		return
	}
}
