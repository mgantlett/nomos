// Package diag implements walkthrough path normalization and boundary airlock checks
// before 2PC trunk fast-forward merges and refs/notes/nomos/releases persistence.
package diag

import (
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// SanitizeAndValidateWalkthrough normalizes workspace and file:// URI paths to repository-relative
// coordinates, scrubs host home directories, and rejects live credentials or prohibited boundary
// package paths before 2PC merge and Git Notes persistence.
// @intent intent.md task_lifecycle_cli_spec.md #ReleaseNotePayload
func SanitizeAndValidateWalkthrough(repoRoot, wtPath, raw string, touchedFiles []string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	if secretKind := detectRawCredentialInWalkthrough(raw); secretKind != "" {
		return "", types.NewNomosError(types.ErrCodeBoundary,
			"walkthrough contains raw credential material (%s)", secretKind).
			WithRemedy("remove all tokens, API keys, and private keys from the walkthrough file before running 'nomos task sync'")
	}

	normalized := normalizeReleaseProseDashes(normalizeWorkspacePathsInWalkthrough(raw, repoRoot, wtPath))
	scrubbed := SanitizeDiagnosticText(normalized, repoRoot)

	if err := checkWalkthroughProhibitedPackages(repoRoot, scrubbed, touchedFiles); err != nil {
		return "", err
	}
	return scrubbed, nil
}

// NormalizeAndScrubWalkthrough normalizes workspace paths and scrubs host user directories and secrets.
func NormalizeAndScrubWalkthrough(repoRoot, wtPath, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return SanitizeDiagnosticText(normalizeReleaseProseDashes(normalizeWorkspacePathsInWalkthrough(raw, repoRoot, wtPath)), repoRoot)
}

func normalizeReleaseProseDashes(text string) string {
	r := strings.NewReplacer(" \u2014 ", " - ", "\u2014", " - ", " \u2013 ", " - ", "\u2013", "-")
	return r.Replace(text)
}

func detectRawCredentialInWalkthrough(raw string) string {
	if pemBlockRe.MatchString(raw) || pemHeaderRe.MatchString(raw) {
		return "PEM private key"
	}
	if ghTokenRe.MatchString(raw) {
		return "GitHub personal access token"
	}
	if cloudKeyRe.MatchString(raw) {
		return "cloud API key"
	}
	return ""
}

func normalizeWorkspacePathsInWalkthrough(text, repoRoot, wtPath string) string {
	out := text
	out = stripWorkspacePrefixVariants(out, wtPath, "")
	out = stripWorkspacePrefixVariants(out, repoRoot, "")
	return out
}

func stripWorkspacePrefixVariants(text, rootPath, replacement string) string {
	clean := strings.TrimSpace(rootPath)
	if clean == "" || clean == "." || clean == "/" {
		return text
	}
	abs := clean
	if resolved, err := filepath.Abs(clean); err == nil && resolved != "" {
		abs = resolved
	}
	slash := strings.TrimRight(filepath.ToSlash(abs), "/")
	if slash == "" {
		return text
	}
	out := strings.ReplaceAll(text, "file://"+slash+"/", replacement)
	out = strings.ReplaceAll(out, "file://"+slash, strings.TrimRight(replacement, "/"))
	out = strings.ReplaceAll(out, slash+"/", replacement)
	if replacement == "" {
		out = strings.ReplaceAll(out, slash, ".")
	} else {
		out = strings.ReplaceAll(out, slash, strings.TrimRight(replacement, "/"))
	}
	return out
}

func checkWalkthroughProhibitedPackages(repoRoot, scrubbed string, touchedFiles []string) error {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	cfg, err := config.LoadConfig(repoRoot)
	if err != nil || cfg == nil || !cfg.Governance.Boundary.Enforce {
		return nil
	}
	for _, pkg := range cfg.Governance.Boundary.ProhibitedPackages {
		trimmed := strings.TrimSpace(pkg)
		if trimmed == "" || !strings.Contains(scrubbed, trimmed) {
			continue
		}
		if touchedFilesJustifyToken(repoRoot, touchedFiles, trimmed) {
			continue
		}
		return types.NewNomosError(types.ErrCodeBoundary,
			"walkthrough references prohibited boundary package %q (Law 7)", trimmed).
			WithRemedy("remove prohibited internal package references from the walkthrough summary before running 'nomos task sync'")
	}
	return nil
}

func touchedFilesJustifyToken(repoRoot string, touchedFiles []string, token string) bool {
	for _, rel := range touchedFiles {
		cleanRel := filepath.Clean(rel)
		if strings.HasPrefix(cleanRel, "..") {
			continue
		}
		if strings.Contains(filepath.ToSlash(cleanRel), token) {
			return true
		}
	}
	return false
}
