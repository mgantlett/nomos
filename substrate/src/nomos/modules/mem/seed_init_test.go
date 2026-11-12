package mem

import (
	"path/filepath"
	"testing"
)

func TestSeedRepositoryArchitectureBaseline_IdempotentAndSupersedes(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "hex-service")

	v1 := "[Architecture Baseline in hex-service (INIT)] stack=[Go, Python] roots=[src] boundaries=[src-domain-purity]"
	if err := SeedRepositoryArchitectureBaseline(repo, v1); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	if err := SeedRepositoryArchitectureBaseline(repo, v1); err != nil {
		t.Fatalf("idempotent repeat seed: %v", err)
	}

	store, err := NewStore("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	active1, qErr := store.Query(MemoryFilter{RepoID: "hex-service", Kind: KindArchitecture, Status: StatusActive})
	if qErr != nil || len(active1) != 1 {
		t.Fatalf("expected 1 active architecture record after duplicate seed, got err=%v len=%d", qErr, len(active1))
	}

	v2 := "[Architecture Baseline in hex-service (INIT)] stack=[Go, Python, TypeScript] roots=[src, web] boundaries=[src-domain-purity]"
	if err := SeedRepositoryArchitectureBaseline(repo, v2); err != nil {
		t.Fatalf("evolved seed: %v", err)
	}
	active2, qErr2 := store.Query(MemoryFilter{RepoID: "hex-service", Kind: KindArchitecture, Status: StatusActive})
	if qErr2 != nil || len(active2) != 1 || active2[0].Content != v2 {
		t.Fatalf("expected evolved v2 to supersede v1 leaving 1 active record, got err=%v records=%+v", qErr2, active2)
	}
}
