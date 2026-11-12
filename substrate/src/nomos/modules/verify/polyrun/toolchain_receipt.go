// Package polyrun implements the deterministic Definition of Done (DoD) quality gate pipeline.
package polyrun

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	receiptSchemaVersion = "nomos-toolchain-receipt-v2"
	maxCachedReceipts    = 128
)

// computeWorkspaceSnapshotDigest computes a canonical content-addressed SHA-256 tree digest
// across committed HEAD blobs and live working-tree modifications, invariant to relative paths
// and worktree-vs-trunk directory location.
func ComputeWorkspaceSnapshotDigest(ctx *vscope.VerifyContext, root, targetDir string) string {
	return ComputeWorkspaceSnapshotDigestWithPackages(ctx, root, targetDir, nil)
}

// ComputeWorkspaceSnapshotDigestWithPackages executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ComputeWorkspaceSnapshotDigestWithPackages(ctx *vscope.VerifyContext, root, targetDir string, pkgs []string) string {
	gitDir := resolveSnapshotGitDir(ctx, root)
	if gitDir == "" {
		return ""
	}
	entries, oidHexLen, ok := loadCommittedTreeEntries(gitDir)
	if !ok {
		return ""
	}
	overlayDirtyTreeEntries(gitDir, entries, oidHexLen)

	relPaths := make([]string, 0, len(entries))
	for rel := range entries {
		relPaths = append(relPaths, rel)
	}
	sort.Strings(relPaths)

	h := sha256.New()
	_, _ = h.Write([]byte(receiptSchemaVersion + "\n"))
	_, _ = h.Write([]byte("target:" + canonicalTargetRelPath(gitDir, targetDir) + "\n"))
	hashScopedPackageTargets(pkgs, h)
	hashVerifyConfigFile(ctx, h)
	for _, rel := range relPaths {
		_, _ = h.Write([]byte(rel + "\x00" + entries[rel] + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashScopedPackageTargets(pkgs []string, h interface{ Write([]byte) (int, error) }) {
	if len(pkgs) == 0 {
		return
	}
	sorted := append([]string(nil), pkgs...)
	sort.Strings(sorted)
	if len(sorted) == 1 && sorted[0] == "./..." {
		return
	}
	if _, err := h.Write([]byte("pkgs:" + strings.Join(sorted, ",") + "\n")); err != nil {
		return
	}
}

func resolveSnapshotGitDir(ctx *vscope.VerifyContext, root string) string {
	candidate := ""
	if ctx != nil {
		candidate = vscope.ResolveGitDir(ctx)
	}
	if candidate == "" && root != "" {
		if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
			candidate = root
		}
	}
	if candidate == "" {
		return ""
	}
	if abs, err := filepath.Abs(candidate); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(candidate)
}

func canonicalTargetRelPath(gitDir, targetDir string) string {
	if targetDir == "" {
		return "."
	}
	absTarget := targetDir
	if abs, err := filepath.Abs(targetDir); err == nil {
		absTarget = abs
	}
	if rel, err := filepath.Rel(gitDir, filepath.Clean(absTarget)); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filepath.Clean(absTarget))
}

func loadCommittedTreeEntries(gitDir string) (map[string]string, int, bool) {
	out, err := workspace.RunSafeGit(gitDir, "ls-tree", "-r", "-z", constants.ValHead)
	if err != nil {
		return nil, 40, false
	}
	entries := make(map[string]string)
	oidHexLen := 40
	for _, tok := range strings.Split(string(out), "\x00") {
		tabIdx := strings.IndexByte(tok, '\t')
		if tabIdx <= 0 {
			continue
		}
		meta := strings.Fields(tok[:tabIdx])
		rel := filepath.ToSlash(tok[tabIdx+1:])
		if len(meta) < 3 || isTransientSnapshotRelPath(rel) {
			continue
		}
		if len(meta[2]) == 64 {
			oidHexLen = 64
		}
		entries[rel] = normalizeTreeEntryMode(meta[0]) + ":" + meta[2]
	}
	return entries, oidHexLen, true
}

func overlayDirtyTreeEntries(gitDir string, entries map[string]string, oidHexLen int) {
	out, err := workspace.RunSafeGit(gitDir, "status", "--porcelain", "-z", "-uall")
	if err != nil {
		return
	}
	dirty := collectDirtyNulStatusPaths(string(out))
	for rel := range dirty {
		applyDirtyWorkspaceEntry(gitDir, rel, entries, oidHexLen)
	}
}

func collectDirtyNulStatusPaths(raw string) map[string]struct{} {
	dirty := make(map[string]struct{})
	tokens := strings.Split(raw, "\x00")
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if len(tok) < 4 || tok[2] != ' ' {
			continue
		}
		addScopedSnapshotPath(tok[3:], dirty)
		if (tok[0] == 'R' || tok[0] == 'C' || tok[1] == 'R' || tok[1] == 'C') && i+1 < len(tokens) {
			i++
			addScopedSnapshotPath(tokens[i], dirty)
		}
	}
	return dirty
}

func addScopedSnapshotPath(rawRel string, dirty map[string]struct{}) {
	rel := strings.TrimSuffix(filepath.ToSlash(rawRel), "/")
	if rel != "" && !isTransientSnapshotRelPath(rel) {
		dirty[rel] = struct{}{}
	}
}

func isTransientSnapshotRelPath(rel string) bool {
	return strings.HasPrefix(rel, ".tmp/") || rel == ".tmp" || strings.HasPrefix(rel, "worktrees/") || rel == "worktrees"
}

func normalizeTreeEntryMode(mode string) string {
	switch mode {
	case "120000", "160000", "100755":
		return mode
	default:
		return "100644"
	}
}

func applyDirtyWorkspaceEntry(gitDir, rel string, entries map[string]string, oidHexLen int) {
	full := filepath.Join(gitDir, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil {
		delete(entries, rel)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		linkTarget, lErr := os.Readlink(full)
		if lErr != nil {
			linkTarget = ""
		}
		entries[rel] = "120000:" + computeGitBlobOID([]byte(linkTarget), oidHexLen)
		return
	}
	if info.IsDir() {
		return
	}
	data, rErr := os.ReadFile(full)
	if rErr != nil {
		entries[rel] = "unreadable"
		return
	}
	mode := "100644"
	if info.Mode()&0o111 != 0 {
		mode = "100755"
	}
	entries[rel] = mode + ":" + computeGitBlobOID(data, oidHexLen)
}

func computeGitBlobOID(data []byte, oidHexLen int) string {
	header := "blob " + strconv.Itoa(len(data)) + "\x00"
	if oidHexLen == 64 {
		h := sha256.New()
		_, _ = h.Write([]byte(header))
		_, _ = h.Write(data)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New()
	_, _ = h.Write([]byte(header))
	_, _ = h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func hashVerifyConfigFile(ctx *vscope.VerifyContext, h interface{ Write([]byte) (int, error) }) {
	if ctx == nil {
		return
	}
	cfgPath := vscope.HasLocalConfig(ctx.WorktreePath)
	if cfgPath == "" {
		cfgPath = vscope.HasLocalConfig(ctx.RepoRoot)
	}
	if cfgPath == "" {
		return
	}
	if data, err := os.ReadFile(cfgPath); err == nil {
		sum := sha256.Sum256(data)
		_, _ = h.Write([]byte("cfg:" + hex.EncodeToString(sum[:]) + "\n"))
	}
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
	if wErr := os.WriteFile(target, data, 0644); wErr != nil {
		return
	}
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
		if rErr := os.Remove(files[i].path); rErr != nil {
			continue
		}
	}
}
