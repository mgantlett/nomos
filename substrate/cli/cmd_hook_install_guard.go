// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const maxShellGuardRecursionDepth = 4

// isForbiddenNomosGoInstall evaluates whether a shell command line attempts an unauthorized
// 'go install' of a Nomos binary or plugin outside a clean IDLE primary 'main' trunk.
func isForbiddenNomosGoInstall(cmdLine, baseCwd, hermeticDir string) bool {
	return isForbiddenNomosGoInstallAtDepth(cmdLine, baseCwd, hermeticDir, 0)
}

func isForbiddenNomosGoInstallAtDepth(cmdLine, baseCwd, hermeticDir string, depth int) bool {
	if depth > maxShellGuardRecursionDepth || !strings.Contains(cmdLine, "go") {
		return false
	}
	segments, subs := tui.ParseShellCommandSegments(cmdLine)
	cwd := resolveInitialEvalDir(baseCwd)

	for _, seg := range segments {
		if nextCwd, ok := tui.MatchCdCommand(seg, cwd); ok {
			cwd = nextCwd
			continue
		}
		if subScript, ok := tui.MatchShellDashC(seg); ok {
			if isForbiddenNomosGoInstallAtDepth(subScript, cwd, hermeticDir, depth+1) {
				return true
			}
			continue
		}
		inv, ok := tui.MatchGoInstallCommand(seg)
		if !ok {
			continue
		}
		if targetsNomosPackage(inv.PkgArgs, cwd) && !isPermittedCleanMainGoInstall(inv, cwd, hermeticDir) {
			return true
		}
	}

	for _, sub := range subs {
		if isForbiddenNomosGoInstallAtDepth(sub, cwd, hermeticDir, depth+1) {
			return true
		}
	}
	return false
}

func resolveInitialEvalDir(baseCwd string) string {
	if strings.TrimSpace(baseCwd) != "" {
		return baseCwd
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// targetsNomosPackage checks whether the 'go install' package arguments or target Go module
// resolve to a Nomos binary or plugin ('nomos' or 'nomos-*').
func targetsNomosPackage(pkgArgs []string, cwd string) bool {
	pkgs := pkgArgs
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	for _, pkg := range pkgs {
		if strings.Contains(strings.ToLower(pkg), "nomos") {
			return true
		}
	}
	if strings.Contains(strings.ToLower(readGoModulePath(cwd)), "nomos") {
		return true
	}
	for _, pkg := range pkgs {
		if hasNomosSubcommandDir(cwd, pkg) {
			return true
		}
	}
	return false
}

func hasNomosSubcommandDir(evalDir, pkg string) bool {
	rawPkg := strings.SplitN(pkg, "@", 2)[0]
	isWildcard := rawPkg == "..." || rawPkg == "./..." || strings.HasSuffix(rawPkg, "/...")
	if rawPkg != "." && !isWildcard {
		base := strings.ToLower(filepath.Base(rawPkg))
		return base == "nomos" || strings.HasPrefix(base, "nomos-")
	}
	relDir := "."
	if strings.HasSuffix(rawPkg, "/...") {
		relDir = strings.TrimSuffix(rawPkg, "/...")
	}
	dir := normalizeCanonicalPath(relDir, evalDir)
	base := strings.ToLower(filepath.Base(dir))
	if base == "nomos" || strings.HasPrefix(base, "nomos-") || dirContainsNomosEntry(dir) {
		return true
	}
	if rawPkg == "..." || rawPkg == "./..." {
		return dirContainsNomosEntry(filepath.Join(dir, "cmd"))
	}
	return false
}

func dirContainsNomosEntry(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		name := strings.ToLower(ent.Name())
		if name == "nomos" || strings.HasPrefix(name, "nomos-") {
			return true
		}
	}
	return false
}

func readGoModulePath(startDir string) string {
	cur := normalizeCanonicalPath(".", startDir)
	for i := 0; i < 8 && cur != "" && cur != "/"; i++ {
		data, err := os.ReadFile(filepath.Join(cur, "go.mod"))
		if err == nil {
			return extractModuleDirective(string(data))
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return ""
}

func extractModuleDirective(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				return strings.Trim(fields[1], `"`)
			}
		}
	}
	return ""
}

// isPermittedCleanMainGoInstall fails closed unless the install runs outside hermetic containers
// and worktrees, uses strictly local relative package directories, and sits on a clean IDLE 'main' trunk.
func isPermittedCleanMainGoInstall(inv tui.GoInstallInvocation, cwd, hermeticDir string) bool {
	if hermeticDir != "" || inv.HasUnsafeFlag || strings.TrimSpace(cwd) == "" {
		return false
	}
	canonEvalDir := normalizeCanonicalPath(cwd, "/")
	if strings.Contains(canonEvalDir+"/", "/worktrees/") {
		return false
	}
	info, err := os.Stat(canonEvalDir)
	if err != nil || !info.IsDir() {
		return false
	}
	if !areLocalExistingSubdirs(inv.PkgArgs, canonEvalDir) {
		return false
	}
	return isCleanIdlePrimaryMainTrunk(canonEvalDir)
}

func areLocalExistingSubdirs(pkgArgs []string, canonEvalDir string) bool {
	pkgs := pkgArgs
	if len(pkgs) == 0 {
		pkgs = []string{"."}
	}
	for _, pkg := range pkgs {
		if !isSingleLocalExistingSubdir(pkg, canonEvalDir) {
			return false
		}
	}
	return true
}

func isSingleLocalExistingSubdir(pkg, canonEvalDir string) bool {
	if !isLocalRelativePkgToken(pkg) {
		return false
	}
	relDir := "."
	if pkg != "." && pkg != "./..." {
		relDir = strings.TrimSuffix(pkg, "/...")
	}
	pkgDir := normalizeCanonicalPath(relDir, canonEvalDir)
	if pkgDir != canonEvalDir && !strings.HasPrefix(pkgDir, canonEvalDir+"/") {
		return false
	}
	if strings.Contains(pkgDir+"/", "/worktrees/") {
		return false
	}
	pInfo, err := os.Stat(pkgDir)
	return err == nil && pInfo.IsDir()
}

func isLocalRelativePkgToken(pkg string) bool {
	if strings.ContainsAny(pkg, "@$~") {
		return false
	}
	return pkg == "." || pkg == "./..." || strings.HasPrefix(pkg, "./")
}

func isCleanIdlePrimaryMainTrunk(canonEvalDir string) bool {
	topOut, err := workspace.SafeGitCommand(canonEvalDir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return false
	}
	topLevel := normalizeCanonicalPath(strings.TrimSpace(string(topOut)), "/")
	if topLevel == "" || strings.Contains(topLevel+"/", "/worktrees/") {
		return false
	}
	gitInfo, err := os.Stat(filepath.Join(topLevel, ".git"))
	if err != nil || !gitInfo.IsDir() {
		return false
	}
	branchOut, err := workspace.SafeGitCommand(topLevel, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(branchOut)) != "main" {
		return false
	}
	statusOut, err := workspace.SafeGitCommand(topLevel, "status", "--porcelain").Output()
	if err != nil || strings.TrimSpace(string(statusOut)) != "" {
		return false
	}
	return !hasLinkedGitWorktrees(topLevel) && !hasActiveRepoTasks(topLevel)
}

func hasLinkedGitWorktrees(topLevel string) bool {
	out, err := workspace.SafeGitCommand(topLevel, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return true
	}
	worktreeCount := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			worktreeCount++
		}
	}
	return worktreeCount != 1
}

func hasActiveRepoTasks(topLevel string) bool {
	dbPath := config.ResolveGlobalDBPath()
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return true
	}
	project := task.ResolveRepoProject(topLevel)
	store := task.NewSQLiteTaskStore(db)
	tasks := task.FilterTasksByProject(store.ListTasks(), project, true)
	active := false
	for _, rec := range tasks {
		if task.IsInProgressStatus(rec.Status) {
			active = true
			break
		}
	}
	if closeErr := db.Close(); closeErr != nil {
		return true
	}
	return active
}
