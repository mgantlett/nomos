// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const (
	commitBodyWrapColumns = 72
	maxMechanicsBullets   = 5
)

type commitHydrationContext struct {
	taskKey         string
	worktreeDir     string
	repoRoot        string
	taskRecord      *task.TaskRecord
	walkthroughText string
	stagedFiles     []string
}

func buildCommitHydrationContext(taskKey, dir string, opts *commitCmdOptions) commitHydrationContext {
	var repoRoot string
	if dir != "" {
		if r, err := workspace.FindRepoRoot(dir); err == nil {
			repoRoot = r
		}
	}
	var rec *task.TaskRecord
	var walkPath string
	if opts != nil {
		rec = opts.taskRecord
		walkPath = opts.walkthroughFile
	}
	if rec == nil && taskKey != "" {
		rec = lookupTaskRecordBestEffort(taskKey)
	}
	return commitHydrationContext{
		taskKey:         taskKey,
		worktreeDir:     dir,
		repoRoot:        repoRoot,
		taskRecord:      rec,
		walkthroughText: resolveWalkthroughText(taskKey, dir, repoRoot, walkPath),
		stagedFiles:     collectStagedFiles(dir),
	}
}

func lookupTaskRecordBestEffort(taskKey string) *task.TaskRecord {
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// best-effort read-only lookup close
		}
	}()
	rec, gErr := store.GetTask(taskKey)
	if gErr != nil {
		return nil
	}
	return rec
}

func collectStagedFiles(dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	out, err := workspace.SafeGitCommand(dir, "diff", "--cached", "--name-only").Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, raw := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(raw); p != "" {
			files = append(files, p)
		}
	}
	return files
}

func resolveWalkthroughText(taskKey, wtDir, repoRoot, explicitFile string) string {
	var candidates []string
	if strings.TrimSpace(explicitFile) != "" {
		candidates = append(candidates, explicitFile)
	}
	fname := fmt.Sprintf("walkthrough-%s.md", taskKey)
	for _, base := range []string{wtDir, repoRoot} {
		if strings.TrimSpace(base) == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(base, ".tmp", fname),
			filepath.Join(base, ".tmp", "walkthrough.md"),
		)
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			if txt := strings.TrimSpace(string(data)); txt != "" {
				return txt
			}
		}
	}
	return ""
}

// hydrateCommitMessage synthesizes a Golden Conventional Commit with 72-column rationale prose,
// optional deduplicated mechanics bullets, and RFC 822 Git Trailers (Task: and Twin:).
// @intent nomos_commit_schema_invariant_spec.md
func hydrateCommitMessage(ctx commitHydrationContext, opts *commitCmdOptions) string {
	var rawSummary, expRationale, expMechanics string
	if opts != nil {
		rawSummary = opts.summary
		expRationale = strings.TrimSpace(opts.rationale)
		expMechanics = strings.TrimSpace(opts.mechanics)
	}

	subject := normalizeCommitSubject(ctx.taskKey, rawSummary, ctx.taskRecord, ctx.stagedFiles)
	var card task.ParsedTaskCard
	if ctx.taskRecord != nil {
		card = task.ExtractTaskCardSections(ctx.taskRecord.Description)
	}

	rationale := wrapCommitBlock(resolveCommitRationale(ctx, subject, expRationale, card), commitBodyWrapColumns)
	mechanics := resolveCommitMechanics(ctx, expMechanics, card, rationale)
	trailers := formatCommitTrailers(ctx.taskKey, ctx.taskRecord)

	var sb strings.Builder
	sb.WriteString(subject)
	sb.WriteString("\n\n")
	sb.WriteString(rationale)
	sb.WriteString("\n\n")
	if mechanics != "" {
		sb.WriteString(mechanics)
		sb.WriteString("\n\n")
	}
	sb.WriteString(trailers)
	sb.WriteString("\n")
	return sb.String()
}

func resolveCommitRationale(ctx commitHydrationContext, subject, explicit string, card task.ParsedTaskCard) string {
	if explicit != "" {
		return explicit
	}
	if card.Scope != "" {
		return card.Scope
	}
	if wtSummary := extractWalkthroughLead(ctx.walkthroughText); wtSummary != "" {
		return wtSummary
	}
	title := stripConventionalPrefix(subject)
	if ctx.taskRecord != nil && strings.TrimSpace(ctx.taskRecord.Title) != "" {
		title = stripRedundantTaskKeys(ctx.taskRecord.Title)
	}
	action := strings.TrimRight(lowercaseImperativeStart(title), ".")
	if scope := derivePrimaryScope(ctx.taskKey, ctx.stagedFiles); scope != "" {
		return fmt.Sprintf("Update %s to %s.", scope, action)
	}
	return fmt.Sprintf("Apply %s on isolated feature branch.", action)
}

func resolveCommitMechanics(ctx commitHydrationContext, explicit string, card task.ParsedTaskCard, rationale string) string {
	if explicit != "" {
		return wrapCommitBlock(explicit, commitBodyWrapColumns)
	}
	if wtBullets := deduplicateMechanics(extractWalkthroughItems(ctx.walkthroughText), rationale); len(wtBullets) > 0 {
		return formatMechanicsBullets(wtBullets)
	}
	if acBullets := deduplicateMechanics(card.AcceptanceCriteria, rationale); len(acBullets) > 0 {
		return formatMechanicsBullets(acBullets)
	}
	return ""
}

func deduplicateMechanics(items []string, rationale string) []string {
	normRationale := normalizeComparableProse(rationale)
	seen := map[string]bool{normRationale: true}
	var out []string
	for _, raw := range items {
		cleaned := cleanBulletLine(raw)
		norm := normalizeComparableProse(cleaned)
		if cleaned == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, cleaned)
		if len(out) >= maxMechanicsBullets {
			break
		}
	}
	return out
}

func formatMechanicsBullets(items []string) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, wrapBulletItem(item, commitBodyWrapColumns))
	}
	return strings.Join(lines, "\n")
}

func formatCommitTrailers(taskKey string, rec *task.TaskRecord) string {
	lines := []string{fmt.Sprintf("Task: %s", strings.ToUpper(strings.TrimSpace(taskKey)))}
	if rec != nil {
		if twin := strings.ToUpper(strings.TrimSpace(rec.TwinKey)); twin != "" {
			lines = append(lines, fmt.Sprintf("Twin: %s", twin))
		}
	}
	return strings.Join(lines, "\n")
}

func extractWalkthroughLead(walkthrough string) string {
	for _, item := range extractWalkthroughItems(walkthrough) {
		if utf8.RuneCountInString(item) >= 12 {
			return item
		}
	}
	return ""
}

func extractWalkthroughItems(walkthrough string) []string {
	if strings.TrimSpace(walkthrough) == "" {
		return nil
	}
	var items []string
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
			continue
		}
		if cleaned := cleanBulletLine(line); cleaned != "" {
			items = append(items, cleaned)
		}
	}
	return items
}

func cleanBulletLine(s string) string {
	trimmed := strings.TrimSpace(s)
	trimmed = strings.TrimPrefix(trimmed, "- ")
	trimmed = strings.TrimPrefix(trimmed, "* ")
	return strings.TrimSpace(trimmed)
}

func normalizeComparableProse(s string) string {
	return strings.ToLower(strings.TrimRight(collapseSingleLine(s), "."))
}

func wrapCommitBlock(text string, width int) string {
	paragraphs := strings.Split(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")), "\n\n")
	wrapped := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			continue
		}
		wrapped = append(wrapped, wrapParagraphLines(p, width))
	}
	return strings.Join(wrapped, "\n\n")
}

func wrapParagraphLines(paragraph string, width int) string {
	lines := strings.Split(strings.TrimSpace(paragraph), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(line, "  ") {
			return strings.TrimSpace(paragraph)
		}
	}
	return wrapPlainWords(collapseSingleLine(paragraph), "", "", width)
}

func wrapBulletItem(item string, width int) string {
	return wrapPlainWords(collapseSingleLine(item), "- ", "  ", width)
}

func wrapPlainWords(text, firstIndent, nextIndent string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	cur := firstIndent + words[0]
	for _, w := range words[1:] {
		if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(w) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = nextIndent + w
	}
	lines = append(lines, cur)
	return strings.Join(lines, "\n")
}
