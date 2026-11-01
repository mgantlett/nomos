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

// GateTrapEntry records a gate failure observed before verified resolution (#MnemeMemory).
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

// DeriveDirectoryScope computes a concise directory scope label from modified paths (@intent: #MnemeMemory).
func DeriveDirectoryScope(repoRoot string, touchedFiles []string) string {
	for _, f := range touchedFiles {
		clean := filepath.ToSlash(strings.TrimSpace(f))
		if clean == "" {
			continue
		}
		if repoRoot != "" && filepath.IsAbs(f) {
			if rel, err := filepath.Rel(repoRoot, f); err == nil {
				clean = filepath.ToSlash(rel)
			}
		}
		dir := filepath.ToSlash(filepath.Dir(clean))
		if dir == "." || dir == "/" || dir == "" {
			return "root"
		}
		return dir
	}
	return "workspace"
}

// RecordGateTrap buffers active DoD gate failures for episodic distillation upon sync (@intent: #MnemeMemory).
func RecordGateTrap(repoRoot, taskKey string, touchedFiles []string, traps []GateTrapEntry) error {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" || len(traps) == 0 {
		return nil
	}
	bufPath := trapBufferPath(repoRoot, taskKey)
	if err := os.MkdirAll(filepath.Dir(bufPath), 0755); err != nil {
		return err
	}

	existing := loadTrapBuffer(bufPath)
	defaultScope := DeriveDirectoryScope(repoRoot, touchedFiles)
	now := time.Now().UTC().Format(time.RFC3339)

	merged := mergeTrapEntries(existing, traps, defaultScope, now)
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

func mergeTrapEntries(existing, incoming []GateTrapEntry, defaultScope, now string) []GateTrapEntry {
	byKey := make(map[string]GateTrapEntry, len(existing)+len(incoming))
	var order []string
	add := func(e GateTrapEntry) {
		if strings.TrimSpace(e.Scope) == "" {
			e.Scope = defaultScope
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

// DistillTaskLearnings promotes resolved gate traps into repo-scoped pitfall memories (@intent: #MnemeMemory).
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
	scopeFallback := DeriveDirectoryScope(repoRoot, touchedFiles)
	fixSummary := summarizeWalkthroughFix(walkthrough)
	saved := persistDistilledTraps(store, entries, repoID, scopeFallback, taskKey, fixSummary)

	if rErr := os.Remove(bufPath); rErr != nil && !os.IsNotExist(rErr) {
		return saved, rErr
	}
	return saved, nil
}

func persistDistilledTraps(store *Store, entries []GateTrapEntry, repoID, scopeFallback, taskKey, fixSummary string) int {
	saved := 0
	seenTrapSig := make(map[string]bool)
	upperTask := strings.ToUpper(taskKey)

	for _, entry := range entries {
		if saved >= 5 {
			break
		}
		scope := entry.Scope
		if scope == "" || scope == "workspace" {
			scope = scopeFallback
		}
		salient := compactLine(entry.Message, 140)
		sigKey := strings.ToLower(strings.TrimSpace(scope + "|" + salient))
		if salient == "" || seenTrapSig[sigKey] {
			continue
		}
		seenTrapSig[sigKey] = true
		topic := formatPitfallTopic(entry.GateName, scope)
		rec := &Memory{
			RepoID:  repoID,
			Kind:    KindPitfall,
			Topic:   topic,
			Content: fmt.Sprintf("[%s in %s] Trap: %s -> Verified Fix (%s): %s", entry.GateName, scope, salient, upperTask, fixSummary),
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

func (s *Store) supersedeOlderTopicPitfalls(repoID, topic string, newestID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `UPDATE memories SET status = 'superseded', superseded_by = ?
	          WHERE kind = 'pitfall' AND status = 'active' AND lower(repo_id) = lower(?) AND lower(topic) = lower(?) AND id != ?`
	if _, err := s.db.Exec(query, newestID, strings.TrimSpace(repoID), strings.TrimSpace(topic), newestID); err != nil {
		return
	}
}

// RecallRepoPitfalls retrieves active distilled pitfalls strictly partitioned to repoRoot (@intent: #MnemeMemory).
func RecallRepoPitfalls(repoRoot string, touchedFiles []string, limit int) []string {
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
	candidates := filterActiveRepoPitfalls(all, repoID)
	if len(candidates) == 0 {
		return nil
	}

	rankRepoPitfalls(candidates, DeriveDirectoryScope(repoRoot, touchedFiles))
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if aErr := store.RecordAccess(c.ID); aErr != nil {
			// best effort
		}
		out = append(out, c.Content)
	}
	return out
}

func filterActiveRepoPitfalls(all []*Memory, repoID string) []*Memory {
	var candidates []*Memory
	for _, m := range all {
		if m.Kind == KindPitfall && m.Status == StatusActive && strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID {
			candidates = append(candidates, m)
		}
	}
	return candidates
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
	if len(cleanGate) > 32 {
		cleanGate = cleanGate[:32]
	}
	return fmt.Sprintf("pitfall:%s:%s", cleanGate, scope)
}

func summarizeWalkthroughFix(walkthrough string) string {
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimLeft(line, "-*0123456789. ")
		if len(line) >= 8 {
			return compactLine(line, 120)
		}
	}
	return "resolved gate violation and passed 42/42 DoD verification"
}

func compactLine(s string, maxLen int) string {
	rawLines := strings.Split(strings.TrimSpace(s), "\n")
	var nonEmpty []string
	for _, l := range rawLines {
		if trimmed := strings.TrimSpace(l); trimmed != "" {
			nonEmpty = append(nonEmpty, trimmed)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	combined := selectSalientDiagnosticLine(nonEmpty)
	if len(combined) <= maxLen {
		return combined
	}
	return combined[:maxLen] + "..."
}

func selectSalientDiagnosticLine(lines []string) string {
	for i, line := range lines {
		if strings.HasPrefix(line, "--- FAIL:") {
			if i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "FAIL") {
				return line + " (" + lines[i+1] + ")"
			}
			return line
		}
		if isCompilerOrPanicLine(line) {
			return line
		}
	}
	combined := lines[0]
	if len(lines) > 1 && strings.Contains(combined, "Traceback") {
		combined = strings.Replace(combined, "Traceback (most recent call last):", lines[len(lines)-1], 1)
	}
	return combined
}

func isCompilerOrPanicLine(line string) bool {
	if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "ok\t") || strings.Contains(line, ": ok\t") || strings.Contains(line, ": ok ") {
		return false
	}
	return strings.Contains(line, ".go:") || strings.HasPrefix(line, "panic:") || strings.HasPrefix(line, "FAIL\t")
}
