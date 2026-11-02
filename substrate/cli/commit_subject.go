// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const maxCommitSummaryRunes = 110

var (
	leadingTaskKeyRe      = regexp.MustCompile(`^(?i)\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]\s*`)
	trailingTaskKeyRe     = regexp.MustCompile(`(?i)\s*(?:\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]|\((?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\))\s*$`)
	conventionalSubjectRe = regexp.MustCompile(`^([a-zA-Z]+)(\([a-zA-Z0-9_,/-]+\))?!?:\s+(.+)$`)
)

var canonicalCommitTypes = map[string]string{
	"feat":     "feat",
	"fix":      "fix",
	"docs":     "docs",
	"doc":      "docs",
	"spec":     "docs",
	"chore":    "chore",
	"build":    "chore",
	"ci":       "chore",
	"release":  "chore",
	"refactor": "refactor",
	"perf":     "refactor",
	"style":    "refactor",
	"test":     "test",
	"tests":    "test",
}

var verbToCommitType = map[string]string{
	"fix": "fix", "fixes": "fix", "fixed": "fix", "repair": "fix",
	"resolve": "fix", "bugfix": "fix", "hotfix": "fix", "patch": "fix",
	"preserve": "fix", "prevent": "fix", "restore": "fix",
	"refactor": "refactor", "extract": "refactor", "deduplicate": "refactor",
	"dedup": "refactor", "split": "refactor", "consolidate": "refactor",
	"unify": "refactor", "simplify": "refactor",
	"test": "test", "tests": "test",
	"chore": "chore", "prune": "chore", "remove": "chore",
	"delete": "chore", "bump": "chore", "clean": "chore", "sweep": "chore",
	"doc": "docs", "docs": "docs", "document": "docs",
	"codify": "docs", "specify": "docs", "spec": "docs",
}

// normalizeCommitSubject strips redundant task key tokens and ensures Conventional Commit alignment.
// @intent INT-874 #CommitMessage
func normalizeCommitSubject(taskKey, rawSummary string, rec *task.TaskRecord, stagedFiles []string) string {
	candidate := resolveCandidateSummary(taskKey, rawSummary, rec, stagedFiles)
	if formatted, ok := normalizeExistingConventionalSubject(candidate); ok {
		return formatted
	}
	commitType := inferCommitType(taskKey, candidate, stagedFiles)
	scope := derivePrimaryScope(taskKey, stagedFiles)
	desc := truncateRunesClean(candidate, maxCommitSummaryRunes)
	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", commitType, scope, desc)
	}
	return fmt.Sprintf("%s: %s", commitType, desc)
}

func resolveCandidateSummary(taskKey, rawSummary string, rec *task.TaskRecord, stagedFiles []string) string {
	candidate := collapseSingleLine(rawSummary)
	if candidate == "" && rec != nil {
		candidate = collapseSingleLine(rec.Title)
	}
	candidate = stripRedundantTaskKeys(candidate)
	if candidate != "" {
		return candidate
	}
	if scope := derivePrimaryScope(taskKey, stagedFiles); scope != "" {
		return fmt.Sprintf("update %s module", scope)
	}
	return "update task implementation"
}

func normalizeExistingConventionalSubject(candidate string) (string, bool) {
	m := conventionalSubjectRe.FindStringSubmatch(candidate)
	if m == nil {
		return "", false
	}
	mapped, ok := canonicalCommitTypes[strings.ToLower(m[1])]
	if !ok {
		return "", false
	}
	scopePart := strings.ToLower(m[2])
	if mapped == "chore" && strings.EqualFold(m[1], "release") && scopePart == "" {
		scopePart = "(release)"
	}
	desc := truncateRunesClean(stripRedundantTaskKeys(m[3]), maxCommitSummaryRunes)
	return fmt.Sprintf("%s%s: %s", mapped, scopePart, desc), true
}

func stripRedundantTaskKeys(s string) string {
	cur := strings.TrimSpace(s)
	for {
		next := leadingTaskKeyRe.ReplaceAllString(cur, "")
		next = trailingTaskKeyRe.ReplaceAllString(next, "")
		next = strings.TrimSpace(next)
		if next == cur {
			return cur
		}
		cur = next
	}
}

func inferCommitType(taskKey, summary string, stagedFiles []string) string {
	if mapped, ok := verbToCommitType[strings.ToLower(firstToken(summary))]; ok {
		return mapped
	}
	if strings.HasPrefix(strings.ToUpper(taskKey), "INT-") || allMarkdownFiles(stagedFiles) {
		return "docs"
	}
	return "feat"
}

func firstToken(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], ":()[].,")
}

func collapseSingleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunesClean(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	cut := strings.TrimSpace(string(runes[:limit-3]))
	if idx := strings.LastIndex(cut, " "); idx > limit/2 {
		cut = strings.TrimSpace(cut[:idx])
	}
	return cut + "..."
}
