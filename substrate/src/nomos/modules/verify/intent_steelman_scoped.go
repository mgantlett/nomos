// Package verify implements scoped spec_roots Steel-Man validation (Law 4 & Law 6).
// @intent nomos_intent_traceability_spec.md #ContractsConfig
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

// evaluateConfiguredIntentSteelman dispatches either scoped spec_roots validation or full intent/ validation.
func evaluateConfiguredIntentSteelman(root string, cfg *config.EcosystemConfig) (int, []string, bool) {
	contracts := config.ContractsConfig{}
	strictMode := false
	if cfg != nil {
		contracts = cfg.Governance.QualityGates.GetContractsBoundary().Contracts
		strictMode = cfg.Governance.QualityGates.StrictMode
	}
	if contracts.Enabled && isScopedSpecRoots(contracts.SpecRoots) {
		docCount, issues := validateScopedSpecRootDocuments(root, contracts.SpecRoots, intentSteelmanOptions{
			RequireFrontmatter: true,
			EnforceNCTE:        strictMode,
		})
		return docCount, issues, false
	}
	intentDir := filepath.Join(root, "intent")
	if _, err := os.Stat(intentDir); os.IsNotExist(err) {
		return 0, nil, true
	}
	docCount, issues := validateIntentDocumentsWithOptions(intentDir, intentSteelmanOptions{
		RequireFrontmatter: contracts.Enabled,
		StrictStandard:     contracts.Enabled && strictMode,
	})
	return docCount, issues, false
}

// isScopedSpecRoots reports whether specRoots targets explicit files or nested subpaths rather than full intent/ specs.
func isScopedSpecRoots(specRoots []string) bool {
	if len(specRoots) == 0 {
		return false
	}
	for _, raw := range specRoots {
		norm := strings.Trim(filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw))), "/")
		switch norm {
		case "", ".", "intent", "intent/architecture", "intent/architecture/specs":
			return false
		}
	}
	return true
}

// validateScopedSpecRootDocuments audits only Markdown documents declared under scoped specRoots.
func validateScopedSpecRootDocuments(root string, specRoots []string, opts intentSteelmanOptions) (int, []string) {
	var docCount int
	var issues []string
	seen := make(map[string]bool)
	for _, rawRoot := range specRoots {
		cleanRel := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rawRoot)))
		target := filepath.Join(root, filepath.FromSlash(cleanRel))
		info, err := os.Stat(target)
		if err != nil {
			issues = append(issues, fmt.Sprintf("%s: configured spec_roots path not found", cleanRel))
			continue
		}
		if !info.IsDir() {
			auditScopedMarkdownCandidate(root, target, opts, seen, &docCount, &issues)
			continue
		}
		if walkErr := filepath.Walk(target, func(p string, fi os.FileInfo, wErr error) error {
			if wErr == nil && fi != nil && !fi.IsDir() {
				auditScopedMarkdownCandidate(root, p, opts, seen, &docCount, &issues)
			}
			return nil
		}); walkErr != nil {
			issues = append(issues, fmt.Sprintf("%s: %v", cleanRel, walkErr))
		}
	}
	return docCount, issues
}

func auditScopedMarkdownCandidate(root, path string, opts intentSteelmanOptions, seen map[string]bool, docCount *int, issues *[]string) {
	if !strings.HasSuffix(path, ".md") || seen[path] {
		return
	}
	seen[path] = true
	*docCount++
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	rel = filepath.ToSlash(rel)
	if docErr := auditSingleIntentMarkdownWithOptions(rel, path, opts); docErr != "" {
		*issues = append(*issues, fmt.Sprintf("%s: %s", rel, docErr))
	}
}
