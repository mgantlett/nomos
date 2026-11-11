// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	protocolStartMarker = "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->"
	protocolEndMarker   = "<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->"
	claudeAgentsImport  = "@AGENTS.md"
)

func scaffoldClaudeConfig(dir string) error {
	claudePath := filepath.Join(dir, "CLAUDE.md")
	existingBytes, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(claudePath, []byte(claudeAgentsImport+"\n"), 0644)
		}
		return err
	}
	mergedBytes := mergeClaudeImportContent(existingBytes)
	return os.WriteFile(claudePath, mergedBytes, 0644)
}

func scaffoldAgentsConfig(dir string) error {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	return scaffoldOrMergeProtocolFile(agentsPath)
}

func scaffoldOrMergeProtocolFile(targetPath string) error {
	tmplBytes, err := initTemplates.ReadFile("templates/AGENTS.md")
	if err != nil {
		return err
	}

	existingBytes, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(targetPath, tmplBytes, 0644)
		}
		return err
	}

	mergedBytes := mergeProtocolContent(existingBytes, tmplBytes)
	return os.WriteFile(targetPath, mergedBytes, 0644)
}

func mergeProtocolContent(existingBytes, templateBytes []byte) []byte {
	existing := string(existingBytes)
	template := strings.TrimSpace(string(templateBytes))

	if strings.TrimSpace(existing) == "" {
		return []byte(template + "\n")
	}

	startIdx := strings.Index(existing, protocolStartMarker)
	endIdx := strings.Index(existing, protocolEndMarker)

	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		endIdx += len(protocolEndMarker)
		var sb strings.Builder
		sb.WriteString(existing[:startIdx])
		sb.WriteString(template)
		sb.WriteString(existing[endIdx:])
		return []byte(sb.String())
	}

	var sb strings.Builder
	sb.WriteString(template)
	sb.WriteString("\n\n---\n\n")
	sb.WriteString(strings.TrimSpace(existing))
	sb.WriteString("\n")
	return []byte(sb.String())
}

// mergeClaudeImportContent collapses legacy duplicated protocol blocks in CLAUDE.md
// into a canonical '@AGENTS.md' import line while preserving custom Claude-only notes.
func mergeClaudeImportContent(existingBytes []byte) []byte {
	existing := strings.TrimSpace(string(existingBytes))
	if existing == "" {
		return []byte(claudeAgentsImport + "\n")
	}

	startIdx := strings.Index(existing, protocolStartMarker)
	endIdx := strings.Index(existing, protocolEndMarker)
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		prefix := strings.TrimSpace(existing[:startIdx])
		suffix := stripLeadingHorizontalRule(existing[endIdx+len(protocolEndMarker):])
		return assembleClaudeSections(prefix, suffix)
	}

	if hasClaudeAgentsImportLine(existing) {
		return []byte(existing + "\n")
	}
	return []byte(claudeAgentsImport + "\n\n" + existing + "\n")
}

func assembleClaudeSections(prefix, suffix string) []byte {
	var parts []string
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if !hasClaudeAgentsImportLine(prefix) && !hasClaudeAgentsImportLine(suffix) {
		parts = append(parts, claudeAgentsImport)
	}
	if suffix != "" {
		parts = append(parts, suffix)
	}
	if len(parts) == 0 {
		return []byte(claudeAgentsImport + "\n")
	}
	return []byte(strings.Join(parts, "\n\n") + "\n")
}

func stripLeadingHorizontalRule(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "---" {
		return ""
	}
	if strings.HasPrefix(trimmed, "---\n") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "---\n"))
	}
	return trimmed
}

func hasClaudeAgentsImportLine(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == claudeAgentsImport {
			return true
		}
	}
	return false
}
