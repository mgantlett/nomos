// Package cli implements external nomos-plugin-* executable discovery and dispatch.
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

const (
	externalPluginGroupID    = "plugins"
	externalPluginGroupTitle = "External Plugins:"
)

// discoverPlugins scans the configured plugin binary root for executable shims starting
// with "nomos-plugin-" and registers them as subcommands on the root CLI without shadowing
// built-in substrate commands (Law 6).
func discoverPlugins(rootCmd *cobra.Command) {
	pluginsRoot := getPluginsRoot()
	if pluginsRoot == "" {
		return
	}

	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return
	}

	groupAdded := false
	for _, entry := range entries {
		if !isValidPlugin(pluginsRoot, entry) {
			continue
		}
		pluginCmd := buildPluginCmd(pluginsRoot, entry.Name())
		// Law 6 (Canonical Singularity): Never shadow or duplicate native substrate commands.
		if existing, _, fErr := rootCmd.Find([]string{pluginCmd.Use}); fErr == nil && existing != nil && existing != rootCmd {
			continue
		}
		if !groupAdded {
			rootCmd.AddGroup(&cobra.Group{ID: externalPluginGroupID, Title: externalPluginGroupTitle})
			groupAdded = true
		}
		rootCmd.AddCommand(pluginCmd)
	}
}

// getPluginsRoot returns NOMOS_PLUGINS_ROOT, GOBIN, or the first $(go env GOPATH)/bin directory.
func getPluginsRoot() string {
	if root := strings.TrimSpace(os.Getenv("NOMOS_PLUGINS_ROOT")); root != "" {
		return root
	}
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		if _, err := os.Stat(gobin); err == nil {
			return gobin
		}
	}
	// Zero-Hardcoded-Path Law (Law 5): Dynamic GOPATH resolution with multi-entry SplitList support.
	gopath := firstGoPathEntry(build.Default.GOPATH)
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

func firstGoPathEntry(raw string) string {
	for _, entry := range filepath.SplitList(strings.TrimSpace(raw)) {
		if trimmed := strings.TrimSpace(entry); trimmed != "" {
			return trimmed
		}
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
		GroupID:            externalPluginGroupID,
		Short:              fmt.Sprintf("External plugin: %s", pluginName),
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
