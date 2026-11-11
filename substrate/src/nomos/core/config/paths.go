// Package config provides path and directory discovery resolvers for the Nomos ecosystem.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// LocateConfigPath resolves the canonical nomos.yaml location dynamically.
func LocateConfigPath() (string, error) {
	if custom := os.Getenv(constants.EnvNomosConfig); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom, nil
		}
	}
	if found := locateInHierarchy(); found != "" {
		return found, nil
	}
	if home := locateHomeConfig(); home != "" {
		return home, nil
	}
	return "", fmt.Errorf("nomos.yaml could not be located in workspace hierarchy or home")
}

func locateInHierarchy() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	curr := cwd
	for i := 0; i < 5; i++ {
		if found := checkConfigPaths(curr); found != "" {
			return found
		}
		if hasGitRootMarker(curr) {
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}

func hasGitRootMarker(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	// Stop at standalone repository roots (.git directory), while allowing git worktrees (.git file) to inspect enclosing workspace roots.
	return fi.IsDir()
}

func checkConfigPaths(dir string) string {
	candidates := []string{
		filepath.Join(dir, constants.FileNomosYAML),
		filepath.Join(dir, constants.DirAgents, constants.FileNomosYAML),
		filepath.Join(dir, constants.DirNomosHome, constants.FileNomosYAML),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func locateHomeConfig() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(homeDir, constants.DirNomosHome, constants.FileNomosYAML),
		filepath.Join(homeDir, constants.DirNomosHome, constants.DirConfig, constants.FileNomosYAML),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// GetNomosHome resolves the nomos home directory from NOMOS_HOME or defaults to ~/.nomos.
func GetNomosHome() string {
	if custom := strings.TrimSpace(os.Getenv(constants.EnvNomosHome)); custom != "" {
		return custom
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return constants.DirNomosHome
	}
	return filepath.Join(home, constants.DirNomosHome)
}

// GetNomosDBDir returns the canonical database directory under NOMOS_HOME.
func GetNomosDBDir() string {
	return filepath.Join(GetNomosHome(), constants.DirData, constants.DirDB)
}

// GetNomosVerifyCacheDir returns the local cryptographic verification receipt cache directory under NOMOS_HOME.
func GetNomosVerifyCacheDir() string {
	return filepath.Join(GetNomosHome(), constants.DirCache, constants.DirVerifyReceipts)
}

// ResolveLocalDBDir checks if a local .nomos/data directory exists in the workspace hierarchy.
func ResolveLocalDBDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	curr := cwd
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(curr, constants.DirNomosHome, constants.DirData, constants.DirDB)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
		candidateData := filepath.Join(curr, constants.DirNomosHome, constants.DirData)
		if fi, err := os.Stat(candidateData); err == nil && fi.IsDir() {
			return candidateData
		}
		if hasGitRootMarker(curr) {
			break
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}

// ResolveGlobalDBPath dynamically resolves the canonical nomos_global.db path.
func ResolveGlobalDBPath() string {
	if custom := strings.TrimSpace(os.Getenv(constants.EnvNomosDB)); custom != "" {
		return custom
	}
	if local := ResolveLocalDBDir(); local != "" {
		return filepath.Join(local, constants.FileNomosGlobalDB)
	}
	return filepath.Join(GetNomosDBDir(), constants.FileNomosGlobalDB)
}

// ResolveMnemeDBPath dynamically resolves the canonical mneme.db path.
func ResolveMnemeDBPath() string {
	if custom := strings.TrimSpace(os.Getenv(constants.EnvNomosBrainDB)); custom != "" {
		return custom
	}
	if local := ResolveLocalDBDir(); local != "" {
		return filepath.Join(local, constants.FileMnemeDB)
	}
	return filepath.Join(GetNomosDBDir(), constants.FileMnemeDB)
}

// ResolveIntentPath searches for the authoritative intent directory in workspace hierarchy.
func ResolveIntentPath(startDir string) string {
	base := startDir
	if base == "" {
		if cwd, err := os.Getwd(); err == nil {
			base = cwd
		}
	}
	curr := base
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(curr, constants.DirIntent)
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}
