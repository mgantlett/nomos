// Package mcpcmd implements the zero-dependency read-only MCP JSON-RPC 2.0 stdio server.
// @intent task_lifecycle_cli_spec.md
package mcpcmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	jsonRPCVersion     = "2.0"
	mcpProtocolVersion = "2024-11-05"
	maxRPCFrameBytes   = 4 * 1024 * 1024
	rpcErrParse        = -32700
	rpcErrInvalidReq   = -32600
	rpcErrMethodFound  = -32601
)

// MCPRequest models a single JSON-RPC 2.0 request or notification frame (#MCPRequest).
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Meta    map[string]any  `json:"_meta,omitempty"`
}

// MCPResponse models a JSON-RPC 2.0 response frame (#MCPResponse).
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError models a JSON-RPC 2.0 structured error payload.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCPToolDescriptor declares a read-only MCP tool entry for tools/list.
type MCPToolDescriptor struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	Annotations  map[string]any `json:"annotations,omitempty"`
	Meta         map[string]any `json:"_meta,omitempty"`
}

// MCPToolCallParams decodes parameters passed to tools/call.
type MCPToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// MCPTextContent represents a single text block in an MCP tool result.
type MCPTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult wraps structured tool output or fail-closed read-only errors.
type MCPToolResult struct {
	Content           []MCPTextContent `json:"content"`
	StructuredContent map[string]any   `json:"structuredContent,omitempty"`
	IsError           bool             `json:"isError,omitempty"`
}

// NewMCPCmd constructs the hidden agent command for serving read-only MCP over stdio.
func NewMCPCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve read-only Model Context Protocol (MCP) JSON-RPC 2.0 over stdio",
		Long: `Start a zero-dependency stdio MCP server exposing read-only orientation,
backlog DAG, Mneme memory recall, blast-radius impact, and X-Ray tools.`,
		Example: `  # Connect an MCP client over stdio:
  nomos mcp`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := emitInteractiveHint(cmd.InOrStdin(), cmd.ErrOrStderr(), version); err != nil {
				return err
			}
			return ServeStdio(cmd.InOrStdin(), cmd.OutOrStdout(), version)
		},
	}
}

// ServeStdio reads newline-delimited JSON-RPC 2.0 frames and writes responses until EOF.
func ServeStdio(in io.Reader, out io.Writer, version string) error {
	if in == nil || out == nil {
		return errors.New("mcp stdio streams must be non-nil")
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxRPCFrameBytes)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		resp, shouldRespond := dispatchRPCFrame([]byte(line), version)
		if !shouldRespond {
			continue
		}
		if err := writeRPCFrame(out, resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func dispatchRPCFrame(raw []byte, version string) (MCPResponse, bool) {
	var req MCPRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return errorResponse(nil, rpcErrParse, "parse error"), true
	}
	if req.Method == "" {
		return errorResponse(req.ID, rpcErrInvalidReq, "invalid request: missing method"), true
	}
	// JSON-RPC notifications omit ID or use the notifications/ prefix and emit no response.
	if strings.HasPrefix(req.Method, "notifications/") || len(req.ID) == 0 || string(req.ID) == "null" {
		return MCPResponse{}, false
	}
	return routeRPCMethod(req, version), true
}

func routeRPCMethod(req MCPRequest, version string) MCPResponse {
	if errResp, blocked := validateStatelessMeta(req); blocked {
		return errResp
	}
	if resResp, handled := routeResourceOrPromptMethod(req); handled {
		return resResp
	}
	switch req.Method {
	case "server/discover":
		return okResponse(req.ID, buildServerDiscoverPayload(version))
	case "initialize":
		return okResponse(req.ID, buildInitializePayload(req.Params, version))
	case "ping":
		return okResponse(req.ID, map[string]any{})
	case "tools/list":
		return okResponse(req.ID, map[string]any{"tools": ReadOnlyTools()})
	case "tools/call":
		return handleToolCall(req)
	default:
		return errorResponse(req.ID, rpcErrMethodFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func handleToolCall(req MCPRequest) MCPResponse {
	var params MCPToolCallParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, rpcErrInvalidReq, fmt.Sprintf("invalid tools/call params: %v", err))
		}
	}
	if strings.TrimSpace(params.Name) == "" {
		return errorResponse(req.ID, rpcErrInvalidReq, "invalid tools/call params: missing tool name")
	}
	text, isErr := CallReadOnlyTool(params.Name, params.Arguments)
	res := MCPToolResult{
		Content: []MCPTextContent{{Type: "text", Text: text}},
		IsError: isErr,
	}
	if !isErr {
		res.StructuredContent = parseStructuredToolContent(text)
	}
	return okResponse(req.ID, res)
}

func parseStructuredToolContent(raw string) map[string]any {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	if obj, ok := parsed.(map[string]any); ok {
		return obj
	}
	if arr, ok := parsed.([]any); ok {
		return map[string]any{"items": arr, "count": len(arr)}
	}
	return nil
}

func okResponse(id json.RawMessage, result any) MCPResponse {
	return MCPResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id json.RawMessage, code int, msg string) MCPResponse {
	return errorDataResponse(id, code, msg, nil)
}

func errorDataResponse(id json.RawMessage, code int, msg string, data any) MCPResponse {
	return MCPResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   &MCPError{Code: code, Message: msg, Data: data},
	}
}

func writeRPCFrame(out io.Writer, resp MCPResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal mcp response: %w", err)
	}
	_, err = fmt.Fprintf(out, "%s\n", payload)
	return err
}

type statReader interface {
	io.Reader
	Stat() (os.FileInfo, error)
}

func isCharDevice(r io.Reader) bool {
	sr, ok := r.(statReader)
	if !ok {
		return false
	}
	info, err := sr.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func emitInteractiveHint(in io.Reader, errOut io.Writer, version string) error {
	if errOut == nil || !isCharDevice(in) {
		return nil
	}
	_, err := fmt.Fprintf(
		errOut,
		"nomos mcp (v%s): stdio JSON-RPC 2.0 server ready; waiting for MCP client frames on stdin (Ctrl+C or Ctrl+D to exit)\n",
		strings.TrimSpace(version),
	)
	return err
}
