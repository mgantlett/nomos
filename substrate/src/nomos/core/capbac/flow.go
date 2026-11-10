// Package capbac implements Capability-Based Access Control (CapBAC), compositional
// capability-flow taint tracking, and zero-trust boundary contracts.
package capbac

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TaintLabel classifies untrusted or sensitive provenance edges in a session flow graph.
type TaintLabel string

const (
	// TaintUntrustedExternal marks sessions that ingested untrusted external network content.
	TaintUntrustedExternal TaintLabel = "untrusted_external"
	// TaintCredentialSource marks sessions that read sensitive local credential material.
	TaintCredentialSource TaintLabel = "credential_source"
	// TaintMCPUntrustedIngress marks sessions that ingested untrusted external MCP server content.
	TaintMCPUntrustedIngress TaintLabel = "mcp_untrusted_ingress"
)

// FlowRole classifies how an individual tool invocation interacts with the capability-flow graph.
type FlowRole string

const (
	FlowRoleNeutral              FlowRole = "neutral"
	FlowRoleSource               FlowRole = "source"
	FlowRoleSanitizer            FlowRole = "sanitizer"
	FlowRoleSinkShellEval        FlowRole = "sink_shell_eval"
	FlowRoleSinkPrivilegedRelease FlowRole = "sink_privileged_release"
	FlowRoleSinkExfiltration     FlowRole = "sink_exfiltration"
)

const maxActiveTaintEdges = 32

var (
	remoteShellPipeRe = regexp.MustCompile(`(?i)\b(?:curl|wget)\b[^;\n]*\|\s*(?:sudo\s+)?(?:ba)?sh\b`)
	remoteHTTPFetchRe = regexp.MustCompile(`(?i)\b(?:curl|wget)\b[^\n]*https?://([^\s/'"]+)`)
	curlExfilFlagRe   = regexp.MustCompile(`(?i)\bcurl\b[^\n]*(?:\s-(?:d|F|T)\b|--(?:data|data-raw|data-binary|form|upload-file)\b|-X\s*(?:POST|PUT|PATCH)\b)`)
	wgetExfilFlagRe   = regexp.MustCompile(`(?i)\bwget\b[^\n]*--post-(?:data|file)\b`)
	netExfilCmdRe     = regexp.MustCompile(`(?i)(?:^|[;&|]\s*)(?:scp|nc|ncat|netcat)\s+`)
	releaseSinkCmdRe  = regexp.MustCompile(`(?i)(?:^|[;&|]\s*)(?:git\s+push|gh\s+(?:release|pr\s+merge)|npm\s+publish|cargo\s+publish|twine\s+upload|docker\s+push)\b`)
	verifySanitizerRe = regexp.MustCompile(`(?:^|[;&|\s])nomos\s+(?:verify|task\s+review)\b`)

	externalFetchTools = map[string]bool{
		"webfetch": true, "web_search": true, "search_web": true,
		"fetch_url": true, "browse": true,
	}
	allowedTemplateEnvFiles = map[string]bool{
		".env.example": true, ".env.sample": true, ".env.template": true,
		".env.test": true, ".env.defaults": true, ".env.schema": true,
	}
	sensitiveBasenames = map[string]bool{
		".npmrc": true, ".netrc": true, ".pypirc": true,
		"id_rsa": true, "id_ed25519": true, "id_ecdsa": true,
	}
)

// TaintEdge records an active provenance taint edge inside a workspace session.
type TaintEdge struct {
	Label      TaintLabel `json:"label"`
	SourceTool string     `json:"source_tool"`
	Resource   string     `json:"resource"`
	RecordedAt string     `json:"recorded_at"`
}

// CapabilityFlowGraph stores active session taint edges for compositional PreToolUse enforcement.
type CapabilityFlowGraph struct {
	Version      int         `json:"version"`
	Workspace    string      `json:"workspace"`
	ActiveTaints []TaintEdge `json:"active_taints"`
}

// ToolFlowInvocation captures normalized tool metadata evaluated by CapabilityFlowGraph.
type ToolFlowInvocation struct {
	ToolName    string
	TargetFile  string
	CommandLine string
	ArgsDigest  string
}

// FlowDecision reports whether a tool invocation is permitted under active taint state.
type FlowDecision struct {
	Allowed       bool
	Role          FlowRole
	ViolatedLabel TaintLabel
	Reason        string
}

// HasTaint reports whether label is currently active in the flow graph.
// @intent nomos_hermetic_worktree_spec.md #CapabilityFlowGraph
func (g *CapabilityFlowGraph) HasTaint(label TaintLabel) (TaintEdge, bool) {
	if g == nil {
		return TaintEdge{}, false
	}
	for _, edge := range g.ActiveTaints {
		if edge.Label == label {
			return edge, true
		}
	}
	return TaintEdge{}, false
}

// AddTaint appends or refreshes a taint edge with bounded FIFO compaction.
// @intent nomos_hermetic_worktree_spec.md #TaintEdge
func (g *CapabilityFlowGraph) AddTaint(label TaintLabel, sourceTool, resource string) {
	if g == nil || label == "" {
		return
	}
	filtered := make([]TaintEdge, 0, len(g.ActiveTaints)+1)
	for _, existing := range g.ActiveTaints {
		if existing.Label == label && existing.SourceTool == sourceTool && existing.Resource == resource {
			continue
		}
		filtered = append(filtered, existing)
	}
	filtered = append(filtered, TaintEdge{
		Label:      label,
		SourceTool: sourceTool,
		Resource:   resource,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if len(filtered) > maxActiveTaintEdges {
		filtered = filtered[len(filtered)-maxActiveTaintEdges:]
	}
	g.ActiveTaints = filtered
}

// ClearTaint removes all active edges matching label.
func (g *CapabilityFlowGraph) ClearTaint(label TaintLabel) {
	if g == nil || len(g.ActiveTaints) == 0 {
		return
	}
	kept := make([]TaintEdge, 0, len(g.ActiveTaints))
	for _, edge := range g.ActiveTaints {
		if edge.Label != label {
			kept = append(kept, edge)
		}
	}
	g.ActiveTaints = kept
}

// EvaluateToolFlow evaluates inv against active taints and advances graph state when permitted.
// @intent nomos_hermetic_worktree_spec.md #CapabilityFlowGraph
func (g *CapabilityFlowGraph) EvaluateToolFlow(inv ToolFlowInvocation) FlowDecision {
	cmd := strings.TrimSpace(inv.CommandLine)
	if cmd != "" && remoteShellPipeRe.MatchString(cmd) {
		return FlowDecision{
			Allowed: false,
			Role:    FlowRoleSinkShellEval,
			Reason:  "❌ [NOMOS CAPABILITY-FLOW INTERCEPT] Piping remote network content directly into a shell interpreter ('curl|sh', 'wget|bash') is prohibited (Law 7). Download to an inspected file and verify before execution.",
		}
	}
	if sinkDec, blocked := g.evaluateSinkViolation(cmd); blocked {
		return sinkDec
	}
	if cmd != "" && verifySanitizerRe.MatchString(cmd) {
		g.ClearTaint(TaintUntrustedExternal)
		g.ClearTaint(TaintMCPUntrustedIngress)
		return FlowDecision{Allowed: true, Role: FlowRoleSanitizer}
	}
	labels, resource := classifyFlowSources(inv)
	if len(labels) > 0 {
		toolLabel := strings.TrimSpace(inv.ToolName)
		if toolLabel == "" {
			toolLabel = "shell"
		}
		for _, lbl := range labels {
			g.AddTaint(lbl, toolLabel, resource)
		}
		return FlowDecision{Allowed: true, Role: FlowRoleSource}
	}
	return FlowDecision{Allowed: true, Role: FlowRoleNeutral}
}

func (g *CapabilityFlowGraph) evaluateSinkViolation(cmd string) (FlowDecision, bool) {
	if cmd == "" {
		return FlowDecision{}, false
	}
	if isExfiltrationCommand(cmd) {
		if edge, tainted := g.HasTaint(TaintCredentialSource); tainted {
			return FlowDecision{
				Allowed:       false,
				Role:          FlowRoleSinkExfiltration,
				ViolatedLabel: TaintCredentialSource,
				Reason: fmt.Sprintf(
					"❌ [NOMOS CAPABILITY-FLOW INTERCEPT] Outbound payload transmission blocked (Law 7): session carries active credential taint from '%s' (%s). Start a clean session or avoid transmitting after reading credential files.",
					edge.SourceTool, edge.Resource,
				),
			}, true
		}
	}
	if releaseSinkCmdRe.MatchString(cmd) {
		if edge, tainted := g.HasTaint(TaintUntrustedExternal); tainted {
			return FlowDecision{
				Allowed:       false,
				Role:          FlowRoleSinkPrivilegedRelease,
				ViolatedLabel: TaintUntrustedExternal,
				Reason: fmt.Sprintf(
					"❌ [NOMOS CAPABILITY-FLOW INTERCEPT] Privileged release sink blocked (Law 2 & Law 7): session ingested untrusted external input via '%s' (%s). Run 'nomos verify' to sanitize workspace state before pushing or publishing.",
					edge.SourceTool, edge.Resource,
				),
			}, true
		}
	}
	return FlowDecision{}, false
}

func isExfiltrationCommand(cmd string) bool {
	return curlExfilFlagRe.MatchString(cmd) || wgetExfilFlagRe.MatchString(cmd) || netExfilCmdRe.MatchString(cmd)
}

func classifyFlowSources(inv ToolFlowInvocation) ([]TaintLabel, string) {
	var labels []TaintLabel
	var resource string

	toolLower := strings.ToLower(strings.TrimSpace(inv.ToolName))
	if externalFetchTools[toolLower] {
		labels = append(labels, TaintUntrustedExternal)
		resource = toolLower
	}
	if host, ok := extractRemoteFetchHost(inv.CommandLine); ok {
		labels = appendUniqueLabel(labels, TaintUntrustedExternal)
		resource = host
	}
	if credRes, ok := detectCredentialAccess(inv.TargetFile, inv.CommandLine); ok {
		labels = appendUniqueLabel(labels, TaintCredentialSource)
		resource = credRes
	}
	return labels, resource
}

func appendUniqueLabel(labels []TaintLabel, candidate TaintLabel) []TaintLabel {
	for _, existing := range labels {
		if existing == candidate {
			return labels
		}
	}
	return append(labels, candidate)
}

func extractRemoteFetchHost(cmd string) (string, bool) {
	matches := remoteHTTPFetchRe.FindStringSubmatch(cmd)
	if len(matches) < 2 {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(matches[1]))
	if host == "localhost" || strings.HasPrefix(host, "localhost:") || host == "127.0.0.1" || strings.HasPrefix(host, "127.0.0.1:") {
		return "", false
	}
	return host, true
}

// IsSensitiveCredentialPath reports whether path targets a credential store or non-template .env file.
// @intent nomos_hermetic_worktree_spec.md #TaintEdge
func IsSensitiveCredentialPath(rawPath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(rawPath))
	if clean == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(clean))
	if allowedTemplateEnvFiles[base] {
		return false
	}
	if base == ".env" || strings.HasPrefix(base, ".env.") || sensitiveBasenames[base] {
		return true
	}
	lower := strings.ToLower(clean)
	return strings.Contains(lower, "/.aws/credentials") ||
		strings.Contains(lower, "/.kube/config") ||
		strings.Contains(lower, "/.ssh/id_")
}

func detectCredentialAccess(targetFile, cmdLine string) (string, bool) {
	if IsSensitiveCredentialPath(targetFile) {
		return filepath.Base(filepath.ToSlash(strings.TrimSpace(targetFile))), true
	}
	if strings.TrimSpace(cmdLine) == "" {
		return "", false
	}
	for _, tok := range strings.Fields(cmdLine) {
		cleaned := strings.Trim(tok, `"'`+"`"+`;|&()`)
		if IsSensitiveCredentialPath(cleaned) {
			return filepath.Base(filepath.ToSlash(cleaned)), true
		}
	}
	return "", false
}

