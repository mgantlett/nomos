// Package workspace provides workspace isolation, git hook management, and hermetic subprocess execution.
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// AssetType defines the category of canonical embedded asset governed under Law 4.
type AssetType string

const (
	AssetTypeHook      AssetType = "hook"
	AssetTypeRule      AssetType = "rule"
	AssetTypeDirective AssetType = "directive"
	AssetTypeSchema    AssetType = "schema"
	AssetTypeConfig    AssetType = "config"
)

// IsValid validates whether an AssetType matches the canonical enum definition.
func (t AssetType) IsValid() bool {
	switch t {
	case AssetTypeHook, AssetTypeRule, AssetTypeDirective, AssetTypeSchema, AssetTypeConfig:
		return true
	default:
		return false
	}
}

// Regex patterns for asset parity validation.
var (
	regexAssetPath = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
	regexSHA256    = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// EmbeddedAsset models a SHA-256 checksummed workspace asset contract.
type EmbeddedAsset struct {
	RelativePath string    `json:"relative_path"`
	AssetType    AssetType `json:"asset_type"`
	SHA256       string    `json:"sha256"`
	SizeBytes    int64     `json:"size_bytes"`
}

// Validate ensures EmbeddedAsset satisfies all structural constraints.
func (a *EmbeddedAsset) Validate() error {
	if a == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "embedded asset cannot be nil")
	}
	if !regexAssetPath.MatchString(a.RelativePath) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "relative_path %q must match ^[a-zA-Z0-9_./-]+$", a.RelativePath)
	}
	if !a.AssetType.IsValid() {
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid asset_type %q: must be hook, rule, directive, schema, or config", a.AssetType)
	}
	if !regexSHA256.MatchString(a.SHA256) {
		return types.NewNomosError(types.ErrCodeValidationFailed, "sha256 %q must be 64 lowercase hex characters", a.SHA256)
	}
	if a.SizeBytes < 0 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "size_bytes must be non-negative, got %d", a.SizeBytes)
	}
	return nil
}

// ParityMismatchReason defines the cause of an asset parity failure.
type ParityMismatchReason string

const (
	MismatchMissing    ParityMismatchReason = "missing"
	MismatchDrift      ParityMismatchReason = "drift"
	MismatchUnreadable ParityMismatchReason = "unreadable"
)

// ParityMismatch records a cryptographic or missing-file asset parity failure.
type ParityMismatch struct {
	SourcePath string               `json:"source_path"`
	TargetPath string               `json:"target_path"`
	SourceHash string               `json:"source_hash"`
	TargetHash string               `json:"target_hash"`
	Reason     ParityMismatchReason `json:"reason"`
}

// Validate ensures ParityMismatch satisfies all structural constraints.
func (m *ParityMismatch) Validate() error {
	if m == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "parity mismatch cannot be nil")
	}
	switch m.Reason {
	case MismatchMissing, MismatchDrift, MismatchUnreadable:
		return nil
	default:
		return types.NewNomosError(types.ErrCodeValidationFailed, "invalid reason %q: must be missing, drift, or unreadable", m.Reason)
	}
}

// ParityAuditReport summarizes the workspace parity audit outcome.
type ParityAuditReport struct {
	WorktreePath string           `json:"worktree_path"`
	Timestamp    string           `json:"timestamp"`
	TotalAssets  int              `json:"total_assets"`
	Mismatches   []ParityMismatch `json:"mismatches"`
	Passed       bool             `json:"passed"`
}

// Validate ensures ParityAuditReport satisfies all structural constraints.
func (r *ParityAuditReport) Validate() error {
	if r == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "parity audit report cannot be nil")
	}
	if r.TotalAssets < 0 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "total_assets must be non-negative, got %d", r.TotalAssets)
	}
	for i, m := range r.Mismatches {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("mismatch[%d] invalid: %w", i, err)
		}
	}
	return nil
}

// ComputeFileSHA256 reads a file and computes its SHA256 hex string and byte count.
func ComputeFileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), size, nil
}

// CheckAssetParity audits assets between sourceRoot and targetRoot, returning a ParityAuditReport.
func CheckAssetParity(sourceRoot, targetRoot string, assets []EmbeddedAsset) (*ParityAuditReport, error) {
	if len(assets) == 0 {
		return &ParityAuditReport{
			WorktreePath: targetRoot,
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			TotalAssets:  0,
			Mismatches:   []ParityMismatch{{SourcePath: sourceRoot, TargetPath: targetRoot, Reason: MismatchMissing}},
			Passed:       false,
		}, types.NewNomosError(types.ErrCodeInvariant, "zero canonical assets discovered for parity audit (ERR_ZERO_ASSETS_DISCOVERED)")
	}

	report := &ParityAuditReport{
		WorktreePath: targetRoot,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		TotalAssets:  len(assets),
		Mismatches:   []ParityMismatch{},
		Passed:       true,
	}

	for _, asset := range assets {
		if mismatch, ok := auditSingleAsset(sourceRoot, targetRoot, asset); ok {
			report.Mismatches = append(report.Mismatches, *mismatch)
			report.Passed = false
		}
	}

	return report, nil
}

func auditSingleAsset(sourceRoot, targetRoot string, asset EmbeddedAsset) (*ParityMismatch, bool) {
	srcFile := filepath.Join(sourceRoot, asset.RelativePath)
	tgtFile := filepath.Join(targetRoot, asset.RelativePath)

	srcHash := asset.SHA256
	if srcHash == "" {
		h, _, err := ComputeFileSHA256(srcFile)
		if err != nil {
			return &ParityMismatch{SourcePath: srcFile, TargetPath: tgtFile, Reason: MismatchUnreadable}, true
		}
		srcHash = h
	}

	if _, err := os.Stat(tgtFile); os.IsNotExist(err) {
		return &ParityMismatch{SourcePath: srcFile, TargetPath: tgtFile, SourceHash: srcHash, Reason: MismatchMissing}, true
	}

	tgtHash, _, err := ComputeFileSHA256(tgtFile)
	if err != nil {
		return &ParityMismatch{SourcePath: srcFile, TargetPath: tgtFile, SourceHash: srcHash, Reason: MismatchUnreadable}, true
	}

	if srcHash != tgtHash {
		return &ParityMismatch{
			SourcePath: srcFile,
			TargetPath: tgtFile,
			SourceHash: srcHash,
			TargetHash: tgtHash,
			Reason:     MismatchDrift,
		}, true
	}

	return nil, false
}

// DiscoverParityAssets dynamically discovers canonical assets governed under Law 4.
func DiscoverParityAssets(sourceRoot string) ([]EmbeddedAsset, error) {
	var assets []EmbeddedAsset

	// 1. Root AGENTS.md
	agentsPath := filepath.Join(sourceRoot, "AGENTS.md")
	if _, err := os.Stat(agentsPath); err == nil {
		if hash, size, hErr := ComputeFileSHA256(agentsPath); hErr == nil {
			assets = append(assets, EmbeddedAsset{
				RelativePath: "AGENTS.md",
				AssetType:    AssetTypeDirective,
				SHA256:       hash,
				SizeBytes:    size,
			})
		}
	}

	// 2. Root nomos.schema.json contract artifact
	schemaPath := filepath.Join(sourceRoot, "nomos.schema.json")
	if _, err := os.Stat(schemaPath); err == nil {
		if hash, size, hErr := ComputeFileSHA256(schemaPath); hErr == nil {
			assets = append(assets, EmbeddedAsset{
				RelativePath: "nomos.schema.json",
				AssetType:    AssetTypeSchema,
				SHA256:       hash,
				SizeBytes:    size,
			})
		}
	}

	// 3. Ambient lifecycle hooks under .agents
	assets = append(assets, discoverHookParityAssets(sourceRoot)...)

	return assets, nil
}

func discoverHookParityAssets(sourceRoot string) []EmbeddedAsset {
	var assets []EmbeddedAsset
	agentsDir := filepath.Join(sourceRoot, ".agents")
	hooksManifest := filepath.Join(agentsDir, "hooks.json")
	if hash, size, hErr := ComputeFileSHA256(hooksManifest); hErr == nil {
		if rel, rErr := filepath.Rel(sourceRoot, hooksManifest); rErr == nil {
			assets = append(assets, EmbeddedAsset{
				RelativePath: filepath.ToSlash(rel),
				AssetType:    AssetTypeHook,
				SHA256:       hash,
				SizeBytes:    size,
			})
		}
	}

	scriptsDir := filepath.Join(agentsDir, "scripts")
	_ = filepath.Walk(scriptsDir, func(p string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			if rel, rErr := filepath.Rel(sourceRoot, p); rErr == nil {
				if hash, size, hErr := ComputeFileSHA256(p); hErr == nil {
					assets = append(assets, EmbeddedAsset{
						RelativePath: filepath.ToSlash(rel),
						AssetType:    AssetTypeHook,
						SHA256:       hash,
						SizeBytes:    size,
					})
				}
			}
		}
		return nil
	})
	return assets
}

// CheckBilateralParity audits holistic isomorphic parity between Intent and Substrate.
func CheckBilateralParity(sourceRoot, targetRoot string) (*ParityAuditReport, error) {
	assets, err := DiscoverParityAssets(sourceRoot)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "failed discovering parity assets: %v", err)
	}
	return CheckAssetParity(sourceRoot, targetRoot, assets)
}


