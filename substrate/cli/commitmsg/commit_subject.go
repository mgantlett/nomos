// Package commitmsg implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package commitmsg

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const (
	TargetSubjectLineRunes = 80
	minSummaryDescRunes    = 24
	commitTypeFeat         = "feat"
	commitTypePerf         = "perf"
	commitTypeFix          = "fix"
	commitTypeDocs         = "docs"
	commitTypeChore        = "chore"
	commitTypeRefactor     = "refactor"
	commitTypeTest         = "test"
)

var (
	leadingTaskKeyRe      = regexp.MustCompile(`^(?i)\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]\s*`)
	trailingTaskKeyRe     = regexp.MustCompile(`(?i)\s*(?:\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]|\((?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\))\s*$`)
	conventionalSubjectRe = regexp.MustCompile(`^([a-zA-Z]+)(\([a-zA-Z0-9_,/-]+\))?!?:\s+(.+)$`)
)

var canonicalCommitTypes = map[string]string{
	commitTypeFeat:     commitTypeFeat,
	commitTypePerf:     commitTypePerf,
	commitTypeFix:      commitTypeFix,
	commitTypeDocs:     commitTypeDocs,
	"doc":              commitTypeDocs,
	"spec":             commitTypeDocs,
	commitTypeChore:    commitTypeChore,
	"build":            commitTypeChore,
	"ci":               commitTypeChore,
	"release":          commitTypeChore,
	commitTypeRefactor: commitTypeRefactor,
	"style":            commitTypeRefactor,
	commitTypeTest:     commitTypeTest,
	"tests":            commitTypeTest,
}

var verbToCommitType = map[string]string{
	commitTypeFix: commitTypeFix, "fixes": commitTypeFix, "fixed": commitTypeFix, "repair": commitTypeFix,
	"resolve": commitTypeFix, "bugfix": commitTypeFix, "hotfix": commitTypeFix, "patch": commitTypeFix,
	"preserve": commitTypeFix, "prevent": commitTypeFix, "restore": commitTypeFix,
	commitTypePerf: commitTypePerf, "memoize": commitTypePerf, "optimize": commitTypePerf, "cache": commitTypePerf,
	"accelerate": commitTypePerf, "streamline": commitTypePerf, "batch": commitTypePerf,
	commitTypeRefactor: commitTypeRefactor, "extract": commitTypeRefactor, "deduplicate": commitTypeRefactor,
	"dedup": commitTypeRefactor, "split": commitTypeRefactor, "consolidate": commitTypeRefactor,
	"unify": commitTypeRefactor, "simplify": commitTypeRefactor, "evolve": commitTypeRefactor,
	"harden": commitTypeRefactor, "tighten": commitTypeRefactor, "bound": commitTypeRefactor,
	"calibrate": commitTypeRefactor, "normalize": commitTypeRefactor, "align": commitTypeRefactor,
	"enforce": commitTypeRefactor, "guard": commitTypeRefactor, "isolate": commitTypeRefactor, "sanitize": commitTypeRefactor,
	commitTypeTest: commitTypeTest, "tests": commitTypeTest,
	commitTypeChore: commitTypeChore, "prune": commitTypeChore, "remove": commitTypeChore,
	"delete": commitTypeChore, "bump": commitTypeChore, "clean": commitTypeChore, "sweep": commitTypeChore,
	"doc": commitTypeDocs, commitTypeDocs: commitTypeDocs, "document": commitTypeDocs,
	"codify": commitTypeDocs, "specify": commitTypeDocs, "spec": commitTypeDocs,
}

// normalizeCommitSubject strips redundant task key tokens and emits a Conventional Commit clause
// budgeted so '[<KEY>] <type>(<scope>): <summary>' stays within targetSubjectLineRunes.
// @intent nomos_hermetic_worktree_spec.md
func NormalizeCommitSubject(taskKey, rawSummary string, rec *task.TaskRecord, stagedFiles []string) string {
	keyOverhead := taskKeyPrefixLen(taskKey)
	candidate := resolveCandidateSummary(taskKey, rawSummary, rec, stagedFiles)
	if formatted, ok := normalizeExistingConventionalSubject(candidate, keyOverhead); ok {
		return formatted
	}
	commitType := inferCommitType(taskKey, candidate, stagedFiles)
	scope := derivePrimaryScope(taskKey, stagedFiles)
	prefix := formatConventionalPrefix(commitType, scope)
	desc := fitSubjectDescription(lowercaseImperativeStart(candidate), prefix, keyOverhead)
	return prefix + desc
}

func taskKeyPrefixLen(taskKey string) int {
	trimmed := strings.TrimSpace(taskKey)
	if trimmed == "" {
		return 0
	}
	return utf8.RuneCountInString(trimmed) + 3 // "[<KEY>] "
}

func formatConventionalPrefix(commitType, scope string) string {
	if scope != "" {
		return fmt.Sprintf("%s(%s): ", commitType, scope)
	}
	return fmt.Sprintf("%s: ", commitType)
}

func fitSubjectDescription(desc, prefix string, keyOverhead int) string {
	cleaned := strings.TrimRight(strings.TrimSpace(desc), ".")
	budget := TargetSubjectLineRunes - keyOverhead - utf8.RuneCountInString(prefix)
	if budget < minSummaryDescRunes {
		budget = minSummaryDescRunes
	}
	return truncateRunesClean(cleaned, budget)
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

func normalizeExistingConventionalSubject(candidate string, keyOverhead int) (string, bool) {
	m := conventionalSubjectRe.FindStringSubmatch(candidate)
	if m == nil {
		return "", false
	}
	mapped, ok := canonicalCommitTypes[strings.ToLower(m[1])]
	if !ok {
		return "", false
	}
	scopePart := strings.ToLower(m[2])
	if mapped == commitTypeChore && strings.EqualFold(m[1], "release") && scopePart == "" {
		scopePart = "(release)"
	}
	prefix := fmt.Sprintf("%s%s: ", mapped, scopePart)
	desc := fitSubjectDescription(lowercaseImperativeStart(stripRedundantTaskKeys(m[3])), prefix, keyOverhead)
	return prefix + desc, true
}

func lowercaseImperativeStart(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	word := strings.Trim(firstToken(trimmed), "\"'`")
	if !isPlainTitleCaseWord(word) {
		return trimmed
	}
	runes := []rune(trimmed)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func isPlainTitleCaseWord(word string) bool {
	if word == "" || word == "Go" {
		return false
	}
	runes := []rune(word)
	if len(runes) < 2 || !unicode.IsUpper(runes[0]) {
		return false
	}
	for _, r := range runes[1:] {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func stripConventionalPrefix(subject string) string {
	if m := conventionalSubjectRe.FindStringSubmatch(strings.TrimSpace(subject)); m != nil {
		return strings.TrimSpace(m[3])
	}
	return strings.TrimSpace(subject)
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
		return commitTypeDocs
	}
	return commitTypeFeat
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
