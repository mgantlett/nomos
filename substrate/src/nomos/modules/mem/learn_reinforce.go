// Package mem implements per-task reinforcement deduplication and symbol-delta pitfall synthesis.
package mem

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	recallLedgerPrefix               = "mneme_recalled_"
	defaultUnscopedPitfallCoolWindow = 14 * 24 * time.Hour
)

var pyOrTSSymbolPattern = regexp.MustCompile(`(?m)^(?:\s*(?:async\s+)?function|(?:export\s+)?(?:async\s+)?function|def|class)\s+([A-Za-z_][A-Za-z0-9_]*)`)

func recallLedgerPath(repoRoot, taskKey string) string {
	cleanKey := strings.ToUpper(strings.TrimSpace(taskKey))
	return filepath.Join(repoRoot, ".tmp", recallLedgerPrefix+cleanKey+".json")
}

func shouldRecordTaskAccess(repoRoot, taskKey string, memID int) bool {
	if memID <= 0 {
		return false
	}
	if strings.TrimSpace(taskKey) == "" || strings.TrimSpace(repoRoot) == "" {
		return true
	}
	ledgerFile := recallLedgerPath(repoRoot, taskKey)
	seen := loadRecalledIDSet(ledgerFile)
	if seen[memID] {
		return false
	}
	seen[memID] = true
	saveRecalledIDSet(ledgerFile, seen)
	return true
}

func loadRecalledIDSet(path string) map[int]bool {
	out := make(map[int]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var ids []int
	if uErr := json.Unmarshal(data, &ids); uErr != nil {
		return out
	}
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func saveRecalledIDSet(path string, seen map[int]bool) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return
	}
	if wErr := os.WriteFile(path, data, 0644); wErr != nil {
		// best-effort transient task ledger
	}
}

func removeTaskRecallLedger(repoRoot, taskKey string) {
	if strings.TrimSpace(repoRoot) == "" || strings.TrimSpace(taskKey) == "" {
		return
	}
	if err := os.Remove(recallLedgerPath(repoRoot, taskKey)); err != nil {
		// transient file may not exist
	}
}

// RecallRepoPitfallsForTask retrieves active pitfalls for repoRoot and reinforces each memory at most once per taskKey (@intent: #Memory).
func RecallRepoPitfallsForTask(repoRoot, taskKey string, touchedFiles []string, limit int) []string {
	return recallRepoPitfallsWithTask(repoRoot, taskKey, touchedFiles, limit, true)
}

// RecallRepoPatternsForTask retrieves active structural blueprints and reinforces each memory at most once per taskKey (@intent: #Memory).
func RecallRepoPatternsForTask(repoRoot, taskKey, queryHint string, touchedFiles []string, limit int) []string {
	return recallRepoPatternsWithTask(repoRoot, taskKey, queryHint, touchedFiles, limit, true)
}

func collectRecalledContents(store *Store, repoRoot, taskKey, targetScope string, candidates []*Memory, recordAccess bool) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		scopedMatch := targetScope != "" && targetScope != workspaceScopeLabel && strings.Contains(c.Topic, targetScope)
		if recordAccess && scopedMatch && shouldRecordTaskAccess(repoRoot, taskKey, c.ID) {
			if aErr := store.RecordAccess(c.ID); aErr != nil {
				// best effort
			}
		}
		out = append(out, c.Content)
	}
	return out
}

func synthesizeTrapResolution(repoRoot string, entry GateTrapEntry, salient string, touchedFiles []string, walkthrough, fallbackSummary string) string {
	relFile := locateTrappedRepoFile(repoRoot, salient, entry.Message, touchedFiles)
	if relFile == "" {
		return fallbackSummary
	}
	absPath := filepath.Join(repoRoot, filepath.FromSlash(relFile))
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fallbackSummary
	}
	symbols := extractAllFileSymbols(absPath, data)
	base := filepath.Base(relFile)
	matchedNote := findMatchingWalkthroughLine(walkthrough, base)
	prefix := fmt.Sprintf("Refactored %s", base)
	if len(symbols) > 0 {
		prefix = fmt.Sprintf("Refactored %s [%s]", base, strings.Join(symbols, ", "))
	}
	if matchedNote != "" {
		return fmt.Sprintf("%s - %s", prefix, matchedNote)
	}
	if len(symbols) > 0 {
		return prefix
	}
	return fmt.Sprintf("%s - %s", prefix, fallbackSummary)
}

func findMatchingWalkthroughLine(walkthrough, base string) string {
	lowerBase := strings.ToLower(strings.TrimSpace(base))
	stem := strings.TrimSuffix(lowerBase, strings.ToLower(filepath.Ext(lowerBase)))
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := cleanWalkthroughProseLine(raw)
		if len(line) < 8 {
			continue
		}
		lowerLine := strings.ToLower(line)
		if (lowerBase != "" && strings.Contains(lowerLine, lowerBase)) || (len(stem) >= 5 && strings.Contains(lowerLine, stem)) {
			return truncateEllipsis(line, 120)
		}
	}
	return ""
}

func locateTrappedRepoFile(repoRoot, salient, fullMessage string, touchedFiles []string) string {
	if found := scanMessageForTrappedFile(repoRoot, salient, touchedFiles); found != "" {
		return found
	}
	return scanMessageForTrappedFile(repoRoot, fullMessage, touchedFiles)
}

func scanMessageForTrappedFile(repoRoot, message string, touchedFiles []string) string {
	for _, m := range trapRelPathPattern.FindAllStringSubmatch(message, 12) {
		if len(m) < 2 {
			continue
		}
		cand := resolveCandidateRelPath(repoRoot, m[1])
		if len(touchedFiles) > 0 {
			if isTouchedRepoFile(repoRoot, cand, touchedFiles) {
				return cand
			}
			continue
		}
		if isRecognizedRepoFile(repoRoot, cand, nil) {
			return cand
		}
	}
	for _, m := range trapBaseLinePattern.FindAllStringSubmatch(message, 8) {
		if len(m) > 1 {
			for _, tf := range touchedFiles {
				norm := normalizePatternPath(repoRoot, tf)
				if filepath.Base(norm) == m[1] {
					return norm
				}
			}
		}
	}
	return ""
}

func extractAllFileSymbols(absPath string, data []byte) []string {
	if strings.HasSuffix(strings.ToLower(absPath), ".go") {
		return extractGoFuncAndTypeNames(absPath, data)
	}
	matches := pyOrTSSymbolPattern.FindAllStringSubmatch(string(data), maxBlueprintItems)
	var names []string
	for _, m := range matches {
		if len(m) > 1 {
			names = appendUniqueBounded(names, m[1], maxBlueprintItems)
		}
	}
	return names
}

func extractGoFuncAndTypeNames(absPath string, data []byte) []string {
	fset := token.NewFileSet()
	fileNode, err := parser.ParseFile(fset, absPath, data, 0)
	if err != nil {
		return nil
	}
	var names []string
	for _, decl := range fileNode.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil {
			names = appendUniqueBounded(names, fn.Name.Name, maxBlueprintItems)
		}
	}
	return names
}

func filterEligibleRepoPitfalls(all []*Memory, repoID, targetScope string, now time.Time) []*Memory {
	topicDepths := countPitfallTopicChain(all, repoID)
	var candidates []*Memory
	for _, m := range all {
		if isPitfallEligibleForScope(m, repoID, targetScope, topicDepths[strings.ToLower(m.Topic)], now) {
			candidates = append(candidates, m)
		}
	}
	return candidates
}

func isActiveRepoPitfall(m *Memory, repoID string) bool {
	if m == nil || m.Kind != KindPitfall || m.Status != StatusActive {
		return false
	}
	return strings.ToLower(strings.TrimSpace(m.RepoID)) == repoID
}

func isPitfallEligibleForScope(m *Memory, repoID, targetScope string, topicDepth int, now time.Time) bool {
	if !isActiveRepoPitfall(m, repoID) {
		return false
	}
	if targetScope != workspaceScopeLabel && strings.Contains(m.Topic, targetScope) {
		return true
	}
	if isEphemeralTestAssertionContent(m.Content) {
		return false
	}
	if topicDepth >= 2 || m.CreatedAt.IsZero() {
		return true
	}
	return now.Sub(m.CreatedAt) <= defaultUnscopedPitfallCoolWindow
}

