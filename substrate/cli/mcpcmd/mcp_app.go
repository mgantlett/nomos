// Package mcpcmd implements read-only MCP Agent Skill (nomos://skill) and Fullscreen MCP App (ui://nomos/xray) resources.
// @intent task_lifecycle_cli_spec.md
package mcpcmd

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

const (
	mcpSkillURI    = "nomos://skill"
	mcpAppXrayURI  = "ui://nomos/xray"
	mcpAppMimeType = "text/html;profile=mcp-app"
)

const fallbackAgentSkillMarkdown = `---
name: nomos
description: Thin binary bootstrapper for Nomos (zero-token AST and Git guardrails across 11 languages).
---

# Nomos Thin Binary Bootstrapper

1. Verify ` + "`nomos`" + ` is on PATH (or run via ` + "`npx --yes @mgantlett/nomos`" + `).
2. Run zero-write audit: ` + "`nomos xray .`" + ` (or read MCP App ` + "`ui://nomos/xray`" + `).
3. Enable Branch Guard: ` + "`nomos init`" + ` (or ` + "`nomos init --local --agents vscode`" + ` for stealth VS Code hooks).
4. Inspect compiled quality gates and contracts: ` + "`nomos capabilities --json`" + ` or MCP resource ` + "`nomos://capabilities`" + `.
`

func readAgentSkillMarkdown(repoDir string) string {
	target := resolveTargetDir(repoDir)
	skillPath := filepath.Join(target, "skills", "nomos", "SKILL.md")
	if raw, err := os.ReadFile(skillPath); err == nil && strings.TrimSpace(string(raw)) != "" {
		return string(raw)
	}
	return fallbackAgentSkillMarkdown
}

func renderXrayMCPAppHTML(repoDir string) (string, error) {
	target := resolveTargetDir(repoDir)
	report, err := xray.ScanRepository(target, xray.DefaultScanOptions())
	if err != nil {
		return "", err
	}
	return buildXrayMCPAppDocument(report), nil
}

func buildXrayMCPAppDocument(rep *xray.XrayReport) string {
	project := html.EscapeString(rep.ProjectName)
	grade := html.EscapeString(rep.Grade)
	rows := buildXrayHotspotRowsHTML(rep.Hotspots)
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	b.WriteString("<title>Nomos Codebase X-Ray</title>")
	b.WriteString("<style>")
	b.WriteString("body{margin:0;padding:20px;background:#0d1117;color:#e6edf3;font:13px/1.5 ui-monospace,SFMono-Regular,Menlo,monospace}")
	b.WriteString(".hdr{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #30363d;padding-bottom:12px;margin-bottom:16px}")
	b.WriteString(".badge{padding:4px 10px;border-radius:6px;background:#1f6feb;color:#fff;font-weight:700}")
	b.WriteString(".grid{display:grid;grid-template-columns:repeat(4,minmax(120px,1fr));gap:10px;margin-bottom:16px}")
	b.WriteString(".card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:10px}")
	b.WriteString(".lbl{color:#8b949e;font-size:11px;text-transform:uppercase}.val{font-size:18px;font-weight:700;margin-top:2px}")
	b.WriteString("input{width:100%;box-sizing:border-box;background:#161b22;color:#e6edf3;border:1px solid #30363d;border-radius:6px;padding:7px 10px;margin-bottom:12px}")
	b.WriteString("table{width:100%;border-collapse:collapse}th,td{text-align:left;padding:7px 8px;border-bottom:1px solid #21262d}")
	b.WriteString("th{color:#8b949e;font-size:11px;text-transform:uppercase}")
	b.WriteString("</style></head><body>")
	b.WriteString(fmt.Sprintf("<div class=\"hdr\"><div><strong>Nomos Codebase X-Ray</strong> · %s</div><span class=\"badge\">Grade %s · %d/100</span></div>", project, grade, rep.HangoverScore))
	b.WriteString("<div class=\"grid\">")
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"lbl\">AI Hangover</div><div class=\"val\">%d/100</div></div>", rep.HangoverScore))
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"lbl\">Files Scanned</div><div class=\"val\">%d</div></div>", rep.FilesScanned))
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"lbl\">Total LOC</div><div class=\"val\">%d</div></div>", rep.TotalLOC))
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"lbl\">Scan Latency</div><div class=\"val\">%dms</div></div>", rep.DurationMillis))
	b.WriteString("</div>")
	b.WriteString("<input id=\"flt\" type=\"search\" placeholder=\"Filter structural hotspots by file or reason...\" aria-label=\"Filter hotspots\">")
	b.WriteString("<table><thead><tr><th>File</th><th>LOC</th><th>CDS</th><th>Reasons</th></tr></thead><tbody id=\"rows\">")
	b.WriteString(rows)
	b.WriteString("</tbody></table>")
	b.WriteString("<script>const f=document.getElementById('flt'),r=Array.from(document.querySelectorAll('#rows tr'));")
	b.WriteString("f.addEventListener('input',()=>{const q=f.value.toLowerCase();r.forEach(tr=>{tr.style.display=tr.textContent.toLowerCase().includes(q)?'':'none';});});")
	b.WriteString("window.parent&&window.parent.postMessage({jsonrpc:'2.0',method:'ui/notifications/initialized',params:{}},'*');")
	b.WriteString("</script></body></html>")
	return b.String()
}

func buildXrayHotspotRowsHTML(hotspots []xray.XrayHotspot) string {
	if len(hotspots) == 0 {
		return "<tr><td colspan=\"4\">Zero structural hotspots detected (clean baseline).</td></tr>"
	}
	var b strings.Builder
	for _, h := range hotspots {
		reasons := html.EscapeString(strings.Join(h.Reasons, "; "))
		if reasons == "" {
			reasons = "structural debt"
		}
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%d</td><td>%d</td><td>%s</td></tr>",
			html.EscapeString(h.File), h.LOC, h.CDS, reasons))
	}
	return b.String()
}
