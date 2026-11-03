// Package capbac implements Capability-Based Access Control (CapBAC), compositional
// capability-flow taint tracking, and zero-trust boundary contracts.
package capbac

import (
	"fmt"
	"regexp"
	"strings"
)

// MCPAccessMode classifies the side-effect level of an MCP tool invocation.
type MCPAccessMode string

const (
	// MCPModeRead designates side-effect-free MCP inspection or query calls.
	MCPModeRead MCPAccessMode = "read"
	// MCPModeMutate designates state-changing or outbound egress MCP calls.
	MCPModeMutate MCPAccessMode = "mutate"
	// MCPModePrivileged designates high-impact remote merge, publish, delete, or DDL calls.
	MCPModePrivileged MCPAccessMode = "privileged"
)

var (
	privilegedMCPTokens = map[string]bool{
		"merge": true, "push": true, "publish": true, "delete": true,
		"remove": true, "drop": true, "truncate": true, "alter": true,
		"deploy": true, "exec": true, "execute": true,
	}
	mutatingMCPTokens = map[string]bool{
		"write": true, "edit": true, "replace": true, "create": true,
		"update": true, "patch": true, "post": true, "send": true,
		"upload": true, "commit": true, "fork": true, "close": true,
		"reopen": true, "assign": true, "comment": true, "insert": true,
	}
	untrustedIngressMCPServers = map[string]bool{
		"fetch": true, "webfetch": true, "browser": true, "puppeteer": true,
		"playwright": true, "firecrawl": true, "brave-search": true,
		"brave": true, "exa": true, "tavily": true, "searxng": true,
	}
	privilegedSQLRe = regexp.MustCompile(`(?i)\b(?:DROP|TRUNCATE|ALTER|GRANT|REVOKE)\s+`)
	mutatingSQLRe   = regexp.MustCompile(`(?i)\b(?:INSERT|UPDATE|DELETE)\s+`)
	jsonPathTokenRe = regexp.MustCompile(`"([^"]*(?:\.env(?:\.[A-Za-z0-9_-]+)?|\.aws/credentials|\.kube/config|\.ssh/id_[A-Za-z0-9_]+)[^"]*)"`)
)

// MCPInvocation captures normalized server, action, and access-mode coordinates for an MCP tool call.
type MCPInvocation struct {
	RawToolName string        `json:"raw_tool_name"`
	Server      string        `json:"server"`
	Action      string        `json:"action"`
	Mode        MCPAccessMode `json:"mode"`
	TargetFile  string        `json:"target_file,omitempty"`
	ArgsDigest  string        `json:"args_digest,omitempty"`
}

// MCPBoundaryPolicy defines optional allowlist and denylist server constraints from nomos.yaml.
type MCPBoundaryPolicy struct {
	AllowedServers []string
	BlockedServers []string
}

// IsMCPToolName reports whether name uses an MCP tool namespace ('mcp__*' or 'mcp_*').
// @intent zero_trust_mcp_ifc_spec.md #MCPInvocation
func IsMCPToolName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "mcp__") || strings.HasPrefix(lower, "mcp_")
}

// ParseMCPInvocation parses Claude Code ('mcp__<server>__<action>') and Cursor/AGY
// ('mcp_<server>_<action>') tool names into canonical MCPInvocation coordinates.
// @intent zero_trust_mcp_ifc_spec.md #MCPInvocation
func ParseMCPInvocation(toolName, targetFile, argsDigest string) (MCPInvocation, bool) {
	trimmed := strings.TrimSpace(toolName)
	lower := strings.ToLower(trimmed)
	if !IsMCPToolName(lower) {
		return MCPInvocation{}, false
	}

	server, action := splitMCPServerAndAction(lower)
	if server == "" {
		return MCPInvocation{}, false
	}
	mode := classifyMCPAccessMode(action, argsDigest)
	return MCPInvocation{
		RawToolName: trimmed,
		Server:      server,
		Action:      action,
		Mode:        mode,
		TargetFile:  strings.TrimSpace(targetFile),
		ArgsDigest:  argsDigest,
	}, true
}

func splitMCPServerAndAction(lower string) (string, string) {
	if strings.HasPrefix(lower, "mcp__") {
		rest := strings.TrimPrefix(lower, "mcp__")
		parts := strings.SplitN(rest, "__", 2)
		if len(parts) == 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
		return parts[0], ""
	}
	rest := strings.TrimPrefix(lower, "mcp_")
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) == 2 && parts[0] != "" {
		return parts[0], parts[1]
	}
	return parts[0], ""
}

func classifyMCPAccessMode(action, argsDigest string) MCPAccessMode {
	tokens := strings.FieldsFunc(action, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/'
	})
	if strings.Contains(action, "query") || strings.Contains(action, "sql") {
		if privilegedSQLRe.MatchString(argsDigest) {
			return MCPModePrivileged
		}
		if mutatingSQLRe.MatchString(argsDigest) {
			return MCPModeMutate
		}
	}
	for _, tok := range tokens {
		if privilegedMCPTokens[tok] {
			return MCPModePrivileged
		}
	}
	for _, tok := range tokens {
		if mutatingMCPTokens[tok] {
			return MCPModeMutate
		}
	}
	return MCPModeRead
}

// EvaluateMCPBoundary enforces server allow/block policy, credential payload scanning,
// and cross-server Information Flow Control (IFC) against the workspace CapabilityFlowGraph.
// @intent zero_trust_mcp_ifc_spec.md #MCPInvocation
func EvaluateMCPBoundary(workspaceDir string, inv MCPInvocation, policy MCPBoundaryPolicy) FlowDecision {
	if dec, denied := checkMCPServerPolicy(inv, policy); denied {
		return dec
	}
	if dec, denied := checkMCPPayloadSecrets(inv); denied {
		return dec
	}
	return evaluateMCPFlowGraph(workspaceDir, inv)
}

func checkMCPServerPolicy(inv MCPInvocation, policy MCPBoundaryPolicy) (FlowDecision, bool) {
	for _, blocked := range policy.BlockedServers {
		if strings.EqualFold(strings.TrimSpace(blocked), inv.Server) {
			return FlowDecision{
				Allowed: false,
				Reason:  fmt.Sprintf("❌ [NOMOS MCP BOUNDARY INTERCEPT] MCP server '%s' is denied by blocked_mcp_servers policy in nomos.yaml (Law 7).", inv.Server),
			}, true
		}
	}
	if len(policy.AllowedServers) == 0 {
		return FlowDecision{}, false
	}
	for _, allowed := range policy.AllowedServers {
		if strings.EqualFold(strings.TrimSpace(allowed), inv.Server) {
			return FlowDecision{}, false
		}
	}
	return FlowDecision{
		Allowed: false,
		Reason:  fmt.Sprintf("❌ [NOMOS MCP BOUNDARY INTERCEPT] MCP server '%s' is not listed in allowed_mcp_servers policy in nomos.yaml (Law 7).", inv.Server),
	}, true
}

func checkMCPPayloadSecrets(inv MCPInvocation) (FlowDecision, bool) {
	if IsSensitiveCredentialPath(inv.TargetFile) {
		return FlowDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("❌ [NOMOS MCP BOUNDARY INTERCEPT] Sensitive credential file '%s' cannot be accessed or transmitted via MCP tool '%s' (Law 7).", inv.TargetFile, inv.RawToolName),
		}, true
	}
	if label := MatchSecretTokenLabel(inv.ArgsDigest); label != "" {
		return FlowDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("❌ [NOMOS MCP BOUNDARY INTERCEPT] Detected %s in MCP tool '%s' parameters (Law 7). Never transmit raw credentials across MCP server boundaries.", label, inv.RawToolName),
		}, true
	}
	if matches := jsonPathTokenRe.FindAllStringSubmatch(inv.ArgsDigest, -1); len(matches) > 0 {
		for _, m := range matches {
			if len(m) > 1 && IsSensitiveCredentialPath(m[1]) {
				return FlowDecision{
					Allowed: false,
					Reason:  fmt.Sprintf("❌ [NOMOS MCP BOUNDARY INTERCEPT] Sensitive credential path '%s' referenced in MCP tool '%s' arguments (Law 7).", m[1], inv.RawToolName),
				}, true
			}
		}
	}
	return FlowDecision{}, false
}

func evaluateMCPFlowGraph(workspaceDir string, inv MCPInvocation) FlowDecision {
	graph := LoadWorkspaceFlowGraph(workspaceDir)

	if inv.Mode != MCPModeRead {
		if edge, tainted := graph.HasTaint(TaintCredentialSource); tainted {
			return FlowDecision{
				Allowed:       false,
				ViolatedLabel: TaintCredentialSource,
				Reason: fmt.Sprintf(
					"❌ [NOMOS MCP IFC INTERCEPT] Mutating/egress MCP call '%s' (server '%s') blocked (Law 7): session carries active credential taint from '%s' (%s).",
					inv.RawToolName, inv.Server, edge.SourceTool, edge.Resource,
				),
			}
		}
	}

	if inv.Mode == MCPModePrivileged {
		if dec, blocked := checkPrivilegedMCPEscalation(&graph, inv); blocked {
			return dec
		}
	}

	if untrustedIngressMCPServers[inv.Server] {
		graph.AddTaint(TaintMCPUntrustedIngress, inv.RawToolName, inv.Server)
		if saveErr := SaveWorkspaceFlowGraph(workspaceDir, graph); saveErr != nil {
			return FlowDecision{Allowed: true, Role: FlowRoleSource}
		}
		return FlowDecision{Allowed: true, Role: FlowRoleSource}
	}

	return FlowDecision{Allowed: true, Role: FlowRoleNeutral}
}

func checkPrivilegedMCPEscalation(graph *CapabilityFlowGraph, inv MCPInvocation) (FlowDecision, bool) {
	if edge, tainted := graph.HasTaint(TaintMCPUntrustedIngress); tainted && !strings.EqualFold(edge.Resource, inv.Server) {
		return FlowDecision{
			Allowed:       false,
			ViolatedLabel: TaintMCPUntrustedIngress,
			Reason: fmt.Sprintf(
				"❌ [NOMOS MCP IFC INTERCEPT] Cross-server privileged escalation blocked (Law 1 & Law 7): untrusted ingress from MCP server '%s' cannot trigger privileged action '%s' on MCP server '%s'. Run 'nomos verify' to sanitize session state first.",
				edge.Resource, inv.Action, inv.Server,
			),
		}, true
	}
	if edge, tainted := graph.HasTaint(TaintUntrustedExternal); tainted {
		return FlowDecision{
			Allowed:       false,
			ViolatedLabel: TaintUntrustedExternal,
			Reason: fmt.Sprintf(
				"❌ [NOMOS MCP IFC INTERCEPT] Privileged MCP mutation '%s' blocked (Law 1 & Law 7): session ingested untrusted external input via '%s' (%s). Run 'nomos verify' to sanitize session state first.",
				inv.RawToolName, edge.SourceTool, edge.Resource,
			),
		}, true
	}
	return FlowDecision{}, false
}

