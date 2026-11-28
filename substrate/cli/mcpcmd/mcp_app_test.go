package mcpcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

// @verifies TC-MCP-03
func TestMCP_AgentSkillAndFullscreenXrayApp(t *testing.T) {
	frames := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"nomos://skill"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"ui://nomos/xray"}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := ServeStdio(strings.NewReader(frames), &out, "v4.35.0"); err != nil {
		t.Fatalf("ServeStdio failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 JSON-RPC responses, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"io.modelcontextprotocol/ui"`) || !strings.Contains(lines[0], mcpAppMimeType) {
		t.Fatalf("expected MCP Apps UI capability in initialize response: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"resourceUri":"ui://nomos/xray"`) {
		t.Fatalf("expected nomos_xray _meta.ui.resourceUri in tools/list: %s", lines[1])
	}
	if !strings.Contains(lines[2], `"nomos://skill"`) || !strings.Contains(lines[2], `"ui://nomos/xray"`) {
		t.Fatalf("expected nomos://skill and ui://nomos/xray in resources/list: %s", lines[2])
	}

	assertMCPResourceContent(t, lines[3], "nomos://skill", "text/markdown", "nomos xray")
	assertMCPResourceContent(t, lines[4], "ui://nomos/xray", mcpAppMimeType, "<!doctype html>")
	if !strings.Contains(lines[4], "ui/notifications/initialized") {
		t.Fatalf("expected MCP Apps postMessage initialization handshake in ui://nomos/xray")
	}

	escaped := buildXrayMCPAppDocument(&xray.XrayReport{
		ProjectName: `<script>alert(1)</script>`,
		Grade:       "B",
		Hotspots:    []xray.XrayHotspot{{File: `a"<b>.go`, LOC: 420, CDS: 55, Reasons: []string{"<monolith>"}}},
	})
	if strings.Contains(escaped, "<script>alert(1)</script>") || !strings.Contains(escaped, "&lt;script&gt;") {
		t.Fatalf("expected HTML escaping of project name and hotspot fields in MCP App document")
	}
}

func assertMCPResourceContent(t *testing.T, rawLine, wantURI, wantMime, wantSubstr string) {
	t.Helper()
	var resp struct {
		Result struct {
			Contents []struct {
				URI      string `json:"uri"`
				MimeType string `json:"mimeType"`
				Text     string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(rawLine), &resp); err != nil {
		t.Fatalf("unmarshal resource response: %v (%s)", err, rawLine)
	}
	if len(resp.Result.Contents) != 1 {
		t.Fatalf("expected 1 resource content item for %s, got %d", wantURI, len(resp.Result.Contents))
	}
	got := resp.Result.Contents[0]
	if got.URI != wantURI || got.MimeType != wantMime || !strings.Contains(got.Text, wantSubstr) {
		t.Fatalf("unexpected resource %s: mime=%q text_len=%d", got.URI, got.MimeType, len(got.Text))
	}
}
