// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/telemetry"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/spf13/cobra"
)

// CapabilityItem represents a single authoritative capability in the Nomos ecosystem.
type CapabilityItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`       // "hook", "directive", "engine", "gate", "cli"
	Plane           string   `json:"plane"`      // "intent", "substrate"
	Invocation      string   `json:"invocation"` // CLI or lifecycle event syntax
	Description     string   `json:"description"`
	Status          string   `json:"status"` // "AVAILABLE", "GROUNDED", "ACTIVE"
	InvocationCount int      `json:"invocation_count"`
	LastAccessedAt  string   `json:"last_accessed_at,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

// CapabilitiesCatalog encapsulates the full set of ecosystem capabilities with summary counts.
type CapabilitiesCatalog struct {
	TotalCount      int              `json:"total_count"`
	HooksCount      int              `json:"hooks_count"`
	DirectivesCount int              `json:"directives_count"`
	EnginesCount    int              `json:"engines_count"`
	GatesCount      int              `json:"gates_count"`
	CLICount        int              `json:"cli_count"`
	Items           []CapabilityItem `json:"items"`
}

// NewCapabilitiesCmd constructs the canonical 'nomos capabilities' command.
func NewCapabilitiesCmd() *cobra.Command {
	var (
		jsonOutput bool
		filterType string
	)

	cmd := &cobra.Command{
		Use:   "capabilities [id]",
		Short: "List and inspect authoritative ambient hooks, directives, engines, and quality gates",
		Long: `Authoritative catalog of Nomos capabilities across Intent and Substrate planes.
Surfaces ambient lifecycle hooks, JIT phase directives, architectural engines,
the 7x3 Invariant Cells (Definition of Done quality gates), and top-level CLI commands.
Pass a capability, cell, or gate probe ID (e.g. 'nomos capabilities plane_separation') to inspect full remediation details.`,
		Example: tui.HelpCapabilitiesExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return executeCapabilityDetail(cmd, args[0], jsonOutput)
			}
			return executeCapabilities(cmd, filterType, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output capabilities catalog as machine-readable JSON")
	cmd.Flags().StringVar(&filterType, "type", "", "Filter capabilities by type (hook, directive, engine, gate, cli, all)")
	return cmd
}

func executeCapabilities(cmd *cobra.Command, filterType string, jsonOutput bool) error {
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
	filtered, fErr := filterCapabilities(catalog, filterType)
	if fErr != nil {
		return fErr
	}

	if jsonOutput {
		outCatalog := catalog
		outCatalog.Items = filtered
		outCatalog.TotalCount = len(filtered)
		data, mErr := json.MarshalIndent(outCatalog, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling capabilities JSON: %v", mErr)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	renderCapabilitiesUI(cmd, catalog, filtered, filterType)
	return nil
}

func collectAllCapabilities(cmd *cobra.Command, repoRoot string) CapabilitiesCatalog {
	hooks := collectHookCapabilities(repoRoot)
	directives := collectDirectiveCapabilities(repoRoot)
	engines := collectEngineCapabilities(repoRoot)
	gates := collectGateCapabilities()
	clis := collectCLICapabilities(cmd)

	var items []CapabilityItem
	items = append(items, hooks...)
	items = append(items, directives...)
	items = append(items, engines...)
	items = append(items, gates...)
	items = append(items, clis...)

	enrichCapabilityMetrics(items)

	return CapabilitiesCatalog{
		TotalCount:      len(items),
		HooksCount:      len(hooks),
		DirectivesCount: len(directives),
		EnginesCount:    len(engines),
		GatesCount:      len(gates),
		CLICount:        len(clis),
		Items:           items,
	}
}

func enrichCapabilityMetrics(items []CapabilityItem) {
	metrics, err := telemetry.GetCapabilityMetrics("")
	if err != nil || len(metrics) == 0 {
		return
	}
	for i := range items {
		if m, ok := metrics[items[i].ID]; ok {
			items[i].InvocationCount = m.InvocationCount
			items[i].LastAccessedAt = m.LastAccessedAt
		}
	}
}

type hookEntrySpec struct {
	Enabled      *bool             `json:"enabled"`
	SessionStart []hookCommandSpec `json:"SessionStart"`
	PreToolUse   []hookMatcherSpec `json:"PreToolUse"`
}

type hookMatcherSpec struct {
	Matcher string            `json:"matcher"`
	Hooks   []hookCommandSpec `json:"hooks"`
}

type hookCommandSpec struct {
	Command string `json:"command"`
}

func collectHookCapabilities(repoRoot string) []CapabilityItem {
	if repoRoot != "" {
		manifestPath := filepath.Join(repoRoot, ".agents", "hooks.json")
		if data, err := os.ReadFile(manifestPath); err == nil {
			if items := parseHookManifest(data); len(items) > 0 {
				return items
			}
		}
	}
	if data, err := tui.ReadEmbeddedTemplate("templates/hooks/hooks.json"); err == nil {
		return parseHookManifest(data)
	}
	return nil
}

func parseHookManifest(data []byte) []CapabilityItem {
	var raw map[string]hookEntrySpec
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var items []CapabilityItem
	for name, spec := range raw {
		status := "ACTIVE"
		if spec.Enabled != nil && !*spec.Enabled {
			status = "DISABLED"
		}
		event, desc := describeHookSpec(spec)
		items = append(items, CapabilityItem{
			ID:          "hook-" + name,
			Name:        name,
			Type:        "hook",
			Plane:       "substrate",
			Invocation:  event,
			Description: desc,
			Status:      status,
			Tags:        []string{"ambient_hook"},
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func describeHookSpec(spec hookEntrySpec) (string, string) {
	if len(spec.SessionStart) > 0 {
		return "SessionStart", "Script: " + spec.SessionStart[0].Command
	}
	if len(spec.PreToolUse) > 0 && len(spec.PreToolUse[0].Hooks) > 0 {
		return "PreToolUse (" + spec.PreToolUse[0].Matcher + ")", "Script: " + spec.PreToolUse[0].Hooks[0].Command
	}
	return "LifecycleHook", "Ambient lifecycle defense hook"
}

func collectDirectiveCapabilities(repoRoot string) []CapabilityItem {
	dirs, err := handshake.ListDirectives(repoRoot)
	if err != nil {
		return nil
	}
	var items []CapabilityItem
	for _, d := range dirs {
		base := strings.TrimSuffix(d.Name, filepath.Ext(d.Name))
		var tags []string
		for _, p := range d.Phases {
			tags = append(tags, "phase:"+string(p))
		}
		items = append(items, CapabilityItem{
			ID:          "dir-" + base,
			Name:        d.Title,
			Type:        "directive",
			Plane:       "intent",
			Invocation:  "nomos directive " + base,
			Description: d.Summary,
			Status:      "ACTIVE",
			Tags:        tags,
		})
	}
	return items
}

func collectEngineCapabilities(repoRoot string) []CapabilityItem {
	engines := verify.AuditArchitecturalEngines(repoRoot)
	inSourceTree := hasSubstrateSource(repoRoot)
	var items []CapabilityItem
	for _, eng := range engines {
		status := "GROUNDED"
		if inSourceTree && !eng.IsImplemented {
			status = "UNGROUNDED"
		}
		slug := strings.ToLower(eng.Name)
		slug = strings.ReplaceAll(slug, " ", "-")
		slug = strings.ReplaceAll(slug, "(", "")
		slug = strings.ReplaceAll(slug, ")", "")
		items = append(items, CapabilityItem{
			ID:          "eng-" + slug,
			Name:        eng.Name,
			Type:        "engine",
			Plane:       "substrate",
			Invocation:  eng.EnforcedBy,
			Description: fmt.Sprintf("Charter: %s", eng.IntentSpec),
			Status:      status,
			Tags:        []string{"spec:" + eng.IntentSpec},
		})
	}
	return items
}

func hasSubstrateSource(dir string) bool {
	if dir == "" {
		return false
	}
	fi, err := os.Stat(filepath.Join(dir, "substrate"))
	return err == nil && fi.IsDir()
}

func collectGateCapabilities() []CapabilityItem {
	cells := verify.DefaultBaselineCells()
	var items []CapabilityItem
	for _, cell := range cells {
		items = append(items, CapabilityItem{
			ID:          cell.ID,
			Name:        cell.Name,
			Type:        "gate",
			Plane:       "substrate",
			Invocation:  fmt.Sprintf("nomos verify --law %d", cell.Law),
			Description: fmt.Sprintf("Law %d (%s) Ring: %s", cell.Law, cell.Domain, cell.Ring),
			Status:      "ACTIVE",
			Tags:        []string{fmt.Sprintf("law:%d", cell.Law), "ring:" + cell.Ring, "phase:" + cell.Phase},
		})
	}
	return items
}

func collectCLICapabilities(cmd *cobra.Command) []CapabilityItem {
	if cmd == nil {
		return nil
	}
	root := cmd.Root()
	var items []CapabilityItem
	for _, sub := range root.Commands() {
		if sub.Hidden || sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		items = append(items, CapabilityItem{
			ID:          "cli-" + sub.Name(),
			Name:        sub.Name(),
			Type:        "cli",
			Plane:       "substrate",
			Invocation:  "nomos " + sub.Name(),
			Description: sub.Short,
			Status:      "ACTIVE",
			Tags:        []string{"group:" + sub.GroupID},
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func filterCapabilities(catalog CapabilitiesCatalog, filter string) ([]CapabilityItem, error) {
	norm := strings.ToLower(strings.TrimSpace(filter))
	if norm == "" || norm == "all" {
		return catalog.Items, nil
	}
	switch norm {
	case "hook", "hooks":
		norm = "hook"
	case "directive", "directives":
		norm = "directive"
	case "engine", "engines":
		norm = "engine"
	case "gate", "gates":
		norm = "gate"
	case "cli", "command", "commands":
		norm = "cli"
	default:
		return nil, types.NewNomosError(
			types.ErrCodeValidationFailed,
			"unknown capability type %q (valid types: hook, directive, engine, gate, cli, all)",
			filter,
		)
	}

	var matched []CapabilityItem
	for _, it := range catalog.Items {
		if it.Type == norm {
			matched = append(matched, it)
		}
	}
	return matched, nil
}
