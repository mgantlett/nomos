// Package verify orchestrates configured obligation, contract signature, stale receipt,
// and directional architecture boundary checks inside IntentTraceabilityGate.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
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

// evaluateContractObligationChecks enforces strict contract struct parity, executable TC-*
// bindings, and stale-digest freshness while isolating concurrent sibling SUB-* twins.
func evaluateContractObligationChecks(gateName string, ctx *vscope.VerifyContext, cfg *config.EcosystemConfig, root, targetDir string) *vscope.GateResult {
	contracts := cfg.Governance.QualityGates.GetContractsBoundary().Contracts
	if !contracts.Enabled || isIntentPlaneTask(ctx) {
		return nil
	}
	activeTaskID, scanRoot := resolveContractScanScope(ctx, root)
	deferredSpecs := collectActiveSiblingTwinSpecs(scanRoot, activeTaskID)
	effectiveSpecs, hasDeferred := filterDeferredSpecRoots(scanRoot, contracts.SpecRoots, deferredSpecs)
	hasSpecs := !hasDeferred || len(effectiveSpecs) > 0
	if contracts.StrictTypes && hasSpecs {
		if sigRes := checkContractTypeParity(gateName, scanRoot, targetDir, effectiveSpecs); sigRes != nil {
			return sigRes
		}
	}
	if contracts.VerifyObligations && hasSpecs {
		if oblRes := checkExecutableObligations(gateName, scanRoot, effectiveSpecs); oblRes != nil {
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

// checkContractTypeParity verifies fenced spec structs match compiled source AST declarations.
func checkContractTypeParity(gateName, root, targetDir string, specRoots []string) *vscope.GateResult {
	sigIssues := VerifySpecTypeSignatures(root, targetDir, specRoots)
	if len(sigIssues) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("specification contract signature drift: %s", strings.Join(sigIssues, "; ")),
		Remedy:  "align source struct/interface/class fields with specification contract blocks (Law 4)",
	}
}

// checkExecutableObligations verifies every table-declared TC-* vector is bound in tests.
func checkExecutableObligations(gateName, root string, specRoots []string) *vscope.GateResult {
	oblIssues := VerifyExecutableObligations(root, specRoots)
	if len(oblIssues) == 0 {
		return nil
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: "intent_traceability", Law: 4, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("unbound specification obligation vector(s): %s", strings.Join(oblIssues, "; ")),
		Remedy:  "bind each TC-* vector in a unit test via '// @verifies <TC-ID>' (or '# @verifies <TC-ID>') (Law 2 & Law 4)",
	}
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
		GateName: gateName, ProbeID: "intent_traceability", Law: 4, Ring: RingCryptographic,
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
		GateName: gateName, ProbeID: "intent_traceability", Law: 7, Ring: RingSyntactic,
		Passed:  false,
		Message: fmt.Sprintf("architecture boundary violation(s): %s", strings.Join(violations, "; ")),
		Remedy:  "remove forbidden cross-layer imports or route calls through an allowed interface package per nomos.yaml architecture_boundaries (Law 7)",
	}
}
