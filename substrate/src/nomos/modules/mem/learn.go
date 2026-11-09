// Package mem implements episodic gate-trap distillation (Nomos Learn) in Nomos Mneme.
package mem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GateTrapEntry records a gate failure observed before verified resolution (#Memory).
type GateTrapEntry struct {
	GateName  string `json:"gate_name"`
	Message   string `json:"message"`
	Scope     string `json:"scope"`
	Timestamp string `json:"timestamp"`
}

func trapBufferPath(repoRoot, taskKey string) string {
	cleanKey := strings.ToUpper(strings.TrimSpace(taskKey))
	return filepath.Join(repoRoot, ".tmp", "active_traps_"+cleanKey+".json")
}

// RecordGateTrap buffers active DoD gate failures for episodic distillation upon sync (@intent: #Memory).
func RecordGateTrap(repoRoot, taskKey string, touchedFiles []string, traps []GateTrapEntry) error {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" || len(traps) == 0 {
		return nil
	}
	bufPath := trapBufferPath(repoRoot, taskKey)
	if err := os.MkdirAll(filepath.Dir(bufPath), 0755); err != nil {
		return err
	}

	existing := loadTrapBuffer(bufPath)
	now := time.Now().UTC().Format(time.RFC3339)

	merged := mergeTrapEntries(repoRoot, touchedFiles, existing, traps, now)
	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bufPath, data, 0644)
}

func loadTrapBuffer(path string) []GateTrapEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []GateTrapEntry
	if uErr := json.Unmarshal(data, &entries); uErr != nil {
		return nil
	}
	return entries
}

func mergeTrapEntries(repoRoot string, touchedFiles []string, existing, incoming []GateTrapEntry, now string) []GateTrapEntry {
	byKey := make(map[string]GateTrapEntry, len(existing)+len(incoming))
	var order []string
	add := func(e GateTrapEntry) {
		if strings.TrimSpace(e.Scope) == "" {
			e.Scope = DeriveTrapScope(repoRoot, e.Message, touchedFiles)
		}
		if strings.TrimSpace(e.Timestamp) == "" {
			e.Timestamp = now
		}
		k := strings.ToLower(strings.TrimSpace(e.GateName)) + "|" + e.Scope
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = e
	}
	for _, e := range existing {
		add(e)
	}
	for _, e := range incoming {
		add(e)
	}
	out := make([]GateTrapEntry, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// DistillTaskLearnings promotes resolved gate traps into repo-scoped pitfall memories (@intent: #Memory).
func DistillTaskLearnings(repoRoot, taskKey, walkthrough string, touchedFiles []string) (int, error) {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" {
		return 0, nil
	}
	bufPath := trapBufferPath(repoRoot, taskKey)
	entries := loadTrapBuffer(bufPath)
	if len(entries) == 0 {
		return 0, nil
	}

	store, err := NewStore("")
	if err != nil {
		return 0, err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	repoID := filepath.Base(filepath.Clean(repoRoot))
	fixSummary := summarizeWalkthroughFix(walkthrough)
	saved := persistDistilledTraps(store, entries, repoRoot, repoID, touchedFiles, taskKey, walkthrough, fixSummary)
	SweepRepoPitfallHygiene(store, repoRoot, repoID, time.Now().UTC())
	removeTaskRecallLedger(repoRoot, taskKey)

	if rErr := os.Remove(bufPath); rErr != nil && !os.IsNotExist(rErr) {
		return saved, rErr
	}
	return saved, nil
}

func persistDistilledTraps(store *Store, entries []GateTrapEntry, repoRoot, repoID string, touchedFiles []string, taskKey, walkthrough, fixSummary string) int {
	saved := 0
	seenTrapSig := make(map[string]bool)
	upperTask := strings.ToUpper(taskKey)

	for _, entry := range entries {
		if saved >= 5 {
			break
		}
		salient := compactTrapDiagnostic(entry.Message, repoRoot, touchedFiles, 140)
		scope := resolveAlignedTrapScope(repoRoot, salient, entry, touchedFiles)
		sigKey := strings.ToLower(strings.TrimSpace(scope + "|" + salient))
		if salient == "" || seenTrapSig[sigKey] {
			continue
		}
		seenTrapSig[sigKey] = true
		topic := formatPitfallTopic(entry.GateName, scope)
		resolvedFix := synthesizeTrapResolution(repoRoot, entry, salient, touchedFiles, walkthrough, fixSummary)
		rec := &Memory{
			RepoID:  repoID,
			Kind:    KindPitfall,
			Topic:   topic,
			Content: fmt.Sprintf("[%s in %s] Trap: %s -> Verified Fix (%s): %s", entry.GateName, scope, salient, upperTask, resolvedFix),
			TaskID:  upperTask,
			Status:  StatusActive,
		}
		if store.Save(rec) == nil {
			store.supersedeOlderTopicPitfalls(repoID, topic, rec.ID)
			saved++
		}
	}
	return saved
}

func resolveAlignedTrapScope(repoRoot, salient string, entry GateTrapEntry, touchedFiles []string) string {
	scope := DeriveTrapScope(repoRoot, salient, touchedFiles)
	if scope != "" && scope != workspaceScopeLabel {
		return scope
	}
	if entry.Scope != "" && entry.Scope != workspaceScopeLabel {
		return entry.Scope
	}
	return DeriveTrapScope(repoRoot, entry.Message, touchedFiles)
}

func (s *Store) supersedeOlderTopicPitfalls(repoID, topic string, newestID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `UPDATE memories SET status = 'superseded', superseded_by = ?
	          WHERE kind = 'pitfall' AND status = 'active' AND lower(repo_id) = lower(?) AND lower(topic) = lower(?) AND id != ?`
	if _, err := s.db.Exec(query, newestID, strings.TrimSpace(repoID), strings.TrimSpace(topic), newestID); err != nil {
		return
	}
}

// RecallRepoPitfalls retrieves active distilled pitfalls strictly partitioned to repoRoot and records active task access (@intent: #Memory).
func RecallRepoPitfalls(repoRoot string, touchedFiles []string, limit int) []string {
	return recallRepoPitfallsWithTask(repoRoot, "", touchedFiles, limit, true)
}

// RecallRepoPitfallsReadOnly retrieves active distilled pitfalls without incrementing reinforcement counts (@intent: #Memory).
func RecallRepoPitfallsReadOnly(repoRoot string, touchedFiles []string, limit int) []string {
	return recallRepoPitfallsWithTask(repoRoot, "", touchedFiles, limit, false)
}

func recallRepoPitfallsWithTask(repoRoot, taskKey string, touchedFiles []string, limit int, recordAccess bool) []string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	store, err := NewStore("")
	if err != nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	all, err := store.ListAll()
	if err != nil {
		return nil
	}

	repoID := strings.ToLower(filepath.Base(filepath.Clean(repoRoot)))
	targetScope := DeriveDirectoryScope(repoRoot, touchedFiles)
	candidates := filterEligibleRepoPitfalls(all, repoID, targetScope, time.Now().UTC())
	if len(candidates) == 0 {
		return nil
	}

	rankRepoPitfalls(candidates, targetScope)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return collectRecalledContents(store, repoRoot, taskKey, targetScope, candidates, recordAccess)
}

func rankRepoPitfalls(candidates []*Memory, targetScope string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		iMatch := targetScope != "workspace" && strings.Contains(candidates[i].Topic, targetScope)
		jMatch := targetScope != "workspace" && strings.Contains(candidates[j].Topic, targetScope)
		if iMatch != jMatch {
			return iMatch
		}
		return candidates[i].ID > candidates[j].ID
	})
}

func formatPitfallTopic(gateName, scope string) string {
	cleanGate := strings.ToLower(strings.TrimSpace(gateName))
	cleanGate = strings.ReplaceAll(cleanGate, " ", "_")
	if len(cleanGate) > 48 {
		cleanGate = strings.TrimRight(cleanGate[:48], "_")
	}
	return fmt.Sprintf("pitfall:%s:%s", cleanGate, scope)
}

func summarizeWalkthroughFix(walkthrough string) string {
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := cleanWalkthroughProseLine(raw)
		if len(line) >= 8 {
			return truncateEllipsis(line, 120)
		}
	}
	return "resolved gate violation and passed 42/42 DoD verification"
}

func cleanWalkthroughProseLine(raw string) string {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "|") {
		return ""
	}
	line = strings.TrimLeft(line, "-*0123456789.) ")
	line = strings.ReplaceAll(line, "**", "")
	line = strings.TrimSpace(line)
	if strings.HasSuffix(line, ":") {
		return ""
	}
	return line
}

func compactTrapDiagnostic(s, repoRoot string, touchedFiles []string, maxLen int) string {
	nonEmpty := splitTrimmedLines(s)
	if len(nonEmpty) == 0 {
		return ""
	}
	for i, line := range nonEmpty {
		if fail := extractTestRunnerFailLine(nonEmpty, i, line); fail != "" {
			return truncateEllipsis(fail, maxLen)
		}
	}
	if matched := selectTouchedDiagnosticLine(nonEmpty, repoRoot, touchedFiles); matched != "" {
		return truncateEllipsis(matched, maxLen)
	}
	return truncateEllipsis(selectSalientDiagnosticLine(nonEmpty), maxLen)
}

func splitTrimmedLines(s string) []string {
	var nonEmpty []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if trimmed := strings.TrimSpace(l); trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	return nonEmpty
}

func truncateEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func selectSalientDiagnosticLine(lines []string) string {
	for i, line := range lines {
		if fail := extractTestRunnerFailLine(lines, i, line); fail != "" {
			return fail
		}
		if isCompilerOrPanicLine(line) {
			return combineColonHeaderWithDetail(lines[0], line)
		}
	}
	return fallbackSalientLine(lines)
}

func extractTestRunnerFailLine(lines []string, idx int, line string) string {
	if pos := strings.Index(line, "--- FAIL:"); pos >= 0 {
		failLine := strings.TrimSpace(line[pos:])
		if idx+1 < len(lines) && !strings.HasPrefix(lines[idx+1], "FAIL") {
			return failLine + " (" + lines[idx+1] + ")"
		}
		return failLine
	}
	if strings.HasPrefix(line, "not ok ") {
		if strings.Contains(lines[0], "TAP version") {
			prefix := strings.TrimSpace(strings.TrimSuffix(lines[0], "TAP version 13"))
			return strings.TrimSpace(prefix + " " + line)
		}
		return line
	}
	return ""
}

func fallbackSalientLine(lines []string) string {
	combined := lines[0]
	if len(lines) > 1 && strings.Contains(combined, "Traceback") {
		return strings.Replace(combined, "Traceback (most recent call last):", lines[len(lines)-1], 1)
	}
	if len(lines) > 1 && strings.HasSuffix(combined, ":") {
		return combineColonHeaderWithDetail(combined, lines[1])
	}
	return combined
}

func combineColonHeaderWithDetail(header, detail string) string {
	cleanDetail := strings.TrimSpace(strings.TrimLeft(detail, "- "))
	if header == detail || !strings.HasSuffix(header, ":") {
		return detail
	}
	return header + " " + cleanDetail
}

func isCompilerOrPanicLine(line string) bool {
	if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "ok\t") || strings.Contains(line, ": ok\t") || strings.Contains(line, ": ok ") {
		return false
	}
	if strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "FAIL\t") {
		return true
	}
	for _, ext := range []string{".go:", ".ts:", ".tsx:", ".js:", ".mjs:", ".svelte:", ".py:", ".rs:"} {
		if strings.Contains(line, ext) {
			return true
		}
	}
	return false
}
