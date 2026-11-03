// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	envInToolchainGate    = "NOMOS_IN_TOOLCHAIN_GATE"
	envSkipToolchainGate  = "NOMOS_SKIP_TOOLCHAIN_GATE"
	envNoVerifyCache      = "NOMOS_NO_VERIFY_CACHE"
	envForceFullGoTest    = "NOMOS_FORCE_FULL_GO_TEST"
	goToolchainTimeout    = 120 * time.Second
	maxToolchainErrOutput = 400
)

var goCoverageLineRe = regexp.MustCompile(`coverage:\s+([0-9.]+)%`)

// ToolchainSuiteReceipt stores the shared compilation, test, and statement coverage result for a workspace snapshot.
type ToolchainSuiteReceipt struct {
	SnapshotSHA256 string  `json:"snapshot_sha256"`
	GoPassed       bool    `json:"go_passed"`
	GoSkippedNonGo bool    `json:"go_skipped_non_go,omitempty"`
	GoOutput       string  `json:"go_output,omitempty"`
	AvgCoverage    float64 `json:"avg_coverage"`
	CacheHit       bool    `json:"-"`
	TimestampUTC   string  `json:"timestamp_utc"`
}

func evaluateSharedGoSuite(ctx *VerifyContext, root, targetDir string) *ToolchainSuiteReceipt {
	if cachedCtx := loadContextReceipt(ctx); cachedCtx != nil {
		return cachedCtx
	}
	if shouldSkipGoSuiteForNonGoDiff(ctx, root) {
		skipped := &ToolchainSuiteReceipt{
			GoPassed:       true,
			GoSkippedNonGo: true,
			AvgCoverage:    100.0,
			TimestampUTC:   time.Now().UTC().Format(time.RFC3339),
		}
		storeContextReceipt(ctx, skipped)
		return skipped
	}

	snapshotKey := computeWorkspaceSnapshotDigest(ctx, root, targetDir)
	if diskHit := tryLoadDiskReceipt(ctx, snapshotKey); diskHit != nil {
		storeContextReceipt(ctx, diskHit)
		return diskHit
	}

	rec := executeGoTestWithCoverage(targetDir, snapshotKey)
	storeContextReceipt(ctx, rec)
	trySaveDiskReceipt(ctx, snapshotKey, rec)
	return rec
}

func loadContextReceipt(ctx *VerifyContext) *ToolchainSuiteReceipt {
	if ctx == nil {
		return nil
	}
	existing, ok := ctx.CoverageRep.(*ToolchainSuiteReceipt)
	if !ok {
		return nil
	}
	return existing
}

func tryLoadDiskReceipt(ctx *VerifyContext, snapshotKey string) *ToolchainSuiteReceipt {
	if !canUseToolchainReceiptCache(ctx) || snapshotKey == "" {
		return nil
	}
	cached := loadToolchainReceipt(snapshotKey)
	if cached == nil || !cached.GoPassed {
		return nil
	}
	cached.CacheHit = true
	return cached
}

func trySaveDiskReceipt(ctx *VerifyContext, snapshotKey string, rec *ToolchainSuiteReceipt) {
	if rec != nil && rec.GoPassed && canUseToolchainReceiptCache(ctx) && snapshotKey != "" {
		saveToolchainReceipt(snapshotKey, rec)
	}
}

func storeContextReceipt(ctx *VerifyContext, rec *ToolchainSuiteReceipt) {
	if ctx != nil && rec != nil {
		ctx.CoverageRep = rec
	}
}

func canUseToolchainReceiptCache(ctx *VerifyContext) bool {
	if os.Getenv(envNoVerifyCache) == "1" {
		return false
	}
	if ctx != nil && ctx.AllFiles {
		return false
	}
	return true
}

func executeGoTestWithCoverage(targetDir, snapshotKey string) *ToolchainSuiteReceipt {
	cmdCtx, cancel := context.WithTimeout(context.Background(), goToolchainTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "go", "test", "-cover", "./...")
	cmd.Dir = targetDir
	env := os.Environ()
	if val := os.Getenv("GOWORK"); val != "" {
		env = append(env, "GOWORK="+val)
	}
	cmd.Env = append(env, envInToolchainGate+"=1")
	out, err := cmd.CombinedOutput()
	outStr := string(out)

	if err != nil {
		return &ToolchainSuiteReceipt{
			SnapshotSHA256: snapshotKey,
			GoPassed:       false,
			GoOutput:       outStr,
			TimestampUTC:   time.Now().UTC().Format(time.RFC3339),
		}
	}

	return &ToolchainSuiteReceipt{
		SnapshotSHA256: snapshotKey,
		GoPassed:       true,
		AvgCoverage:    parseAverageCoverage(outStr),
		TimestampUTC:   time.Now().UTC().Format(time.RFC3339),
	}
}

func parseAverageCoverage(output string) float64 {
	matches := goCoverageLineRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 100.0
	}
	var sum float64
	for _, m := range matches {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			sum += val
		}
	}
	return sum / float64(len(matches))
}

func truncateToolchainOutput(raw string) string {
	msg := strings.TrimSpace(raw)
	if len(msg) > maxToolchainErrOutput {
		return msg[:maxToolchainErrOutput] + "..."
	}
	return msg
}

func shouldSkipGoSuiteForNonGoDiff(ctx *VerifyContext, root string) bool {
	if ctx == nil || ctx.AllFiles || ctx.IsBaseline || os.Getenv(envForceFullGoTest) == "1" {
		return false
	}
	if !isWorktreeContext(ctx, root) {
		return false
	}
	files := resolveToolchainScopeFiles(ctx, root)
	if len(files) == 0 {
		return false
	}
	return !hasGoOrContractImpact(files)
}

func resolveToolchainScopeFiles(ctx *VerifyContext, root string) []string {
	if ctx == nil || ctx.AllFiles || ctx.IsBaseline {
		return nil
	}
	workDir := root
	if ctx.WorktreePath != "" {
		workDir = ctx.WorktreePath
	}
	if hasCommitRangeScope(ctx) {
		return resolveUnattestedRangeFiles(ctx, workDir)
	}
	fileSet := make(map[string]struct{})
	for _, f := range getModifiedFilesInScope(ctx, workDir) {
		fileSet[filepath.ToSlash(f)] = struct{}{}
	}
	for _, f := range ListModifiedWorktreeFiles(workDir, ctx.TaskID) {
		fileSet[filepath.ToSlash(f)] = struct{}{}
	}
	collectUntrackedAllFiles(workDir, fileSet)

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		if f != "" {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

func collectUntrackedAllFiles(workDir string, fileSet map[string]struct{}) {
	out, err := workspace.RunSafeGit(workDir, "status", "--porcelain", "-uall")
	if err != nil {
		return
	}
	for _, rawLine := range strings.Split(string(out), "\n") {
		if len(rawLine) < 4 {
			continue
		}
		rel := strings.TrimSpace(strings.Trim(rawLine[3:], `"`))
		if idx := strings.Index(rel, sepArrow); idx >= 0 {
			rel = strings.TrimSpace(rel[idx+len(sepArrow):])
		}
		if rel != "" && !strings.HasPrefix(rel, ".tmp/") && !strings.HasPrefix(rel, "worktrees/") {
			fileSet[filepath.ToSlash(rel)] = struct{}{}
		}
	}
}

func hasGoOrContractImpact(files []string) bool {
	for _, rel := range files {
		lower := strings.ToLower(filepath.ToSlash(rel))
		base := filepath.Base(lower)
		ext := filepath.Ext(lower)
		if ext == ".go" {
			return true
		}
		switch base {
		case "go.mod", "go.sum", "nomos.schema.json", "nomos.yaml", "nomos.yml", ".nomos.yaml", ".nomos.yml":
			return true
		}
	}
	return false
}
