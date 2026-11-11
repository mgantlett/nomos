// Package config defines switchable verification scope, TDD, and statement coverage enforcement modes for nomos.yaml.
package config

import "strings"

const (
	// VerifyScopeDiff evaluates only modified or unattested files and packages inside task worktrees (default).
	VerifyScopeDiff = "diff"
	// VerifyScopeRepo scans 100% of repository files and packages even inside task worktrees.
	VerifyScopeRepo = "repo"

	// TDDModeOff disables paired-test enforcement (permitted only when strict_mode is false).
	TDDModeOff = "off"
	// TDDModeRatchet requires paired tests on net-new modules while permitting legacy baseline edits.
	TDDModeRatchet = "ratchet"
	// TDDModeDiff requires every modified or added source file in the diff to have a paired or modified test.
	TDDModeDiff = "diff"
	// TDDModeStrict requires 100% of source directories/modules in scope to possess unit tests.
	TDDModeStrict = "strict"

	// CoverageModeOff disables statement coverage enforcement (permitted only when strict_mode is false).
	CoverageModeOff = "off"
	// CoverageModeArtifact evaluates on-disk coverage reports (LCOV, Cobertura/JaCoCo, SimpleCov) without live probes.
	CoverageModeArtifact = "artifact"
	// CoverageModeLive executes zero-dependency live coverage probes (Go, Python trace, Node V8, Ruby Coverage) and parses artifacts.
	CoverageModeLive = "live"

	// CredentialReadModeHostDeny denies PreToolUse reads of host credential stores while taint-tracking workspace .env (default).
	CredentialReadModeHostDeny = "host_deny"
	// CredentialReadModeStrict denies PreToolUse reads of both host credential stores and non-template workspace .env files.
	CredentialReadModeStrict = "strict"
	// CredentialReadModeTaintOnly permits credential reads on PreToolUse and records TaintCredentialSource to block outbound egress.
	CredentialReadModeTaintOnly = "taint_only"
)

// EffectiveVerifyScope returns the active worktree verification scope, defaulting to "diff" (@intent: #QualityGatesConfig).
func (q *QualityGatesConfig) EffectiveVerifyScope() string {
	if q == nil {
		return VerifyScopeDiff
	}
	norm := strings.ToLower(strings.TrimSpace(q.VerifyScope))
	if norm == "" {
		return VerifyScopeDiff
	}
	return norm
}

// VerifyScopeRank returns strictness rank (1=diff, 2=repo) and validity (@intent: #QualityGatesConfig).
func VerifyScopeRank(scope string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "":
		return -1, true
	case VerifyScopeDiff:
		return 1, true
	case VerifyScopeRepo:
		return 2, true
	default:
		return -1, false
	}
}

// EffectiveTDDMode returns the active TDD mode, defaulting to "strict" in strict_mode and "ratchet" in brownfield mode (@intent: #QualityGatesConfig).
func (q *QualityGatesConfig) EffectiveTDDMode() string {
	if q == nil {
		return TDDModeStrict
	}
	norm := strings.ToLower(strings.TrimSpace(q.TDDMode))
	if norm != "" {
		return norm
	}
	if q.StrictMode {
		return TDDModeStrict
	}
	return TDDModeRatchet
}

// TDDModeRank returns strictness rank (0=off, 1=ratchet, 2=diff, 3=strict) and validity (@intent: #QualityGatesConfig).
func TDDModeRank(mode string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return -1, true
	case TDDModeOff:
		return 0, true
	case TDDModeRatchet:
		return 1, true
	case TDDModeDiff:
		return 2, true
	case TDDModeStrict:
		return 3, true
	default:
		return -1, false
	}
}

// EffectiveCoverageMode returns the active statement coverage mode, defaulting to "live" (@intent: #QualityGatesConfig).
func (q *QualityGatesConfig) EffectiveCoverageMode() string {
	if q == nil {
		return CoverageModeLive
	}
	norm := strings.ToLower(strings.TrimSpace(q.CoverageMode))
	if norm == "" {
		return CoverageModeLive
	}
	return norm
}

// CoverageModeRank returns strictness rank (0=off, 1=artifact, 2=live) and validity (@intent: #QualityGatesConfig).
func CoverageModeRank(mode string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "":
		return -1, true
	case CoverageModeOff:
		return 0, true
	case CoverageModeArtifact:
		return 1, true
	case CoverageModeLive:
		return 2, true
	default:
		return -1, false
	}
}

// EffectiveCredentialReadMode returns normalized credential_read_mode, defaulting to "host_deny" (@intent: #BoundaryConfig).
func (b *BoundaryConfig) EffectiveCredentialReadMode() string {
	if b == nil {
		return CredentialReadModeHostDeny
	}
	norm := strings.ToLower(strings.TrimSpace(b.CredentialReadMode))
	if norm == "" {
		return CredentialReadModeHostDeny
	}
	return norm
}

// IsValidCredentialReadMode reports whether mode is a valid BoundaryConfig.CredentialReadMode setting (@intent: #BoundaryConfig).
func IsValidCredentialReadMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", CredentialReadModeHostDeny, CredentialReadModeStrict, CredentialReadModeTaintOnly:
		return true
	default:
		return false
	}
}
