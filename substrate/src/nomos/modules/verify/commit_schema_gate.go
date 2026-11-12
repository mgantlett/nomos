// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const (
	remedyAmendCommit   = "re-commit via `nomos task commit` to enforce Golden Commit + Task trailer compliance"
	maxSubjectLineRunes = 110
	minBodyProseRunes   = 12
)

var (
	prefixKeyExtractRegex      = regexp.MustCompile(`^\[((?:INT|SUB)-[1-9][0-9]*(?:-[a-z0-9-]+)?)\]`)
	goldenSubjectSchema        = regexp.MustCompile(`^\[((?:INT|SUB)-[1-9][0-9]*(?:-[a-z0-9-]+)?)\]\s+(feat|perf|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+`)
	euphemisticBuzzwordRe      = regexp.MustCompile(`(?i)\bself[- ]healing\b`)
	trailingDupTaskKeyRegex    = regexp.MustCompile(`(?i)(\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\]|\((?:int|sub|arc)-[0-9]+[a-z]?(?:-[a-z0-9-]+)?\))\s*$`)
	canonicalTaskTrailerValRe  = regexp.MustCompile(`^(INT|SUB)-[1-9][0-9]*(-[a-z0-9-]+)?$`)
	gitTrailerLineRegex        = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9-]*):\s+(\S.*)$`)
	mailinfoScissorsLineRegex  = regexp.MustCompile(`(?m)^---\s*$`)
	commonMarkDividerLineRegex = regexp.MustCompile(`^(?:___+|\*\s*\*\s*\*+)$`)
	legacyDualLayerHeaderRegex = regexp.MustCompile(`(?im)^\*\*(architectural rationale|blast radius & contracts|resolution mechanics):\*\*`)
	rawDiffDumpLineRegex       = regexp.MustCompile(`(?m)^[-*]\s+[MADRCU][0-9]*\s+\S+`)
	truncationFillerRegex      = regexp.MustCompile(`(?m)^[-*]\s+\.\.\.\s+\(\+[0-9]+\s+additional\s+`)
	cannedCommitShimRegex      = regexp.MustCompile(`(?i)(satisfy (int|sub)-[0-9]+ deterministic execution requirements|scoped strictly to (int|sub)-[0-9]+ implementation files|implemented and verified via deterministic quality gates for (int|sub)-[0-9]+)`)
)

// CommitSchemaGate enforces the Golden Conventional Commit schema ([KEY] subject prefix,
// flowing rationale prose, anti-dump hygiene, and RFC 822 Git Trailers) across branch commits.
// @intent nomos_hermetic_worktree_spec.md #CommitSchemaGate
type CommitSchemaGate struct{}

// Name returns the canonical name of the Commit Schema quality gate.
func (g *CommitSchemaGate) Name() string {
	return "Commit Schema Invariant Gate"
}

// Evaluate parses all active branch commits (or HEAD fallback) to ensure format and anti-shim compliance.
func (g *CommitSchemaGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	if ctx != nil && ctx.IsBaseline {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "baseline trunk verification mode; historical commit schema check bypassed",
		}, nil
	}
	root := vscope.ResolveBoundariesRoot(ctx)
	if isPreCommitExempt(root) {
		return &vscope.GateResult{
			GateName: g.Name(),
			Passed:   true,
			Message:  "worktree branch has zero commits relative to trunk; exempt from HEAD commit schema gate",
		}, nil
	}

	mode := resolveIntentTwinMode(ctx)
	commits := collectBranchCommitsForSchemaAudit(root)
	if len(commits) == 0 {
		return evaluateSingleHeadCommitWithMode(g.Name(), root, mode)
	}

	for _, entry := range commits {
		res, err := checkCommitComplianceInWorktree(g.Name(), root, entry.body, mode)
		if err != nil || !res.Passed {
			if res != nil && entry.shortSHA != "" {
				res.Message = fmt.Sprintf("commit %s: %s", entry.shortSHA, res.Message)
			}
			return res, err
		}
	}

	return &vscope.GateResult{
		GateName: g.Name(),
		Passed:   true,
		Message:  fmt.Sprintf("verified %d branch commit(s) against Golden Commit + Task trailer schema", len(commits)),
	}, nil
}

func evaluateSingleHeadCommit(gateName, root string) (*vscope.GateResult, error) {
	return evaluateSingleHeadCommitWithMode(gateName, root, "")
}

func evaluateSingleHeadCommitWithMode(gateName, root, mode string) (*vscope.GateResult, error) {
	cmd := workspace.SafeGitCommand(root, "log", "-1", "--format=%B")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &vscope.GateResult{GateName: gateName, Passed: true, Message: "no commit history to evaluate"}, nil
	}
	commitMsg := strings.TrimSpace(string(out))
	if commitMsg == "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: "HEAD commit message is empty", Remedy: remedyAmendCommit}, nil
	}
	return checkCommitComplianceInWorktree(gateName, root, commitMsg, mode)
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

func checkCommitCompliance(gateName, commitMsg string) (*vscope.GateResult, error) {
	return checkCommitComplianceInWorktree(gateName, "", commitMsg, "")
}

func checkCommitComplianceWithMode(gateName, commitMsg, mode string) (*vscope.GateResult, error) {
	return checkCommitComplianceInWorktree(gateName, "", commitMsg, mode)
}

func checkCommitComplianceInWorktree(gateName, worktreeDir, commitMsg, mode string) (*vscope.GateResult, error) {
	trimmedMsg := strings.TrimSpace(commitMsg)
	if trimmedMsg == "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: "commit message is empty", Remedy: remedyAmendCommit}, nil
	}
	lines := strings.Split(strings.ReplaceAll(trimmedMsg, "\r\n", "\n"), "\n")
	firstLine := strings.TrimSpace(lines[0])

	if isMergeCommit(firstLine) {
		return &vscope.GateResult{GateName: gateName, Passed: true, Message: "merge commit is exempt from Golden Commit trailer invariant"}, nil
	}
	subjectKey, violation := validateSubjectLine(firstLine)
	if violation != "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if violation := validateAntiShimAndAntiDump(trimmedMsg); violation != "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if violation := validateBodyAndTrailers(lines[1:], subjectKey); violation != "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if violation := ValidateWorktreeIntentTwin(worktreeDir, trimmedMsg, mode); violation != "" {
		return &vscope.GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: IntentTwinModeRemedy(subjectKey)}, nil
	}

	return &vscope.GateResult{GateName: gateName, Passed: true, Message: "HEAD commit schema satisfies Golden Commit + Task trailer invariant"}, nil
}

func validateSubjectLine(firstLine string) (string, string) {
	m := prefixKeyExtractRegex.FindStringSubmatch(firstLine)
	if m == nil {
		return "", fmt.Sprintf("commit subject missing canonical [INT-N] or [SUB-N] prefix on first line: %q", firstLine)
	}
	if trailingDupTaskKeyRegex.MatchString(firstLine) {
		return "", fmt.Sprintf("commit subject contains duplicate trailing task key token: %q", firstLine)
	}
	if !goldenSubjectSchema.MatchString(firstLine) {
		return "", fmt.Sprintf("commit subject must match '[<INT|SUB-N>] <feat|perf|fix|docs|chore|refactor|test>(<scope>): <summary>': %q", firstLine)
	}
	if euphemisticBuzzwordRe.MatchString(firstLine) {
		return "", fmt.Sprintf("commit subject contains marketing euphemism in %q; use engineering framing: Parity (feat), Throughput (perf), Hardening/Calibration (refactor), or Defect (fix)", firstLine)
	}
	if n := utf8.RuneCountInString(firstLine); n > maxSubjectLineRunes {
		return "", fmt.Sprintf("commit subject exceeds %d-rune ceiling (%d runes); target <= 80 runes: %q", maxSubjectLineRunes, n, firstLine)
	}
	return m[1], ""
}

func validateAntiShimAndAntiDump(commitMsg string) string {
	if cannedCommitShimRegex.MatchString(commitMsg) {
		return "commit message contains prohibited canned boilerplate shim (Law 2 Anti-Shim)"
	}
	if mailinfoScissorsLineRegex.MatchString(commitMsg) {
		return "commit message uses '---' patch-scissors line which truncates git interpret-trailers; use '___' divider instead"
	}
	if legacyDualLayerHeaderRegex.MatchString(commitMsg) {
		return "commit message contains legacy Dual-Layer bold header; write flowing plain-text rationale prose"
	}
	if rawDiffDumpLineRegex.MatchString(commitMsg) || truncationFillerRegex.MatchString(commitMsg) {
		return "commit message contains raw git diff file-list dump or truncation filler (Law 4 SSoT: Git tree diff owns file manifests)"
	}
	return ""
}

func validateBodyAndTrailers(rest []string, subjectKey string) string {
	if len(rest) == 0 || strings.TrimSpace(rest[0]) != "" {
		return "commit message requires a blank line after subject followed by rationale prose and Git trailers"
	}
	prose, trailers, errMsg := splitBodyAndGitTrailers(rest[1:])
	if errMsg != "" {
		return errMsg
	}
	if utf8.RuneCountInString(prose) < minBodyProseRunes {
		return "commit message requires substantive rationale prose (>= 12 chars) above the Git trailer block"
	}
	return validateRequiredTrailers(trailers, subjectKey)
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
	var proseLines []string
	for _, raw := range lines[:trailerStart] {
		trimmed := strings.TrimSpace(raw)
		if commonMarkDividerLineRegex.MatchString(trimmed) {
			continue
		}
		proseLines = append(proseLines, raw)
	}
	prose := strings.TrimSpace(strings.Join(proseLines, "\n"))
	return prose, trailers, ""
}

func validateRequiredTrailers(trailers map[string][]string, subjectKey string) string {
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
	if subjectKey != "" && tasks[0] != subjectKey {
		return fmt.Sprintf("subject key [%s] does not match 'Task: %s' trailer", subjectKey, tasks[0])
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
