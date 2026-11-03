// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const remedyAmendCommit = "re-commit via `nomos task commit` to enforce Dual-Layer schema compliance"

var (
	prefixRegex               = regexp.MustCompile(`^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]`)
	conventionalSubjectSchema = regexp.MustCompile(`^\[(INT|SUB)-[1-9][0-9]*(-[a-z0-9\-]+)?\]\s+(feat|fix|docs|chore|refactor|test)(\([a-z0-9_,/-]+\))?:\s+\S+`)
	trailingDupTaskKeyRegex   = regexp.MustCompile(`(?i)(\[(?:task\s+)?(?:int|sub|arc)-[0-9]+[a-z]?\]|\((?:int|sub|arc)-[0-9]+[a-z]?\))\s*$`)
	cannedCommitShimRegex     = regexp.MustCompile(`(?i)(satisfy (int|sub)-[0-9]+ deterministic execution requirements|scoped strictly to (int|sub)-[0-9]+ implementation files|implemented and verified via deterministic quality gates for (int|sub)-[0-9]+)`)
)

var dualLayerHeaders = []string{
	"**architectural rationale:**",
	"**blast radius & contracts:**",
	"**resolution mechanics:**",
}

// CommitSchemaGate enforces the Dual-Layer Hybrid AI schema and Canonical Singularity Law
// across all unmerged branch commits.
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
		cmd := workspace.SafeGitCommand(root, "log", "-1", "--format=%B")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return &GateResult{GateName: g.Name(), Passed: true, Message: "no commit history to evaluate"}, nil
		}
		commitMsg := strings.TrimSpace(string(out))
		if commitMsg == "" {
			return &GateResult{GateName: g.Name(), Passed: false, Message: "HEAD commit message is empty", Remedy: remedyAmendCommit}, nil
		}
		return checkCommitCompliance(g.Name(), commitMsg)
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
		Message:  fmt.Sprintf("verified %d branch commit(s) against Dual-Layer schema", len(commits)),
	}, nil
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
		var shas []string
		for _, line := range strings.Split(strings.TrimSpace(string(revOut)), "\n") {
			if s := strings.TrimSpace(line); s != "" {
				shas = append(shas, s)
			}
		}
		if len(shas) > 0 && (best == nil || len(shas) < len(best)) {
			best = shas
		}
	}
	return best
}

func isPreCommitExempt(worktreePath string) bool {
	if worktreePath == "" {
		return false
	}
	currentBranchCmd := workspace.SafeGitCommand(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, branchErr := currentBranchCmd.CombinedOutput()
	if branchErr != nil {
		return false
	}
	branchName := strings.TrimSpace(string(branchOut))
	if workspace.IsProtectedBranch(branchName) || branchName == "" || branchName == "HEAD" {
		return false
	}
	for _, t := range workspace.ProtectedTrunkBranches {
		checkCmd := workspace.SafeGitCommand(worktreePath, "rev-parse", "--verify", t)
		if err := checkCmd.Run(); err == nil {
			revCmd := workspace.SafeGitCommand(worktreePath, "rev-list", t+"..HEAD")
			revOut, revErr := revCmd.CombinedOutput()
			if revErr == nil && strings.TrimSpace(string(revOut)) == "" {
				return true
			}
		}
	}
	return false
}

func checkCommitCompliance(gateName, commitMsg string) (*GateResult, error) {
	if strings.TrimSpace(commitMsg) == "" {
		return &GateResult{GateName: gateName, Passed: false, Message: "commit message is empty", Remedy: remedyAmendCommit}, nil
	}
	lines := strings.Split(commitMsg, "\n")
	firstLine := strings.TrimSpace(lines[0])

	if isMergeCommit(firstLine) {
		return &GateResult{GateName: gateName, Passed: true, Message: "merge commit is exempt from task ID prefix invariant"}, nil
	}
	if violation := validateSubjectLine(firstLine); violation != "" {
		return &GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}
	if cannedCommitShimRegex.MatchString(commitMsg) {
		return &GateResult{
			GateName: gateName,
			Passed:   false,
			Message:  "commit message contains prohibited canned boilerplate shim (Law 2 Anti-Shim)",
			Remedy:   remedyAmendCommit,
		}, nil
	}
	if violation := validateDualLayerSections(lines[1:]); violation != "" {
		return &GateResult{GateName: gateName, Passed: false, Message: violation, Remedy: remedyAmendCommit}, nil
	}

	return &GateResult{GateName: gateName, Passed: true, Message: "HEAD commit schema is properly formatted"}, nil
}

func validateSubjectLine(firstLine string) string {
	if !prefixRegex.MatchString(firstLine) {
		return fmt.Sprintf("commit message missing canonical [TaskID] prefix on first line: %q", firstLine)
	}
	if !conventionalSubjectSchema.MatchString(firstLine) {
		return fmt.Sprintf("commit subject must include Conventional Commit type (feat|fix|docs|chore|refactor|test): %q", firstLine)
	}
	if trailingDupTaskKeyRegex.MatchString(firstLine) {
		return fmt.Sprintf("commit subject contains duplicate trailing task key token: %q", firstLine)
	}
	return ""
}

func validateDualLayerSections(bodyLines []string) string {
	sections := extractDualLayerBodies(bodyLines)
	if len(sections) == 0 {
		return "commit message missing dual-layer headings (**Architectural Rationale:**, **Blast Radius & Contracts:**, or **Resolution Mechanics:**)"
	}
	for header, content := range sections {
		if utf8.RuneCountInString(strings.TrimSpace(content)) < 4 {
			return fmt.Sprintf("dual-layer heading %q has empty or trivial body", header)
		}
	}
	return ""
}

func extractDualLayerBodies(lines []string) map[string]string {
	found := make(map[string][]string)
	active := ""
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		lower := strings.ToLower(trimmed)
		matchedHeader := ""
		for _, h := range dualLayerHeaders {
			if strings.HasPrefix(lower, h) {
				matchedHeader = h
				remainder := strings.TrimSpace(trimmed[len(h):])
				found[h] = nil
				if remainder != "" {
					found[h] = append(found[h], remainder)
				}
				break
			}
		}
		if matchedHeader != "" {
			active = matchedHeader
			continue
		}
		if active != "" {
			found[active] = append(found[active], raw)
		}
	}
	out := make(map[string]string, len(found))
	for k, v := range found {
		out[k] = strings.TrimSpace(strings.Join(v, "\n"))
	}
	return out
}

func isMergeCommit(firstLine string) bool {
	return strings.HasPrefix(firstLine, "Merge pull request ") ||
		strings.HasPrefix(firstLine, "Merge branch ") ||
		strings.HasPrefix(firstLine, "Merge remote-tracking branch ")
}
