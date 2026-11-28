// Package mcpcmd implements read-only MCP resources/*, prompts/*, and capabilities inspection.
// @intent task_lifecycle_cli_spec.md
package mcpcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/cli/diag"
	"github.com/mgantlett/nomos/substrate/cli/inspectcmd"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

const (
	statelessMCPProtocolVersion = "2026-07-28"
	mcpProtocol20251125         = "2025-11-25"
	modernMCPProtocolVersion    = "2025-06-18"
	mcpProtocol20250326         = "2025-03-26"
	mcpMetaVersionKey           = "io.modelcontextprotocol/protocolVersion"
	rpcErrInvalidParams         = -32602
	rpcErrUnsupportedVersion    = -32022
	mcpServerInstructions       = "Read-only local Nomos MCP server. Use nomos_capabilities (or resources nomos://capabilities, nomos://skill, ui://nomos/xray), nomos_handshake, nomos_xray, nomos_verify_impact, and nomos_mem_recall to inspect repository governance without mutating files."
)

func supportedMCPVersions() []string {
	return []string{
		statelessMCPProtocolVersion,
		mcpProtocol20251125,
		modernMCPProtocolVersion,
		mcpProtocol20250326,
		mcpProtocolVersion,
	}
}

func isSupportedMCPVersion(v string) bool {
	for _, s := range supportedMCPVersions() {
		if v == s {
			return true
		}
	}
	return false
}

type mcpMetaEnvelope struct {
	Meta map[string]any `json:"_meta"`
}

type mcpInitParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Meta            map[string]any `json:"_meta,omitempty"`
}

func extractMetaProtocolVersion(req MCPRequest) string {
	if len(req.Params) > 0 {
		var env mcpMetaEnvelope
		if err := json.Unmarshal(req.Params, &env); err == nil {
			if v, ok := env.Meta[mcpMetaVersionKey].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	if v, ok := req.Meta[mcpMetaVersionKey].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return ""
}

func validateStatelessMeta(req MCPRequest) (MCPResponse, bool) {
	requested := extractMetaProtocolVersion(req)
	if requested == "" || isSupportedMCPVersion(requested) {
		return MCPResponse{}, false
	}
	return errorDataResponse(req.ID, rpcErrUnsupportedVersion, "unsupported protocol version", map[string]any{
		"supported": supportedMCPVersions(),
		"requested": requested,
	}), true
}

func routeResourceOrPromptMethod(req MCPRequest) (MCPResponse, bool) {
	switch req.Method {
	case "resources/list":
		return okResponse(req.ID, map[string]any{"resources": listReadOnlyResources()}), true
	case "resources/templates/list":
		return okResponse(req.ID, map[string]any{"resourceTemplates": []any{}}), true
	case "resources/read":
		return handleResourceRead(req), true
	case "prompts/list":
		return okResponse(req.ID, map[string]any{"prompts": listReadOnlyPrompts()}), true
	case "prompts/get":
		return handlePromptGet(req), true
	default:
		return MCPResponse{}, false
	}
}

type mcpResourceReadParams struct {
	URI string `json:"uri"`
}

type mcpPromptGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

func negotiateProtocolVersion(rawParams json.RawMessage) string {
	if len(rawParams) == 0 {
		return mcpProtocolVersion
	}
	var p mcpInitParams
	if err := json.Unmarshal(rawParams, &p); err != nil {
		return mcpProtocolVersion
	}
	req := strings.TrimSpace(p.ProtocolVersion)
	if req == "" {
		if v, ok := p.Meta[mcpMetaVersionKey].(string); ok {
			req = strings.TrimSpace(v)
		}
	}
	if isSupportedMCPVersion(req) {
		return req
	}
	if req == "" {
		return mcpProtocolVersion
	}
	if strings.HasPrefix(req, "2026-") || strings.HasPrefix(req, "2027-") {
		return statelessMCPProtocolVersion
	}
	if strings.HasPrefix(req, "2025-") {
		return modernMCPProtocolVersion
	}
	return mcpProtocolVersion
}

func mcpCapabilitiesMap() map[string]any {
	return map[string]any{
		"tools":     map[string]any{"listChanged": false},
		"resources": map[string]any{"subscribe": false, "listChanged": false},
		"prompts":   map[string]any{"listChanged": false},
		"extensions": map[string]any{
			"io.modelcontextprotocol/ui": map[string]any{
				"mimeTypes": []string{mcpAppMimeType},
			},
		},
	}
}

func buildInitializePayload(rawParams json.RawMessage, version string) map[string]any {
	return map[string]any{
		"protocolVersion": negotiateProtocolVersion(rawParams),
		"capabilities":    mcpCapabilitiesMap(),
		"serverInfo":      map[string]any{"name": "nomos", "version": version},
		"instructions":    mcpServerInstructions,
	}
}

func buildServerDiscoverPayload(version string) map[string]any {
	return map[string]any{
		"protocolVersion":   statelessMCPProtocolVersion,
		"supportedVersions": supportedMCPVersions(),
		"capabilities":      mcpCapabilitiesMap(),
		"serverInfo":        map[string]any{"name": "nomos", "version": version},
		"instructions":      mcpServerInstructions,
		"_meta": map[string]any{
			mcpMetaVersionKey: statelessMCPProtocolVersion,
		},
	}
}

func listReadOnlyResources() []map[string]any {
	return []map[string]any{
		{
			"uri":       "nomos://status",
			"name":      "nomos-workspace-status",
			"title":     "Nomos Workspace Status & Wavefront",
			propDescKey: "Current governance phase, active branch, database health, and unblocked task wavefront.",
			"mimeType":  "application/json",
		},
		{
			"uri":       "nomos://capabilities",
			"name":      "nomos-capabilities-catalog",
			"title":     "Nomos Quality Gates, Hooks & Directives Catalog",
			propDescKey: "Authoritative JSON catalog of active quality gates, ambient hooks, engines, and directives.",
			"mimeType":  "application/json",
		},
		{
			"uri":       "nomos://directives",
			"name":      "nomos-jit-directives",
			"title":     "Embedded JIT Phase Directives",
			propDescKey: "List of embedded and workspace Just-In-Time (JIT) phase steering playbooks.",
			"mimeType":  "application/json",
		},
		{
			"uri":       mcpSkillURI,
			"name":      "nomos-agent-skill",
			"title":     "Nomos Agent Skill (Thin Binary Bootstrapper)",
			propDescKey: "Canonical Nomos Agent Skill Markdown instructions exposed via MCP for VS Code Agent Host Protocol 1.1.",
			"mimeType":  "text/markdown",
		},
		{
			"uri":       mcpAppXrayURI,
			"name":      "nomos-xray-app",
			"title":     "Nomos Codebase X-Ray (Interactive MCP App)",
			propDescKey: "Read-only interactive HTML X-Ray scorecard and hotspot filter rendered inline or fullscreen via MCP Apps.",
			"mimeType":  mcpAppMimeType,
		},
	}
}

func listReadOnlyPrompts() []map[string]any {
	return []map[string]any{
		{
			"name":      "xray-audit",
			"title":     "Run Zero-Write Brownfield X-Ray",
			propDescKey: "Audit repository structural health via nomos_xray and summarize the AI Hangover Score and top hotspots.",
			"arguments": []map[string]any{
				{"name": "dir", propDescKey: "Optional repository path", "required": false},
			},
		},
		{
			"name":      "verify-impact",
			"title":     "Inspect 1-Hop AST Blast Radius",
			propDescKey: "Inspect reverse-dependency blast radius for modified files via nomos_verify_impact.",
			"arguments": []map[string]any{
				{"name": "dir", propDescKey: "Optional repository path", "required": false},
			},
		},
		{
			"name":      "recall-pitfalls",
			"title":     "Recall Verified Mneme Pitfalls",
			propDescKey: "Query verified episodic trap-to-fix lessons via nomos_mem_recall before editing.",
			"arguments": []map[string]any{
				{"name": "query", propDescKey: "Search keyword or package name", "required": true},
			},
		},
	}
}

func handleResourceRead(req MCPRequest) MCPResponse {
	var p mcpResourceReadParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errorResponse(req.ID, rpcErrInvalidParams, fmt.Sprintf("invalid resources/read params: %v", err))
		}
	}
	uri := strings.TrimSpace(p.URI)
	mimeType := "application/json"
	var text string
	switch uri {
	case "nomos://status":
		raw, isErr := marshalToolJSON(diag.CollectStatusReport())
		if isErr {
			return errorResponse(req.ID, rpcErrInvalidParams, raw)
		}
		text = raw
	case "nomos://capabilities", "nomos://capabilities/catalog":
		raw, isErr := execCapabilitiesTool(nil)
		if isErr {
			return errorResponse(req.ID, rpcErrInvalidParams, raw)
		}
		text = raw
	case "nomos://directives":
		dirs, err := handshake.ListDirectives(resolveTargetDir(""))
		if err != nil {
			return errorResponse(req.ID, rpcErrInvalidParams, err.Error())
		}
		raw, isErr := marshalToolJSON(dirs)
		if isErr {
			return errorResponse(req.ID, rpcErrInvalidParams, raw)
		}
		text = raw
	case mcpSkillURI:
		mimeType = "text/markdown"
		text = readAgentSkillMarkdown("")
	case mcpAppXrayURI:
		htmlDoc, err := renderXrayMCPAppHTML("")
		if err != nil {
			return errorResponse(req.ID, rpcErrInvalidParams, err.Error())
		}
		mimeType = mcpAppMimeType
		text = htmlDoc
	default:
		return errorResponse(req.ID, rpcErrInvalidParams, fmt.Sprintf("unknown MCP resource URI: %s", uri))
	}
	return okResponse(req.ID, map[string]any{
		"contents": []map[string]any{
			{"uri": uri, "mimeType": mimeType, "text": text},
		},
	})
}

func handlePromptGet(req MCPRequest) MCPResponse {
	var p mcpPromptGetParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errorResponse(req.ID, rpcErrInvalidParams, fmt.Sprintf("invalid prompts/get params: %v", err))
		}
	}
	name := strings.TrimSpace(p.Name)
	var desc, body string
	switch name {
	case "xray-audit":
		dir := coreOrDefault(stringArg(p.Arguments, "dir"), ".")
		desc = "Audit repository structural health via nomos_xray."
		body = fmt.Sprintf("Call nomos_xray on dir=%q, report the AI Hangover Score (0-100), Grade, and top 3 structural hotspots without modifying any files.", dir)
	case "verify-impact":
		dir := coreOrDefault(stringArg(p.Arguments, "dir"), ".")
		desc = "Inspect 1-hop AST reverse-dependency blast radius."
		body = fmt.Sprintf("Call nomos_verify_impact on dir=%q and inspect affected downstream packages and paired unit tests before mutating shared exports.", dir)
	case "recall-pitfalls":
		q := coreOrDefault(stringArg(p.Arguments, "query"), "verify")
		desc = "Recall verified Mneme pitfalls and patterns."
		body = fmt.Sprintf("Call nomos_mem_recall with query=%q and apply any recorded trap-to-fix lessons.", q)
	default:
		return errorResponse(req.ID, rpcErrInvalidParams, fmt.Sprintf("unknown MCP prompt: %s", name))
	}
	return okResponse(req.ID, map[string]any{
		"description": desc,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": map[string]any{"type": "text", "text": body},
			},
		},
	})
}

func coreOrDefault(val, fallback string) string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func execCapabilitiesTool(args map[string]any) (string, bool) {
	if id := strings.TrimSpace(stringArg(args, "id")); id != "" {
		targetDir := resolveTargetDir(stringArg(args, "dir"))
		detail, err := inspectcmd.ResolveCapabilityDetailInDir(targetDir, id)
		if err != nil {
			return err.Error(), true
		}
		return marshalToolJSON(detail)
	}
	cmd := inspectcmd.NewCapabilitiesCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cliArgs := []string{"--json"}
	if fType := strings.TrimSpace(stringArg(args, "type")); fType != "" {
		cliArgs = []string{"--type", fType, "--json"}
	}
	cmd.SetArgs(cliArgs)
	if err := cmd.Execute(); err != nil {
		return err.Error(), true
	}
	return strings.TrimSpace(outBuf.String()), false
}
