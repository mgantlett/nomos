// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

const embeddedTemplatesRelPrefix = "substrate/cli/templates/"

var templateSchemaSemVerPinRe = regexp.MustCompile(`(#\s*yaml-language-server:\s*\$schema=\S+/nomos/)v[0-9]+\.[0-9]+\.[0-9]+(/nomos\.schema\.json)`)

// NormativeSurfaceDelta captures AST and template contract changes between trunk and worktree (#NormativeSurfaceDelta).
type NormativeSurfaceDelta struct {
	CLISurface      []string `json:"cli_surface,omitempty"`
	ConfigSchema    []string `json:"config_schema,omitempty"`
	GovernanceRules []string `json:"governance_rules,omitempty"`
	TemplateAssets  []string `json:"template_assets,omitempty"`
}

// HasChanges reports whether any normative public surface changed relative to trunk.
// @intent nomos_hermetic_worktree_spec.md #NormativeSurfaceDelta
func (d NormativeSurfaceDelta) HasChanges() bool {
	return len(d.CLISurface) > 0 || len(d.ConfigSchema) > 0 || len(d.GovernanceRules) > 0 || len(d.TemplateAssets) > 0
}

// Summary formats a deterministic, bounded description of changed normative contract surfaces.
// @intent nomos_hermetic_worktree_spec.md #NormativeSurfaceDelta
func (d NormativeSurfaceDelta) Summary() string {
	var parts []string
	if s := formatBoundedTokens("cli_surface", d.CLISurface); s != "" {
		parts = append(parts, s)
	}
	if s := formatBoundedTokens("config_schema", d.ConfigSchema); s != "" {
		parts = append(parts, s)
	}
	if s := formatBoundedTokens("governance_rules", d.GovernanceRules); s != "" {
		parts = append(parts, s)
	}
	if s := formatBoundedTokens("template_assets", d.TemplateAssets); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func formatBoundedTokens(label string, items []string) string {
	if len(items) == 0 {
		return ""
	}
	limit := len(items)
	suffix := ""
	if limit > 3 {
		suffix = fmt.Sprintf(" +%d", limit-3)
		limit = 3
	}
	return fmt.Sprintf("%s[%s%s]", label, strings.Join(items[:limit], ", "), suffix)
}

// InspectWorktreeNormativeSurface diffs trunk and worktree Go ASTs and embedded templates to detect contract expansion.
// @intent nomos_hermetic_worktree_spec.md #NormativeSurfaceDelta
func InspectWorktreeNormativeSurface(worktreeDir string) NormativeSurfaceDelta {
	if strings.TrimSpace(worktreeDir) == "" {
		return NormativeSurfaceDelta{}
	}
	baseRef := resolveClosestTrunkBaseRef(worktreeDir)
	if baseRef == "" {
		return NormativeSurfaceDelta{}
	}
	changedPaths := collectChangedPathsAgainstBase(worktreeDir, baseRef)
	if len(changedPaths) == 0 {
		return NormativeSurfaceDelta{}
	}

	cliSet := make(map[string]bool)
	cfgSet := make(map[string]bool)
	govSet := make(map[string]bool)
	tplSet := make(map[string]bool)

	for _, rel := range changedPaths {
		inspectChangedNormativeFile(worktreeDir, baseRef, rel, cliSet, cfgSet, govSet, tplSet)
	}

	return NormativeSurfaceDelta{
		CLISurface:      sortedSetKeys(cliSet),
		ConfigSchema:    sortedSetKeys(cfgSet),
		GovernanceRules: sortedSetKeys(govSet),
		TemplateAssets:  sortedSetKeys(tplSet),
	}
}

func inspectChangedNormativeFile(worktreeDir, baseRef, rel string, cliSet, cfgSet, govSet, tplSet map[string]bool) {
	normRel := filepath.ToSlash(strings.TrimSpace(rel))
	if normRel == "" {
		return
	}
	if strings.HasPrefix(normRel, embeddedTemplatesRelPrefix) {
		assetName := strings.TrimPrefix(normRel, embeddedTemplatesRelPrefix)
		baseBytes := normalizeEmbeddedTemplateBytes(assetName, readGitRevisionBytes(worktreeDir, baseRef, normRel))
		currBytes := normalizeEmbeddedTemplateBytes(assetName, readWorktreeDiskBytes(filepath.Join(worktreeDir, filepath.FromSlash(normRel))))
		if !bytes.Equal(baseBytes, currBytes) {
			tplSet[assetName] = true
		}
		return
	}
	if !strings.HasSuffix(normRel, ".go") || strings.HasSuffix(normRel, "_test.go") {
		return
	}
	baseFP := parseGoSurfaceFingerprint(readGitRevisionBytes(worktreeDir, baseRef, normRel))
	currFP := parseGoSurfaceFingerprint(readWorktreeDiskBytes(filepath.Join(worktreeDir, filepath.FromSlash(normRel))))
	mergeSymmetricDiff(cliSet, baseFP.cliTokens, currFP.cliTokens)
	mergeSymmetricDiff(cfgSet, baseFP.yamlFields, currFP.yamlFields)
	mergeSymmetricDiff(govSet, baseFP.govTokens, currFP.govTokens)
}

func normalizeEmbeddedTemplateBytes(assetName string, raw []byte) []byte {
	if assetName != "nomos.yaml" || len(raw) == 0 {
		return raw
	}
	return templateSchemaSemVerPinRe.ReplaceAll(raw, []byte("${1}v0.0.0${2}"))
}

// ValidateWorktreeIntentTwin enforces intent_twin_mode across commit headers, AST surface deltas, and Twin provenance.
// @intent nomos_hermetic_worktree_spec.md #CommitSchemaGate
func ValidateWorktreeIntentTwin(worktreeDir, commitMsg, mode string) string {
	if violation := ValidateCommitIntentTwinMode(commitMsg, mode); violation != "" {
		return violation
	}
	normMode := strings.ToLower(strings.TrimSpace(mode))
	rank, ok := config.IntentTwinModeRank(normMode)
	if !ok || rank == 0 || strings.TrimSpace(worktreeDir) == "" {
		return ""
	}
	subjectKey, commitType, twinKeys := extractSubCommitTwinContext(commitMsg)
	if !strings.HasPrefix(subjectKey, "SUB-") {
		return ""
	}
	if len(twinKeys) == 0 {
		delta := InspectWorktreeNormativeSurface(worktreeDir)
		if delta.HasChanges() {
			return fmt.Sprintf(
				"Substrate %s commit [%s] modifies normative contract surface (%s) and requires a bound 'Twin: INT-<N>' trailer when governance.quality_gates.intent_twin_mode=%s (Law 2 & Law 4)",
				commitType, subjectKey, delta.Summary(), normMode,
			)
		}
		return ""
	}
	for _, twinKey := range twinKeys {
		if provErr := verifyIntentTwinProvenance(worktreeDir, subjectKey, twinKey); provErr != "" {
			return provErr
		}
	}
	return ""
}

func extractSubCommitTwinContext(commitMsg string) (string, string, []string) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(commitMsg), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return "", "", nil
	}
	firstLine := strings.TrimSpace(lines[0])
	if isMergeCommit(firstLine) {
		return "", "", nil
	}
	m := goldenSubjectSchema.FindStringSubmatch(firstLine)
	if m == nil {
		return "", "", nil
	}
	_, trailers, errMsg := splitBodyAndGitTrailers(lines[1:])
	if errMsg != "" {
		return m[1], m[2], nil
	}
	var twins []string
	for _, twin := range trailers["Twin"] {
		trimmed := strings.TrimSpace(twin)
		if strings.HasPrefix(trimmed, "INT-") && canonicalTaskTrailerValRe.MatchString(trimmed) {
			twins = append(twins, trimmed)
		}
	}
	return m[1], m[2], twins
}

func verifyIntentTwinProvenance(worktreeDir, subjectKey, twinKey string) string {
	if hasGitCommitForTaskKey(worktreeDir, twinKey) || hasLedgerTaskRecord(twinKey) {
		return ""
	}
	return fmt.Sprintf(
		"Substrate commit [%s] declares unresolvable 'Twin: %s' absent from Git history and the Living Ledger (Law 4)",
		subjectKey, twinKey,
	)
}

func hasGitCommitForTaskKey(worktreeDir, taskKey string) bool {
	needle := "[" + strings.TrimSpace(taskKey) + "]"
	out, err := workspace.SafeGitCommand(worktreeDir, "log", "--all", "--fixed-strings", "--grep="+needle, "-n", "1", "--format=%H").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func hasLedgerTaskRecord(taskKey string) bool {
	dbPath := config.ResolveGlobalDBPath()
	if strings.TrimSpace(dbPath) == "" {
		return false
	}
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return false
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			// connection closed
		}
	}()
	rec, gErr := task.NewSQLiteTaskStore(db).GetTask(strings.TrimSpace(taskKey))
	return gErr == nil && rec != nil && rec.Status != task.StatusAborted
}

func resolveClosestTrunkBaseRef(worktreeDir string) string {
	bestBase := ""
	bestDist := -1
	for _, trunk := range workspace.ProtectedTrunkBranches {
		if workspace.SafeGitCommand(worktreeDir, "rev-parse", "--verify", trunk).Run() != nil {
			continue
		}
		mbOut, err := workspace.SafeGitCommand(worktreeDir, "merge-base", trunk, "HEAD").Output()
		if err != nil {
			continue
		}
		mb := strings.TrimSpace(string(mbOut))
		if mb == "" {
			continue
		}
		revOut, rErr := workspace.SafeGitCommand(worktreeDir, "rev-list", "--count", mb+"..HEAD").Output()
		if rErr != nil {
			continue
		}
		dist, cErr := strconv.Atoi(strings.TrimSpace(string(revOut)))
		if cErr != nil {
			continue
		}
		if bestBase == "" || dist < bestDist {
			bestBase = mb
			bestDist = dist
		}
	}
	return bestBase
}

func collectChangedPathsAgainstBase(worktreeDir, baseRef string) []string {
	seen := make(map[string]bool)
	if diffOut, err := workspace.SafeGitCommand(worktreeDir, "diff", "--name-only", baseRef).Output(); err == nil {
		for _, line := range nonEmptyLines(string(diffOut)) {
			seen[filepath.ToSlash(line)] = true
		}
	}
	if untrackedOut, err := workspace.SafeGitCommand(worktreeDir, "ls-files", "--others", "--exclude-standard").Output(); err == nil {
		for _, line := range nonEmptyLines(string(untrackedOut)) {
			seen[filepath.ToSlash(line)] = true
		}
	}
	return sortedSetKeys(seen)
}

func readGitRevisionBytes(worktreeDir, rev, relSlashPath string) []byte {
	out, err := workspace.SafeGitCommand(worktreeDir, "show", rev+":"+relSlashPath).Output()
	if err != nil {
		return nil
	}
	return out
}

func readWorktreeDiskBytes(absPath string) []byte {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}
	return raw
}

func sortedSetKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
