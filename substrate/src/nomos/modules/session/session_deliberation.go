// Package session implements polymorphic Tier 1 transcript discovery, Sentinel
// governance scoring, and deliberation verbosity telemetry.
package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// DeliberationMetrics quantifies reasoning verbosity, read streaks, and deliberation-to-mutation ratios (#DeliberationMetrics).
type DeliberationMetrics struct {
	ThinkingBytes               int     `json:"thinking_bytes"`
	ThinkingStepCount           int     `json:"thinking_step_count"`
	MaxReadStreak               int     `json:"max_read_streak"`
	MutationCallCount           int     `json:"mutation_call_count"`
	DeliberationToMutationRatio float64 `json:"deliberation_to_mutation_ratio"`
}

type fullThinkingRecord struct {
	Thinking string `json:"thinking"`
}

var sessionReadTools = map[string]bool{
	"read_file":          true,
	"read":               true,
	"grep_search":        true,
	"grep":               true,
	"find_by_name":       true,
	"glob":               true,
	"list_dir":           true,
	"ls":                 true,
	"view_file_outline":  true,
	"view_code_item":     true,
}

var sessionMutatingTools = map[string]bool{
	"write_file":                 true,
	"edit_file":                  true,
	"replace_file_content":       true,
	"multi_replace_file_content": true,
	"apply_patch":                true,
	"write":                      true,
	"edit":                       true,
	"multiedit":                  true,
	"notebookedit":               true,
}

// ComputeDeliberationFromEvents derives reasoning verbosity, peak read-streak, and mutation ratios across session events.
// @intent software_physics_engine_spec.md #DeliberationMetrics
func ComputeDeliberationFromEvents(events []SessionEvent) *DeliberationMetrics {
	dm := &DeliberationMetrics{}
	readStreak := 0
	for _, ev := range events {
		if ev.Type == "THINKING" && ev.Detail != "" {
			dm.ThinkingStepCount++
			dm.ThinkingBytes += len(ev.Detail)
			continue
		}
		if ev.Type != "TOOL_CALL" {
			continue
		}
		readStreak = updateDeliberationToolCounters(ev, readStreak, dm)
	}
	recomputeDeliberationRatio(dm)
	return dm
}

func updateDeliberationToolCounters(ev SessionEvent, readStreak int, dm *DeliberationMetrics) int {
	lowerTool := strings.ToLower(strings.TrimSpace(ev.Tool))
	if sessionMutatingTools[lowerTool] {
		dm.MutationCallCount++
		return 0
	}
	if sessionReadTools[lowerTool] || isReadOnlyShellDetail(lowerTool, ev.Detail) {
		readStreak++
		if readStreak > dm.MaxReadStreak {
			dm.MaxReadStreak = readStreak
		}
		return readStreak
	}
	return 0
}

func isReadOnlyShellDetail(lowerTool, detail string) bool {
	if lowerTool != "run_command" && lowerTool != "bash" && lowerTool != "shell" {
		return false
	}
	trimmed := strings.TrimSpace(detail)
	for _, prefix := range []string{"rg ", "grep ", "cat ", "ls", "find ", "head ", "tail ", "git status", "git diff", "git log"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func recomputeDeliberationRatio(dm *DeliberationMetrics) {
	if dm == nil || dm.ThinkingBytes <= 0 {
		if dm != nil {
			dm.DeliberationToMutationRatio = 0
		}
		return
	}
	denom := dm.MutationCallCount
	if denom <= 0 {
		denom = 1
	}
	raw := float64(dm.ThinkingBytes) / float64(denom)
	dm.DeliberationToMutationRatio = math.Round(raw*100) / 100
}

func enrichBrainDeliberationFromFullTranscript(transcriptPath string, lines []string, dm *DeliberationMetrics) {
	if dm == nil || !hasTruncatedThinkingMarker(lines) {
		return
	}
	fullPath := filepath.Join(filepath.Dir(transcriptPath), "transcript_full.jsonl")
	fullBytes, fullSteps, ok := scanFullTranscriptThinkingBytes(fullPath)
	if !ok || fullBytes < dm.ThinkingBytes {
		return
	}
	dm.ThinkingBytes = fullBytes
	if fullSteps > dm.ThinkingStepCount {
		dm.ThinkingStepCount = fullSteps
	}
	recomputeDeliberationRatio(dm)
}

func hasTruncatedThinkingMarker(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, `"truncated_fields"`) && strings.Contains(line, `"thinking"`) {
			return true
		}
	}
	return false
}

func scanFullTranscriptThinkingBytes(fullPath string) (int, int, bool) {
	f, err := os.Open(fullPath)
	if err != nil {
		return 0, 0, false
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			// file handle closed
		}
	}()

	reader := bufio.NewReaderSize(f, 64*1024)
	thinkingNeedle := []byte(`"thinking":`)
	totalBytes, totalSteps := 0, 0

	for {
		line, rErr := reader.ReadBytes('\n')
		if len(line) > 0 && bytes.Contains(line, thinkingNeedle) {
			var rec fullThinkingRecord
			if uErr := json.Unmarshal(line, &rec); uErr == nil && rec.Thinking != "" {
				totalSteps++
				totalBytes += len(rec.Thinking)
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			return totalBytes, totalSteps, false
		}
	}
	return totalBytes, totalSteps, totalSteps > 0
}

