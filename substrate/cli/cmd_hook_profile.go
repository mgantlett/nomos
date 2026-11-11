// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultBudgetWarnPct     = 75
	defaultBudgetCritPct     = 90
	defaultLoopStrikes       = 3
	carbonDefaultLoopStrikes = 2
	carbonReadSpiralWarn     = 7
	carbonReadSpiralCrit     = 12
	minBudgetWarnPct         = 25
	maxBudgetWarnPct         = 90
	minBudgetCritFloorPct    = 50
	maxBudgetCritPct         = 99
	minLoopStrikes           = 2
	maxLoopStrikesBound      = 10
	advisoryStyleStandard    = "standard"
	advisoryStyleTerse       = "terse"
	advisoryProfileCarbon    = "carbon"
)

// HookAdvisoryProfile defines bounded, model-family-tunable advisory parameters
// for nomos hook while preserving strict invariance of all hard safety gates.
type HookAdvisoryProfile struct {
	WarnPct        int
	CritPct        int
	LoopStrikes    int
	ReadSpiralWarn int
	ReadSpiralCrit int
	AdvisoryStyle  string
}

// hookAdvisoryFlags binds CLI persistent flags on 'nomos hook' before clamping.
type hookAdvisoryFlags struct {
	warnPct         int
	critPct         int
	loopStrikes     int
	advisoryStyle   string
	advisoryProfile string
}

// resolve normalizes CLI flag values and environment fallbacks into a clamped HookAdvisoryProfile.
func (f hookAdvisoryFlags) resolve() HookAdvisoryProfile {
	return resolveHookAdvisoryProfileWithPreset(f.warnPct, f.critPct, f.loopStrikes, f.advisoryStyle, f.advisoryProfile)
}

// resolveHookAdvisoryProfile computes bounded advisory thresholds from CLI flags
// and environment variables (NOMOS_BUDGET_WARN_PCT, NOMOS_BUDGET_CRIT_PCT,
// NOMOS_LOOP_STRIKES, NOMOS_ADVISORY_STYLE, NOMOS_ADVISORY_PROFILE).
func resolveHookAdvisoryProfile(warnFlag, critFlag, strikesFlag int, styleFlag string) HookAdvisoryProfile {
	return resolveHookAdvisoryProfileWithPreset(warnFlag, critFlag, strikesFlag, styleFlag, "")
}

func resolveHookAdvisoryProfileWithPreset(warnFlag, critFlag, strikesFlag int, styleFlag, presetFlag string) HookAdvisoryProfile {
	isCarbon := resolveAdvisoryPreset(presetFlag, "NOMOS_ADVISORY_PROFILE") == advisoryProfileCarbon
	strikesFallback := defaultLoopStrikes
	spiralWarn := readSpiralWarnThreshold
	spiralCrit := readSpiralCritThreshold
	if isCarbon {
		strikesFallback = carbonDefaultLoopStrikes
		spiralWarn = carbonReadSpiralWarn
		spiralCrit = carbonReadSpiralCrit
	}

	rawWarn := resolveAdvisoryInt(warnFlag, "NOMOS_BUDGET_WARN_PCT", defaultBudgetWarnPct)
	warn := clampAdvisoryInt(rawWarn, minBudgetWarnPct, maxBudgetWarnPct)

	critMin := warn + 5
	if critMin < minBudgetCritFloorPct {
		critMin = minBudgetCritFloorPct
	}
	rawCrit := resolveAdvisoryInt(critFlag, "NOMOS_BUDGET_CRIT_PCT", defaultBudgetCritPct)
	crit := clampAdvisoryInt(rawCrit, critMin, maxBudgetCritPct)

	rawStrikes := resolveAdvisoryInt(strikesFlag, "NOMOS_LOOP_STRIKES", strikesFallback)
	strikes := clampAdvisoryInt(rawStrikes, minLoopStrikes, maxLoopStrikesBound)

	return HookAdvisoryProfile{
		WarnPct:        warn,
		CritPct:        crit,
		LoopStrikes:    strikes,
		ReadSpiralWarn: spiralWarn,
		ReadSpiralCrit: spiralCrit,
		AdvisoryStyle:  resolveAdvisoryStyle(styleFlag, "NOMOS_ADVISORY_STYLE"),
	}
}

func resolveAdvisoryPreset(flagVal, envKey string) string {
	candidate := strings.ToLower(strings.TrimSpace(flagVal))
	if candidate == "" {
		candidate = strings.ToLower(strings.TrimSpace(os.Getenv(envKey)))
	}
	if candidate == advisoryProfileCarbon {
		return advisoryProfileCarbon
	}
	return "standard"
}

// resolveAdvisoryInt selects an explicit non-zero flag value, an integer environment
// fallback, or the statutory default prior to safety clamping.
func resolveAdvisoryInt(flagVal int, envKey string, fallback int) int {
	if flagVal != 0 {
		return flagVal
	}
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed == 0 {
		return fallback
	}
	return parsed
}

// resolveAdvisoryStyle normalizes the PostToolUse advisory style to "standard" or "terse".
func resolveAdvisoryStyle(flagVal, envKey string) string {
	candidate := strings.ToLower(strings.TrimSpace(flagVal))
	if candidate == "" {
		candidate = strings.ToLower(strings.TrimSpace(os.Getenv(envKey)))
	}
	if candidate == advisoryStyleTerse {
		return advisoryStyleTerse
	}
	return advisoryStyleStandard
}

func clampAdvisoryInt(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// formatBudgetCheckpointAdvisory renders either standard or terse wall-clock budget guidance.
func formatBudgetCheckpointAdvisory(pct, elapsed, budgetSec int, critical bool, style string) string {
	rem := budgetSec - elapsed
	if rem < 0 {
		rem = 0
	}
	if style == advisoryStyleTerse {
		if critical {
			return fmt.Sprintf("🚨 [Nomos Budget] %d%% (%ds/%ds, %ds left): finalize and run final verification now.", pct, elapsed, budgetSec, rem)
		}
		return fmt.Sprintf("⏱️ [Nomos Budget] %d%% (%ds/%ds, %ds left): checkpoint working baseline now.", pct, elapsed, budgetSec, rem)
	}
	if critical {
		return fmt.Sprintf(
			"🚨 [Nomos Budget Radar] %d%% of session time budget elapsed (%ds / %ds, %ds remaining): finalize outputs, stop background scratch processes, and run final verification immediately.",
			pct, elapsed, budgetSec, rem,
		)
	}
	return fmt.Sprintf(
		"⏱️ [Nomos Budget Radar] %d%% of session time budget elapsed (%ds / %ds, %ds remaining): commit or verify a working baseline artifact now and bound heavy commands with timeout.",
		pct, elapsed, budgetSec, rem,
	)
}

func formatSlowStepTimeoutAdvisory(stepSec, remainingSec int, style string) string {
	bound := remainingSec / 2
	if bound < 15 {
		bound = 15
	}
	if style == advisoryStyleTerse {
		return fmt.Sprintf("⏱️ [Nomos Timeout Guard] Step took %ds (%ds left): wrap heavy commands in 'timeout %ds'.", stepSec, remainingSec, bound)
	}
	return fmt.Sprintf(
		"⏱️ [Nomos Slow-Step Timeout Radar] Previous shell command consumed %ds (%ds budget remaining): wrap subsequent heavy build, test, or solver commands in 'timeout %ds <cmd>' to prevent trial timeout.",
		stepSec, remainingSec, bound,
	)
}

func formatWorktreeChurnAdvisory(unverifiedCalls int, style string) string {
	if style == advisoryStyleTerse {
		return fmt.Sprintf("🧭 [Nomos Churn] %d unverified worktree calls: run 'nomos verify' now.", unverifiedCalls)
	}
	return fmt.Sprintf(
		"🧭 [Nomos Worktree Churn Radar] %d worktree tool calls elapsed since last verification: run 'nomos verify' now to catch AST, LOC, or complexity drift early.",
		unverifiedCalls,
	)
}

// formatHostHealthAdvisory renders either standard or compact single-clause host-health guidance.
func formatHostHealthAdvisory(faultKey, standardMsg, style string) string {
	if style != advisoryStyleTerse {
		return standardMsg
	}
	switch faultKey {
	case "egress_dns_offline":
		return "🩺 [Nomos Host] Egress/DNS offline: use local cache or --offline."
	case "apt_archive_mirror":
		return "🩺 [Nomos Host] EOL Debian mirror: use archive.debian.org or installed runtimes."
	case "readonly_fs":
		return "🩺 [Nomos Host] Read-only mount (EROFS): redirect outputs to /tmp or worktree."
	case "resource_exhausted":
		return "🩺 [Nomos Host] Disk/RAM exhausted (ENOSPC/OOM): prune scratch and lower concurrency."
	default:
		if strings.HasPrefix(faultKey, "missing_bin:") {
			exe := strings.TrimPrefix(faultKey, "missing_bin:")
			return fmt.Sprintf("🩺 [Nomos Host] Missing executable '%s': use installed runtime.", exe)
		}
		return standardMsg
	}
}
