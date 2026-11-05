// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	opaqueBinaryReadExts = map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true,
		".mp3": true, ".wav": true, ".flac": true, ".ogg": true,
		".bin": true, ".so": true, ".dylib": true, ".dll": true, ".exe": true,
		".o": true, ".a": true, ".pyc": true, ".wasm": true, ".class": true,
		".pt": true, ".pth": true, ".onnx": true, ".safetensors": true, ".gguf": true,
		".h5": true, ".hdf5": true, ".npy": true, ".npz": true, ".pkl": true, ".pickle": true,
		".parquet": true, ".feather": true, ".sqlite": true, ".sqlite3": true, ".db": true,
		".gz": true, ".tgz": true, ".bz2": true, ".xz": true, ".zst": true,
		".tar": true, ".zip": true, ".7z": true, ".rar": true, ".iso": true, ".qcow2": true, ".img": true,
	}
	geminiInlineMediaExts = map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
		".bmp": true, ".tiff": true, ".tif": true, ".ico": true, ".pdf": true,
	}
	interactiveEditors = map[string]bool{
		"vim": true, "nvim": true, "vi": true, "nano": true,
	}
	interactivePagersAndMonitors = map[string]bool{
		"less": true, "more": true, "htop": true, "watch": true,
	}
)

func isNativeTextReadTool(toolName string) bool {
	lower := strings.ToLower(strings.TrimSpace(toolName))
	return lower == "read" || lower == "read_file"
}

func evaluateBinaryReadGuard(p *HookPayload, sandboxMode bool) HookDecision {
	if p == nil || p.TargetFile == "" || !isNativeTextReadTool(p.ToolName) {
		return HookDecision{Decision: hookDecisionAllow}
	}
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(p.TargetFile)))
	if ext == "" {
		return HookDecision{Decision: hookDecisionAllow}
	}
	if opaqueBinaryReadExts[ext] {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason: fmt.Sprintf(
				"❌ [NOMOS INTERCEPT] Binary Context Guard (Law 7): Native text read tool '%s' cannot safely ingest binary/archive artifact '%s' (%s). Inspect metadata or parse structured records via shell utilities (e.g. file, python3, sqlite3, ffprobe, tar -tf, unzip -l) instead.",
				p.ToolName, filepath.Base(p.TargetFile), ext,
			),
		}
	}
	guardGeminiMedia := sandboxMode || strings.TrimSpace(os.Getenv("NOMOS_GUARD_GEMINI_INLINE_MEDIA")) == "1"
	if p.IsGeminiEvent && guardGeminiMedia && geminiInlineMediaExts[ext] {
		return HookDecision{
			Decision: hookDecisionDeny,
			Reason: fmt.Sprintf(
				"❌ [NOMOS INTERCEPT] Gemini Inline-Media Guard (Law 7): Direct read_file on '%s' (%s) triggers a headless Gemini CLI Vertex HTTP 400 turn-serialization error. Inspect, OCR, or parse this file via run_shell_command using python3 (PIL, cv2, numpy, pypdf) or CLI tools instead.",
				filepath.Base(p.TargetFile), ext,
			),
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func evaluateInteractiveCommandGuard(cmdLine string) HookDecision {
	for _, seg := range splitUnquotedChainSegments(cmdLine) {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" || isBackgroundSegment(trimmed) {
			continue
		}
		if offender := detectHangingForegroundSegment(trimmed); offender != "" {
			return HookDecision{
				Decision: hookDecisionDeny,
				Reason: fmt.Sprintf(
					"❌ [NOMOS INTERCEPT] Headless Hang Guard (Law 7): Foreground interactive or unbounded follow command ('%s') hangs non-TTY sandbox execution until timeout. Use bounded flags (e.g. tail -n 100, top -b -n 1, timeout 10s ...) or non-interactive file tools instead.",
					offender,
				),
			}
		}
	}
	return HookDecision{Decision: hookDecisionAllow}
}

func isBackgroundSegment(seg string) bool {
	return strings.HasSuffix(seg, "&") && !strings.HasSuffix(seg, "&&")
}

func detectHangingForegroundSegment(seg string) string {
	pipes := splitUnquotedPipes(seg)
	if len(pipes) == 0 {
		return ""
	}
	words := strings.Fields(pipes[0])
	cmdIdx := skipCommandPrefixes(words)
	if cmdIdx >= len(words) {
		return ""
	}
	exe := filepath.Base(strings.Trim(words[cmdIdx], "\"'"))
	if exe == "timeout" {
		return ""
	}
	args := words[cmdIdx+1:]
	if hasHelpOrVersionFlag(args) {
		return ""
	}
	return classifyInteractiveExe(exe, args, pipes[1:])
}

func classifyInteractiveExe(exe string, args, downstreamPipes []string) string {
	if interactiveEditors[exe] && !hasBatchEditorFlag(args) {
		return exe
	}
	if interactivePagersAndMonitors[exe] {
		return exe
	}
	if exe == "top" && !hasShortOrLongFlag(args, "-b", "--batch") {
		return "top"
	}
	if (exe == "tail" || exe == "journalctl") && hasFollowFlag(args) && !pipesIntoHead(downstreamPipes) {
		return exe + " -f"
	}
	return ""
}

func skipCommandPrefixes(words []string) int {
	i := 0
	for i < len(words) {
		w := words[i]
		if w == "sudo" || w == "env" || w == "nohup" || w == "command" {
			i++
			continue
		}
		if strings.Contains(w, "=") && !strings.HasPrefix(w, "-") && !strings.HasPrefix(w, "/") {
			i++
			continue
		}
		break
	}
	return i
}

func hasHelpOrVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "--version" || a == "-V" {
			return true
		}
	}
	return false
}

func hasBatchEditorFlag(args []string) bool {
	for _, a := range args {
		if a == "-e" || a == "-es" || a == "-Es" || a == "+q" || a == "+qa!" {
			return true
		}
	}
	return false
}

func hasShortOrLongFlag(args []string, shortFlag, longFlag string) bool {
	shortCh := strings.TrimPrefix(shortFlag, "-")
	for _, a := range args {
		if a == shortFlag || a == longFlag {
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.Contains(a[1:], shortCh) {
			return true
		}
	}
	return false
}

func hasFollowFlag(args []string) bool {
	for _, a := range args {
		if a == "-f" || a == "-F" || strings.HasPrefix(a, "--follow") {
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsAny(a[1:], "fF") {
			return true
		}
	}
	return false
}

func pipesIntoHead(downstream []string) bool {
	for _, stage := range downstream {
		fields := strings.Fields(stage)
		idx := skipCommandPrefixes(fields)
		if idx < len(fields) && filepath.Base(fields[idx]) == "head" {
			return true
		}
	}
	return false
}

func splitUnquotedChainSegments(cmdLine string) []string {
	var segments []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(cmdLine); i++ {
		nextQuote, wasQuoted := advanceQuoteState(cmdLine, i, quote)
		quote = nextQuote
		if wasQuoted || quote != 0 {
			cur.WriteByte(cmdLine[i])
			continue
		}
		if sepWidth := chainSeparatorWidth(cmdLine, i); sepWidth > 0 {
			segments = append(segments, cur.String())
			cur.Reset()
			i += sepWidth - 1
			continue
		}
		cur.WriteByte(cmdLine[i])
	}
	if cur.Len() > 0 {
		segments = append(segments, cur.String())
	}
	return segments
}

func splitUnquotedPipes(seg string) []string {
	var parts []string
	var cur strings.Builder
	var quote byte
	for i := 0; i < len(seg); i++ {
		nextQuote, wasQuoted := advanceQuoteState(seg, i, quote)
		quote = nextQuote
		if wasQuoted || quote != 0 {
			cur.WriteByte(seg[i])
			continue
		}
		if seg[i] == '|' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(seg[i])
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

func advanceQuoteState(src string, i int, quote byte) (byte, bool) {
	ch := src[i]
	if quote != 0 {
		if ch == quote && isUnescapedQuote(src, i, quote) {
			return 0, true
		}
		return quote, true
	}
	if ch == '\'' || ch == '"' {
		return ch, false
	}
	return 0, false
}

func isUnescapedQuote(src string, i int, quote byte) bool {
	if quote == '\'' || i == 0 {
		return true
	}
	return src[i-1] != '\\'
}

func chainSeparatorWidth(src string, i int) int {
	ch := src[i]
	if ch == ';' || ch == '\n' {
		return 1
	}
	if (ch == '&' || ch == '|') && i+1 < len(src) && src[i+1] == ch {
		return 2
	}
	return 0
}
