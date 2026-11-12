package mem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// @verifies TC-MEM-08 TC-MEM-16 TC-MEM-17 TC-MEM-18
func TestResolveHotScopeReflexAndCleanStreakCooling(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "reflex-repo")
	cliDir := filepath.Join(repo, "substrate", "cli")
	if err := os.MkdirAll(cliDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	targetFile := filepath.Join(cliDir, "cmd.go")
	if err := os.WriteFile(targetFile, []byte("package cli\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatalf("write cmd.go: %v", err)
	}
	touched := []string{"substrate/cli/cmd.go"}

	// Recurrence 1..2: below HotScopeMinRecurrence (3), reflex remains nil and built-in AST banner is suppressed (TC-MEM-18).
	for _, tk := range []string{"SUB-1", "SUB-2"} {
		if err := RecordGateTrap(repo, tk, touched, []GateTrapEntry{{
			GateName: "Cognitive Complexity Invariant",
			Message:  "- substrate/cli/cmd.go:10 (Run) cyclomatic complexity 18 exceeds max 15",
		}}); err != nil {
			t.Fatalf("RecordGateTrap %s: %v", tk, err)
		}
		if _, err := DistillTaskLearnings(repo, tk, "- Split Run helper", touched); err != nil {
			t.Fatalf("DistillTaskLearnings %s: %v", tk, err)
		}
	}
	if got := ResolveHotScopeReflex(repo, "substrate/cli"); got != nil {
		t.Fatalf("TC-MEM-16: expected nil reflex at recurrence=2, got %+v", got)
	}
	if cands := RecallGraduationCandidates(repo, 3); len(cands) != 0 {
		t.Fatalf("TC-MEM-18: expected built-in AST gate excluded from nomos.yaml banners, got %v", cands)
	}

	// Third recurrence reaches HotScopeMinRecurrence (3) with head AccessCount=0 -> activates HotScopeReflex (TC-MEM-16).
	if err := RecordGateTrap(repo, "SUB-3", touched, []GateTrapEntry{{
		GateName: "Cognitive Complexity Invariant",
		Message:  "- substrate/cli/cmd.go:12 (Run) cyclomatic complexity 19 exceeds max 15",
	}}); err != nil {
		t.Fatalf("RecordGateTrap SUB-3: %v", err)
	}
	if _, err := DistillTaskLearnings(repo, "SUB-3", "- Extracted subcommand dispatch", touched); err != nil {
		t.Fatalf("DistillTaskLearnings SUB-3: %v", err)
	}
	reflex := ResolveHotScopeReflex(repo, "substrate/cli")
	if reflex == nil || reflex.RecurrenceDepth != 3 || reflex.CleanStreak != 0 {
		t.Fatalf("TC-MEM-16: expected active HotScopeReflex(depth=3, streak=0), got %+v", reflex)
	}
	if len(reflex.ActiveGates) != 1 || !strings.Contains(reflex.ActiveGates[0], "cognitive_complexity") {
		t.Fatalf("TC-MEM-16: expected cognitive_complexity active gate, got %+v", reflex.ActiveGates)
	}
	if other := ResolveHotScopeReflex(repo, "substrate/other"); other != nil {
		t.Fatalf("TC-MEM-16: expected unrelated directory scope to remain nil, got %+v", other)
	}

	// Add a policy pitfall at recurrence=2 and verify both HotScopeReflex and Tier B graduation cool at AccessCount >= 5 (TC-MEM-17).
	for _, tk := range []string{"SUB-4", "SUB-5"} {
		if err := RecordGateTrap(repo, tk, touched, []GateTrapEntry{{
			GateName: "Forbidden Literals Invariant",
			Message:  "- substrate/cli/cmd.go:4 contains forbidden literal",
		}}); err != nil {
			t.Fatalf("policy RecordGateTrap %s: %v", tk, err)
		}
		if _, err := DistillTaskLearnings(repo, tk, "- Removed hardcoded literal", touched); err != nil {
			t.Fatalf("policy DistillTaskLearnings %s: %v", tk, err)
		}
	}
	if cands := RecallGraduationCandidates(repo, 3); len(cands) != 1 {
		t.Fatalf("TC-MEM-08: expected 1 actionable policy graduation candidate before cooling, got %v", cands)
	}

	// Accumulate 5 consecutive clean task recalls in substrate/cli -> cools both HotScopeReflex and policy graduation nagging.
	for _, tk := range []string{"SUB-10", "SUB-11", "SUB-12", "SUB-13", "SUB-14"} {
		_ = RecallRepoPitfallsForTask(repo, tk, touched, 5)
	}
	if cooled := ResolveHotScopeReflex(repo, "substrate/cli"); cooled != nil {
		t.Fatalf("TC-MEM-17: expected HotScopeReflex cooled to nil at CleanStreak >= 5, got %+v", cooled)
	}
	if cooledCands := RecallGraduationCandidates(repo, 3); len(cooledCands) != 0 {
		t.Fatalf("TC-MEM-17: expected policy graduation candidate cooled at AccessCount >= 5, got %v", cooledCands)
	}
}

func TestRecallRepoPatterns_DayZeroArchitectureFallback(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := filepath.Join(t.TempDir(), "cold-brownfield")
	baseline := "[Architecture Baseline in cold-brownfield (INIT)] stack=[Python] roots=[app] boundaries=[app-domain-purity]"
	if err := SeedRepositoryArchitectureBaseline(repo, baseline); err != nil {
		t.Fatalf("seed baseline: %v", err)
	}
	got := RecallRepoPatternsReadOnly(repo, "", nil, 3)
	if len(got) != 1 || !strings.Contains(got[0], "app-domain-purity") {
		t.Fatalf("expected cold-start fallback to recall Day-0 architecture baseline, got: %v", got)
	}
}

