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

type sessionBudgetState struct {
	StartedUnix int64 `json:"started_unix"`
	Warned75    bool  `json:"warned_75"`
	Warned90    bool  `json:"warned_90"`
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
	st := loadBudgetState(path)
	if st.StartedUnix > 0 {
		return
	}
	st.StartedUnix = time.Now().Unix()
	saveBudgetState(path, st)
}

func evaluateSessionBudgetRadar(p *HookPayload, hermeticDir string, budgetSec int) string {
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
	pct := (elapsed * 100) / budgetSec
	if pct >= 90 && !st.Warned90 {
		st.Warned75 = true
		st.Warned90 = true
		saveBudgetState(path, st)
		return fmt.Sprintf(
			"🚨 [Nomos Budget Radar] %d%% of session time budget elapsed (%ds / %ds): finalize outputs, stop background scratch processes, and run final verification immediately.",
			pct, elapsed, budgetSec,
		)
	}
	if pct >= 75 && !st.Warned75 {
		st.Warned75 = true
		saveBudgetState(path, st)
		return fmt.Sprintf(
			"⏱️ [Nomos Budget Radar] %d%% of session time budget elapsed (%ds / %ds): commit or verify a working baseline artifact now before further iteration.",
			pct, elapsed, budgetSec,
		)
	}
	return ""
}

func resolveBudgetStateFilePath(p *HookPayload, hermeticDir string) string {
	keySeed := strings.TrimSpace(p.SessionID)
	if keySeed == "" {
		keySeed = filepath.Clean(resolveLoopTrackingDir(p, hermeticDir))
	}
	sum := sha256.Sum256([]byte(keySeed))
	name := fmt.Sprintf("nomos-hook-budget-%s.json", hex.EncodeToString(sum[:8]))
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
	return filepath.Join(hookDir, name)
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

