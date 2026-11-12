// Package vscope implements the deterministic Definition of Done (DoD) quality gate pipeline.
package vscope

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	ReleaseNotesRef   = "refs/notes/nomos/releases"
	zeroGitOIDPrefix  = "0000000"
	receiptHashMarker = `"sha256_receipt"`
	receiptGateMarker = `"dod_gates_passed"`
)

var (
	rangeTaskSubjectRe = regexp.MustCompile(`^\[(?:Task\s+)?((?:INT|SUB)-[1-9][0-9]*[A-Z]?)`)
	receiptTaskKeyRe   = regexp.MustCompile(`"task_key"\s*:\s*"([^"]+)"`)
)

// HasCommitRangeScope executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func HasCommitRangeScope(ctx *VerifyContext) bool {
	return ctx != nil && !ctx.AllFiles && strings.TrimSpace(ctx.CommitRange) != ""
}

func resolveRangeRevisionSpec(commitRange string) string {
	trimmed := strings.TrimSpace(commitRange)
	parts := strings.SplitN(trimmed, "..", 2)
	if len(parts) != 2 {
		return trimmed
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if right == "" {
		right = "HEAD"
	}
	if strings.HasPrefix(left, zeroGitOIDPrefix) {
		return right + "~1.." + right
	}
	return left + ".." + right
}

func listRangeCommits(workDir, commitRange string) []string {
	spec := resolveRangeRevisionSpec(commitRange)
	if spec == "" {
		return nil
	}
	out, err := workspace.RunSafeGit(workDir, "rev-list", spec)
	if err != nil && strings.Contains(spec, "~1..") {
		tip := spec[strings.Index(spec, "..")+2:]
		out, err = workspace.RunSafeGit(workDir, "rev-list", "-n", "1", tip)
	}
	if err != nil {
		return nil
	}
	var shas []string
	for _, line := range strings.Split(string(out), "\n") {
		if sha := strings.TrimSpace(line); sha != "" {
			shas = append(shas, sha)
		}
	}
	return shas
}

func collectAttestedReleaseState(workDir string, shas []string) (map[string]bool, map[string]bool) {
	attestedSHAs := make(map[string]bool)
	attestedTasks := make(map[string]bool)
	for _, sha := range shas {
		out, err := workspace.RunSafeGit(workDir, "notes", "--ref="+ReleaseNotesRef, "show", sha)
		if err != nil {
			continue
		}
		note := string(out)
		if !strings.Contains(note, receiptHashMarker) || !strings.Contains(note, receiptGateMarker) {
			continue
		}
		attestedSHAs[sha] = true
		if m := receiptTaskKeyRe.FindStringSubmatch(note); len(m) == 2 {
			attestedTasks[m[1]] = true
		}
	}
	return attestedSHAs, attestedTasks
}

func isCommitAttestedByTask(workDir, sha string, attestedSHAs, attestedTasks map[string]bool) bool {
	if attestedSHAs[sha] {
		return true
	}
	if len(attestedTasks) == 0 {
		return false
	}
	out, err := workspace.RunSafeGit(workDir, "log", "-1", "--format=%s", sha)
	if err != nil {
		return false
	}
	m := rangeTaskSubjectRe.FindStringSubmatch(strings.TrimSpace(string(out)))
	return len(m) == 2 && attestedTasks[m[1]]
}

func collectCommitTouchedFiles(workDir, sha string, fileSet map[string]struct{}) {
	out, err := workspace.RunSafeGit(workDir, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if rel := strings.TrimSpace(strings.Trim(line, `"`)); rel != "" {
			fileSet[rel] = struct{}{}
		}
	}
}

// resolveUnattestedRangeFiles returns repo-relative paths modified by unattested commits in ctx.CommitRange plus dirty working-tree files.
func ResolveUnattestedRangeFiles(ctx *VerifyContext, workDir string) []string {
	fileSet := make(map[string]struct{})
	shas := listRangeCommits(workDir, ctx.CommitRange)
	attestedSHAs, attestedTasks := collectAttestedReleaseState(workDir, shas)
	for _, sha := range shas {
		if !isCommitAttestedByTask(workDir, sha, attestedSHAs, attestedTasks) {
			collectCommitTouchedFiles(workDir, sha, fileSet)
		}
	}
	collectScopeStatusFiles(workDir, fileSet)

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

func collectRangePolyglotFiles(ctx *VerifyContext) map[string]bool {
	dir := ResolveGitDir(ctx)
	if dir == "" {
		return nil
	}
	modified := make(map[string]bool)
	for _, rel := range ResolveUnattestedRangeFiles(ctx, dir) {
		if IsSupportedPolyglotSourceFile(rel) {
			modified[filepath.Join(dir, rel)] = true
		}
	}
	return modified
}
