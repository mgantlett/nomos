// Package xray implements Day-0 baseline manifest persistence.
package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

const baselineFileNameConst = constants.FileNomosBaselineJSON

// BuildBaselineManifest synthesizes an XrayBaselineManifest from a scanned XrayReport (@intent tree_sitter_ast_gate_spec.md #XrayBaselineManifest).
func BuildBaselineManifest(targetDir string, rep *XrayReport) *XrayBaselineManifest {
	if rep == nil {
		rep = &XrayReport{ProjectName: "workspace", Grade: "A"}
	}
	headCommit := resolveGitHeadCommit(targetDir)
	return &XrayBaselineManifest{
		SchemaVersion:      BaselineSchemaVersion,
		ProjectName:        rep.ProjectName,
		RecordedAt:         time.Now().UTC().Format(time.RFC3339),
		HeadCommit:         headCommit,
		HangoverScore:      rep.HangoverScore,
		Grade:              rep.Grade,
		FilesScanned:       rep.FilesScanned,
		TotalLOC:           rep.TotalLOC,
		InferredBoundaries: rep.InferredBoundaries,
		BaselineEnvelope:   rep.BaselineEnvelope,
	}
}

func resolveGitHeadCommit(targetDir string) string {
	if strings.TrimSpace(targetDir) == "" {
		return ""
	}
	out, err := workspace.RunSafeGit(targetDir, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func resolveConfinedBaselinePath(targetDir string) (string, error) {
	absBase, err := filepath.Abs(targetDir)
	if err != nil {
		return "", err
	}
	absTarget, tErr := filepath.Abs(filepath.Join(absBase, baselineFileNameConst))
	if tErr != nil {
		return "", tErr
	}
	relOut, rErr := filepath.Rel(absBase, absTarget)
	if rErr != nil || strings.HasPrefix(relOut, "..") {
		return "", fmt.Errorf("baseline path escaped repository boundary")
	}
	return absTarget, nil
}

// WriteBaselineManifest writes nomos.baseline.json atomically inside targetDir and returns its file path.
func WriteBaselineManifest(targetDir string, rep *XrayReport) (string, error) {
	outPath, err := resolveConfinedBaselinePath(targetDir)
	if err != nil {
		return "", err
	}
	manifest := BuildBaselineManifest(targetDir, rep)
	encoded, mErr := json.MarshalIndent(manifest, "", "  ")
	if mErr != nil {
		return "", fmt.Errorf("failed marshaling baseline manifest: %w", mErr)
	}
	encoded = append(encoded, '\n')
	if wErr := os.WriteFile(outPath, encoded, 0644); wErr != nil {
		return "", fmt.Errorf("failed writing %s: %w", baselineFileNameConst, wErr)
	}
	return outPath, nil
}

// LoadBaselineManifest reads and validates nomos.baseline.json from targetDir.
func LoadBaselineManifest(targetDir string) (*XrayBaselineManifest, error) {
	inPath, err := resolveConfinedBaselinePath(targetDir)
	if err != nil {
		return nil, err
	}
	raw, rErr := os.ReadFile(inPath)
	if rErr != nil {
		return nil, rErr
	}
	var m XrayBaselineManifest
	if uErr := json.Unmarshal(raw, &m); uErr != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", baselineFileNameConst, uErr)
	}
	if m.SchemaVersion == "" {
		return nil, fmt.Errorf("invalid %s: missing schema_version", baselineFileNameConst)
	}
	return &m, nil
}
