// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	receiptSchemaVersion = "nomos-toolchain-receipt-v1"
	maxCachedReceipts    = 128
)

func computeWorkspaceSnapshotDigest(ctx *VerifyContext, root, targetDir string) string {
	gitDir := resolveSnapshotGitDir(ctx, root)
	if gitDir == "" {
		return ""
	}
	baseSHA := resolveBaseCommitSHA(gitDir)
	if baseSHA == "" {
		return ""
	}

	deltaFiles := collectSnapshotDeltaFiles(gitDir, baseSHA)
	h := sha256.New()
	_, _ = h.Write([]byte(receiptSchemaVersion + "\n"))
	_, _ = h.Write([]byte("target:" + filepath.ToSlash(targetDir) + "\n"))
	_, _ = h.Write([]byte("base:" + baseSHA + "\n"))
	hashVerifyConfigFile(ctx, h)

	for _, rel := range deltaFiles {
		hashSingleWorkspacePath(gitDir, rel, h)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func resolveSnapshotGitDir(ctx *VerifyContext, root string) string {
	if ctx != nil {
		if d := resolveGitDir(ctx); d != "" {
			return d
		}
	}
	if root != "" {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			return root
		}
	}
	return ""
}

func resolveBaseCommitSHA(gitDir string) string {
	candidates := []string{constants.ValMain, "origin/main", constants.ValMaster}
	for _, branch := range candidates {
		out, err := workspace.RunSafeGit(gitDir, "merge-base", branch, constants.ValHead)
		if err == nil {
			if sha := strings.TrimSpace(string(out)); len(sha) == 40 {
				return sha
			}
		}
	}
	out, err := workspace.RunSafeGit(gitDir, constants.CmdRevParse, constants.ValHead)
	if err != nil {
		return ""
	}
	sha := strings.TrimSpace(string(out))
	if len(sha) != 40 {
		return ""
	}
	return sha
}

func collectSnapshotDeltaFiles(gitDir, baseSHA string) []string {
	fileSet := make(map[string]struct{})
	collectBaseDiffPaths(gitDir, baseSHA, fileSet)
	collectUntrackedAllFiles(gitDir, fileSet)
	collectPorcelainStatusPaths(gitDir, fileSet)

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		if !strings.HasPrefix(f, ".tmp/") && !strings.HasPrefix(f, "worktrees/") {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

func collectBaseDiffPaths(gitDir, baseSHA string, fileSet map[string]struct{}) {
	out, err := workspace.RunSafeGit(gitDir, "diff", "--name-only", baseSHA, constants.ValHead)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if clean := strings.TrimSpace(strings.Trim(line, `"`)); clean != "" {
			fileSet[filepath.ToSlash(clean)] = struct{}{}
		}
	}
}

func collectPorcelainStatusPaths(gitDir string, fileSet map[string]struct{}) {
	out, err := workspace.RunSafeGit(gitDir, "status", "--porcelain")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		addPorcelainLinePaths(line, fileSet)
	}
}

func addPorcelainLinePaths(line string, fileSet map[string]struct{}) {
	if len(line) < 4 {
		return
	}
	clean := strings.TrimSpace(strings.Trim(line[3:], `"`))
	if idx := strings.Index(clean, sepArrow); idx >= 0 {
		if oldRel := strings.TrimSpace(clean[:idx]); oldRel != "" {
			fileSet[filepath.ToSlash(oldRel)] = struct{}{}
		}
		clean = strings.TrimSpace(clean[idx+len(sepArrow):])
	}
	if clean != "" {
		fileSet[filepath.ToSlash(clean)] = struct{}{}
	}
}

func hashVerifyConfigFile(ctx *VerifyContext, h interface{ Write([]byte) (int, error) }) {
	if ctx == nil {
		return
	}
	cfgPath := hasLocalConfig(ctx.WorktreePath)
	if cfgPath == "" {
		cfgPath = hasLocalConfig(ctx.RepoRoot)
	}
	if cfgPath == "" {
		return
	}
	if data, err := os.ReadFile(cfgPath); err == nil {
		sum := sha256.Sum256(data)
		_, _ = h.Write([]byte("cfg:" + hex.EncodeToString(sum[:]) + "\n"))
	}
}

func hashSingleWorkspacePath(gitDir, rel string, h interface{ Write([]byte) (int, error) }) {
	full := filepath.Join(gitDir, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil {
		_, _ = h.Write([]byte("del:" + rel + "\n"))
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, lErr := os.Readlink(full)
		if lErr != nil {
			linkTarget = ""
		}
		_, _ = h.Write([]byte("sym:" + rel + ":" + linkTarget + "\n"))
		return
	}
	if info.IsDir() {
		_, _ = h.Write([]byte("dir:" + rel + "\n"))
		return
	}
	data, rErr := os.ReadFile(full)
	if rErr != nil {
		_, _ = h.Write([]byte("unreadable:" + rel + "\n"))
		return
	}
	sum := sha256.Sum256(data)
	_, _ = h.Write([]byte("file:" + rel + ":" + hex.EncodeToString(sum[:]) + "\n"))
}

func loadToolchainReceipt(snapshotKey string) *ToolchainSuiteReceipt {
	if len(snapshotKey) != 64 {
		return nil
	}
	path := filepath.Join(config.GetNomosVerifyCacheDir(), snapshotKey+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rec ToolchainSuiteReceipt
	if json.Unmarshal(data, &rec) != nil || rec.SnapshotSHA256 != snapshotKey {
		return nil
	}
	return &rec
}

func saveToolchainReceipt(snapshotKey string, rec *ToolchainSuiteReceipt) {
	if len(snapshotKey) != 64 || rec == nil || !rec.GoPassed {
		return
	}
	cacheDir := config.GetNomosVerifyCacheDir()
	if os.MkdirAll(cacheDir, 0755) != nil {
		return
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	target := filepath.Join(cacheDir, snapshotKey+".json")
	_ = os.WriteFile(target, data, 0644)
	pruneVerifyReceiptCache(cacheDir)
}

func pruneVerifyReceiptCache(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil || len(entries) <= maxCachedReceipts {
		return
	}
	type timedFile struct {
		path    string
		modNano int64
	}
	files := make([]timedFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if fi, iErr := e.Info(); iErr == nil {
			files = append(files, timedFile{path: filepath.Join(cacheDir, e.Name()), modNano: fi.ModTime().UnixNano()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modNano < files[j].modNano })
	for i := 0; i < len(files)-maxCachedReceipts; i++ {
		_ = os.Remove(files[i].path)
	}
}
