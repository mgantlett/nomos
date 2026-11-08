// Package cli implements read-only exploration spiral pacing advisories for active task worktrees.
package cli

import (
	"fmt"
	"strings"
)

const (
	readSpiralWarnThreshold = 12
	readSpiralCritThreshold = 20
)

var exploratoryReadToolNames = map[string]bool{
	"read": true, "read_file": true, "view_file": true,
	"view_file_outline": true, "view_code_item": true,
	"grep": true, "grep_search": true, "search_file_content": true,
	"glob": true, "find_by_name": true,
	"list_dir": true, "list_directory": true, "list": true, "ls": true, "lsp": true,
}

var readOnlyShellPrefixes = []string{
	"git grep", "grep ", "rg ", "find ", "cat ", "head ", "tail ", "sed -n", "nl ",
}

// evaluateReadSpiralAdvisory tracks consecutive read/search calls without a source edit
// inside an active task worktree and emits non-blocking pacing nudges at 12 and 20 reads.
func evaluateReadSpiralAdvisory(p *HookPayload, hermeticDir, style string) string {
	if p == nil || p.IsPostInvoke {
		return ""
	}
	wtRoot := resolveWorktreeTraceRoot(p, hermeticDir)
	if wtRoot == "" {
		return ""
	}
	path := resolveTraceStateFilePath(wtRoot)
	if path == "" {
		return ""
	}
	tr := loadOrInitHookSessionTrace(path, wtRoot)
	if isPacingSourceMutation(p) {
		resetReadSpiralCounters(&tr)
		saveHookSessionTrace(path, tr)
		return ""
	}
	if !isExploratoryReadCall(p) {
		return ""
	}
	tr.ConsecutiveReadCalls++
	var msg string
	if tr.ConsecutiveReadCalls >= readSpiralCritThreshold && !tr.WarnedReadSpiral20 {
		tr.WarnedReadSpiral12 = true
		tr.WarnedReadSpiral20 = true
		msg = formatReadSpiralAdvisory(tr.ConsecutiveReadCalls, true, style)
	} else if tr.ConsecutiveReadCalls >= readSpiralWarnThreshold && !tr.WarnedReadSpiral12 {
		tr.WarnedReadSpiral12 = true
		msg = formatReadSpiralAdvisory(tr.ConsecutiveReadCalls, false, style)
	}
	saveHookSessionTrace(path, tr)
	return msg
}

func resetReadSpiralCounters(tr *hookSessionTrace) {
	tr.ConsecutiveReadCalls = 0
	tr.WarnedReadSpiral12 = false
	tr.WarnedReadSpiral20 = false
}

func isPacingSourceMutation(p *HookPayload) bool {
	if p.CommandLine == "" {
		return p.TargetFile != "" && !isReadOnlyTool(p.ToolName)
	}
	lower := strings.ToLower(strings.TrimSpace(p.CommandLine))
	return hasAnySubstring(lower, "sed -i", "git apply", "patch ")
}

func isExploratoryReadCall(p *HookPayload) bool {
	if p.CommandLine == "" {
		name := strings.ToLower(strings.TrimSpace(p.ToolName))
		return exploratoryReadToolNames[name]
	}
	if isRecognizedTestCommand(p.CommandLine) {
		return false
	}
	return isReadOnlyInspectionShellCmd(p.CommandLine)
}

func isReadOnlyInspectionShellCmd(cmdLine string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(cmdLine))
	if trimmed == "" || strings.Contains(trimmed, ">") {
		return false
	}
	for _, prefix := range readOnlyShellPrefixes {
		if trimmed == strings.TrimSpace(prefix) || strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func formatReadSpiralAdvisory(readCalls int, urgent bool, style string) string {
	if style == advisoryStyleTerse {
		return fmt.Sprintf("🧭 [Nomos Pacing] %d read calls without edit: apply a minimal fix and run tests now.", readCalls)
	}
	if urgent {
		return fmt.Sprintf(
			"🧭 [Nomos Pacing Advisory] %d consecutive read/search calls without a source edit: stop passive reading, apply your best minimal fix hypothesis now, and run tests to verify.",
			readCalls,
		)
	}
	return fmt.Sprintf(
		"🧭 [Nomos Pacing Advisory] %d consecutive read/search calls without a source edit: formulate a minimal draft edit now and validate iteratively with tests.",
		readCalls,
	)
}
