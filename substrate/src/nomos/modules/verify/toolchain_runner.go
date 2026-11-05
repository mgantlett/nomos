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

	rec := executeGoTestWithCoverage(ctx, root, targetDir, snapshotKey)
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

func executeGoTestWithCoverage(ctx *VerifyContext, root, targetDir, snapshotKey string) *ToolchainSuiteReceipt {
	cmdCtx, cancel := context.WithTimeout(context.Background(), goToolchainTimeout)
	defer cancel()

	args := append([]string{"test", "-cover"}, resolveGoTestPackages(ctx, root, targetDir)...)
	cmd := exec.CommandContext(cmdCtx, "go", args...)
	cmd.Dir = targetDir
	cmd.Env = buildGoToolchainEnv(root, targetDir)
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

func buildGoToolchainEnv(root, targetDir string) []string {
	env := os.Environ()
	if val := os.Getenv("GOWORK"); val != "" {
		env = append(env, "GOWORK="+val)
	} else if !hasLocalGoWork(root, targetDir) {
		env = append(env, "GOWORK=off")
	}
	if os.Getenv("GOTOOLCHAIN") == "" {
		env = append(env, "GOTOOLCHAIN=auto")
	}
	return append(env, envInToolchainGate+"=1")
}

func resolveGoTestPackages(ctx *VerifyContext, root, targetDir string) []string {
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil || cfg.Governance.QualityGates.StrictMode || len(cfg.Governance.QualityGates.SubstrateSourceRoots) == 0 {
		return []string{"./..."}
	}
	var pkgs []string
	for _, sr := range cfg.Governance.QualityGates.SubstrateSourceRoots {
		candidate := sr
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(root, filepath.Clean(sr))
		}
		fi, err := os.Stat(candidate)
		if err != nil || !fi.IsDir() {
			continue
		}
		rel, rErr := filepath.Rel(targetDir, candidate)
		if rErr != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "." || relSlash == "" {
			return []string{"./..."}
		}
		pkgs = append(pkgs, "./"+relSlash+"/...")
	}
	if len(pkgs) == 0 {
		return []string{"./..."}
	}
	return pkgs
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
	if len(msg) <= maxToolchainErrOutput {
		return msg
	}
	if anchor := findToolchainFailureOffset(msg); anchor > 0 {
		msg = strings.TrimSpace(msg[anchor:])
		if len(msg) <= maxToolchainErrOutput {
			return msg
		}
	}
	return msg[:maxToolchainErrOutput] + "..."
}

func findToolchainFailureOffset(msg string) int {
	for _, marker := range []string{"\n--- FAIL:", "--- FAIL:", "\npanic:", "\nFAIL\t"} {
		if idx := strings.Index(msg, marker); idx >= 0 {
			return idx
		}
	}
	return -1
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
