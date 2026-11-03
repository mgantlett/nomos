package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

const defaultGoWorkVersion = "1.26.5"

// discoverWorkspaceModules identifies all Go modules located at the root or immediate child directories.
func discoverWorkspaceModules(wtPath string) []string {
	var uses []string
	if fi, err := os.Stat(filepath.Join(wtPath, "go.mod")); err == nil && !fi.IsDir() {
		uses = append(uses, "\t.")
	}
	entries, err := os.ReadDir(wtPath)
	if err != nil {
		return uses
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == constants.DirWorktrees || name == "vendor" || name == "testdata" {
			continue
		}
		subMod := filepath.Join(wtPath, name, "go.mod")
		if fi, err := os.Stat(subMod); err == nil && !fi.IsDir() {
			uses = append(uses, "\t./"+name)
		}
	}
	return uses
}

func siblingNomosReplace(repoRoot, wtPath string, uses []string) string {
	if filepath.Base(repoRoot) == constants.AppName || !modulesImportNomosSubstrate(wtPath, uses) {
		return ""
	}
	siblingNomos := filepath.Join(repoRoot, "..", constants.AppName, constants.DirSubstrate)
	fi, err := os.Stat(siblingNomos)
	if err != nil || !fi.IsDir() {
		return ""
	}
	rel, errRel := filepath.Rel(wtPath, siblingNomos)
	if errRel != nil {
		return ""
	}
	return fmt.Sprintf("\nreplace github.com/mgantlett/nomos/substrate => %s\n", rel)
}

func modulesImportNomosSubstrate(wtPath string, uses []string) bool {
	for _, u := range uses {
		modDir := strings.TrimSpace(u)
		raw, err := os.ReadFile(filepath.Join(wtPath, modDir, "go.mod"))
		if err == nil && strings.Contains(string(raw), "github.com/mgantlett/nomos/substrate") {
			return true
		}
	}
	return false
}

// resolveHighestGoVersion inspects discovered go.mod files and returns the highest declared Go version.
func resolveHighestGoVersion(wtPath string, uses []string) string {
	best := ""
	for _, u := range uses {
		modDir := strings.TrimSpace(u)
		ver := parseGoDirectiveFromMod(filepath.Join(wtPath, modDir, "go.mod"))
		if ver == "" {
			continue
		}
		if best == "" || compareGoSemver(ver, best) > 0 {
			best = ver
		}
	}
	if best == "" {
		return defaultGoWorkVersion
	}
	return best
}

func parseGoDirectiveFromMod(modPath string) string {
	raw, err := os.ReadFile(modPath)
	if err != nil {
		return ""
	}
	for _, rawLine := range strings.Split(string(raw), "\n") {
		line := strings.TrimSpace(rawLine)
		if after, ok := strings.CutPrefix(line, "go "); ok {
			fields := strings.Fields(after)
			if len(fields) > 0 {
				return strings.TrimSpace(fields[0])
			}
		}
	}
	return ""
}

func compareGoSemver(a, b string) int {
	pa := parseSemverParts(a)
	pb := parseSemverParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	var out [3]int
	clean := strings.SplitN(strings.TrimPrefix(v, "go"), "-", 2)[0]
	parts := strings.Split(clean, ".")
	for i := 0; i < len(parts) && i < 3; i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			out[i] = n
		}
	}
	return out
}

// scaffoldGoWork synthesizes a transient go.work file inside the hermetic worktree if Go modules exist.
func (m *Manager) scaffoldGoWork(wtPath string) error {
	uses := discoverWorkspaceModules(wtPath)
	if len(uses) == 0 {
		return nil
	}

	goVer := resolveHighestGoVersion(wtPath, uses)
	payload := fmt.Sprintf("go %s\n\nuse (\n%s\n)\n", goVer, strings.Join(uses, "\n"))
	payload += siblingNomosReplace(m.RepoRoot, wtPath, uses)
	return os.WriteFile(filepath.Join(wtPath, "go.work"), []byte(payload), 0644)
}
