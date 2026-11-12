// Package commitmsg implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package commitmsg

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
	maxMechanicsBullets      = 5
	commitSectionDividerLine = "___"
)

type CommitCmdOptions struct {
	Summary         string
	Rationale       string
	Mechanics       string
	WalkthroughFile string
	Dir             string
	AutoStage       bool
	TaskRecord      *task.TaskRecord
}

type commitHydrationContext struct {
	taskKey         string
	worktreeDir     string
	repoRoot        string
	taskRecord      *task.TaskRecord
	walkthroughText string
	stagedFiles     []string
}

// BuildCommitHydrationContext executes deterministic commitmsg verification logic (@intent task_lifecycle_cli_spec.md).
func BuildCommitHydrationContext(taskKey, dir string, opts *CommitCmdOptions) commitHydrationContext {
	var repoRoot string
	if dir != "" {
		if r, err := workspace.FindRepoRoot(dir); err == nil {
			repoRoot = r
		}
	}
	var rec *task.TaskRecord
	var walkPath string
	if opts != nil {
		rec = opts.TaskRecord
		walkPath = opts.WalkthroughFile
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

// hydrateCommitMessage synthesizes a Golden Conventional Commit with a canonical [<KEY>] subject prefix,
// flowing unwrapped CommonMark paragraphs/bullets, a git-interpret-trailers-safe ___ divider, and Task/Twin trailers.
// @intent nomos_hermetic_worktree_spec.md
func HydrateCommitMessage(ctx commitHydrationContext, opts *CommitCmdOptions) string {
	var rawSummary, expRationale, expMechanics string
	if opts != nil {
		rawSummary = opts.Summary
		expRationale = strings.TrimSpace(opts.Rationale)
		expMechanics = strings.TrimSpace(opts.Mechanics)
	}

	canonicalKey := strings.ToUpper(strings.TrimSpace(ctx.taskKey))
	subject := NormalizeCommitSubject(canonicalKey, rawSummary, ctx.taskRecord, ctx.stagedFiles)
	var card task.ParsedTaskCard
	if ctx.taskRecord != nil {
		card = task.ExtractTaskCardSections(ctx.taskRecord.Description)
	}

	rationale := normalizeFlowingBlock(resolveCommitRationale(ctx, subject, expRationale, card))
	mechanics := resolveCommitMechanics(ctx, expMechanics, card, rationale)
	trailers := formatCommitTrailers(canonicalKey, ctx.taskRecord)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n\n", canonicalKey, subject))
	sb.WriteString(rationale)
	sb.WriteString("\n\n")
	if mechanics != "" {
		sb.WriteString(mechanics)
		sb.WriteString("\n\n")
	}
	sb.WriteString(commitSectionDividerLine)
	sb.WriteString("\n\n")
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
		return normalizeFlowingBlock(explicit)
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
		cleaned := collapseSingleLine(cleanBulletLine(raw))
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
		lines = append(lines, "- "+collapseSingleLine(item))
	}
	return strings.Join(lines, "\n")
}

func formatCommitTrailers(taskKey string, rec *task.TaskRecord) string {
	lines := []string{fmt.Sprintf("Task: %s", taskKey)}
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
	skipSection := false
	inFence := false
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			skipSection = isVerificationHeading(line)
			continue
		}
		if skipSection {
			continue
		}
		if cleaned := cleanBulletLine(line); cleaned != "" {
			items = append(items, cleaned)
		}
	}
	return items
}

func isVerificationHeading(headingLine string) bool {
	title := strings.ToLower(strings.TrimSpace(strings.TrimLeft(headingLine, "#")))
	return strings.HasPrefix(title, "verification") ||
		strings.HasPrefix(title, "test") ||
		strings.HasPrefix(title, "quality gate") ||
		strings.HasPrefix(title, "gates")
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

func normalizeFlowingBlock(text string) string {
	paragraphs := strings.Split(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")), "\n\n")
	out := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, normalizeParagraphOrList(p))
	}
	return strings.Join(out, "\n\n")
}

func normalizeParagraphOrList(paragraph string) string {
	lines := strings.Split(strings.TrimSpace(paragraph), "\n")
	hasBullets := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			hasBullets = true
			break
		}
	}
	if !hasBullets {
		return collapseSingleLine(paragraph)
	}
	var bullets []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			bullets = append(bullets, "- "+collapseSingleLine(cleanBulletLine(t)))
		} else if len(bullets) > 0 {
			bullets[len(bullets)-1] += " " + collapseSingleLine(t)
		} else {
			bullets = append(bullets, collapseSingleLine(t))
		}
	}
	return strings.Join(bullets, "\n")
}
