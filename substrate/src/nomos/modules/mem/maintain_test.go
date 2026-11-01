package mem

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveIdempotentDeduplicationAndMaintain(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "mneme_maint.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// 1. Write-time idempotent deduplication on identical (repo_id, kind, normalized content)
	first := &Memory{RepoID: "nomos", Kind: KindReflection, Topic: "Hygiene", Content: "Always checkpoint SQLite WAL on sync."}
	second := &Memory{RepoID: "nomos", Kind: KindReflection, Topic: "Hygiene", Content: "  always checkpoint   sqlite wal on sync. "}
	if err := store.Save(first); err != nil {
		t.Fatalf("save first: %v", err)
	}
	if err := store.Save(second); err != nil {
		t.Fatalf("save second: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected second save to reuse canonical ID %d, got %d", first.ID, second.ID)
	}
	got, err := store.Get(first.ID)
	if err != nil || got.AccessCount != 1 {
		t.Fatalf("expected access_count reinforced to 1, got %+v (err=%v)", got, err)
	}

	// 2. Expired superseded record (>30d old) purged by Maintain()
	oldSuperseded := &Memory{
		RepoID:    "nomos",
		Kind:      KindPitfall,
		Topic:     "OldTrap",
		Content:   "Obsolete superseded pitfall from 45 days ago",
		CreatedAt: time.Now().UTC().Add(-45 * 24 * time.Hour),
		Status:    StatusSuperseded,
	}
	if err := store.Save(oldSuperseded); err != nil {
		t.Fatalf("save oldSuperseded: %v", err)
	}

	rep, err := store.Maintain(MaintainOptions{InactiveRetentionDays: 30})
	if err != nil {
		t.Fatalf("Maintain failed: %v", err)
	}
	if rep.InactivePurged != 1 || !rep.WALCheckpointed {
		t.Fatalf("expected 1 inactive purged and WAL checkpointed, got %+v", rep)
	}
}

func TestSweepAndRemoveTaskTrapBuffers(t *testing.T) {
	repoDir := t.TempDir()
	if err := RecordGateTrap(repoDir, "SUB-9", []string{"substrate/a.go"}, []GateTrapEntry{{GateName: "TDD", Message: "missing test"}}); err != nil {
		t.Fatalf("RecordGateTrap SUB-9: %v", err)
	}
	if err := RecordGateTrap(repoDir, "SUB-10", []string{"substrate/b.go"}, []GateTrapEntry{{GateName: "TDD", Message: "missing test"}}); err != nil {
		t.Fatalf("RecordGateTrap SUB-10: %v", err)
	}

	RemoveTaskTrapBuffer(repoDir, "SUB-9")
	if _, err := os.Stat(trapBufferPath(repoDir, "SUB-9")); !os.IsNotExist(err) {
		t.Fatalf("expected SUB-9 trap buffer removed")
	}

	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	_ = os.Chtimes(trapBufferPath(repoDir, "SUB-10"), oldTime, oldTime)
	if swept := SweepStaleTrapBuffers(repoDir, 7*24*time.Hour); swept != 1 {
		t.Fatalf("expected 1 stale trap buffer swept, got %d", swept)
	}
}
