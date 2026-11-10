// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
)

const staleTrapSweepWindow = 7 * 24 * time.Hour

func runAutomaticSubstrateMaintenance(repoRoot string, force bool) {
	if iErr := EnsureIDEBoundarySettings(repoRoot); iErr != nil {
		// best-effort IDE watcher boundary protection
	}
	if repoRoot != "" {
		if _, pErr := pruneWorkspaceOrphans(repoRoot, false); pErr != nil {
			// best-effort orphan worktree sweep
		}
	}
	if !storage.ShouldRunPeriodicMaintenance(force) {
		return
	}
	if _, err := storage.MaintainRuntimeStorage(true); err != nil {
		// best-effort background hygiene
	}
	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err == nil {
		if _, mErr := store.Maintain(mem.MaintainOptions{}); mErr != nil {
			// best-effort Mneme compaction
		}
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}
	mem.SweepStaleTrapBuffers(repoRoot, staleTrapSweepWindow)
}

func distillAndMaintainOnSync(repoRoot, wtPath, key, walkthroughPath string, touchedFiles []string) {
	wtContent := readWalkthroughContent(walkthroughPath)
	if distilled, dErr := mem.DistillTaskLearnings(repoRoot, key, wtContent, touchedFiles); dErr == nil && distilled > 0 {
		printlnDistilledCount(distilled)
	}
	if tr := loadHookSessionTrace(wtPath); tr != nil && len(tr.MutatedFiles) > 0 {
		if deadEnds, deErr := mem.RecordRolledBackDeadEnds(repoRoot, key, tr.MutatedFiles, touchedFiles, wtContent); deErr == nil && deadEnds > 0 {
			fmt.Printf("🧠 Nomos Learn: recorded %d rolled-back dead-end(s) into local Mneme\n", deadEnds)
		}
	}
	if patterns, pErr := mem.DistillVerifiedPattern(repoRoot, key, wtContent, touchedFiles); pErr == nil && patterns > 0 {
		fmt.Printf("🧠 Nomos Learn: distilled %d verified structural pattern(s) into local Mneme\n", patterns)
	}
	runAutomaticSubstrateMaintenance(repoRoot, true)
}

func printlnDistilledCount(distilled int) {
	fmt.Printf("🧠 Nomos Learn: distilled %d verified trap->fix pitfall(s) into local Mneme\n", distilled)
}
