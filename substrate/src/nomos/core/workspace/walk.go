// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

var defaultIntermediateDirs = []string{
	"Intermediate",
	"Saved",
	"DerivedDataCache",
	"Binaries",
	"node_modules",
	"target",
	".build",
	"build",
	"dist",
	"vendor",
	".git",
	".tmp",
	".svelte-kit",
	".vitepress",
	"cache",
}

// IsDefaultIntermediateDir checks if a directory name matches standard engine or dependency build trees.
func IsDefaultIntermediateDir(name string) bool {
	for _, d := range defaultIntermediateDirs {
		if name == d {
			return true
		}
	}
	return false
}

// IsDefaultGeneratedFile checks if a file name matches standard compiler reflection or generated artifacts.
func IsDefaultGeneratedFile(name string) bool {
	return strings.HasSuffix(name, ".gen.cpp") ||
		strings.HasSuffix(name, ".gen.h") ||
		strings.HasSuffix(name, ".generated.h") ||
		strings.HasPrefix(name, "moc_") ||
		strings.HasPrefix(name, "qrc_")
}

// IsExcluded checks if a path matches standard intermediate/generated patterns or explicit exclude rules.
func IsExcluded(absPath string, isDir bool, name string, excludes []string) bool {
	if isDir {
		if name == constants.DirWorktrees || IsDefaultIntermediateDir(name) {
			return true
		}
	} else {
		if IsDefaultGeneratedFile(name) {
			return true
		}
	}
	for _, ex := range excludes {
		if matchesExcludePattern(absPath, name, ex) {
			return true
		}
	}
	return false
}

func matchesExcludePattern(absPath, name, rawRule string) bool {
	trimmed := strings.TrimSpace(rawRule)
	if trimmed == "" {
		return false
	}
	slashRule := filepath.ToSlash(trimmed)
	if strings.HasSuffix(slashRule, "/**") || strings.HasSuffix(slashRule, "/*") {
		prefix := strings.TrimSuffix(strings.TrimSuffix(slashRule, "/**"), "/*")
		cleanPrefix := filepath.Clean(filepath.FromSlash(prefix))
		return isSameOrChildPath(absPath, cleanPrefix)
	}
	cleanEx := filepath.Clean(trimmed)
	if isSameOrChildPath(absPath, cleanEx) {
		return true
	}
	base := filepath.Base(cleanEx)
	if base == "*" || base == "**" || !strings.ContainsAny(base, "*?[") {
		return false
	}
	dirPrefix := filepath.Dir(cleanEx)
	if dirPrefix != "." && !isSameOrChildPath(filepath.Dir(absPath), dirPrefix) {
		return false
	}
	matched, err := filepath.Match(base, name)
	return err == nil && matched
}

func isSameOrChildPath(absPath, cleanPrefix string) bool {
	if cleanPrefix == "" || cleanPrefix == "." {
		return false
	}
	if absPath == cleanPrefix || strings.EqualFold(absPath, cleanPrefix) {
		return true
	}
	return strings.HasPrefix(absPath, cleanPrefix+string(filepath.Separator))
}

// GetExcludes resolves repository-level and target-level exclusion paths.
// File exclusions must be declared declaratively in nomos.yaml (governance.quality_gates.exclude).
func GetExcludes(targetDir string) []string {
	return nil
}

// Walk traverses the target directory and executes fn, bypassing standard intermediate paths.
func Walk(targetDir string, fn filepath.WalkFunc) error {
	return WalkWithExcludes(targetDir, nil, fn)
}

// WalkWithExcludes traverses the target directory and executes fn, bypassing standard intermediate paths and explicit excludes.
func WalkWithExcludes(targetDir string, excludes []string, fn filepath.WalkFunc) error {
	return filepath.Walk(targetDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return fn(path, info, walkErr)
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		if IsExcluded(absPath, info.IsDir(), info.Name(), excludes) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(path, info, walkErr)
	})
}

// WalkDir is the fs.DirEntry equivalent of Walk.
func WalkDir(targetDir string, fn fs.WalkDirFunc) error {
	return WalkDirWithExcludes(targetDir, nil, fn)
}

// WalkDirWithExcludes is the fs.DirEntry equivalent of WalkWithExcludes.
func WalkDirWithExcludes(targetDir string, excludes []string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fn(path, d, walkErr)
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			absPath = path
		}

		if IsExcluded(absPath, d.IsDir(), d.Name(), excludes) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(path, d, walkErr)
	})
}

// WalkScope traverses the repository directory scope avoiding worktrees and build directories.
func WalkScope(rootDir string, fn filepath.WalkFunc) error {
	return Walk(rootDir, fn)
}
