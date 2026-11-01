package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// recompileSubstrateBinary synchronizes canonical Go command binaries into the Go bin SSoT
// whenever a synced commit modifies Go source or module manifests under substrate/,
// and purges any obsolete legacy binaries in ~/.nomos/bin (Law 6).
func recompileSubstrateBinary(repoRoot string) {
	homeDir := resolveUserHomeDir()
	if homeDir == "" {
		return
	}
	binDir := resolveCanonicalGoBinDir(homeDir)
	purgeLegacyDuplicateBinary(homeDir, filepath.Join(binDir, "nomos"))

	substrateDir := filepath.Join(repoRoot, "substrate")
	if info, err := os.Stat(filepath.Join(substrateDir, "go.mod")); err != nil || info.IsDir() {
		return
	}
	if !commitTouchedSubstrateGo(repoRoot) {
		return
	}
	targets := discoverRecompileTargets(substrateDir, binDir)
	if len(targets) == 0 {
		return
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fmt.Printf("⚠️  Warning: failed to create %s: %v\n", binDir, err)
		return
	}
	buildTargetsAtomically(substrateDir, binDir, targets)
}

func resolveUserHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	return strings.TrimSpace(homeDir)
}

// commitTouchedSubstrateGo returns true if HEAD~1..HEAD modified Go sources under substrate/,
// or if git history cannot be inspected (conservative fail-safe).
func commitTouchedSubstrateGo(repoRoot string) bool {
	if info, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil && info == nil {
		return true
	}
	out, err := workspace.SafeGitCommand(repoRoot, "diff", "--name-only", "HEAD~1..HEAD", "--", "substrate").Output()
	if err != nil {
		return true
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		p := strings.TrimSpace(line)
		if strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "go.mod") || strings.HasSuffix(p, "go.sum") {
			return true
		}
	}
	return false
}

// discoverRecompileTargets enumerates substrate/cmd/<name>/main.go packages that belong in binDir SSoT:
// core "nomos", standard "nomos-plugin-*" plugins, and any command already installed in binDir.
func discoverRecompileTargets(substrateDir, binDir string) []string {
	entries, err := os.ReadDir(filepath.Join(substrateDir, "cmd"))
	if err != nil {
		return nil
	}
	var targets []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		mainPath := filepath.Join(substrateDir, "cmd", name, "main.go")
		if info, statErr := os.Stat(mainPath); statErr != nil || info.IsDir() {
			continue
		}
		if shouldManageBinaryInSSoT(name, binDir) {
			targets = append(targets, name)
		}
	}
	sort.Strings(targets)
	return targets
}

func shouldManageBinaryInSSoT(name, binDir string) bool {
	if name == "nomos" || strings.HasPrefix(name, "nomos-plugin-") {
		return true
	}
	info, err := os.Stat(filepath.Join(binDir, name))
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func buildTargetsAtomically(substrateDir, binDir string, targets []string) {
	fmt.Printf("🔨 Recompiling %d substrate binary target(s) into %s (SSoT)...\n", len(targets), binDir)
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, name := range targets {
		wg.Add(1)
		go func(binName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			compileSingleTarget(substrateDir, binDir, binName)
		}(name)
	}
	wg.Wait()
}

func compileSingleTarget(substrateDir, binDir, name string) {
	tmpBin := filepath.Join(binDir, fmt.Sprintf(".tmp_%s_%d", name, os.Getpid()))
	targetBin := filepath.Join(binDir, name)
	buildCmd := exec.Command("go", "build", "-o", tmpBin, "./cmd/"+name)
	buildCmd.Dir = substrateDir
	buildCmd.Env = os.Environ()

	if out, bErr := buildCmd.CombinedOutput(); bErr != nil {
		removeBestEffort(tmpBin)
		fmt.Printf("⚠️  Warning: recompilation of %s failed: %v\n%s\n", name, bErr, string(out))
		return
	}
	if cErr := os.Chmod(tmpBin, 0o755); cErr != nil {
		removeBestEffort(tmpBin)
		fmt.Printf("⚠️  Warning: failed to chmod %s: %v\n", name, cErr)
		return
	}
	if rErr := os.Rename(tmpBin, targetBin); rErr != nil {
		removeBestEffort(tmpBin)
		fmt.Printf("⚠️  Warning: failed to atomically install %s: %v\n", targetBin, rErr)
		return
	}
	fmt.Printf("✅ Recompiled and installed %s (SSoT)\n", targetBin)
}

func removeBestEffort(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return
	}
}

// resolveCanonicalGoBinDir resolves the single authoritative Go binary directory
// matching install.sh and uninstall.sh (GOBIN -> go env GOBIN -> GOPATH/bin -> ~/go/bin).
func resolveCanonicalGoBinDir(homeDir string) string {
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		return gobin
	}
	if out, err := exec.Command("go", "env", "GOBIN").Output(); err == nil {
		if gobin := strings.TrimSpace(string(out)); gobin != "" {
			return gobin
		}
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		if gopath := strings.TrimSpace(string(out)); gopath != "" {
			first := strings.Split(gopath, string(os.PathListSeparator))[0]
			if first != "" {
				return filepath.Join(first, "bin")
			}
		}
	}
	return filepath.Join(homeDir, "go", "bin")
}

// purgeLegacyDuplicateBinary removes obsolete nomos and nomos-plugin-* binaries in ~/.nomos/bin (Law 6).
func purgeLegacyDuplicateBinary(homeDir, canonicalTarget string) {
	legacyDir := filepath.Join(homeDir, constants.DirNomosHome, "bin")
	if filepath.Clean(legacyDir) == filepath.Clean(filepath.Dir(canonicalTarget)) {
		return
	}
	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		n := entry.Name()
		if n == "nomos" || strings.HasPrefix(n, "nomos-plugin-") {
			p := filepath.Join(legacyDir, n)
			if rmErr := os.Remove(p); rmErr == nil {
				fmt.Printf("🧹 Purged legacy duplicate binary %s (Law 6 SSoT)\n", p)
			}
		}
	}
	removeBestEffort(legacyDir)
}
