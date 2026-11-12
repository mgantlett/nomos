package tui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestPluginDiscovery_NeutralBrandingAndNoCoreShadowing(t *testing.T) {
	pluginDir := t.TempDir()
	customPlugin := filepath.Join(pluginDir, "nomos-plugin-demo")
	shadowPlugin := filepath.Join(pluginDir, "nomos-plugin-verify")
	if err := os.WriteFile(customPlugin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("WriteFile custom plugin: %v", err)
	}
	if err := os.WriteFile(shadowPlugin, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("WriteFile shadow plugin: %v", err)
	}
	t.Setenv("NOMOS_PLUGINS_ROOT", pluginDir)

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root --help failed: %v", err)
	}
	helpText := out.String()
	if !strings.Contains(helpText, "External Plugins:") || !strings.Contains(helpText, "External plugin: demo") {
		t.Fatalf("expected neutral External Plugins group in help, got:\n%s", helpText)
	}
	if strings.Contains(helpText, "Sovereign") || strings.Contains(helpText, "ADR-0031") {
		t.Fatalf("expected zero Sovereign/ADR-0031 branding on community plugins, got:\n%s", helpText)
	}
	if strings.Contains(helpText, "External plugin: verify") {
		t.Fatalf("Law 6 violation: plugin must never shadow core 'verify' command")
	}
}
