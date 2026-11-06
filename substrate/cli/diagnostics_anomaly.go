// Package cli implements deterministic Cobra command dispatch, software physics telemetry,
// session management, and terminal UI contracts for Nomos.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

const (
	anomalyKindPanic          = "internal_panic"
	anomalyKindSubstrateError = "substrate_error"
	anomalyKindGateSlow       = "gate_slow"
	anomalyKindGateThrash     = "gate_thrash"

	maxAnomalyRingEntries   = 32
	slowGateThresholdMicros = 5_000_000
	gateThrashStrikeFloor   = 4
)

// SubstrateAnomalyEvent records a scrubbed, local-only substrate anomaly or gate thrash event (#SubstrateAnomalyEvent).
type SubstrateAnomalyEvent struct {
	Timestamp      string `json:"timestamp"`
	NomosVersion   string `json:"nomos_version"`
	TaskKey        string `json:"task_key,omitempty"`
	Command        string `json:"command"`
	Kind           string `json:"kind"`
	Component      string `json:"component"`
	Summary        string `json:"summary"`
	Occurrences    int    `json:"occurrences"`
	DurationMillis int64  `json:"duration_ms,omitempty"`
	Surfaced       bool   `json:"surfaced,omitempty"`
}

type anomalyLedgerState struct {
	Events      []SubstrateAnomalyEvent   `json:"events"`
	GateStrikes map[string]map[string]int `json:"gate_strikes,omitempty"`
}

// RecordSubstrateAnomaly appends or coalesces a scrubbed substrate anomaly in the 0700 user cache ring buffer.
func RecordSubstrateAnomaly(repoRoot, taskKey, command, kind, component, summary string, durationMillis int64) {
	path := resolveAnomalyLedgerFilePath(repoRoot)
	if path == "" {
		return
	}
	st := loadAnomalyLedger(path)
	cleanTask := normalizeDiagnosticTaskKey(taskKey)
	cleanSummary := truncateRunes(sanitizeDiagnosticText(strings.TrimSpace(summary), repoRoot), 240)
	now := time.Now().UTC().Format(time.RFC3339)

	for i := len(st.Events) - 1; i >= 0; i-- {
		ev := &st.Events[i]
		if !ev.Surfaced && ev.TaskKey == cleanTask && ev.Kind == kind && ev.Component == component {
			ev.Timestamp = now
			ev.Occurrences++
			if durationMillis > ev.DurationMillis {
				ev.DurationMillis = durationMillis
			}
			if cleanSummary != "" {
				ev.Summary = cleanSummary
			}
			saveAnomalyLedger(path, st)
			return
		}
	}

	st.Events = append(st.Events, SubstrateAnomalyEvent{
		Timestamp:      now,
		NomosVersion:   RootCmdVersion,
		TaskKey:        cleanTask,
		Command:        strings.TrimSpace(command),
		Kind:           kind,
		Component:      strings.TrimSpace(component),
		Summary:        cleanSummary,
		Occurrences:    1,
		DurationMillis: durationMillis,
	})
	if len(st.Events) > maxAnomalyRingEntries {
		st.Events = st.Events[len(st.Events)-maxAnomalyRingEntries:]
	}
	saveAnomalyLedger(path, st)
}

// recordVerificationAnomalies captures internal pipeline errors, slow gates (>=5000ms), and >=4x gate thrash.
func recordVerificationAnomalies(repoRoot, taskKey, command string, report *verify.PipelineReport, execErr error) {
	if execErr != nil {
		RecordSubstrateAnomaly(repoRoot, taskKey, command, anomalyKindSubstrateError, "verify.Pipeline", execErr.Error(), 0)
		return
	}
	if report == nil {
		return
	}
	for _, res := range report.GateResults {
		if res == nil {
			continue
		}
		if res.DurationMicros >= slowGateThresholdMicros {
			ms := res.DurationMicros / 1000
			RecordSubstrateAnomaly(
				repoRoot, taskKey, command, anomalyKindGateSlow, res.GateName,
				fmt.Sprintf("gate evaluation took %dms", ms), ms,
			)
		}
	}
	recordTaskGateThrash(repoRoot, taskKey, command, report)
}

func recordTaskGateThrash(repoRoot, taskKey, command string, report *verify.PipelineReport) {
	cleanTask := normalizeDiagnosticTaskKey(taskKey)
	if cleanTask == "" || report == nil {
		return
	}
	path := resolveAnomalyLedgerFilePath(repoRoot)
	if path == "" {
		return
	}
	st := loadAnomalyLedger(path)
	if st.GateStrikes == nil {
		st.GateStrikes = make(map[string]map[string]int)
	}
	if report.Passed {
		saveAnomalyLedger(path, st)
		return
	}
	taskMap := st.GateStrikes[cleanTask]
	if taskMap == nil {
		taskMap = make(map[string]int)
		st.GateStrikes[cleanTask] = taskMap
	}
	var thrashed []string
	for _, res := range report.GateResults {
		if res != nil && !res.Passed && !res.Skipped {
			taskMap[res.GateName]++
			if taskMap[res.GateName] >= gateThrashStrikeFloor {
				thrashed = append(thrashed, fmt.Sprintf("%s (%dx)", res.GateName, taskMap[res.GateName]))
			}
		}
	}
	saveAnomalyLedger(path, st)
	for _, label := range thrashed {
		gateName := strings.TrimSpace(strings.SplitN(label, " (", 2)[0])
		RecordSubstrateAnomaly(
			repoRoot, cleanTask, command, anomalyKindGateThrash, gateName,
			fmt.Sprintf("gate failed %d times during active task", taskMap[gateName]), 0,
		)
	}
}

// LoadRecentAnomalies returns recorded local substrate anomalies for repoRoot in chronological order.
func LoadRecentAnomalies(repoRoot string) []SubstrateAnomalyEvent {
	path := resolveAnomalyLedgerFilePath(repoRoot)
	if path == "" {
		return nil
	}
	st := loadAnomalyLedger(path)
	out := make([]SubstrateAnomalyEvent, len(st.Events))
	copy(out, st.Events)
	return out
}

// ClearSubstrateAnomalies removes recorded anomaly entries for repoRoot.
func ClearSubstrateAnomalies(repoRoot string) error {
	path := resolveAnomalyLedgerFilePath(repoRoot)
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// renderTaskSyncAnomalyAlert prints a deterministic warning during 'nomos task sync' when unsurfaced
// anomalies occurred during taskKey, preventing coding agents from silently masking substrate friction.
func renderTaskSyncAnomalyAlert(out io.Writer, repoRoot, taskKey string) {
	path := resolveAnomalyLedgerFilePath(repoRoot)
	if path == "" || out == nil {
		return
	}
	st := loadAnomalyLedger(path)
	cleanTask := normalizeDiagnosticTaskKey(taskKey)
	matchedCount := 0
	kindSet := make(map[string]bool)
	var kinds []string

	for i := range st.Events {
		ev := &st.Events[i]
		if ev.Surfaced {
			continue
		}
		if ev.TaskKey == cleanTask || ev.TaskKey == "" {
			matchedCount++
			ev.Surfaced = true
			if !kindSet[ev.Kind] {
				kindSet[ev.Kind] = true
				kinds = append(kinds, ev.Kind)
			}
		}
	}
	if st.GateStrikes != nil && cleanTask != "" {
		delete(st.GateStrikes, cleanTask)
	}
	if matchedCount == 0 {
		return
	}
	saveAnomalyLedger(path, st)
	fmt.Fprintf(out, "⚠️  %s %d anomaly event(s) recorded during %s (%s)\n",
		uiWarnStyle.Render("Substrate Diagnostics:"),
		matchedCount,
		styleTaskKey(cleanTask),
		strings.Join(kinds, ", "))
	fmt.Fprintf(out, "    %s\n",
		uiMutedStyle.Render("Inspect & export a scrubbed offline report: nomos issue \"Anomaly during "+cleanTask+"\""))
}

func resolveAnomalyLedgerFilePath(repoRoot string) string {
	dir := resolveDiagnosticsCacheDir()
	if dir == "" {
		return ""
	}
	seed := strings.TrimSpace(repoRoot)
	if seed == "" || seed == "." {
		if cwd, err := os.Getwd(); err == nil {
			seed = cwd
		}
	}
	if abs, err := filepath.Abs(seed); err == nil {
		seed = filepath.ToSlash(abs)
	}
	sum := sha256.Sum256([]byte(seed))
	return filepath.Join(dir, fmt.Sprintf("anomalies-%s.json", hex.EncodeToString(sum[:8])))
}

func resolveDiagnosticsCacheDir() string {
	baseDir := strings.TrimSpace(os.Getenv("NOMOS_DIAGNOSTICS_CACHE_DIR"))
	if baseDir == "" {
		var err error
		baseDir, err = os.UserCacheDir()
		if err != nil || baseDir == "" {
			baseDir = filepath.Join(os.TempDir(), fmt.Sprintf("nomos-cache-%d", os.Getuid()))
		}
		baseDir = filepath.Join(baseDir, "nomos", "diagnostics")
	}
	if mkErr := os.MkdirAll(baseDir, 0700); mkErr != nil {
		return ""
	}
	fi, statErr := os.Lstat(baseDir)
	if statErr != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	return baseDir
}

func loadAnomalyLedger(path string) anomalyLedgerState {
	var st anomalyLedgerState
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return st
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	if uErr := json.Unmarshal(data, &st); uErr != nil {
		return anomalyLedgerState{}
	}
	return st
}

func saveAnomalyLedger(path string, st anomalyLedgerState) {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func normalizeDiagnosticTaskKey(taskKey string) string {
	clean := strings.ToUpper(strings.TrimSpace(taskKey))
	if strings.HasPrefix(clean, "INT-") || strings.HasPrefix(clean, "SUB-") {
		return clean
	}
	return ""
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
