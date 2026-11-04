// Package verify provides Definition of Done (DoD) quality gate pipeline execution.
package verify

import (
	"fmt"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

// ValidateCommitIntentTwinMode enforces governance.quality_gates.intent_twin_mode
// ("off", "feat", "strict") against a hydrated or committed Golden Commit message.
// @intent nomos_commit_schema_invariant_spec.md #QualityGatesConfig
func ValidateCommitIntentTwinMode(commitMsg, mode string) string {
	normMode := strings.ToLower(strings.TrimSpace(mode))
	rank, ok := config.IntentTwinModeRank(normMode)
	if !ok {
		return fmt.Sprintf("invalid governance.quality_gates.intent_twin_mode %q (expected off, feat, or strict)", mode)
	}
	if rank == 0 {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(commitMsg), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return ""
	}
	firstLine := strings.TrimSpace(lines[0])
	if isMergeCommit(firstLine) {
		return ""
	}
	m := goldenSubjectSchema.FindStringSubmatch(firstLine)
	if m == nil {
		return ""
	}
	subjectKey, commitType := m[1], m[2]
	if !strings.HasPrefix(subjectKey, "SUB-") {
		return ""
	}
	if normMode == config.IntentTwinModeFeat && commitType != "feat" {
		return ""
	}
	_, trailers, errMsg := splitBodyAndGitTrailers(lines[1:])
	if errMsg != "" || !hasValidIntentTwinTrailer(trailers) {
		return fmt.Sprintf(
			"Substrate %s commit [%s] requires a bound 'Twin: INT-<N>' trailer when governance.quality_gates.intent_twin_mode=%s (Law 4 & Law 7)",
			commitType, subjectKey, normMode,
		)
	}
	return ""
}

func hasValidIntentTwinTrailer(trailers map[string][]string) bool {
	if trailers == nil {
		return false
	}
	for _, twin := range trailers["Twin"] {
		trimmed := strings.TrimSpace(twin)
		if strings.HasPrefix(trimmed, "INT-") && canonicalTaskTrailerValRe.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// IntentTwinModeRemedy returns the deterministic HATEOAS remediation string for missing INT twins.
// @intent nomos_commit_schema_invariant_spec.md #QualityGatesConfig
func IntentTwinModeRemedy(subjectKey string) string {
	key := strings.TrimSpace(subjectKey)
	if key == "" {
		key = "SUB-<N>"
	}
	return fmt.Sprintf(
		"bind an Intent specification twin via 'nomos task create \"<spec title>\" --plane intent --twin-of %s' (or 'nomos task pair') and re-commit via 'nomos task commit'",
		key,
	)
}

func resolveIntentTwinMode(ctx *VerifyContext) string {
	cfg := resolveVerifyConfig(ctx)
	if cfg == nil {
		return config.IntentTwinModeOff
	}
	return cfg.Governance.QualityGates.EffectiveIntentTwinMode()
}

func checkIntentTwinModeDowngrade(trunkCfg, wtCfg *config.EcosystemConfig) string {
	if trunkCfg == nil || wtCfg == nil {
		return ""
	}
	wtRank, wtOK := config.IntentTwinModeRank(wtCfg.Governance.QualityGates.IntentTwinMode)
	if !wtOK {
		return fmt.Sprintf("worktree nomos.yaml contains invalid governance.quality_gates.intent_twin_mode %q (expected off, feat, or strict)", wtCfg.Governance.QualityGates.IntentTwinMode)
	}
	trunkMode := trunkCfg.Governance.QualityGates.EffectiveIntentTwinMode()
	trunkRank, trunkOK := config.IntentTwinModeRank(trunkMode)
	if !trunkOK {
		return fmt.Sprintf("trunk nomos.yaml contains invalid governance.quality_gates.intent_twin_mode %q", trunkMode)
	}
	wtMode := wtCfg.Governance.QualityGates.EffectiveIntentTwinMode()
	if wtRank < trunkRank {
		return fmt.Sprintf("worktree nomos.yaml illegally downgrades governance.quality_gates.intent_twin_mode from trunk baseline (%s -> %s)", trunkMode, wtMode)
	}
	return ""
}
