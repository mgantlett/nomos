package mem

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveAndGet(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_brain.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed initializing brain store: %v", err)
	}
	defer store.Close()

	memRecord := &Memory{
		RepoID:    "nomos",
		Kind:      KindReflection,
		Topic:     "HermeticWorktree",
		Content:   "Protected trunk branches must remain strictly immutable (Law 3).",
		TaskID:    "SUB-1212",
		CreatedAt: time.Now().UTC(),
	}

	if err := store.Save(memRecord); err != nil {
		t.Fatalf("failed saving memory: %v", err)
	}
	if memRecord.ID <= 0 {
		t.Errorf("expected positive ID, got %d", memRecord.ID)
	}

	got, err := store.Get(memRecord.ID)
	if err != nil {
		t.Fatalf("failed getting memory: %v", err)
	}
	if got.Topic != memRecord.Topic || got.Content != memRecord.Content {
		t.Errorf("mismatch: got %+v, want %+v", got, memRecord)
	}
	if len(got.Embedding) != 768 && len(got.Embedding) != DefaultEmbeddingDimension {
		t.Errorf("expected 768-dim or 64-dim embedding, got %d", len(got.Embedding))
	}
}

func TestStore_QueryAndSearch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_search.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}
	defer store.Close()

	m1 := &Memory{
		RepoID:  "nomos",
		Kind:    KindAxiom,
		Topic:   "Verification",
		Content: "Non-verbal proof requires exit code 0 and compilation proof (Law 2).",
		TaskID:  "INT-701",
	}
	m2 := &Memory{
		RepoID:  "acme-app",
		Kind:    KindReflection,
		Topic:   "Frontend",
		Content: "React and CSS styling patterns for dark mode dashboard.",
		TaskID:  "SUB-500",
	}

	if err := store.Save(m1); err != nil {
		t.Fatalf("save m1 failed: %v", err)
	}
	if err := store.Save(m2); err != nil {
		t.Fatalf("save m2 failed: %v", err)
	}

	// Local repo search for "nomos" repo
	results, err := store.SearchWithOptions("verification proof", SearchOptions{
		RepoID: "nomos",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected matches for verification proof")
	}
	if results[0].Memory.Topic != "Verification" {
		t.Errorf("expected Verification topic to match, got %s", results[0].Memory.Topic)
	}

	// Verify local repo isolation: searching for "React" in "nomos" repo returns 0 results
	repoResults, err := store.SearchWithOptions("React", SearchOptions{
		RepoID: "nomos",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(repoResults) != 0 {
		t.Errorf("expected 0 results due to repo isolation, got %d", len(repoResults))
	}

	// Cross-repo search with -A
	crossResults, err := store.SearchWithOptions("React", SearchOptions{
		IncludeCrossRepo: true,
		Limit:            5,
	})
	if err != nil {
		t.Fatalf("cross-repo search failed: %v", err)
	}
	if len(crossResults) == 0 {
		t.Errorf("expected cross-repo search to find React memory")
	}
}

func TestStore_RepoLocalityBoostAndGlobalAxiomSlotCap(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "test_locality.db"))
	if err != nil {
		t.Fatalf("failed initializing store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := store.Save(&Memory{
			RepoID:    "global",
			Kind:      KindAxiom,
			Topic:     "Verification & Invariant Gates",
			Content:   fmt.Sprintf("AXIOM %d: Verification gate invariant checks enforce deterministic AST proof.", i+1),
			TaskID:    "SYSTEM-4-DREAM",
			CreatedAt: now,
		}); err != nil {
			t.Fatalf("save global axiom %d: %v", i, err)
		}
	}
	for i := 0; i < 4; i++ {
		if err := store.Save(&Memory{
			RepoID:    "nomos",
			Kind:      KindPattern,
			Topic:     "pattern:substrate/verify",
			Content:   fmt.Sprintf("Pattern %d: Verification gate invariant checks enforce deterministic AST proof.", i+1),
			TaskID:    "SUB-1806",
			CreatedAt: now,
		}); err != nil {
			t.Fatalf("save repo pattern %d: %v", i, err)
		}
	}

	got, err := store.SearchWithOptions("verification gate invariant", SearchOptions{
		RepoID:   "nomos",
		Limit:    5,
		ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("SearchWithOptions failed: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("expected 5 results, got %d", len(got))
	}
	if got[0].Memory.RepoID != "nomos" {
		t.Fatalf("expected boosted repo-local match at rank 0, got repo=%q kind=%q", got[0].Memory.RepoID, got[0].Memory.Kind)
	}
	globalCount := 0
	for _, r := range got {
		if r.Memory.RepoID == "global" {
			globalCount++
		}
	}
	if globalCount != 1 {
		t.Fatalf("TC-MEM-14: expected global axioms capped at 1 slot (limit=5), got %d", globalCount)
	}

	// Sparse repository with zero local matches backfills from global axioms cleanly.
	sparse, err := store.SearchWithOptions("verification gate invariant", SearchOptions{
		RepoID:   "empty-repo",
		Limit:    3,
		ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("sparse SearchWithOptions failed: %v", err)
	}
	if len(sparse) != 3 {
		t.Fatalf("TC-MEM-14 sparse backfill: expected 3 global fallback items, got %d", len(sparse))
	}
}
