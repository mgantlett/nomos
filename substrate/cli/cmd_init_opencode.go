// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	openCodePluginStartMarker = "// NOMOS_OPEN_CORE_PLUGIN_START"
	openCodePluginEndMarker   = "// NOMOS_OPEN_CORE_PLUGIN_END"
)

// DefaultOpenCodePluginTS returns the canonical embedded OpenCode ambient plugin source.
func DefaultOpenCodePluginTS() ([]byte, error) {
	return initTemplates.ReadFile("templates/hooks/nomos.ts.tmpl")
}

func scaffoldOpenCodePlugin(dir string) error {
	pluginsDir := filepath.Join(dir, ".opencode", "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return err
	}
	tmplBytes, err := DefaultOpenCodePluginTS()
	if err != nil {
		return err
	}
	targetPath := filepath.Join(pluginsDir, "nomos.ts")
	existingBytes, rErr := os.ReadFile(targetPath)
	if rErr != nil {
		if os.IsNotExist(rErr) {
			return os.WriteFile(targetPath, tmplBytes, 0644)
		}
		return rErr
	}
	merged := mergeOpenCodePluginContent(existingBytes, tmplBytes)
	return os.WriteFile(targetPath, merged, 0644)
}

func mergeOpenCodePluginContent(existingBytes, templateBytes []byte) []byte {
	existing := string(existingBytes)
	tmpl := strings.TrimSpace(string(templateBytes))
	if strings.TrimSpace(existing) == "" {
		return []byte(tmpl + "\n")
	}

	startIdx := strings.Index(existing, openCodePluginStartMarker)
	endIdx := strings.Index(existing, openCodePluginEndMarker)
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		endIdx += len(openCodePluginEndMarker)
		var sb strings.Builder
		sb.WriteString(existing[:startIdx])
		sb.WriteString(tmpl)
		sb.WriteString(existing[endIdx:])
		return []byte(sb.String())
	}

	if strings.Contains(existing, "NomosGovernancePlugin") {
		return []byte(tmpl + "\n")
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimRight(existing, "\n"))
	sb.WriteString("\n\n")
	sb.WriteString(tmpl)
	sb.WriteString("\n")
	return []byte(sb.String())
}
