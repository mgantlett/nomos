// Package selfupdate implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package selfupdate

import (
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"path/filepath"
	"strings"
)

const (
	OpenCodePluginStartMarker = "// NOMOS_OPEN_CORE_PLUGIN_START"
	OpenCodePluginEndMarker   = "// NOMOS_OPEN_CORE_PLUGIN_END"
)

// DefaultOpenCodePluginTS returns the canonical embedded OpenCode ambient plugin source.
func DefaultOpenCodePluginTS() ([]byte, error) {
	return tui.ReadEmbeddedTemplate("templates/hooks/nomos.ts.tmpl")
}

// ScaffoldOpenCodePlugin executes deterministic selfupdate verification logic (@intent task_lifecycle_cli_spec.md).
func ScaffoldOpenCodePlugin(dir string) error {
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
	merged := MergeOpenCodePluginContent(existingBytes, tmplBytes)
	return os.WriteFile(targetPath, merged, 0644)
}

// MergeOpenCodePluginContent executes deterministic selfupdate verification logic (@intent task_lifecycle_cli_spec.md).
func MergeOpenCodePluginContent(existingBytes, templateBytes []byte) []byte {
	existing := string(existingBytes)
	tmpl := strings.TrimSpace(string(templateBytes))
	if strings.TrimSpace(existing) == "" {
		return []byte(tmpl + "\n")
	}

	startIdx := strings.Index(existing, OpenCodePluginStartMarker)
	endIdx := strings.Index(existing, OpenCodePluginEndMarker)
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		endIdx += len(OpenCodePluginEndMarker)
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
