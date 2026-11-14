// Package verify orchestrates configured obligation, contract signature, stale receipt,
// and directional architecture boundary checks inside IntentTraceabilityGate.
package verify

import (
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

// evaluateConfiguredObligations evaluates architecture boundaries and specification contracts
// declared in nomos.yaml for the active repository or task worktree.
func evaluateConfiguredObligations(gateName string, ctx *vscope.VerifyContext, root, targetDir string) *vscope.GateResult {
	cfg := vscope.ResolveVerifyConfig(ctx)
	if cfg == nil {
		return nil
	}
	if len(cfg.Governance.QualityGates.ArchitectureBoundaries) > 0 {
		if boundaryFailure := evaluateConfiguredArchitectureBoundaries(gateName, ctx, root, cfg.Governance.QualityGates.ArchitectureBoundaries); boundaryFailure != nil {
			return boundaryFailure
		}
	}
	return evaluateContractObligationChecks(gateName, ctx, cfg, root, targetDir)
}

// evaluateContractObligationChecks enforces bijective contract struct parity, executable TC-*
// bindings, orphan-free tests, normative surjection, and stale-digest freshness.
func evaluateContractObligationChecks(gateName string, ctx *vscope.VerifyContext, cfg *config.EcosystemConfig, root, targetDir string) *vscope.GateResult {
	contracts := cfg.Governance.QualityGates.GetContractsBoundary().Contracts
	if !contracts.Enabled || isIntentPlaneTask(ctx) {
		return nil
	}
	activeTaskID, scanRoot := resolveContractScanScope(ctx, root)
	siblingDelta := collectActiveSiblingTwinDeltas(scanRoot, activeTaskID)
	deferredSpecs := collectActiveSiblingTwinSpecs(scanRoot, activeTaskID)
	if contracts.StrictTypes {
		if sigRes := checkContractTypeParityWithDelta(gateName, scanRoot, targetDir, contracts.SpecRoots, siblingDelta); sigRes != nil {
			return sigRes
		}
		if strings.EqualFold(strings.TrimSpace(cfg.Project), "nomos") {
			if surjRes := checkNormativeRegistrySurjection(gateName, scanRoot, contracts.SpecRoots); surjRes != nil {
				return surjRes
			}
		}
	}
	if contracts.VerifyObligations {
		if oblRes := checkExecutableObligationsWithDelta(gateName, scanRoot, contracts.SpecRoots, siblingDelta); oblRes != nil {
			return oblRes
		}
	}
	if contracts.FailOnStaleObligations {
		return checkStaleSpecObligations(gateName, ctx, root, cfg.Governance.QualityGates.IntentSourceRoots, deferredSpecs)
	}
	return nil
}

// resolveContractScanScope determines the active SUB-* task key and filesystem scan root.
func resolveContractScanScope(ctx *vscope.VerifyContext, root string) (string, string) {
	if ctx == nil {
		return extractWorktreeSubTaskKey(root), root
	}
	scanRoot := root
	if ctx.WorktreePath != "" {
		scanRoot = ctx.WorktreePath
	}
	taskID := strings.TrimSpace(ctx.TaskID)
	if taskID == "" {
		taskID = extractWorktreeSubTaskKey(scanRoot)
	}
	return taskID, scanRoot
}

func checkContractTypeParityWithDelta(gateName, root, targetDir string, specRoots []string, delta *impact.SiblingDelta) *vscope.GateResult {
	sigIssues := verifySpecTypeSignaturesWithDelta(root, targetDir, specRoots, delta)
	if len(sigIssues) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 4, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("specification contract signature drift: %s", strings.Join(sigIssues, "; ")),
		Remedy:  "align source struct/interface/class fields and types bijectively with specification contract blocks (Law 4)",
	}
}

func checkNormativeRegistrySurjection(gateName, root string, specRoots []string) *vscope.GateResult {
	surjIssues := VerifyNormativeRegistrySurjection(root, specRoots)
	if len(surjIssues) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 4, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("undocumented normative substrate surface: %s", strings.Join(surjIssues, "; ")),
		Remedy:  "document every Gate ProbeID, core/config struct, CLI --flag, and ERR_* code in canonical Layer 2 specs (Law 4 & Law 6)",
	}
}

func checkExecutableObligationsWithDelta(gateName, root string, specRoots []string, delta *impact.SiblingDelta) *vscope.GateResult {
	oblIssues := verifyExecutableObligationsWithDelta(root, specRoots, delta)
	if len(oblIssues) > 0 {
		return &vscope.GateResult{
			GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 4, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("unbound specification obligation vector(s): %s", strings.Join(oblIssues, "; ")),
			Remedy:  "bind each TC-* vector in a unit test via '// @verifies <TC-ID>' (or '# @verifies <TC-ID>') (Law 2 & Law 4)",
		}
	}
	orphanIssues := VerifyNoOrphanedTestObligations(root, specRoots)
	if len(orphanIssues) > 0 {
		return &vscope.GateResult{
			GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 4, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("orphaned @verifies test obligation vector(s) absent from specs: %s", strings.Join(orphanIssues, "; ")),
			Remedy:  "declare each @verifies TC-* vector in its canonical Layer 2 specification table or remove stale test tags (Law 2 & Law 4)",
		}
	}
	return nil
}

func isIntentPlaneTask(ctx *vscope.VerifyContext) bool {
	return ctx != nil && strings.HasPrefix(strings.TrimSpace(ctx.TaskID), "INT-")
}

// checkStaleSpecObligations reports specifications whose SHA-256 digest drifted since release sealing.
func checkStaleSpecObligations(gateName string, ctx *vscope.VerifyContext, root string, intentRoots []string, deferredSpecs map[string]bool) *vscope.GateResult {
	stale, err := task.ComputeStaleSpecObligations(root, "")
	if err != nil || len(stale) == 0 {
		return nil
	}
	inFlightSpecs := collectInFlightSubSpecDigests(ctx, root, intentRoots)
	var items []string
	for _, s := range stale {
		if !inFlightSpecs[s.SpecPath] && !deferredSpecs[s.SpecPath] {
			items = append(items, fmt.Sprintf("%s (last verified by %s)", s.SpecPath, s.LastVerifiedBy))
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 4, Ring: RingCryptographic,
		Passed:  false,
		Message: fmt.Sprintf("stale specification obligation(s) detected: %s", strings.Join(items, "; ")),
		Remedy:  "execute and sync a downstream SUB-* task referencing the updated specification to refresh spec_digests in refs/notes/nomos/releases (Law 4)",
	}
}

func collectInFlightSubSpecDigests(ctx *vscope.VerifyContext, root string, intentRoots []string) map[string]bool {
	if ctx == nil || !strings.HasPrefix(strings.TrimSpace(ctx.TaskID), "SUB-") {
		return nil
	}
	wt := ctx.WorktreePath
	if wt == "" {
		wt = root
	}
	modified := vscope.GetModifiedFilesInScope(ctx, wt)
	if len(modified) == 0 {
		return nil
	}
	digests := task.CollectReferencedSpecDigestsWithRoots(wt, modified, "", intentRoots)
	active := make(map[string]bool, len(digests))
	for k := range digests {
		active[k] = true
	}
	return active
}

func evaluateConfiguredArchitectureBoundaries(gateName string, ctx *vscope.VerifyContext, root string, rules []config.ArchitectureBoundaryRule) *vscope.GateResult {
	scanRoot := root
	if ctx != nil && ctx.WorktreePath != "" {
		scanRoot = ctx.WorktreePath
	}
	violations := impact.EvaluateArchitectureBoundaries(scanRoot, nil, rules)
	if len(violations) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: probeIDIntentTraceability, Law: 7, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("architecture boundary violation(s): %s", strings.Join(violations, "; ")),
		Remedy:  "remove forbidden cross-layer imports or route calls through an allowed interface package per nomos.yaml architecture_boundaries (Law 7)",
	}
}
