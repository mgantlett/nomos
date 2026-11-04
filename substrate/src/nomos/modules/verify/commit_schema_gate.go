// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	remedyAmendCommit   = "re-commit via `nomos task commit` to enforce Golden Commit + Task trailer compliance"
	maxSubjectLineRunes = 100
	minBodyProseRunes   = 12
)

var (
	leadingTaskKeyPrefixRegex  = regexp.MustCompile(`(?i)^\[(?:task\s+)?(?:int|sub|arc)-[0-9]+`)
	goldenSubjectSchema        = regexp.MustCompile(`^(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+`)
	trailingDupTaskKeyRegex    = regexp.MustCompile(`(?i)(\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]|\((?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\))\s*$`)
	canonicalTaskTrailerValRe  = regexp.MustCompile(`^(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?$`)
	gitTrailerLineRegex        = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*):\s+(\S.*)$`)
	legacyDualLayerHeaderRegex = regexp.MustCompile(`(?im)^\*\*(architectural rationale|blast radius & contracts|resolution mechanics):\*\*`)
	rawDiffDumpLineRegex       = regexp.MustCompile(`(?m)^[-*]\s+[MADRCU][0-9]*\s+\S+`)
	truncationFillerRegex      = regexp.MustCompile(`(?m)^[-*]\s+\.\.\.\s+\(\+[0-9]+\s+additional\s+`)
	cannedCommitShimRegex      = regexp.MustCompile(`(?i)(satisfy (int|sub)-[0-9]+ deterministic execution requirements|scoped strictly to (int|sub)-[0-9]+ implementation files|implemented and verified via deterministic quality gates for (int|sub)-[0-9]+)`)
)

// CommitSchemaGate enforces the Golden Conventional Commit schema, anti-dump hygiene,
// and canonical RFC 822 Git Trailers across all unmerged branch commits.
// @intent nomos_commit_schema_invariant_spec.md #CommitSchemaGate
type CommitSchemaGate struct{}

// Name returns the canonical name of the Commit Schema quality gate.
func (g *CommitSchemaGate) Name() string {
	return "Commit Schema Invariant Gate"
}

// Evaluate parses all active branch commits (or HEAD fallback) to ensure format and anti-shim compliance.
func (g *CommitSchemaGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx != nil && ctx.IsBaseline {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "baseline trunk verification mode; historical commit schema check bypassed",
		}, nil
	}
	root := resolveBoundariesRoot(ctx)
	if isPreCommitExempt(root) {
		return &GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "worktree branch has zero commits relative to trunk; exempt from HEAD commit schema gate",
		}, nil
	}

	commits := collectBranchCommitsForSchemaAudit(root)
	if len(commits) == 0 {
		return evaluateSingleHeadCommit(g.Name(), root)
	}

	for _, entry := range commits {
		res, err := checkCommitCompliance(g.Name(), entry.body)
		if err != nil || !res.Passed {
			if res != nil && entry.shortSHA != "" {
				res.Message = fmt.Sprintf("commit %s: %s", entry.shortSHA, res.Message)
			}
			return res, err
		}
	}

	return &GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  fmt.Sprintf("verified %d branch commit(s) against Golden Commit + Task trailer schema", len(commits)),
	}, nil
}

func evaluateSingleHeadCommit(gateName, root string) (*GateResult, error) {
	cmd := workspace.SafeGitCommand(root, "log", "-1", "--format=%B")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &GateResult{GateName: gateName, Passed: true, Message: "no commit history to evaluate"}, nil
	}
	commitMsg := strings.TrimSpace(string(out))
	if commitMsg == "" {
		return &GateResult{GateName: gateName, Passed: false, Message: "HEAD commit message is empty", Remedy: remedyAmendCommit}, nil
	}
	return checkCommitCompliance(gateName, commitMsg)
}

type auditedCommitEntry struct {
	shortSHA string
	body     string
}

func collectBranchCommitsForSchemaAudit(worktreePath string) []auditedCommitEntry {
	if worktreePath == "" {
		return nil
	}
	branchOut, err := workspace.SafeGitCommand(worktreePath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return nil
	}
	branchName := strings.TrimSpace(string(branchOut))
	if workspace.IsProtectedBranch(branchName) || branchName == "" || branchName == "HEAD" {
		return nil
	}
	shas := resolveClosestTrunkDivergenceSHAs(worktreePath)
	var entries []auditedCommitEntry
	for _, sha := range shas {
		out, lErr := workspace.SafeGitCommand(worktreePath, "log", "-1", "--format=%B", sha).Output()
		if lErr != nil {
			continue
		}
		short := sha
		if len(short) > 7 {
			short = short[:7]
		}
		entries = append(entries, auditedCommitEntry{shortSHA: short, body: strings.TrimSpace(string(out))})
	}
	return entries
}

func resolveClosestTrunkDivergenceSHAs(worktreePath string) []string {
	var best []string
	for _, trunk := range workspace.ProtectedTrunkBranches {
		if workspace.SafeGitCommand(worktreePath, "rev-parse", "--verify", trunk).Run() != nil {
			continue
		}
		revOut, err := workspace.SafeGitCommand(worktreePath, "rev-list", trunk+"..HEAD").Output()
		if err != nil {
			continue
		}
		shas := nonEmptyLines(string(revOut))
		if len(shas) > 0 && (best == nil || len(shas) < len(best)) {
			best = shas
		}
	}
	return best
}

func nonEmptyLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func isPreCommitExempt(worktreePath string) bool {
	if worktreePath == "" {
		return false
	}
	branchOut, branchErr := workspace.SafeGitCommand(worktreePath, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if branchErr != nil {
		return false
	}
	branchName := strings.TrimSpace(string(branchOut))
	if workspace.IsProtectedBranch(branchName) || branchName == "" || branchName == "HEAD" {
		return false
	}
	for _, t := range workspace.ProtectedTrunkBranches {
		if workspace.SafeGitCommand(worktreePath, "rev-parse", "--verify", t).Run() != nil {
			continue
		}
		revOut, revErr := workspace.SafeGitCommand(worktreePath, "rev-list", t+"..HEAD").CombinedOutput()
		if revErr == nil && strings.TrimSpace(string(revOut)) == "" {
			return true
		}
	}
	return false
}

func checkCommitCompliance(gateName, commitMsg string) (*GateResult, error) {
	trimmedMsg := strings.TrimSpace(commitMsg)
	if trimmedMsg == "" {
		return &GateResult{GateName: gateName, Passed: false, Message: "commit message is empty", Remedy: remedyAmendCommit}, nil
	}
	lines := strings.Split(strings.ReplaceAll(trimmedMsg, "\r\n", "\n"), "\n")
	firstLine := strings.TrimSpace(lines[0])

	if isMergeCommit(firstLine) {
		return &GateResult{GateName: gateName, Passed: true, Message: "merge commit is exempt from Golden Commit trailer invariant"}, nil
	}
	if violation := validateSubjectLine(firstLine); violation != "" {
		return &GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if violation := validateAntiShimAndAntiDump(trimmedMsg); violation != "" {
		return &GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if violation := validateBodyAndTrailers(lines[1:]); violation != "" {
		return &GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}

	return &GateResult{GateName: gateName, Passed: true, Message: "HEAD commit schema satisfies Golden Commit + Task trailer invariant"}, nil
}

func validateSubjectLine(firstLine string) string {
	if leadingTaskKeyPrefixRegex.MatchString(firstLine) {
		return fmt.Sprintf("commit subject must not include leading [TaskID] prefix; record Task: <KEY> in Git trailers: %q", firstLine)
	}
	if trailingDupTaskKeyRegex.MatchString(firstLine) {
		return fmt.Sprintf("commit subject must not include trailing task key token; record Task: <KEY> in Git trailers: %q", firstLine)
	}
	if !goldenSubjectSchema.MatchString(firstLine) {
		return fmt.Sprintf("commit subject must match Conventional Commit '<feat|fix|docs|chore|refactor|test>(<scope>): <summary>': %q", firstLine)
	}
	if n := utf8.RuneCountInString(firstLine); n > maxSubjectLineRunes {
		return fmt.Sprintf("commit subject exceeds %d-rune ceiling (%d runes); target <= 72 runes: %q", maxSubjectLineRunes, n, firstLine)
	}
	return ""
}

func validateAntiShimAndAntiDump(commitMsg string) string {
	if cannedCommitShimRegex.MatchString(commitMsg) {
		return "commit message contains prohibited canned boilerplate shim (Law 2 Anti-Shim)"
	}
	if legacyDualLayerHeaderRegex.MatchString(commitMsg) {
		return "commit message contains legacy Dual-Layer bold header; write plain-text rationale prose with Task: trailer"
	}
	if rawDiffDumpLineRegex.MatchString(commitMsg) || truncationFillerRegex.MatchString(commitMsg) {
		return "commit message contains raw git diff file-list dump or truncation filler (Law 4 SSoT: Git tree diff owns file manifests)"
	}
	return ""
}

func validateBodyAndTrailers(rest []string) string {
	if len(rest) == 0 || strings.TrimSpace(rest[0]) != "" {
		return "commit message requires a blank line after subject followed by rationale prose and Git trailers"
	}
	prose, trailers, errMsg := splitBodyAndGitTrailers(rest[1:])
	if errMsg != "" {
		return errMsg
	}
	if utf8.RuneCountInString(strings.TrimSpace(prose)) < minBodyProseRunes {
		return "commit message requires substantive rationale prose (>= 12 chars) above the Git trailer block"
	}
	return validateRequiredTrailers(trailers)
}

func splitBodyAndGitTrailers(lines []string) (string, map[string][]string, string) {
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < 0 {
		return "", nil, "commit message body and Task: trailer are missing"
	}
	trailerStart := end
	for trailerStart >= 0 && strings.TrimSpace(lines[trailerStart]) != "" {
		trailerStart--
	}
	trailerBlock := lines[trailerStart+1 : end+1]
	trailers := make(map[string][]string)
	for _, raw := range trailerBlock {
		m := gitTrailerLineRegex.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			return "", nil, "commit message missing blank-line-separated RFC 822 Git trailer block ending with 'Task: <INT|SUB-N>'"
		}
		trailers[m[1]] = append(trailers[m[1]], strings.TrimSpace(m[2]))
	}
	if trailerStart <= 0 {
		return "", trailers, ""
	}
	prose := strings.TrimSpace(strings.Join(lines[:trailerStart], "\n"))
	return prose, trailers, ""
}

func validateRequiredTrailers(trailers map[string][]string) string {
	tasks := trailers["Task"]
	if len(tasks) == 0 {
		return "commit message missing mandatory 'Task: <INT|SUB-N>' Git trailer"
	}
	if len(tasks) > 1 {
		return "commit message must contain exactly one 'Task:' Git trailer"
	}
	if !canonicalTaskTrailerValRe.MatchString(tasks[0]) {
		return fmt.Sprintf("invalid 'Task:' trailer %q; must match ^(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?$", tasks[0])
	}
	for _, twin := range trailers["Twin"] {
		if !canonicalTaskTrailerValRe.MatchString(twin) {
			return fmt.Sprintf("invalid 'Twin:' trailer %q; must match ^(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?$", twin)
		}
	}
	return ""
}

func isMergeCommit(firstLine string) bool {
	return strings.HasPrefix(firstLine, "Merge pull request ") ||
		strings.HasPrefix(firstLine, "Merge branch ") ||
		strings.HasPrefix(firstLine, "Merge remote-tracking branch ")
}
