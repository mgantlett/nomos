package mem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateAdvisoryWriteKind_StatutoryGuard(t *testing.T) {
	tests := []struct {
		name    string
		kind    MemoryKind
		wantErr bool
	}{
		{"TC-MEM-06a default empty", "", false},
		{"TC-MEM-06b reflection", KindReflection, false},
		{"TC-MEM-06c axiom", KindAxiom, false},
		{"TC-MEM-06d decision", KindDecision, false},
		{"TC-MEM-06e architecture", KindArchitecture, false},
		{"TC-MEM-06f reject pitfall", KindPitfall, true},
		{"TC-MEM-06g reject pattern", KindPattern, true},
		{"TC-MEM-06h reject release", KindRelease, true},
		{"TC-MEM-06i reject task", KindTask, true},
		{"TC-MEM-06j reject unknown", MemoryKind("forged_kind"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAdvisoryWriteKind(tc.kind)
			if tc.wantErr && err == nil {
				t.Fatalf("expected statutory/invalid kind %q to fail closed, got nil", tc.kind)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected advisory kind %q to succeed, got: %v", tc.kind, err)
			}
		})
	}
}

func TestDistillVerifiedPattern_ASTExtractionSupersedingAndIsolation(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())

	repoA := filepath.Join(t.TempDir(), "alpha-engine")
	repoB := filepath.Join(t.TempDir(), "beta-engine")
	modDir := filepath.Join(repoA, "substrate", "pkg", "cache")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	goSrc := `package cache

// RingCache stores bounded TTL entries (@intent: #CacheContract).
type RingCache struct{}

// EvictExpired purges stale entries deterministically (@intent: #CacheContract).
func EvictExpired() int { return 0 }
`
	if err := os.WriteFile(filepath.Join(modDir, "ring.go"), []byte(goSrc), 0644); err != nil {
		t.Fatalf("write ring.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "ring_test.go"), []byte("package cache\n"), 0644); err != nil {
		t.Fatalf("write ring_test.go: %v", err)
	}

	touched := []string{"substrate/pkg/cache/ring.go", "substrate/pkg/cache/ring_test.go"}
	saved, err := DistillVerifiedPattern(repoA, "SUB-41", "- Implemented deterministic TTL ring eviction", touched)
	if err != nil || saved != 1 {
		t.Fatalf("TC-MEM-07: expected 1 pattern distilled, got saved=%d err=%v", saved, err)
	}

	recalled := RecallRepoPatterns(repoA, "TTL ring eviction", touched, 3)
	if len(recalled) != 1 {
		t.Fatalf("TC-MEM-07: expected 1 recalled pattern in repoA, got %d", len(recalled))
	}
	got := recalled[0]
	for _, token := range []string{"ring.go", "RingCache", "EvictExpired", "#CacheContract", "ring_test.go", "SUB-41"} {
		if !strings.Contains(got, token) {
			t.Errorf("TC-MEM-07: expected blueprint to contain %q, got: %s", token, got)
		}
	}

	// Strict repository partitioning: repoB must see 0 patterns from repoA
	if other := RecallRepoPatterns(repoB, "TTL ring", touched, 3); len(other) != 0 {
		t.Fatalf("TC-MEM-07: expected 0 patterns in isolated repoB, got %v", other)
	}

	// Superseding older pattern on identical scope upon subsequent verified sync
	if _, err := DistillVerifiedPattern(repoA, "SUB-42", "- Upgraded ring buffer shard lock", touched); err != nil {
		t.Fatalf("second DistillVerifiedPattern: %v", err)
	}
	afterSupersede := RecallRepoPatterns(repoA, "", touched, 5)
	if len(afterSupersede) != 1 || !strings.Contains(afterSupersede[0], "SUB-42") {
		t.Fatalf("TC-MEM-07: expected older scope pattern superseded leaving SUB-42 active, got: %v", afterSupersede)
	}
}

func TestRecallGraduationCandidates_ThresholdAndDecay(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "delta-repo")
	modDir := filepath.Join(repo, "substrate", "src")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "worker.py"), []byte("def run_worker():\n    return True\n"), 0644); err != nil {
		t.Fatalf("write worker.py: %v", err)
	}

	touched := []string{"substrate/src/worker.py"}
	if _, err := DistillVerifiedPattern(repo, "SUB-7", "- Added worker execution entrypoint", touched); err != nil {
		t.Fatalf("DistillVerifiedPattern: %v", err)
	}

	// Also insert a heavily accessed advisory reflection; it must NOT appear in Tier B graduation candidates
	store, err := NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	advisory := &Memory{
		RepoID:      "delta-repo",
		Kind:        KindReflection,
		Topic:       "Notes",
		Content:     "General brainstorming note not eligible for nomos.yaml policy promotion",
		TaskID:      "SUB-7",
		AccessCount: 10,
	}
	if sErr := store.Save(advisory); sErr != nil {
		t.Fatalf("save advisory: %v", sErr)
	}
	_ = store.Close()

	// Recalls 1 & 2 keep access_count < 3 -> 0 graduation candidates
	_ = RecallRepoPatterns(repo, "worker", touched, 3)
	_ = RecallRepoPatterns(repo, "worker", touched, 3)
	if cands := RecallGraduationCandidates(repo, 3); len(cands) != 0 {
		t.Fatalf("TC-MEM-08: expected 0 graduation candidates below threshold 3, got %v", cands)
	}

	// Recall 3 crosses GraduationAccessThreshold (3) -> surfaces statutory pattern candidate
	_ = RecallRepoPatterns(repo, "worker", touched, 3)
	cands := RecallGraduationCandidates(repo, 3)
	if len(cands) != 1 || !strings.Contains(cands[0], "3x recalled") || !strings.Contains(cands[0], "pattern:substrate/src") {
		t.Fatalf("TC-MEM-08: expected 1 Tier A->B graduation candidate at 3x recall, got %v", cands)
	}

	// Verify CalculateTemporalDecay places KindPattern between KindAxiom and KindPitfall
	aged := time.Now().UTC().Add(-30 * 24 * time.Hour)
	dAxiom := CalculateTemporalDecay(aged, KindAxiom)
	dPattern := CalculateTemporalDecay(aged, KindPattern)
	dPitfall := CalculateTemporalDecay(aged, KindPitfall)
	if !(dAxiom > dPattern && dPattern > dPitfall) {
		t.Fatalf("expected decay ordering axiom(%f) > pattern(%f) > pitfall(%f)", dAxiom, dPattern, dPitfall)
	}
}
