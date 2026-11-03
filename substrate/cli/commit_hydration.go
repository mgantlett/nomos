// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const maxManifestEntries = 8

type stagedDiffSnapshot struct {
	files        []string
	statusLines  []string
	numstatLines []string
}

type commitHydrationContext struct {
	taskKey         string
	worktreeDir     string
	repoRoot        string
	taskRecord      *task.TaskRecord
	walkthroughText string
	diff            stagedDiffSnapshot
}

func buildCommitHydrationContext(taskKey, dir string, opts *commitCmdOptions) commitHydrationContext {
	var repoRoot string
	if dir != "" {
		if r, err := workspace.FindRepoRoot(dir); err == nil {
			repoRoot = r
		}
	}
	rec := opts.taskRecord
	if rec == nil && taskKey != "" {
		rec = lookupTaskRecordBestEffort(taskKey)
	}
	var walkPath string
	if opts != nil {
		walkPath = opts.walkthroughFile
	}
	return commitHydrationContext{
		taskKey:         taskKey,
		worktreeDir:     dir,
		repoRoot:        repoRoot,
		taskRecord:      rec,
		walkthroughText: resolveWalkthroughText(taskKey, dir, repoRoot, walkPath),
		diff:            collectStagedDiffSnapshot(dir),
	}
}

func lookupTaskRecordBestEffort(taskKey string) *task.TaskRecord {
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return nil
	}
	defer func() {
		_ = store.Close()
	}()
	rec, gErr := store.GetTask(taskKey)
	if gErr != nil {
		return nil
	}
	return rec
}

func collectStagedDiffSnapshot(dir string) stagedDiffSnapshot {
	if strings.TrimSpace(dir) == "" {
		return stagedDiffSnapshot{}
	}
	var snap stagedDiffSnapshot
	if out, err := workspace.SafeGitCommand(dir, "diff", "--cached", "--name-status").Output(); err == nil {
		for _, raw := range strings.Split(string(out), "\n") {
			fields := strings.Fields(raw)
			if len(fields) >= 2 {
				path := fields[len(fields)-1]
				snap.files = append(snap.files, path)
				snap.statusLines = append(snap.statusLines, fmt.Sprintf("%s %s", fields[0], path))
			}
		}
	}
	if out, err := workspace.SafeGitCommand(dir, "diff", "--cached", "--numstat").Output(); err == nil {
		for _, raw := range strings.Split(string(out), "\n") {
			fields := strings.Fields(raw)
			if len(fields) >= 3 {
				if fields[0] == "-" && fields[1] == "-" {
					snap.numstatLines = append(snap.numstatLines, fmt.Sprintf("%s (binary asset)", fields[2]))
				} else {
					snap.numstatLines = append(snap.numstatLines, fmt.Sprintf("%s (+%s/-%s lines)", fields[2], fields[0], fields[1]))
				}
			}
		}
	}
	return snap
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

// hydrateCommitMessage synthesizes a complete Dual-Layer commit message from explicit flags,
// Living Ledger 3-section cards, walkthrough release notes, and staged Git diff telemetry.
// @intent nomos_commit_schema_invariant_spec.md
func hydrateCommitMessage(ctx commitHydrationContext, opts *commitCmdOptions) string {
	var rawSummary, expRationale, expBlast, expMechanics string
	if opts != nil {
		rawSummary = opts.summary
		expRationale = strings.TrimSpace(opts.rationale)
		expBlast = strings.TrimSpace(opts.blastRadius)
		expMechanics = strings.TrimSpace(opts.mechanics)
	}

	subject := normalizeCommitSubject(ctx.taskKey, rawSummary, ctx.taskRecord, ctx.diff.files)
	var card task.ParsedTaskCard
	if ctx.taskRecord != nil {
		card = task.ExtractTaskCardSections(ctx.taskRecord.Description)
	}

	rationale := resolveCommitRationale(ctx, subject, expRationale, card)
	blast := resolveCommitBlastRadius(ctx, subject, expBlast, card)
	mechanics := resolveCommitMechanics(ctx, subject, expMechanics, card)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s\n\n", ctx.taskKey, subject))
	sb.WriteString(fmt.Sprintf("**Architectural Rationale:**\n%s\n\n", rationale))
	sb.WriteString(fmt.Sprintf("**Blast Radius & Contracts:**\n%s\n\n", blast))
	sb.WriteString(fmt.Sprintf("**Resolution Mechanics:**\n%s\n", mechanics))
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
	title := subject
	if ctx.taskRecord != nil && strings.TrimSpace(ctx.taskRecord.Title) != "" {
		title = stripRedundantTaskKeys(ctx.taskRecord.Title)
	}
	if len(ctx.diff.files) > 0 {
		scope := derivePrimaryScope(ctx.taskKey, ctx.diff.files)
		if scope != "" {
			return fmt.Sprintf("Execute %q across %s (%d staged file(s)) in isolated %s worktree.", title, scope, len(ctx.diff.files), ctx.taskKey)
		}
		return fmt.Sprintf("Execute %q (%d staged file(s)) in isolated %s worktree.", title, len(ctx.diff.files), ctx.taskKey)
	}
	return fmt.Sprintf("Execute %q in isolated %s task branch.", title, ctx.taskKey)
}

func resolveCommitBlastRadius(ctx commitHydrationContext, subject, explicit string, card task.ParsedTaskCard) string {
	if explicit != "" {
		return explicit
	}
	if len(ctx.diff.statusLines) > 0 {
		return formatBoundedBulletList(ctx.diff.statusLines, maxManifestEntries, "staged file(s)")
	}
	if card.Scope != "" {
		return "- " + collapseSingleLine(card.Scope)
	}
	return "- Target changeset for " + subject
}

func resolveCommitMechanics(ctx commitHydrationContext, subject, explicit string, card task.ParsedTaskCard) string {
	if explicit != "" {
		return explicit
	}
	if wtBullets := extractWalkthroughMechanics(ctx.walkthroughText); len(wtBullets) > 0 {
		return formatBoundedBulletList(wtBullets, maxManifestEntries, "walkthrough item(s)")
	}
	if len(card.AcceptanceCriteria) > 0 {
		return formatBoundedBulletList(card.AcceptanceCriteria, maxManifestEntries, "acceptance item(s)")
	}
	if len(ctx.diff.numstatLines) > 0 {
		return formatBoundedBulletList(ctx.diff.numstatLines, maxManifestEntries, "modified file(s)")
	}
	return "- Applied " + subject
}

func extractWalkthroughLead(walkthrough string) string {
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		if len(line) >= 12 {
			return line
		}
	}
	return ""
}

func extractWalkthroughMechanics(walkthrough string) []string {
	if strings.TrimSpace(walkthrough) == "" {
		return nil
	}
	var items []string
	for _, raw := range strings.Split(walkthrough, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		if line != "" {
			items = append(items, line)
		}
	}
	return items
}

func formatBoundedBulletList(items []string, limit int, noun string) string {
	var lines []string
	for i, item := range items {
		if i >= limit {
			lines = append(lines, fmt.Sprintf("- ... (+%d additional %s)", len(items)-limit, noun))
			break
		}
		cleaned := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(item, "- "), "* "))
		lines = append(lines, "- "+cleaned)
	}
	return strings.Join(lines, "\n")
}
