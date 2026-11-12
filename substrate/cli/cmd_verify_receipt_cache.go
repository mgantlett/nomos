// Package cli implements short-lived cryptographic verification receipt caching
// across clean worktree pre-commit and 2PC task sync transitions.
// @intent intent.md task_lifecycle_cli_spec.md
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

const (
	worktreeVerifyReceiptSchema  = "nomos-worktree-verify-receipt-v1"
	worktreeVerifyReceiptTTL     = 5 * time.Minute
	envDisableVerifyReceiptCache = "NOMOS_NO_VERIFY_CACHE"
	minBaselineVerifyGates       = 42
)

// worktreeVerifyReceipt binds a passing 42-gate PipelineReport to the exact
// worktree root tree OID and protected trunk HEAD OID for bounded reuse.
type worktreeVerifyReceipt struct {
	SchemaVersion       string                 `json:"schema_version"`
	RepoRoot            string                 `json:"repo_root"`
	WorktreePath        string                 `json:"worktree_path"`
	TaskKey             string                 `json:"task_key,omitempty"`
	WorktreeTreeSHA     string                 `json:"worktree_tree_sha"`
	TrunkHeadSHA        string                 `json:"trunk_head_sha"`
	Strict              bool                   `json:"strict,omitempty"`
	VerifiedAtUnixMilli int64                  `json:"verified_at_unix_milli"`
	Report              *verify.PipelineReport `json:"report"`
}

// resolveTaskGateTargets resolves the repository root and active task worktree directory.
func resolveTaskGateTargets(taskKey string) (string, string) {
	targetDir, err := os.Getwd()
	if err != nil {
		targetDir = "."
	}
	repoRoot, err := workspace.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	worktreePath := targetDir
	if taskKey != "" {
		if mgr, mErr := workspace.NewManager(repoRoot); mErr == nil {
			wt := mgr.WorktreePath(taskKey)
			if fi, sErr := os.Stat(wt); sErr == nil && fi.IsDir() {
				worktreePath = wt
			}
		}
	}
	return repoRoot, worktreePath
}

// verifyReceiptCachePath computes the content-addressed receipt file path under NOMOS_HOME.
func verifyReceiptCachePath(repoRoot, worktreePath string) string {
	canonRepo, canonWT, ok := normalizeReceiptScopePaths(repoRoot, worktreePath)
	if !ok {
		return ""
	}
	sum := sha256.Sum256([]byte(canonRepo + "|" + canonWT))
	cacheDir := filepath.Join(config.GetNomosHome(), constants.DirCache, constants.DirVerifyReceipts)
	return filepath.Join(cacheDir, hex.EncodeToString(sum[:])+".json")
}

// saveWorktreeVerifyReceipt records a 5-minute verification receipt when a full 42-gate pass completes.
func saveWorktreeVerifyReceipt(repoRoot, worktreePath, taskKey string, report *verify.PipelineReport) {
	saveWorktreeVerifyReceiptAt(repoRoot, worktreePath, taskKey, report, time.Now())
}

// syncVerifyCommandReceipt updates or purges the local worktree verification receipt after 'nomos verify'.
func syncVerifyCommandReceipt(repoRoot, targetDir, taskKey, commitRange string, baseline, strict bool, report *verify.PipelineReport) {
	if report == nil || !report.Passed {
		invalidateWorktreeVerifyReceipt(repoRoot, targetDir)
		return
	}
	if !baseline && strings.TrimSpace(commitRange) == "" {
		saveWorktreeVerifyReceiptWithStrict(repoRoot, targetDir, taskKey, strict, report, time.Now())
	}
}

// saveWorktreeVerifyReceiptAt persists a timestamped verification receipt for deterministic TTL testing.
func saveWorktreeVerifyReceiptAt(repoRoot, worktreePath, taskKey string, report *verify.PipelineReport, now time.Time) {
	saveWorktreeVerifyReceiptWithStrict(repoRoot, worktreePath, taskKey, false, report, now)
}

func saveWorktreeVerifyReceiptWithStrict(repoRoot, worktreePath, taskKey string, strict bool, report *verify.PipelineReport, now time.Time) {
	if os.Getenv(envDisableVerifyReceiptCache) == "1" || !isCacheablePipelineReport(report) {
		return
	}
	canonRepo, canonWT, ok := normalizeReceiptScopePaths(repoRoot, worktreePath)
	if !ok {
		return
	}
	treeSHA := resolveStagedOrCleanWorktreeTreeSHA(canonWT)
	trunkSHA := resolveTrunkHeadSHA(canonRepo)
	if treeSHA == "" || trunkSHA == "" {
		return
	}
	cachePath := verifyReceiptCachePath(canonRepo, canonWT)
	if cachePath == "" {
		return
	}
	if mkErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkErr != nil {
		return
	}
	rec := worktreeVerifyReceipt{
		SchemaVersion:       worktreeVerifyReceiptSchema,
		RepoRoot:            canonRepo,
		WorktreePath:        canonWT,
		TaskKey:             strings.ToUpper(strings.TrimSpace(taskKey)),
		WorktreeTreeSHA:     treeSHA,
		TrunkHeadSHA:        trunkSHA,
		Strict:              strict,
		VerifiedAtUnixMilli: now.UnixMilli(),
		Report:              report,
	}
	data, mErr := json.Marshal(rec)
	if mErr != nil {
		return
	}
	if wErr := os.WriteFile(cachePath, data, 0o600); wErr != nil {
		return
	}
}

// loadValidWorktreeVerifyReceipt returns the cached PipelineReport strictly when the worktree
// has a clean git status, matching HEAD^{tree}, matching trunk HEAD SHA, and fresh TTL <= 5m.
func loadValidWorktreeVerifyReceipt(repoRoot, worktreePath, taskKey string, now time.Time) *verify.PipelineReport {
	if os.Getenv(envDisableVerifyReceiptCache) == "1" {
		return nil
	}
	canonRepo, canonWT, ok := normalizeReceiptScopePaths(repoRoot, worktreePath)
	if !ok || !isWorktreeStatusCompletelyClean(canonWT) {
		return nil
	}
	headTreeSHA := readGitObjectSHA(canonWT, constants.ValHead+"^{tree}")
	trunkSHA := resolveTrunkHeadSHA(canonRepo)
	if headTreeSHA == "" || trunkSHA == "" {
		return nil
	}
	rec := readStoredWorktreeVerifyReceipt(verifyReceiptCachePath(canonRepo, canonWT))
	if !matchesReceiptInvariants(rec, canonRepo, canonWT, taskKey, headTreeSHA, trunkSHA, now) {
		return nil
	}
	return rec.Report
}

// invalidateWorktreeVerifyReceipt removes any cached verification receipt for the worktree.
func invalidateWorktreeVerifyReceipt(repoRoot, worktreePath string) {
	cachePath := verifyReceiptCachePath(repoRoot, worktreePath)
	if cachePath == "" {
		return
	}
	if rmErr := os.Remove(cachePath); rmErr != nil && !os.IsNotExist(rmErr) {
		return
	}
}

func readStoredWorktreeVerifyReceipt(cachePath string) *worktreeVerifyReceipt {
	if cachePath == "" {
		return nil
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	var rec worktreeVerifyReceipt
	if uErr := json.Unmarshal(data, &rec); uErr != nil {
		return nil
	}
	return &rec
}

func matchesReceiptInvariants(
	rec *worktreeVerifyReceipt,
	canonRepo, canonWT, taskKey, headTreeSHA, trunkSHA string,
	now time.Time,
) bool {
	if !matchesReceiptScope(rec, canonRepo, canonWT, taskKey) {
		return false
	}
	ageMs := now.UnixMilli() - rec.VerifiedAtUnixMilli
	if ageMs < 0 || ageMs > worktreeVerifyReceiptTTL.Milliseconds() {
		return false
	}
	return rec.WorktreeTreeSHA == headTreeSHA && rec.TrunkHeadSHA == trunkSHA
}

func matchesReceiptScope(rec *worktreeVerifyReceipt, canonRepo, canonWT, taskKey string) bool {
	if rec == nil || rec.SchemaVersion != worktreeVerifyReceiptSchema || !isCacheablePipelineReport(rec.Report) {
		return false
	}
	if rec.RepoRoot != canonRepo || rec.WorktreePath != canonWT {
		return false
	}
	return rec.TaskKey == "" || taskKey == "" || strings.EqualFold(rec.TaskKey, strings.TrimSpace(taskKey))
}

func isCacheablePipelineReport(report *verify.PipelineReport) bool {
	return report != nil &&
		report.Passed &&
		report.GatesPassed >= minBaselineVerifyGates &&
		report.GatesPassed == report.GatesTotal
}

func normalizeReceiptScopePaths(repoRoot, worktreePath string) (string, string, bool) {
	canonRepo := canonicalReceiptDir(repoRoot)
	canonWT := canonicalWorktreeReceiptDir(worktreePath)
	if canonRepo == "" || canonWT == "" || canonRepo == canonWT {
		return "", "", false
	}
	return canonRepo, canonWT, true
}

func canonicalWorktreeReceiptDir(worktreePath string) string {
	trimmed := strings.TrimSpace(worktreePath)
	if trimmed == "" {
		return ""
	}
	if fi, err := os.Stat(trimmed); err == nil && fi.IsDir() {
		if root, rErr := workspace.FindWorkingTreeRoot(trimmed); rErr == nil && root != "" {
			return canonicalReceiptDir(root)
		}
	}
	return canonicalReceiptDir(trimmed)
}

func canonicalReceiptDir(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return ""
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		abs = trimmed
	}
	if resolved, sErr := filepath.EvalSymlinks(abs); sErr == nil && resolved != "" {
		return filepath.Clean(resolved)
	}
	if parentResolved, pErr := filepath.EvalSymlinks(filepath.Dir(abs)); pErr == nil && parentResolved != "" {
		return filepath.Clean(filepath.Join(parentResolved, filepath.Base(abs)))
	}
	return filepath.Clean(abs)
}

func isWorktreeStatusCompletelyClean(worktreeDir string) bool {
	out, err := workspace.SafeGitCommand(worktreeDir, "status", "--porcelain", "-z", "-uall").Output()
	return err == nil && len(out) == 0
}

// resolveStagedOrCleanWorktreeTreeSHA resolves HEAD^{tree} when clean, git write-tree when purely
// staged, or a temporary GIT_INDEX_FILE write-tree when unstaged working-tree changes exist.
func resolveStagedOrCleanWorktreeTreeSHA(worktreeDir string) string {
	out, err := workspace.SafeGitCommand(worktreeDir, "status", "--porcelain", "-z", "-uall").Output()
	if err != nil {
		return ""
	}
	if len(out) == 0 {
		return readGitObjectSHA(worktreeDir, constants.ValHead+"^{tree}")
	}
	if !isPurelyStagedCleanIndex(string(out)) {
		return resolveUnstagedWorkingTreeSHA(worktreeDir)
	}
	treeOut, wErr := workspace.SafeGitCommand(worktreeDir, "write-tree").Output()
	if wErr != nil {
		return ""
	}
	return normalizeGitOID(string(treeOut))
}

func isPurelyStagedCleanIndex(porcelainZ string) bool {
	tokens := strings.Split(porcelainZ, "\x00")
	sawEntry := false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok == "" {
			continue
		}
		if len(tok) < 4 || tok[2] != ' ' || tok[1] != ' ' || !isStagedIndexStatusCode(tok[0]) {
			return false
		}
		sawEntry = true
		if tok[0] == 'R' || tok[0] == 'C' {
			i++
			if i >= len(tokens) || tokens[i] == "" {
				return false
			}
		}
	}
	return sawEntry
}

func isStagedIndexStatusCode(code byte) bool {
	switch code {
	case 'M', 'A', 'D', 'R', 'C', 'T':
		return true
	default:
		return false
	}
}

func resolveTrunkHeadSHA(repoRoot string) string {
	trunk := detectDefaultBranch(repoRoot)
	if sha := readGitObjectSHA(repoRoot, "refs/heads/"+trunk); sha != "" {
		return sha
	}
	return readGitHeadSHA(repoRoot)
}

func readGitObjectSHA(dir, rev string) string {
	if dir == "" || rev == "" {
		return ""
	}
	out, err := workspace.SafeGitCommand(dir, constants.CmdRevParse, "--verify", rev).Output()
	if err != nil {
		return ""
	}
	return normalizeGitOID(string(out))
}

func normalizeGitOID(raw string) string {
	oid := strings.ToLower(strings.TrimSpace(raw))
	if len(oid) != 40 && len(oid) != 64 {
		return ""
	}
	for i := 0; i < len(oid); i++ {
		c := oid[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return oid
}
