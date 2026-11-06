// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// sessionBudgetState persists wall-clock budget warnings and deduplicated host-health snapshots per session.
type sessionBudgetState struct {
	StartedUnix     int64           `json:"started_unix"`
	LastPreToolUnix int64           `json:"last_pre_tool_unix,omitempty"`
	Warned75        bool            `json:"warned_75"`
	Warned90        bool            `json:"warned_90"`
	WarnedSlowStep  bool            `json:"warned_slow_step,omitempty"`
	HealthAlerts    map[string]bool `json:"health_alerts,omitempty"`
}

func resolveEffectiveBudgetSeconds(flagSec int) int {
	if flagSec > 0 {
		return flagSec
	}
	raw := strings.TrimSpace(os.Getenv("NOMOS_BUDGET_SECONDS"))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func ensureSessionBudgetStamp(p *HookPayload, hermeticDir string, budgetSec int) {
	if p == nil || budgetSec <= 0 {
		return
	}
	path := resolveBudgetStateFilePath(p, hermeticDir)
	if path == "" {
		return
	}
	nowUnix := time.Now().Unix()
	st := loadBudgetState(path)
	if st.StartedUnix <= 0 {
		st.StartedUnix = nowUnix
	}
	st.LastPreToolUnix = nowUnix
	saveBudgetState(path, st)
}

// evaluateSessionBudgetRadar evaluates wall-clock session budget checkpoints using the active advisory profile.
func evaluateSessionBudgetRadar(p *HookPayload, hermeticDir string, budgetSec int) string {
	return evaluateSessionBudgetRadarWithProfile(p, hermeticDir, budgetSec, resolveHookAdvisoryProfile(0, 0, 0, ""))
}

// evaluateSessionBudgetRadarWithProfile emits deduplicated warn and critical budget advisories
// according to the clamped HookAdvisoryProfile thresholds and verbosity style.
func evaluateSessionBudgetRadarWithProfile(p *HookPayload, hermeticDir string, budgetSec int, profile HookAdvisoryProfile) string {
	if p == nil || p.IsPostInvoke || budgetSec <= 0 {
		return ""
	}
	path := resolveBudgetStateFilePath(p, hermeticDir)
	if path == "" {
		return ""
	}
	nowUnix := time.Now().Unix()
	st := loadBudgetState(path)
	if st.StartedUnix <= 0 {
		st.StartedUnix = nowUnix
		saveBudgetState(path, st)
		return ""
	}
	elapsed := int(nowUnix - st.StartedUnix)
	if elapsed < 0 {
		return ""
	}
	warnPct := profile.WarnPct
	if warnPct <= 0 {
		warnPct = defaultBudgetWarnPct
	}
	critPct := profile.CritPct
	if critPct <= warnPct {
		critPct = defaultBudgetCritPct
	}
	pct := (elapsed * 100) / budgetSec
	var checkpointMsg string
	dirty := false
	if pct >= critPct && !st.Warned90 {
		st.Warned75 = true
		st.Warned90 = true
		dirty = true
		checkpointMsg = formatBudgetCheckpointAdvisory(pct, elapsed, budgetSec, true, profile.AdvisoryStyle)
	} else if pct >= warnPct && !st.Warned75 {
		st.Warned75 = true
		dirty = true
		checkpointMsg = formatBudgetCheckpointAdvisory(pct, elapsed, budgetSec, false, profile.AdvisoryStyle)
	}
	slowMsg := evaluateSlowStepBurn(&st, p, nowUnix, elapsed, budgetSec, profile.AdvisoryStyle, &dirty)
	if st.LastPreToolUnix != nowUnix {
		st.LastPreToolUnix = nowUnix
		dirty = true
	}
	if dirty {
		saveBudgetState(path, st)
	}
	return joinHookAdvisories(checkpointMsg, slowMsg)
}

func evaluateSlowStepBurn(st *sessionBudgetState, p *HookPayload, nowUnix int64, elapsed, budgetSec int, style string, dirty *bool) string {
	if st.WarnedSlowStep || st.LastPreToolUnix <= 0 || strings.TrimSpace(p.CommandLine) == "" {
		return ""
	}
	if commandUsesExplicitTimeout(p.CommandLine) {
		return ""
	}
	stepSec := int(nowUnix - st.LastPreToolUnix)
	threshold := budgetSec / 4
	if threshold < 45 {
		threshold = 45
	}
	if stepSec < threshold {
		return ""
	}
	st.WarnedSlowStep = true
	*dirty = true
	rem := budgetSec - elapsed
	if rem < 0 {
		rem = 0
	}
	return formatSlowStepTimeoutAdvisory(stepSec, rem, style)
}

func commandUsesExplicitTimeout(cmdLine string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(cmdLine)))
	idx := skipCommandPrefixes(fields)
	if idx >= len(fields) {
		return false
	}
	exe := filepath.Base(fields[idx])
	return exe == "timeout" || exe == "gtimeout"
}

func resolveBudgetStateFilePath(p *HookPayload, hermeticDir string) string {
	keySeed := strings.TrimSpace(p.SessionID)
	if keySeed == "" {
		keySeed = filepath.Clean(resolveLoopTrackingDir(p, hermeticDir))
	}
	sum := sha256.Sum256([]byte(keySeed))
	name := fmt.Sprintf("nomos-hook-budget-%s.json", hex.EncodeToString(sum[:8]))
	hookDir := resolveHookStateCacheDir()
	if hookDir == "" {
		return ""
	}
	return filepath.Join(hookDir, name)
}

// resolveHookStateCacheDir returns a verified non-symlink 0700 user cache directory for hook state files.
func resolveHookStateCacheDir() string {
	baseDir, err := os.UserCacheDir()
	if err != nil || baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), fmt.Sprintf("nomos-cache-%d", os.Getuid()))
	}
	hookDir := filepath.Join(baseDir, "nomos", "hooks")
	if mkErr := os.MkdirAll(hookDir, 0700); mkErr != nil {
		return ""
	}
	fi, statErr := os.Lstat(hookDir)
	if statErr != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	return hookDir
}

func loadBudgetState(path string) sessionBudgetState {
	var st sessionBudgetState
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if uErr := json.Unmarshal(data, &st); uErr != nil {
		return sessionBudgetState{}
	}
	return st
}

func saveBudgetState(path string, st sessionBudgetState) {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func joinHookAdvisories(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "\n" + b
}

// evaluateVerifiedHostHealth inspects PostToolUse error outputs for verified environmental faults
// and emits a single deduplicated host-state advisory per fault class in the session.
func evaluateVerifiedHostHealth(p *HookPayload, hermeticDir string) string {
	return evaluateVerifiedHostHealthWithStyle(p, hermeticDir, resolveHookAdvisoryProfile(0, 0, 0, "").AdvisoryStyle)
}

// evaluateVerifiedHostHealthWithStyle records verified environmental faults once per session
// and formats the single-turn host-state advisory using the active advisory style.
func evaluateVerifiedHostHealthWithStyle(p *HookPayload, hermeticDir, style string) string {
	if p == nil || p.IsPostInvoke || strings.TrimSpace(p.ToolResponseText) == "" {
		return ""
	}
	faultKey, msg := classifyHostHealthFault(p.ToolResponseText, p.CommandLine)
	if faultKey == "" || msg == "" {
		return ""
	}
	rendered := formatHostHealthAdvisory(faultKey, msg, style)
	path := resolveBudgetStateFilePath(p, hermeticDir)
	if path == "" {
		return rendered
	}
	st := loadBudgetState(path)
	if st.HealthAlerts != nil && st.HealthAlerts[faultKey] {
		return ""
	}
	if st.HealthAlerts == nil {
		st.HealthAlerts = make(map[string]bool)
	}
	st.HealthAlerts[faultKey] = true
	saveBudgetState(path, st)
	return rendered
}

// classifyHostHealthFault maps deterministic OS, filesystem, and network failure signatures to remedies.
func classifyHostHealthFault(respText, cmdLine string) (string, string) {
	lower := strings.ToLower(respText)
	if hasAnySubstring(lower,
		"temporary failure in name resolution",
		"could not resolve host",
		"network is unreachable",
		"name or service not known",
	) {
		return "egress_dns_offline", "🩺 [Nomos Verified Host State] Network egress or DNS resolution is unavailable: avoid remote downloads and use local cached wheels, vendor directories, or offline flags (--offline / --no-index)."
	}
	if hasAnySubstring(lower, "deb.debian.org", "security.debian.org") &&
		hasAnySubstring(lower, "does not have a release file", "404  not found", "404 not found") {
		return "apt_archive_mirror", "🩺 [Nomos Verified Host State] Container APT sources target an archived EOL Debian mirror: rewrite /etc/apt/sources.list to archive.debian.org with Acquire::Check-Valid-Until \"false\" or use installed language toolchains."
	}
	if hasAnySubstring(lower, "read-only file system", "erofs") {
		return "readonly_fs", "🩺 [Nomos Verified Host State] Target mount is read-only (EROFS): redirect scratch, cache, and build outputs to writable /tmp or the active worktree."
	}
	if hasAnySubstring(lower, "no space left on device", "enospc", "cannot allocate memory") {
		return "resource_exhausted", "🩺 [Nomos Verified Host State] Host disk or memory exhaustion detected (ENOSPC/OOM): clean temporary build artifacts and bound worker concurrency before retrying."
	}
	if strings.TrimSpace(cmdLine) != "" && hasAnySubstring(lower, "command not found", "executable file not found") {
		exe := extractPrimaryCommandToken(cmdLine)
		return "missing_bin:" + exe, fmt.Sprintf("🩺 [Nomos Verified Host State] Executable '%s' is not installed on PATH: switch to an available runtime (e.g. python3, sh, go) or install the package before retrying.", exe)
	}
	return "", ""
}

func hasAnySubstring(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func extractPrimaryCommandToken(cmdLine string) string {
	fields := strings.Fields(strings.TrimSpace(cmdLine))
	idx := skipCommandPrefixes(fields)
	if idx >= len(fields) {
		return "command"
	}
	base := filepath.Base(strings.Trim(fields[idx], "\"'`;|&"))
	if base == "" {
		return "command"
	}
	return base
}

