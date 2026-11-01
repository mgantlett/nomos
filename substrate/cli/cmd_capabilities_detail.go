// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

// CapabilityDetail models single-item capability and gate remediation drill-down (#CapabilityDetail).
type CapabilityDetail struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Plane       string   `json:"plane"`
	Status      string   `json:"status"`
	Law         int      `json:"law,omitempty"`
	Ring        string   `json:"ring,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Invocation  string   `json:"invocation"`
	Description string   `json:"description"`
	Remedy      string   `json:"remedy,omitempty"`
	ProbeIDs    []string `json:"probe_ids,omitempty"`
}

// ResolveCapabilityDetail resolves a capability, invariant cell, or subordinate gate probe ID.
// @intent INT-842 #CapabilityDetail
func ResolveCapabilityDetail(catalog CapabilitiesCatalog, query string) (*CapabilityDetail, error) {
	target := strings.ToLower(strings.TrimSpace(query))
	if target == "" {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "capability identifier cannot be empty").
			WithRemedy("run 'nomos capabilities' to list valid capability, cell, and gate probe IDs")
	}

	if cellDetail := matchGateOrProbeDetail(target); cellDetail != nil {
		return cellDetail, nil
	}

	if itemDetail := matchCatalogItemDetail(catalog, target); itemDetail != nil {
		return itemDetail, nil
	}

	return nil, types.NewNomosError(types.ErrCodeValidationFailed, "capability or gate probe %q not found", query).
		WithRemedy("run 'nomos capabilities' or 'nomos capabilities --type=gate' to list valid IDs")
}

func matchGateOrProbeDetail(target string) *CapabilityDetail {
	cells := verify.DefaultBaselineCells()
	for _, cell := range cells {
		if strings.ToLower(cell.ID) == target || strings.ToLower(cell.Name) == target {
			return buildCellCapabilityDetail(cell, cell.ID)
		}
		for _, probe := range cell.ProbeIDs {
			if strings.ToLower(probe) == target {
				return buildCellCapabilityDetail(cell, probe)
			}
		}
	}
	return nil
}

func buildCellCapabilityDetail(cell verify.InvariantCellSpec, matchedID string) *CapabilityDetail {
	lawDesc := fmt.Sprintf("Law %d (%s) · Ring: %s", cell.Law, cell.Domain, cell.Ring)
	if cell.Law >= 1 && cell.Law <= len(verify.CanonicalLaws) {
		lawInfo := verify.CanonicalLaws[cell.Law-1]
		lawDesc = fmt.Sprintf("Law %d (%s): %s", lawInfo.Law, lawInfo.Name, lawInfo.Description)
	}
	remedy := cell.Remedy
	if matchedID == "plane_separation" {
		remedy = verify.PlaneSeparationRemedy("SUB-1")
	}
	return &CapabilityDetail{
		ID:          matchedID,
		Name:        cell.Name,
		Type:        "gate",
		Plane:       "substrate",
		Status:      "ACTIVE",
		Law:         cell.Law,
		Ring:        cell.Ring,
		Domain:      cell.Domain,
		Invocation:  "nomos verify",
		Description: lawDesc,
		Remedy:      remedy,
		ProbeIDs:    cell.ProbeIDs,
	}
}

func matchCatalogItemDetail(catalog CapabilitiesCatalog, target string) *CapabilityDetail {
	for _, it := range catalog.Items {
		idLower := strings.ToLower(it.ID)
		nameLower := strings.ToLower(it.Name)
		trimmedID := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(idLower, "dir-"), "hook-"), "cli-")
		if idLower == target || nameLower == target || trimmedID == target || strings.HasPrefix(idLower, target) || strings.HasPrefix(trimmedID, target) {
			return &CapabilityDetail{
				ID:          it.ID,
				Name:        it.Name,
				Type:        it.Type,
				Plane:       it.Plane,
				Status:      it.Status,
				Invocation:  it.Invocation,
				Description: it.Description,
			}
		}
	}
	return nil
}

func executeCapabilityDetail(cmd *cobra.Command, query string, jsonOutput bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed resolving working directory: %v", err)
	}
	clean := filepath.ToSlash(repoRoot)
	if !strings.Contains(clean, "/worktrees/") && !strings.HasPrefix(clean, "worktrees/") {
		if rRoot, rErr := workspace.FindRepoRoot(repoRoot); rErr == nil && rRoot != "" {
			repoRoot = rRoot
		}
	}

	catalog := collectAllCapabilities(cmd, repoRoot)
	detail, dErr := ResolveCapabilityDetail(catalog, query)
	if dErr != nil {
		return dErr
	}

	out := cmd.OutOrStdout()
	if jsonOutput {
		data, mErr := json.MarshalIndent(detail, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling capability detail JSON: %v", mErr)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	renderCapabilityDetailCard(cmd, detail)
	return nil
}

func renderCapabilityDetailCard(cmd *cobra.Command, d *CapabilityDetail) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, capTitleStyle.Render("🔎 Nomos Capability & Invariant Inspector"))
	fmt.Fprintln(out, capLineStyle.Render("────────────────────────────────────────────────────────────"))
	fmt.Fprintf(out, "  ID          : %s\n", d.ID)
	fmt.Fprintf(out, "  Name        : %s\n", d.Name)
	fmt.Fprintf(out, "  Type        : %s (%s plane) [%s]\n", d.Type, d.Plane, d.Status)
	if d.Law > 0 {
		fmt.Fprintf(out, "  Law & Ring  : Law %d · %s · %s\n", d.Law, d.Domain, d.Ring)
	}
	fmt.Fprintf(out, "  Invocation  : %s\n", d.Invocation)
	if d.Description != "" {
		fmt.Fprintf(out, "  Invariant   : %s\n", d.Description)
	}
	if len(d.ProbeIDs) > 0 {
		fmt.Fprintf(out, "  Probes      : %s\n", strings.Join(d.ProbeIDs, ", "))
	}
	if d.Remedy != "" {
		fmt.Fprintf(out, "  Remedy      : %s\n", d.Remedy)
	}
	fmt.Fprintln(out, capLineStyle.Render("────────────────────────────────────────────────────────────"))
}
