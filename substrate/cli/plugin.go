package cli

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// discoverPlugins scans the NOMOS_PLUGINS_ROOT for executable shims starting with "nomos-plugin-"
// and registers them as native commands on the root.
func discoverPlugins(rootCmd *cobra.Command) {
	pluginsRoot := getPluginsRoot()
	if pluginsRoot == "" {
		return
	}

	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return // Silently ignore if no plugins directory exists
	}

	groupAdded := false
	for _, entry := range entries {
		if isValidPlugin(pluginsRoot, entry) {
			pluginCmd := buildPluginCmd(pluginsRoot, entry.Name())
			// Law 6 (Canonical Singularity): Never shadow or duplicate native substrate commands.
			if existing, _, err := rootCmd.Find([]string{pluginCmd.Use}); err == nil && existing != nil && existing != rootCmd {
				continue
			}
			if !groupAdded {
				rootCmd.AddGroup(&cobra.Group{ID: "sovereign", Title: "Sovereign Commercial Plugins (ADR-0031):"})
				groupAdded = true
			}
			rootCmd.AddCommand(pluginCmd)
		}
	}
}

// getPluginsRoot returns the configured plugin directory, or dynamically resolves to $(go env GOPATH)/bin.
// It checks NOMOS_PLUGINS_ROOT environment variable first for explicit overrides.
// It falls back to resolving $(go env GOPATH)/bin dynamically adhering to Law 5.
func getPluginsRoot() string {
	if root := os.Getenv("NOMOS_PLUGINS_ROOT"); root != "" {
		return root
	}
	// Zero-Hardcoded-Path Law (Law 5): Dynamic resolution only.
	gopath := build.Default.GOPATH
	if gopath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		gopath = filepath.Join(home, "go")
	}
	expectedBinDir := filepath.Join(gopath, "bin")
	if _, err := os.Stat(expectedBinDir); err == nil {
		return expectedBinDir
	}
	return ""
}

func isValidPlugin(pluginsRoot string, entry os.DirEntry) bool {
	if entry.IsDir() || !strings.HasPrefix(entry.Name(), "nomos-plugin-") {
		return false
	}
	fullPath := filepath.Join(pluginsRoot, entry.Name())
	fi, err := os.Lstat(fullPath)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return fi.Mode().Perm()&0002 == 0
}

func buildPluginCmd(pluginsRoot, filename string) *cobra.Command {
	pluginName := strings.TrimPrefix(filename, "nomos-plugin-")
	pluginPath := filepath.Join(pluginsRoot, filename)

	if ext := filepath.Ext(pluginName); ext != "" {
		pluginName = strings.TrimSuffix(pluginName, ext)
	}

	return &cobra.Command{
		Use:                pluginName,
		GroupID:            "sovereign",
		Short:              fmt.Sprintf("Sovereign plugin: %s", pluginName),
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			execCmd := exec.Command(pluginPath, args...)
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			execCmd.Stdin = os.Stdin
			execCmd.Env = os.Environ()

			if err := execCmd.Run(); err != nil {
				return fmt.Errorf("plugin execution failed: %w", err)
			}
			return nil
		},
	}
}
