// Package capbac implements SHA-256 MCP tool-catalog pinning (ASC-PIN-01) and
// shadow .mcp.json approval verification for PreToolUse boundaries.
package capbac

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MCPCatalogEntry describes one MCP tool descriptor for SHA-256 catalog pinning.
// @intent nomos_hermetic_worktree_spec.md #MCPInvocation
type MCPCatalogEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Approved    string `json:"approved"`
}

// ComputeMCPCatalogDigest computes a deterministic SHA-256 hex digest over sorted catalog entries.
// @intent nomos_hermetic_worktree_spec.md #MCPInvocation
func ComputeMCPCatalogDigest(entries []MCPCatalogEntry) string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		normApproved := normalizeApprovedFlag(e.Approved)
		lines = append(lines, fmt.Sprintf("%s|%s|%s", strings.TrimSpace(e.Name), strings.TrimSpace(e.Description), normApproved))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func normalizeApprovedFlag(raw string) string {
	low := strings.ToLower(strings.TrimSpace(raw))
	if low == "" || low == "true" || low == "1" {
		return "true"
	}
	return "false"
}

func resolveEffectiveMCPToolLower(lowerTool, argsDigest string) string {
	if lowerTool != "invoke_mcp" || strings.TrimSpace(argsDigest) == "" {
		return lowerTool
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsDigest), &m); err != nil {
		return "mcp.unknown"
	}
	name, _ := m["name"].(string)
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return "mcp.unknown"
	}
	if IsMCPToolName(trimmed) {
		return trimmed
	}
	return "mcp." + trimmed
}

func evaluateMCPCatalogPin(workspaceDir string, inv MCPInvocation) (FlowDecision, bool) {
	targetName := resolveInvokedMCPCatalogName(inv)
	if dec, denied := checkShadowMCPConfigFile(workspaceDir, targetName, inv.Server); denied {
		return dec, true
	}
	entries, explicitPin, hasCatalog := parsePayloadMCPCatalog(inv.ArgsDigest)
	if !hasCatalog {
		return FlowDecision{}, false
	}
	liveDigest := ComputeMCPCatalogDigest(entries)
	expectedPin := resolvePinnedCatalogDigest(workspaceDir, explicitPin, liveDigest)
	if expectedPin != "" && liveDigest != expectedPin {
		return FlowDecision{
			Allowed: false,
			Reason:  "❌ [NOMOS ASC-PIN-01 INTERCEPT] MCP tool catalog SHA-256 digest mutated mid-session; rug-pull blocked fail-closed (Law 6 & Law 7).",
		}, true
	}
	return checkCatalogToolApproval(entries, targetName)
}

func resolveInvokedMCPCatalogName(inv MCPInvocation) string {
	if strings.ToLower(strings.TrimSpace(inv.RawToolName)) != "invoke_mcp" {
		return strings.ToLower(strings.TrimSpace(inv.RawToolName))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(inv.ArgsDigest), &m); err != nil {
		return ""
	}
	name, _ := m["name"].(string)
	return strings.ToLower(strings.TrimSpace(name))
}

func parsePayloadMCPCatalog(argsDigest string) ([]MCPCatalogEntry, string, bool) {
	if strings.TrimSpace(argsDigest) == "" {
		return nil, "", false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsDigest), &m); err != nil {
		return nil, "", false
	}
	pin, _ := m["pinned_catalog_sha256"].(string)
	rawCat, ok := m["mcp_catalog"].([]any)
	if !ok {
		return nil, strings.TrimSpace(pin), false
	}
	entries := make([]MCPCatalogEntry, 0, len(rawCat))
	for _, item := range rawCat {
		em, isMap := item.(map[string]any)
		if !isMap {
			continue
		}
		entries = append(entries, MCPCatalogEntry{
			Name:        stringifyCatalogField(em["name"]),
			Description: stringifyCatalogField(em["description"]),
			Approved:    stringifyCatalogField(em["approved"]),
		})
	}
	return entries, strings.TrimSpace(pin), true
}

func stringifyCatalogField(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func resolvePinnedCatalogDigest(workspaceDir, explicitPin, liveDigest string) string {
	if explicitPin != "" {
		return explicitPin
	}
	if strings.TrimSpace(workspaceDir) == "" {
		return ""
	}
	pinFile := filepath.Join(workspaceDir, ".nomos", "mcp_catalog.sha256")
	if data, err := os.ReadFile(pinFile); err == nil {
		if saved := strings.TrimSpace(string(data)); saved != "" {
			return saved
		}
	}
	if mkErr := os.MkdirAll(filepath.Dir(pinFile), 0755); mkErr != nil {
		return ""
	}
	if wErr := os.WriteFile(pinFile, []byte(liveDigest+"\n"), 0600); wErr != nil {
		return ""
	}
	return liveDigest
}

func checkCatalogToolApproval(entries []MCPCatalogEntry, targetName string) (FlowDecision, bool) {
	if targetName == "" || len(entries) == 0 {
		return FlowDecision{}, false
	}
	for _, e := range entries {
		if strings.EqualFold(strings.TrimSpace(e.Name), targetName) {
			if normalizeApprovedFlag(e.Approved) != "true" {
				return FlowDecision{
					Allowed: false,
					Reason:  fmt.Sprintf("❌ [NOMOS ASC-PIN-01 INTERCEPT] Unapproved MCP tool '%s' blocked by catalog approval policy (Law 6 & Law 7).", targetName),
				}, true
			}
			return FlowDecision{}, false
		}
	}
	return FlowDecision{
		Allowed: false,
		Reason:  fmt.Sprintf("❌ [NOMOS ASC-PIN-01 INTERCEPT] Undeclared shadow MCP tool '%s' is absent from pinned tool catalog (Law 6 & Law 7).", targetName),
	}, true
}

func checkShadowMCPConfigFile(workspaceDir, targetName, server string) (FlowDecision, bool) {
	if strings.TrimSpace(workspaceDir) == "" {
		return FlowDecision{}, false
	}
	data, err := os.ReadFile(filepath.Join(workspaceDir, ".mcp.json"))
	if err != nil {
		return FlowDecision{}, false
	}
	var root map[string]any
	if uErr := json.Unmarshal(data, &root); uErr != nil {
		return FlowDecision{}, false
	}
	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		return FlowDecision{}, false
	}
	for srvName, rawCfg := range servers {
		cfgMap, isMap := rawCfg.(map[string]any)
		if !isMap {
			continue
		}
		if normalizeApprovedFlag(stringifyCatalogField(cfgMap["approved"])) == "true" {
			continue
		}
		lowSrv := strings.ToLower(strings.TrimSpace(srvName))
		if lowSrv != "" && (lowSrv == server || strings.Contains(targetName, lowSrv)) {
			return FlowDecision{
				Allowed: false,
				Reason:  fmt.Sprintf("❌ [NOMOS ASC-PIN-01 INTERCEPT] Unapproved shadow server '%s' in .mcp.json blocked fail-closed (Law 6 & Law 7).", srvName),
			}, true
		}
	}
	return FlowDecision{}, false
}
